// Package wheeled implements some bases, like a wheeled base.
package wheeled

import (
	"context"
	"fmt"
	"math"
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
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

var modelname = resource.NewDefaultModel("wheeled")

// Config is how you configure a wheeled base.
type Config struct {
	WidthMM              int      `json:"width_mm"`
	WheelCircumferenceMM int      `json:"wheel_circumference_mm"`
	SpinSlipFactor       float64  `json:"spin_slip_factor,omitempty"`
	Left                 []string `json:"left"`
	Right                []string `json:"right"`
	MovementSensor       []string `json:"movement_sensor"`
}

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) ([]string, error) {
	var deps []string

	if config.WidthMM == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "width_mm")
	}

	if config.WheelCircumferenceMM == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "wheel_circumference_mm")
	}

	if len(config.Left) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "left")
	}
	if len(config.Right) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "right")
	}

	if len(config.Left) != len(config.Right) {
		return nil, utils.NewConfigValidationError(path,
			fmt.Errorf("left and right need to have the same number of motors, not %d vs %d",
				len(config.Left), len(config.Right)))
	}

	deps = append(deps, config.Left...)
	deps = append(deps, config.Right...)

	if len(config.MovementSensor) != 0 {
		deps = append(deps, config.MovementSensor...)
	}

	return deps, nil
}

func init() {
	wheeledBaseComp := registry.Component{
		Constructor: func(
			ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger,
		) (interface{}, error) {
			return createWheeledBase(ctx, deps, config.ConvertedAttributes.(*Config), config.Name, logger)
		},
	}

	registry.RegisterComponent(base.Subtype, modelname, wheeledBaseComp)
	config.RegisterComponentAttributeMapConverter(
		base.Subtype,
		modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&Config{})
}

type wheeledBase struct {
	generic.Unimplemented
	widthMm              int
	wheelCircumferenceMm int
	spinSlipFactor       float64

	left                []motor.Motor
	right               []motor.Motor
	allMotors           []motor.Motor
	movementSensor      []movementsensor.MovementSensor
	orienationSupported bool
	orientationSensor   movementsensor.MovementSensor

	opMgr  operation.SingleOperationManager
	logger golog.Logger
	model  referenceframe.Model
}

// createWheeledBase returns a new wheeled base defined by the given config.
func createWheeledBase(
	ctx context.Context,
	deps registry.Dependencies,
	cfg *Config,
	baseName string,
	logger golog.Logger,
) (base.LocalBase, error) {
	base := &wheeledBase{
		widthMm:              cfg.WidthMM,
		wheelCircumferenceMm: cfg.WheelCircumferenceMM,
		spinSlipFactor:       cfg.SpinSlipFactor,
		logger:               logger,
	}

	if base.spinSlipFactor == 0 {
		base.spinSlipFactor = 1
	}

	for _, name := range cfg.Left {
		m, err := motor.FromDependencies(deps, name)
		if err != nil {
			return nil, errors.Wrapf(err, "no left motor named (%s)", name)
		}
		positionReporting, err := m.Properties(ctx, nil)
		if positionReporting["PositonReporting"] && err == nil {
			base.logger.Debugf("have encoders on motor %#v", m)
		}
		base.logger.Debug("no encoders found on left motors")
		base.left = append(base.left, m)
	}

	for _, name := range cfg.Right {
		m, err := motor.FromDependencies(deps, name)
		if err != nil {
			return nil, errors.Wrapf(err, "no right motor named (%s)", name)
		}
		positionReporting, err := m.Properties(ctx, nil)
		if positionReporting["PositonReporting"] && err == nil {
			base.logger.Debugf("have encoders on motor %#v", m)
		}
		base.logger.Debug("no encoders found on right motors")
		base.right = append(base.right, m)
	}

	for _, msName := range cfg.MovementSensor {
		ms, err := movementsensor.FromDependencies(deps, msName)
		if err != nil {
			return nil, errors.Wrapf(err, "no movement_sensor namesd (%s)", msName)
		}
		base.movementSensor = append(base.movementSensor, ms)
		props, err := ms.Properties(ctx, nil)
		if props.OrientationSupported && err == nil {
			base.orienationSupported = props.OrientationSupported
			base.orientationSensor = ms
		}
	}

	base.allMotors = append(base.allMotors, base.left...)
	base.allMotors = append(base.allMotors, base.right...)

	var err error
	base.model, err = base.createModelFrame(baseName, cfg.WidthMM)
	if err != nil {
		logger.Error("error creating base model, check that a name for the base and width are provided in the configuration")
	}

	return base, nil
}

