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
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

// ModelName is the name of the wheeled model of a base component.
var ModelName = resource.NewDefaultModel("wheeled")

// AttrConfig is how you configure a wheeled base.
type AttrConfig struct {
	WidthMM              int      `json:"width_mm"`
	WheelCircumferenceMM int      `json:"wheel_circumference_mm"`
	SpinSlipFactor       float64  `json:"spin_slip_factor,omitempty"`
	Left                 []string `json:"left"`
	Right                []string `json:"right"`
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

	return deps, nil
}

func init() {
	wheeledBaseComp := registry.Component{
		Constructor: func(
			ctx context.Context, deps registry.Dependencies, cfg config.Component, logger golog.Logger,
		) (interface{}, error) {
			return CreateWheeledBase(ctx, deps, cfg, logger)
		},
	}

	registry.RegisterComponent(base.Subtype, ModelName, wheeledBaseComp)
	config.RegisterComponentAttributeMapConverter(
		base.Subtype,
		ModelName,
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

	opMgr  operation.SingleOperationManager
	logger golog.Logger

	name  string
	frame *referenceframe.LinkConfig
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

// runsAll the base motors in parallel with the required speeds and rotations.
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
		"received a SetVelocity with linear.X: %.2f, linear.Y: %.2f linear.Z: %.2f (mmPerSec), angular.X: %.2f, angular.Y: %.2f, angular.Z: %.2f",
		linear.X, linear.Y, linear.Z, angular.X, angular.Y, angular.Z)

	l, r := wb.velocityMath(linear.Y, angular.Z)
	return wb.runAll(ctx, l, 0, r, 0)
}

// SetPower commands the base motors to run at powers correspoinding to input linear and angular powers.
func (wb *wheeledBase) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	wb.opMgr.CancelRunning(ctx)

	wb.logger.Debugf(
		"received a SetPower with linear.X: %.2f, linear.Y: %.2f linear.Z: %.2f, angular.X: %.2f, angular.Y: %.2f, angular.Z: %.2f",
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
	wheelTravel := wb.spinSlipFactor * float64(wb.widthMm) * math.Pi * angleDeg / 360.0
	revolutions := wheelTravel / float64(wb.wheelCircumferenceMm)

	// RPM = revolutions (unit) * deg/sec * (1 rot / 2pi deg) * (60 sec / 1 min) = rot/min
	rpm := revolutions * degsPerSec * 30 / math.Pi
	revolutions = math.Abs(revolutions)

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

// calculates the motor revolutions and speeds that correspond to the reuired distance and linear speeds.
func (wb *wheeledBase) straightDistanceToMotorInputs(distanceMm int, mmPerSec float64) (float64, float64) {
	// takes in base speed and distance to calculate motor rpm and total rotations
	rotations := float64(distanceMm) / float64(wb.wheelCircumferenceMm)

	rotationsPerSec := mmPerSec / float64(wb.wheelCircumferenceMm)
	rpm := 60 * rotationsPerSec

	return rpm, rotations
}

// WaitForMotorsToStop is unused except for tests, polls all motors to see if they're on
// TODO: Audit in  RSDK-1880.
func (wb *wheeledBase) WaitForMotorsToStop(ctx context.Context) error {
	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		}

		anyOn := false
		anyOff := false

		for _, m := range wb.allMotors {
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
			return wb.Stop(ctx, nil)
		}
	}
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

// CreateWheeledBase returns a new wheeled base defined by the given config.
func CreateWheeledBase(
	ctx context.Context,
	deps registry.Dependencies,
	cfg config.Component,
	logger golog.Logger,
) (base.LocalBase, error) {
	attr, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attr, &AttrConfig{})
	}

	wb := &wheeledBase{
		widthMm:              attr.WidthMM,
		wheelCircumferenceMm: attr.WheelCircumferenceMM,
		spinSlipFactor:       attr.SpinSlipFactor,
		logger:               logger,
		name:                 cfg.Name,
		frame:                cfg.Frame,
	}

	if wb.spinSlipFactor == 0 {
		wb.spinSlipFactor = 1
	}

	for _, name := range attr.Left {
		m, err := motor.FromDependencies(deps, name)
		if err != nil {
			return nil, errors.Wrapf(err, "no left motor named (%s)", name)
		}
		props, err := m.Properties(ctx, nil)
		if props[motor.PositionReporting] && err != nil {
			wb.logger.Debugf("motor %s can report its position for base", name)
		}
		wb.left = append(wb.left, m)
	}

	for _, name := range attr.Right {
		m, err := motor.FromDependencies(deps, name)
		if err != nil {
			return nil, errors.Wrapf(err, "no right motor named (%s)", name)
		}
		props, err := m.Properties(ctx, nil)
		if props[motor.PositionReporting] && err != nil {
			wb.logger.Debugf("motor %s can report its position for base", name)
		}
		wb.right = append(wb.right, m)
	}

	wb.allMotors = append(wb.allMotors, wb.left...)
	wb.allMotors = append(wb.allMotors, wb.right...)
	return wb, nil
}
