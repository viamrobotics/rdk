// Package wheeled implements some bases, like a wheeled base.
package wheeled

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	yawPollTime = 50 * time.Millisecond
	errTarget   = 5
)

var modelname = resource.NewDefaultModel("wheeled")

// AttrConfig is how you configure a wheeled base.
type AttrConfig struct {
	WidthMM              int      `json:"width_mm"`
	WheelCircumferenceMM int      `json:"wheel_circumference_mm"`
	SpinSlipFactor       float64  `json:"spin_slip_factor,omitempty"`
	Left                 []string `json:"left"`
	Right                []string `json:"right"`
	MovementSensor       []string `json:"movement_sensor"`
}

// Validate ensures all parts of the config are valid.
func (cfg *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string

	if cfg.WidthMM == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "width_mm")
	}

	if cfg.WheelCircumferenceMM == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "wheel_circumference_mm")
	}

	if len(cfg.Left) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "left")
	}
	if len(cfg.Right) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "right")
	}

	if len(cfg.Left) != len(cfg.Right) {
		return nil, utils.NewConfigValidationError(path,
			fmt.Errorf("left and right need to have the same number of motors, not %d vs %d",
				len(cfg.Left), len(cfg.Right)))
	}

	deps = append(deps, cfg.Left...)
	deps = append(deps, cfg.Right...)
	deps = append(deps, cfg.MovementSensor...)

	return deps, nil
}

func init() {
	wheeledBaseComp := registry.Component{
		Constructor: func(
			ctx context.Context, deps registry.Dependencies, cfg config.Component, logger golog.Logger,
		) (interface{}, error) {
			return createWheeledBase(ctx, deps, cfg, logger)
		},
	}

	registry.RegisterComponent(base.Subtype, modelname, wheeledBaseComp)
	config.RegisterComponentAttributeMapConverter(
		base.Subtype,
		modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{})
}

type wheeledBase struct {
	generic.Unimplemented
	widthMm              int
	wheelCircumferenceMm int
	spinSlipFactor       float64

	left      []motor.Motor
	right     []motor.Motor
	allMotors []motor.Motor

	movementSensors   []movementsensor.MovementSensor
	orientationSensor movementsensor.MovementSensor

	opMgr                   operation.SingleOperationManager
	activeBackgroundWorkers *sync.WaitGroup
	logger                  golog.Logger
	cancelCtx               context.Context
	cancelFunc              func()

	name              string
	collisionGeometry spatialmath.Geometry
}

// Spin commands a base to turn about its center at a angular speed and for a specific angle.
// TODO RSDK-2356 (rh) changing the angle here also changed the speed of the base
// TODO RSDK-2362 check choppiness of movement when run as a remote.
func (base *wheeledBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	ctx, done := base.opMgr.New(ctx)
	defer done()
	base.logger.Debugf("received a Spin with angleDeg:%.2f, degsPerSec:%.2f", angleDeg, degsPerSec)

	if base.orientationSensor != nil {
		return base.spinWithMovementSensor(angleDeg, degsPerSec, extra)
	}
	return base.spin(ctx, angleDeg, degsPerSec)
}

func (base *wheeledBase) spin(ctx context.Context, angleDeg, degsPerSec float64) error {
	// Stop the motors if the speed is 0
	if math.Abs(degsPerSec) < 0.0001 {
		if err := base.Stop(ctx, nil); err != nil {
			return errors.Errorf("error when trying to spin at a speed of 0: %v", err)
		}
	}

	// Spin math
	rpm, revolutions := base.spinMath(angleDeg, degsPerSec)

	return base.runAll(ctx, -rpm, revolutions, rpm, revolutions)
}