func (base *wheeledBase) createModelFrame(baseName string, widthMM int) (referenceframe.Model, error) {
	basePose := spatialmath.NewPose(
		r3.Vector{X: 0, Y: 0, Z: 0},
		&spatialmath.OrientationVectorDegrees{Theta: 0, OX: 0, OY: 1, OZ: -90})
	geometry, err := spatialmath.NewBox(
		basePose,
		r3.Vector{X: float64(widthMM), Y: float64(widthMM), Z: float64(widthMM)},
		"base_dimensions")
	if err != nil {
		return nil, err
	}
	model := referenceframe.NewSimpleModel(baseName)
	f, err := referenceframe.NewStaticFrameWithGeometry(baseName, basePose, geometry)
	if err != nil {
		return nil, err
	}
	model.OrdTransforms = append(model.OrdTransforms, f)
	return model, nil
}

func (base *wheeledBase) Spin(
	ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{},
) error {
	ctx, done := base.opMgr.New(ctx)
	defer done()

	switch {
	case base.orientationSensor != nil && base.orienationSupported:
		return base.spinWithMovementSensor(ctx, angleDeg, degsPerSec, extra)

	default:
		return base.spinWithoutMovementSensor(ctx, angleDeg, degsPerSec)
	}
}

func getCurrentYaw(ctx context.Context, ms movementsensor.MovementSensor, extra map[string]interface{},
) (float64, error) {
	orientation, err := ms.Orientation(ctx, extra)
	if err != nil {
		return 0, err
	}
	return rdkutils.RadToDeg(orientation.EulerAngles().Yaw), nil
}

func (base *wheeledBase) spinWithMovementSensor(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	wheelrpm, revs := base.spinMath(angleDeg, degsPerSec)

	targetYaw := angleDeg
	for {
		base.logger.Info(ctx.Err())
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		currYaw, err := getCurrentYaw(ctx, base.orientationSensor, extra)
		if err != nil {
			base.logger.Errorf("error: %#v", err)
			continue
		}

		errAngle := targetYaw - currYaw
		// runAll calls GoFor, which has a necessary terminating condition of rotations reached
		// poll the sensor for the current error in angle
		if math.Abs(errAngle) < 5 {
			if err := base.Stop(ctx, nil); err != nil {
				return err
			}
			break
		}
		if err := base.runAll(ctx, -wheelrpm, revs, wheelrpm, revs); err != nil {
			return err
		}
	}
	return nil
}

func (base *wheeledBase) spinWithoutMovementSensor(ctx context.Context, angleDeg, degsPerSec float64) error {
	// Stop the motors if the speed is 0
	if math.Abs(degsPerSec) < 0.0001 {
		err := base.Stop(ctx, nil)
		if err != nil {
			return errors.Errorf("error when trying to spin at a speed of 0: %v", err)
		}
		return err
	}
	// Spin math
	rpm, revolutions := base.spinMath(angleDeg, degsPerSec)

	return base.runAll(ctx, -rpm, revolutions, rpm, revolutions)
}

// TODO: should be used as part of RSDK-1698, to deal with imus that
// return values between -180 to 180 and 0-360 (probably using our filters).
//
//nolint:unused
func calculatedDomainLimitedAngleError(heading, bearing float64) float64 {
	calucaltedAngle := math.Mod((heading-bearing+540), 360) - 180
	return calucaltedAngle
}

