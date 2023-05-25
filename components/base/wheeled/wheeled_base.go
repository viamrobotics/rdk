// Package wheeled implements some bases, like a wheeled base.
package wheeled

/*
   The Viam wheeled package implements a wheeled robot base with differential drive control. The base must have an equal
   number of motors on its left and right sides. The base's width and wheel circumference dimensions are required to
   compute wheel speeds to move the base straight distances or spin to headings at the desired input speeds. A spin slip
   factor acts as a multiplier to adjust power delivery to the wheels when each side of the base is undergoing unequal
   friction because of the surface it is moving on.
   Any motors can be used for the base motors (encoded, un-encoded, steppers, servos) as long as they update their position
   continuously (not limited to 0-360 or any other domain).

   Configuring a base with a frame will create a kinematic base that can be used by Viam's motion service to plan paths
   when a SLAM service is also present. This feature is experimental.
   Example Config:
   {
     "name": "myBase",
     "type": "base",
     "model": "wheeled",
     "attributes": {
       "right": ["right1", "right2"],
       "left": ["left1", "left2"],
       "spin_slip_factor": 1.76,
       "wheel_circumference_mm": 217,
       "width_mm": 260,
     },
     "depends_on": ["left1", "left2", "right1", "right2", "local"],
   },
*/

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

// Model is the name of the wheeled model of a base component.
var Model = resource.DefaultModelFamily.WithModel("wheeled")