func (base *wheeledBase) spinWithMovementSensor(angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	// if rdkutils.Float64AlmostEqual(angleDeg, 360, 2) {
	// 	// if we're exactly 360 degrees away from target
	// 	// we are at the target and the base won't move
	// 	angleDeg -= 3
	// 	base.logger.Warn(
	// 		"changing angle to 357, base is already at target by adding 360 degrees",
	// 	)
	// }

	wheelrpm, _ := base.spinMath(angleDeg, degsPerSec)

	base.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		// runAll calls GoFor, which blocks until the timed operation is done, or returns nil if a zero is passed in
		// the code inside this goroutine would block the sensor for loop if taken out
		if err := base.runAll(base.cancelCtx, -wheelrpm, 0, wheelrpm, 0); err != nil {
			base.logger.Error(err)
		}
	}, base.activeBackgroundWorkers.Done)

	startYaw, err := getCurrentYaw(base.cancelCtx, base.orientationSensor, extra)
	if err != nil {
		return err
	} // from 0 -> 360

	errCounter := 0
	numTurns := int(angleDeg) / int(360) // rounds down to full number of turns
	loopIteration := 0

	targetYaw, dir := findSpinParams(angleDeg, degsPerSec, startYaw)
	base.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		ticker := time.NewTicker(yawPollTime)
		defer ticker.Stop()
		for {
			// RSDK-2384 - use a new cancel ctx for sensor loops
			// to have them stop when cotnext is done
			select {
			case <-base.cancelCtx.Done():
				ticker.Stop()
				return
			default:
			}
			select {
			case <-base.cancelCtx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
			}

			// possible fix for 360 degrees
			// just skip the loop
			if loopIteration == 0 {
				base.logger.Debugf("loop iteration is %v", loopIteration)
				loopIteration = 1
				continue
			}

			// imu readings are limited from 0 -> 360
			currYaw, err := getCurrentYaw(base.cancelCtx, base.orientationSensor, extra)
			if err != nil {
				errCounter++
				if errCounter > 100 {
					base.logger.Error(errors.Wrap(
						err, "imu sensor unreachable, 100 error counts when trying to read yaw angle"))
					return
				}
			}
			errCounter = 0 // reset reading error count to zero if we are successfully reading again

			if numTurns != 0 {
				base.logger.Debug("we need to turn more")
				if rdkutils.Float64AlmostEqual(startYaw, currYaw, 1) {
					numTurns -= 1
					base.logger.Debugf("numturns now %v", numTurns)
				}
				continue
			}

			errAngle := targetYaw - currYaw
			overshot := hasOverShot(currYaw, startYaw, targetYaw, dir)

			// poll the sensor for the current error in angle
			// also check if we've overshot our target by fifteen degrees
			base.logger.Debugf(
				"startYaw: %.2f, currentYaw: %.2f, targetYaw:%.2f, overshot:%t",
				startYaw, currYaw, targetYaw, overshot)
			if math.Abs(errAngle) < errTarget || overshot {
				if err := base.Stop(base.cancelCtx, nil); err != nil {
					return
				}

				base.logger.Debugf(
					"stopping base with errAngle:%.2f, overshot? %t",
					math.Abs(errAngle), overshot)
				return
			}
		}

	}, base.activeBackgroundWorkers.Done)

	return nil
}

func getCurrentYaw(ctx context.Context, ms movementsensor.MovementSensor, extra map[string]interface{},
) (float64, error) {
	orientation, err := ms.Orientation(ctx, extra)
	if err != nil {
		return 0, err
	}
	// Add Pi  to make the computation for overshoot simpler
	// turns imus from -180 -> 180 to a 0 -> 360 range
	return addAnglesInDomain(rdkutils.RadToDeg(orientation.EulerAngles().Yaw), 0), nil
}

func addAnglesInDomain(target, current float64) float64 {
	angle := target + current
	// reduce the angle
	angle = math.Mod(angle, 360)

	// force it to be the positive remainder, so that 0 <= angle < 360
	angle = math.Mod(angle+360, 360)
	return angle
}

func findSpinParams(angleDeg, degsPerSec, currYaw float64) (float64, float64) {
	targetYaw := addAnglesInDomain(angleDeg, currYaw)
	dir := 1.0
	if math.Signbit(degsPerSec) != math.Signbit(angleDeg) {
		// both positive or both negative -> counterclockwise spin call
		// counterclockwise spin calls add allowable angles
		// the signs being different --> clockwise spin call
		// cloxkwise spin calls subtract allowable angles
		dir = -1
	}
	return targetYaw, dir
}