// returns rpm, revolutions for a spin motion.
func (base *wheeledBase) spinMath(angleDeg, degsPerSec float64) (float64, float64) {
	wheelTravel := base.spinSlipFactor * float64(base.widthMm) * math.Pi * angleDeg / 360.0
	revolutions := wheelTravel / float64(base.wheelCircumferenceMm)

	// RPM = revolutions (rotation) * deg/sec * (1 rotation / 2pi deg) * (60 sec / 1 min) = rotation/min
	rpm := revolutions * degsPerSec * 30 / math.Pi
	revolutions = math.Abs(revolutions)

	return rpm, revolutions
}

func (base *wheeledBase) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	ctx, done := base.opMgr.New(ctx)
	defer done()

	// Stop the motors if the speed or distance are 0
	if math.Abs(mmPerSec) < 0.0001 || distanceMm == 0 {
		err := base.Stop(ctx, nil)
		if err != nil {
			return errors.Errorf(
				"error when trying to move straight at a speed and/or distance of 0: %v", err,
			)
		}
		return err
	}

	// Straight math
	rpm, rotations := base.straightDistanceToMotorInfo(distanceMm, mmPerSec)

	return base.runAll(ctx, -rpm, rotations, rpm, rotations)
}

// nolint: unused
func (base *wheeledBase) setPowerAll(ctx context.Context, leftPower, rightPower float64) error {
	fs := []rdkutils.SimpleFunc{}

	for _, m := range base.left {
		fs = append(fs, func(ctx context.Context) error { return m.SetPower(ctx, leftPower, nil) })
	}

	for _, m := range base.right {
		fs = append(fs, func(ctx context.Context) error { return m.SetPower(ctx, rightPower, nil) })
	}

	if _, err := rdkutils.RunInParallel(ctx, fs); err != nil {
		err = errors.Wrap(err, "error in runInParallel in setPowerAll")
		return multierr.Combine(err, base.Stop(ctx, nil))
	}
	return nil
}

func (base *wheeledBase) runAll(ctx context.Context, leftRPM, leftRotations, rightRPM, rightRotations float64) error {
	fs := []rdkutils.SimpleFunc{}

	// a right handed frame system is maintained if positive angle yields positive rightRPM and negative leftPRM
	// in this function, assuming the motors are configured to have positive power drive the base forwards
	for _, m := range base.left {
		fs = append(fs, func(ctx context.Context) error { return m.GoFor(ctx, leftRPM, leftRotations, nil) })
	}

	for _, m := range base.right {
		fs = append(fs, func(ctx context.Context) error { return m.GoFor(ctx, rightRPM, rightRotations, nil) })
	}

	if _, err := rdkutils.RunInParallel(ctx, fs); err != nil {
		err = errors.Wrap(err, "error in runInParallel in runAll")
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

func (base *wheeledBase) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	base.opMgr.CancelRunning(ctx)
	l, r := base.velocityMath(linear.Y, angular.Z)
	return base.runAll(ctx, l, 0, r, 0)
}

func (base *wheeledBase) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	base.opMgr.CancelRunning(ctx)
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

// return rpms left, right.
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

func (base *wheeledBase) straightDistanceToMotorInfo(distanceMm int, mmPerSec float64) (float64, float64) {
	rotations := float64(distanceMm) / float64(base.wheelCircumferenceMm)

	rotationsPerSec := mmPerSec / float64(base.wheelCircumferenceMm)
	rpm := 60 * rotationsPerSec

	return rpm, rotations
}

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

func (base *wheeledBase) Stop(ctx context.Context, extra map[string]interface{}) error {
	base.opMgr.CancelRunning(ctx)
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

func (base *wheeledBase) Close(ctx context.Context) error {
	return base.Stop(ctx, nil)
}

func (base *wheeledBase) Width(ctx context.Context) (int, error) {
	return base.widthMm, nil
}