// Config is how you configure a wheeled base.
type Config struct {
	WidthMM              int      `json:"width_mm"`
	WheelCircumferenceMM int      `json:"wheel_circumference_mm"`
	SpinSlipFactor       float64  `json:"spin_slip_factor,omitempty"`
	Left                 []string `json:"left"`
	Right                []string `json:"right"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
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

	return deps, nil
}

func init() {
	wheeledBaseComp := resource.Registration[base.Base, *Config]{
		Constructor: func(
			ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger,
		) (base.Base, error) {
			return createWheeledBase(ctx, deps, conf, logger)
		},
	}

	resource.RegisterComponent(base.API, Model, wheeledBaseComp)
}

type wheeledBase struct {
	resource.Named
	resource.AlwaysRebuild
	widthMm              int
	wheelCircumferenceMm int
	spinSlipFactor       float64

	left      []motor.Motor
	right     []motor.Motor
	allMotors []motor.Motor

	opMgr  operation.SingleOperationManager
	logger golog.Logger

	mu    sync.Mutex
	name  string
	frame *referenceframe.LinkConfig
}

// Reconfigure reconfigures the base atomically and in place.
func (wb *wheeledBase) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	if newConf.SpinSlipFactor == 0 {
		newConf.SpinSlipFactor = 1
	}

	wb.spinSlipFactor = newConf.SpinSlipFactor

	// Check if wb.left is different from newConf.Left before changing wb.left
	if len(wb.left) != len(newConf.Left) {
		// Resetting the left motor list
		wb.left = make([]motor.Motor, 0)

		for _, name := range newConf.Left {
			m, err := motor.FromDependencies(deps, name)
			if err != nil {
				return errors.Wrapf(err, "no left motor named (%s)", name)
			}
			wb.left = append(wb.left, m)
		}
	} else {
		// Compare each element of the slices
		for i := range wb.left {
			if wb.left[i].Name().String() != newConf.Left[i] {
				// Resetting the left motor list
				wb.left = make([]motor.Motor, 0)

				for _, name := range newConf.Left {
					m, err := motor.FromDependencies(deps, name)
					if err != nil {
						return errors.Wrapf(err, "no left motor named (%s)", name)
					}
					wb.left = append(wb.left, m)
				}
				break
			}
		}
	}

	if len(wb.right) != len(newConf.Right) {
		// Resetting the left motor list
		wb.right = make([]motor.Motor, 0)

		for _, name := range newConf.Right {
			m, err := motor.FromDependencies(deps, name)
			if err != nil {
				return errors.Wrapf(err, "no right motor named (%s)", name)
			}
			wb.right = append(wb.right, m)
		}
	} else {
		// Compare each element of the slices
		for i := range wb.right {
			if wb.right[i].Name().String() != newConf.Right[i] {
				wb.right = make([]motor.Motor, 0)

				for _, name := range newConf.Right {
					m, err := motor.FromDependencies(deps, name)
					if err != nil {
						return errors.Wrapf(err, "no right motor named (%s)", name)
					}
					wb.right = append(wb.right, m)
				}
				break
			}
		}
	}

	wb.allMotors = append(wb.allMotors, wb.left...)
	wb.allMotors = append(wb.allMotors, wb.right...)

	if wb.widthMm != newConf.WidthMM {
		wb.widthMm = newConf.WidthMM
	}

	if wb.wheelCircumferenceMm != newConf.WheelCircumferenceMM {
		wb.wheelCircumferenceMm = newConf.WheelCircumferenceMM
	}

	return nil
}

// createWheeledBase returns a new wheeled base defined by the given config.
func createWheeledBase(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger golog.Logger,
) (base.LocalBase, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	wb := wheeledBase{
		Named:                conf.ResourceName().AsNamed(),
		widthMm:              newConf.WidthMM,
		wheelCircumferenceMm: newConf.WheelCircumferenceMM,
		spinSlipFactor:       newConf.SpinSlipFactor,
		logger:               logger,
		name:                 conf.Name,
		frame:                conf.Frame,
	}

	if err := wb.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return &wb, nil
}

// Spin commands a base to turn about its center at a angular speed and for a specific angle.
func (wb *wheeledBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	ctx, done := wb.opMgr.New(ctx)
	defer done()
	wb.logger.Debugf("received a Spin with angleDeg:%.2f, degsPerSec:%.2f", angleDeg, degsPerSec)

	// Stop the motors if the speed is 0
	if math.Abs(degsPerSec) < 0.0001 {
		err := wb.Stop(ctx, nil)
		if err != nil {
			return errors.Errorf("error when trying to spin at a speed of 0: %v", err)
		}
		return err
	}

	// Spin math
	rpm, revolutions := wb.spinMath(angleDeg, degsPerSec)

	return wb.runAll(ctx, -rpm, revolutions, rpm, revolutions)
}

// MoveStraight commands a base to drive forward or backwards  at a linear speed and for a specific distance.
func (wb *wheeledBase) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	ctx, done := wb.opMgr.New(ctx)
	defer done()
	wb.logger.Debugf("received a MoveStraight with distanceMM:%d, mmPerSec:%.2f", distanceMm, mmPerSec)

	// Stop the motors if the speed or distance are 0
	if math.Abs(mmPerSec) < 0.0001 || distanceMm == 0 {
		err := wb.Stop(ctx, nil)
		if err != nil {
			return errors.Errorf("error when trying to move straight at a speed and/or distance of 0: %v", err)
		}
		return err
	}

	// Straight math
	rpm, rotations := wb.straightDistanceToMotorInputs(distanceMm, mmPerSec)

	return wb.runAll(ctx, rpm, rotations, rpm, rotations)
}

// runAll executes motor commands in parallel for left and right motors,
// with specified speeds and rotations and stops the base if an error occurs.
func (wb *wheeledBase) runAll(ctx context.Context, leftRPM, leftRotations, rightRPM, rightRotations float64) error {
	fs := []rdkutils.SimpleFunc{}

	for _, m := range wb.left {
		fs = append(fs, func(ctx context.Context) error { return m.GoFor(ctx, leftRPM, leftRotations, nil) })
	}

	for _, m := range wb.right {
		fs = append(fs, func(ctx context.Context) error { return m.GoFor(ctx, rightRPM, rightRotations, nil) })
	}

	if _, err := rdkutils.RunInParallel(ctx, fs); err != nil {
		return multierr.Combine(err, wb.Stop(ctx, nil))
	}
	return nil
}

// differentialDrive takes forward and left direction inputs from a first person
// perspective on a 2D plane and converts them to left and right motor powers. negative
// forward means backward and negative left means right.
func (wb *wheeledBase) differentialDrive(forward, left float64) (float64, float64) {
	if forward < 0 {
		// Mirror the forward turning arc if we go in reverse
		leftMotor, rightMotor := wb.differentialDrive(-forward, left)
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
func (wb *wheeledBase) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	wb.opMgr.CancelRunning(ctx)

	wb.logger.Debugf(
		"received a SetVelocity with linear.X: %.2f, linear.Y: %.2f linear.Z: %.2f(mmPerSec),"+
			" angular.X: %.2f, angular.Y: %.2f, angular.Z: %.2f",
		linear.X, linear.Y, linear.Z, angular.X, angular.Y, angular.Z)

	l, r := wb.velocityMath(linear.Y, angular.Z)

	return wb.runAll(ctx, l, 0, r, 0)
}

// SetPower commands the base motors to run at powers corresponding to input linear and angular powers.
func (wb *wheeledBase) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	wb.opMgr.CancelRunning(ctx)

	wb.logger.Debugf(
		"received a SetPower with linear.X: %.2f, linear.Y: %.2f linear.Z: %.2f,"+
			" angular.X: %.2f, angular.Y: %.2f, angular.Z: %.2f",
		linear.X, linear.Y, linear.Z, angular.X, angular.Y, angular.Z)

	lPower, rPower := wb.differentialDrive(linear.Y, angular.Z)

	// Send motor commands
	var err error
	for _, m := range wb.left {
		err = multierr.Combine(err, m.SetPower(ctx, lPower, extra))
	}

	for _, m := range wb.right {
		err = multierr.Combine(err, m.SetPower(ctx, rPower, extra))
	}

	if err != nil {
		return multierr.Combine(err, wb.Stop(ctx, nil))
	}

	return nil
}

// returns rpm, revolutions for a spin motion.
func (wb *wheeledBase) spinMath(angleDeg, degsPerSec float64) (float64, float64) {
	wheelTravel := wb.spinSlipFactor * float64(wb.widthMm) * math.Pi * (angleDeg / 360.0)
	revolutions := wheelTravel / float64(wb.wheelCircumferenceMm)
	revolutions = math.Abs(revolutions)

	// RPM = revolutions (unit) * deg/sec * (1 rot / 2pi deg) * (60 sec / 1 min) = rot/min
	// RPM = (revolutions (unit) / angleDeg) * degPerSec * 60
	rpm := (revolutions / angleDeg) * degsPerSec * 60

	return rpm, revolutions
}

// calcualtes wheel rpms from overall base linear and angular movement velocity inputs.
func (wb *wheeledBase) velocityMath(mmPerSec, degsPerSec float64) (float64, float64) {
	// Base calculations
	v := mmPerSec
	r := float64(wb.wheelCircumferenceMm) / (2.0 * math.Pi)
	l := float64(wb.widthMm)

	w0 := degsPerSec / 180 * math.Pi
	wL := (v / r) - (l * w0 / (2 * r))
	wR := (v / r) + (l * w0 / (2 * r))

	// RPM = revolutions (unit) * deg/sec * (1 rot / 2pi deg) * (60 sec / 1 min) = rot/min
	rpmL := (wL / (2 * math.Pi)) * 60
	rpmR := (wR / (2 * math.Pi)) * 60

	return rpmL, rpmR
}

// calculates the motor revolutions and speeds that correspond to the required distance and linear speeds.
func (wb *wheeledBase) straightDistanceToMotorInputs(distanceMm int, mmPerSec float64) (float64, float64) {
	// takes in base speed and distance to calculate motor rpm and total rotations
	rotations := float64(distanceMm) / float64(wb.wheelCircumferenceMm)

	rotationsPerSec := mmPerSec / float64(wb.wheelCircumferenceMm)
	rpm := 60 * rotationsPerSec

	return rpm, rotations
}

// Stop commands the base to stop moving.
func (wb *wheeledBase) Stop(ctx context.Context, extra map[string]interface{}) error {
	var err error
	for _, m := range wb.allMotors {
		err = multierr.Combine(err, m.Stop(ctx, extra))
	}
	return err
}

func (wb *wheeledBase) IsMoving(ctx context.Context) (bool, error) {
	for _, m := range wb.allMotors {
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

// Close is called from the client to close the instance of the wheeledBase.
func (wb *wheeledBase) Close(ctx context.Context) error {
	return wb.Stop(ctx, nil)
}

// Width returns the width of the base as configured by the user.
func (wb *wheeledBase) Width(ctx context.Context) (int, error) {
	return wb.widthMm, nil
}