// this function does not wrap around 360 degrees currently.
func angleBetween(current, bound1, bound2 float64) bool {
	if bound2 > bound1 {
		inBewtween := current >= bound1 && current < bound2
		return inBewtween
	}
	inBewteen := current > bound2 && current <= bound1
	return inBewteen
}

func hasOverShot(angle, start, target, dir float64) bool {
	switch {
	case dir == -1 && start > target:
		// for cases with a quadrant switch from 1 -> 4
		// check if the current angle is in the regions before the
		// target and after the start
		over := angleBetween(angle, target, 0) || angleBetween(angle, 360, start)
		return over
	case dir == -1 && target > start:
		// the overshoot range is the inside range between the start and target
		return angleBetween(angle, target, start)
	case dir == 1 && start > target:
		// for cases with a quadrant switch from 1 -> 4
		// check if the current angle is not in the regions after the
		// target and before the start
		over := !angleBetween(angle, 0, target) && !angleBetween(angle, start, 360)
		return over
	default:
		// the overshoot range is the outside range between the start and target
		return !angleBetween(angle, start, target)
	}
}

// MoveStraight commands a base to drive forward or backwards  at a linear speed and for a specific distance.
// TODO RSDK-2362 check choppiness of movement when run as a remote.
func (base *wheeledBase) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	ctx, done := base.opMgr.New(ctx)
	defer done()
	base.logger.Debugf("received a MoveStraight with distanceMM:%d, mmPerSec:%.2f", distanceMm, mmPerSec)

	// Stop the motors if the speed or distance are 0
	if math.Abs(mmPerSec) < 0.0001 || distanceMm == 0 {
		err := base.Stop(ctx, nil)
		if err != nil {
			return errors.Errorf("error when trying to move straight at a speed and/or distance of 0: %v", err)
		}
		return err
	}

	// Straight math
	rpm, rotations := base.straightDistanceToMotorInputs(distanceMm, mmPerSec)

	return base.runAll(ctx, rpm, rotations, rpm, rotations)
}

// runsAll the base motors in parallel with the required speeds and rotations.
func (base *wheeledBase) runAll(ctx context.Context, leftRPM, leftRotations, rightRPM, rightRotations float64) error {
	fs := []rdkutils.SimpleFunc{}

	for _, m := range base.left {
		fs = append(fs, func(ctx context.Context) error { return m.GoFor(ctx, leftRPM, leftRotations, nil) })
	}

	for _, m := range base.right {
		fs = append(fs, func(ctx context.Context) error { return m.GoFor(ctx, rightRPM, rightRotations, nil) })
	}

	if _, err := rdkutils.RunInParallel(ctx, fs); err != nil {
		return multierr.Combine(err, base.Stop(ctx, nil))
	}
	return nil
}

// differentialDrive takes forward and left direction inputs from a first person
// perspective on a 2D plane and converts them to left and right motor powers. negative
// forward means backward and negative left means right.
func (base *wheeledBase) differentialDrive(forward, left float64) (float64, float64) {
	if forward < 0 {
		// Mirror the forward turning arc if we go in reverse
		leftMotor, rightMotor := base.differentialDrive(-forward, left)
		return -leftMotor, -rightMotor
	}

	// convert to polar coordinates
	r := math.Hypot(forward, left)
	t := math.Atan2(left, forward)

	// rotate by 45 degrees
	t += math.Pi / 4
	if t == 0 {
		// HACK: Fixes a weird ATAN2 corner case. Ensures that when motor that is on the
		// same side as the turn has the same power when going left and right. Without
		// this, the right motor has ZERO power when going forward/backward turning
		// right, when it should have at least some very small value.
		t += 1.224647e-16 / 2
	}

	// convert to cartesian
	leftMotor := r * math.Cos(t)
	rightMotor := r * math.Sin(t)

	// rescale the new coords
	leftMotor *= math.Sqrt(2)
	rightMotor *= math.Sqrt(2)

	// clamp to -1/+1
	leftMotor = math.Max(-1, math.Min(leftMotor, 1))
	rightMotor = math.Max(-1, math.Min(rightMotor, 1))

	return leftMotor, rightMotor
}

// SetVelocity commands the base to move at the input linear and angular velocities.
// TODO RSDK-2362 check choppiness of movement when run as a remote.
func (base *wheeledBase) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	base.opMgr.CancelRunning(ctx)

	base.logger.Debugf(
		"received a SetVelocity with linear.X: %.2f, linear.Y: %.2f linear.Z: %.2f (mmPerSec), angular.X: %.2f, angular.Y: %.2f, angular.Z: %.2f",
		linear.X, linear.Y, linear.Z, angular.X, angular.Y, angular.Z)

	l, r := base.velocityMath(linear.Y, angular.Z)
	return base.runAll(ctx, l, 0, r, 0)
}

// SetPower commands the base motors to run at powers correspoinding to input linear and angular powers.
// TODO RSDK-2362 check choppiness of movement when run as a remote.
func (base *wheeledBase) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	base.opMgr.CancelRunning(ctx)

	base.logger.Debugf(
		"received a SetPower with linear.X: %.2f, linear.Y: %.2f linear.Z: %.2f, angular.X: %.2f, angular.Y: %.2f, angular.Z: %.2f",
		linear.X, linear.Y, linear.Z, angular.X, angular.Y, angular.Z)

	lPower, rPower := base.differentialDrive(linear.Y, angular.Z)

	// Send motor commands
	var err error
	for _, m := range base.left {
		err = multierr.Combine(err, m.SetPower(ctx, lPower, extra))
	}

	for _, m := range base.right {
		err = multierr.Combine(err, m.SetPower(ctx, rPower, extra))
	}

	if err != nil {
		return multierr.Combine(err, base.Stop(ctx, nil))
	}

	return nil
}

// returns rpm, revolutions for a spin motion.
// TODO - RSDK-2356 check math for output speeds.
func (base *wheeledBase) spinMath(angleDeg, degsPerSec float64) (float64, float64) {
	wheelTravel := base.spinSlipFactor * float64(base.widthMm) * math.Pi * angleDeg / 360.0
	revolutions := wheelTravel / float64(base.wheelCircumferenceMm)

	// RPM = revolutions (unit) * deg/sec * (1 rot / 2pi deg) * (60 sec / 1 min) = rot/min
	rpm := revolutions * degsPerSec * 30 / math.Pi
	revolutions = math.Abs(revolutions)

	return rpm, revolutions
}

// calcualtes wheel rpms from overall base linear and angular movement velocity inputs.
func (base *wheeledBase) velocityMath(mmPerSec, degsPerSec float64) (float64, float64) {
	// Base calculations
	v := mmPerSec
	r := float64(base.wheelCircumferenceMm) / (2.0 * math.Pi)
	l := float64(base.widthMm)

	w0 := degsPerSec / 180 * math.Pi
	wL := (v / r) - (l * w0 / (2 * r))
	wR := (v / r) + (l * w0 / (2 * r))

	// RPM = revolutions (unit) * deg/sec * (1 rot / 2pi deg) * (60 sec / 1 min) = rot/min
	rpmL := (wL / (2 * math.Pi)) * 60
	rpmR := (wR / (2 * math.Pi)) * 60

	return rpmL, rpmR
}

// calculates the motor revolutions and speeds that correspond to the reuired distance and linear speeds.
func (base *wheeledBase) straightDistanceToMotorInputs(distanceMm int, mmPerSec float64) (float64, float64) {
	// takes in base speed and distance to calculate motor rpm and total rotations
	rotations := float64(distanceMm) / float64(base.wheelCircumferenceMm)

	rotationsPerSec := mmPerSec / float64(base.wheelCircumferenceMm)
	rpm := 60 * rotationsPerSec

	return rpm, rotations
}

// WaitForMotorsToStop is unused except for tests, polls all motors to see if they're on
// TODO: Audit in  RSDK-1880.
func (base *wheeledBase) WaitForMotorsToStop(ctx context.Context) error {
	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		}

		anyOn := false
		anyOff := false

		for _, m := range base.allMotors {
			isOn, _, err := m.IsPowered(ctx, nil)
			if err != nil {
				return err
			}
			if isOn {
				anyOn = true
			} else {
				anyOff = true
			}
		}

		if !anyOn {
			return nil
		}

		if anyOff {
			// once one motor turns off, we turn them all off
			return base.Stop(ctx, nil)
		}
	}
}

// Stop commands the base to stop moving.
func (base *wheeledBase) Stop(ctx context.Context, extra map[string]interface{}) error {
	var err error
	for _, m := range base.allMotors {
		err = multierr.Combine(err, m.Stop(ctx, extra))
	}
	return err
}

func (base *wheeledBase) IsMoving(ctx context.Context) (bool, error) {
	for _, m := range base.allMotors {
		isMoving, _, err := m.IsPowered(ctx, nil)
		if err != nil {
			return false, err
		}
		if isMoving {
			return true, err
		}
	}
	return false, nil
}

// Close is called from the client to close the instance of the base.
func (base *wheeledBase) Close(ctx context.Context) error {
	return base.Stop(ctx, nil)
}

// Width returns the width of the base as configured by the user.
func (base *wheeledBase) Width(ctx context.Context) (int, error) {
	return base.widthMm, nil
}

// createWheeledBase returns a new wheeled base defined by the given config.
func createWheeledBase(
	ctx context.Context,
	deps registry.Dependencies,
	cfg config.Component,
	logger golog.Logger,
) (base.LocalBase, error) {
	attr, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attr, &AttrConfig{})
	}

	base := &wheeledBase{
		widthMm:                 attr.WidthMM,
		wheelCircumferenceMm:    attr.WheelCircumferenceMM,
		spinSlipFactor:          attr.SpinSlipFactor,
		logger:                  logger,
		activeBackgroundWorkers: &sync.WaitGroup{},
		name:                    cfg.Name,
	}

	base.cancelCtx, base.cancelFunc = context.WithCancel(ctx)
	if base.spinSlipFactor == 0 {
		base.spinSlipFactor = 1
	}

	for _, name := range attr.Left {
		m, err := motor.FromDependencies(deps, name)
		if err != nil {
			return nil, errors.Wrapf(err, "no left motor named (%s)", name)
		}
		props, err := m.Properties(ctx, nil)
		if props[motor.PositionReporting] && err != nil {
			base.logger.Debugf("motor %s can report its position for base", name)
		}
		base.left = append(base.left, m)
	}

	for _, name := range attr.Right {
		m, err := motor.FromDependencies(deps, name)
		if err != nil {
			return nil, errors.Wrapf(err, "no right motor named (%s)", name)
		}
		props, err := m.Properties(ctx, nil)
		if props[motor.PositionReporting] && err != nil {
			base.logger.Debugf("motor %s can report its position for base", name)
		}
		base.right = append(base.right, m)
	}

	for _, msName := range attr.MovementSensor {
		ms, err := movementsensor.FromDependencies(deps, msName)
		if err != nil {
			return nil, errors.Wrapf(err, "no movement_sensor namesd (%s)", msName)
		}
		base.movementSensors = append(base.movementSensors, ms)
		props, err := ms.Properties(ctx, nil)
		if props.OrientationSupported && err == nil {
			base.orientationSensor = ms
		}
	}
	base.allMotors = append(base.allMotors, base.left...)
	base.allMotors = append(base.allMotors, base.right...)

	collisionGeometry, err := collisionGeometry(cfg)
	if err != nil {
		logger.Warnf("could not build geometry for wheeledBase: %s", err.Error())
	}
	base.collisionGeometry = collisionGeometry

	return base, nil
}
