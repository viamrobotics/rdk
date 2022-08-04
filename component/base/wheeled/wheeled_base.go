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

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	rdkutils "go.viam.com/rdk/utils"
)

func init() {
	fourWheelComp := registry.Component{
		Constructor: func(
			ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger,
		) (interface{}, error) {
			return CreateFourWheelBase(ctx, deps, config.ConvertedAttributes.(*FourWheelConfig), logger)
		},
	}

	registry.RegisterComponent(base.Subtype, "four-wheel", fourWheelComp)
	config.RegisterComponentAttributeMapConverter(
		base.SubtypeName,
		"four-wheel",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf FourWheelConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&FourWheelConfig{})

	wheeledBaseComp := registry.Component{
		Constructor: func(
			ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger,
		) (interface{}, error) {
			return CreateWheeledBase(ctx, deps, config.ConvertedAttributes.(*Config), logger)
		},
	}

	registry.RegisterComponent(base.Subtype, "wheeled", wheeledBaseComp)
	config.RegisterComponentAttributeMapConverter(
		base.SubtypeName,
		"wheeled",
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

	left      []motor.Motor
	right     []motor.Motor
	allMotors []motor.Motor

	opMgr operation.SingleOperationManager
}

func (base *wheeledBase) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, extra map[string]interface{}) error {
	ctx, done := base.opMgr.New(ctx)
	defer done()

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

func (base *wheeledBase) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	ctx, done := base.opMgr.New(ctx)
	defer done()

	// Stop the motors if the speed or distance are 0
	if math.Abs(mmPerSec) < 0.0001 || distanceMm == 0 {
		err := base.Stop(ctx, nil)
		if err != nil {
			return errors.Errorf("error when trying to move straight at a speed and/or distance of 0: %v", err)
		}
		return err
	}

	// Straight math
	rpm, rotations := base.straightDistanceToMotorInfo(distanceMm, mmPerSec)

	return base.runAll(ctx, rpm, rotations, rpm, rotations)
}

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

// differentialDrive takes up and left direction inputs from a first person perspective
// on a 2D plane and converts them to left and right motor powers. negative up means down
// and negative left means right.
func (base *wheeledBase) differentialDrive(up, left float64) (float64, float64) {
	if up < 0 {
		// Mirror the forward turning arc if we go in reverse
		leftMotor, rightMotor := base.differentialDrive(-up, left)
		return -leftMotor, -rightMotor
	}

	// convert to polar coordinates
	r := math.Hypot(up, left)
	t := math.Atan2(left, up)

	// rotate by 45 degrees
	t += math.Pi / 4

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

// returns rpm, revolutions for a spin motion.
func (base *wheeledBase) spinMath(angleDeg float64, degsPerSec float64) (float64, float64) {
	wheelTravel := base.spinSlipFactor * float64(base.widthMm) * math.Pi * angleDeg / 360.0
	revolutions := wheelTravel / float64(base.wheelCircumferenceMm)

	// RPM = revolutions (unit) * deg/sec * (1 rot / 2pi deg) * (60 sec / 1 min) = rot/min
	rpm := revolutions * degsPerSec * 30 / math.Pi
	revolutions = math.Abs(revolutions)

	return rpm, revolutions
}

// return rpms left, right.
func (base *wheeledBase) velocityMath(mmPerSec float64, degsPerSec float64) (float64, float64) {
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
			isOn, err := m.IsPowered(ctx, nil)
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
	var err error
	for _, m := range base.allMotors {
		err = multierr.Combine(err, m.Stop(ctx, extra))
	}
	return err
}

func (base *wheeledBase) IsMoving(ctx context.Context) (bool, error) {
	for _, m := range base.allMotors {
		isMoving, err := m.IsPowered(ctx, nil)
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

func (base *wheeledBase) GetWidth(ctx context.Context) (int, error) {
	return base.widthMm, nil
}

// FourWheelConfig is how you configure a four-wheeled base.
type FourWheelConfig struct {
	WidthMM              int     `json:"width_mm"`
	WheelCircumferenceMM int     `json:"wheel_circumference_mm"`
	SpinSlipFactor       float64 `json:"spin_slip_factor,omitempty"`
	FrontLeft            string  `json:"front_left"`
	FrontRight           string  `json:"front_right"`
	BackLeft             string  `json:"back_left"`
	BackRight            string  `json:"back_right"`
}

// Validate ensures all parts of the config are valid.
func (config *FourWheelConfig) Validate(path string) ([]string, error) {
	var deps []string

	if config.WidthMM == 0 {
		return nil, errors.New("need a width_mm for a four-wheel base")
	}

	if config.WheelCircumferenceMM == 0 {
		return nil, errors.New("need a wheel_circumference_mm for a four-wheel base")
	}

	if len(config.FrontLeft) == 0 {
		return nil, errors.New("need a front_left motor")
	}

	if len(config.FrontRight) == 0 {
		return nil, errors.New("need a front_right motor")
	}

	if len(config.BackLeft) == 0 {
		return nil, errors.New("need a back_left motor")
	}

	if len(config.BackRight) == 0 {
		return nil, errors.New("need a back_right motor")
	}

	deps = append(deps, config.FrontLeft)
	deps = append(deps, config.FrontRight)
	deps = append(deps, config.BackLeft)
	deps = append(deps, config.BackRight)

	return deps, nil
}

// CreateFourWheelBase returns a new four wheel base defined by the given config.
func CreateFourWheelBase(
	ctx context.Context,
	deps registry.Dependencies,
	config *FourWheelConfig,
	logger golog.Logger,
) (base.LocalBase, error) {
	frontLeft, err := motor.FromDependencies(deps, config.FrontLeft)
	if err != nil {
		return nil, errors.Wrap(err, "front_left motor not found")
	}
	frontRight, err := motor.FromDependencies(deps, config.FrontRight)
	if err != nil {
		return nil, errors.Wrap(err, "front_right motor not found")
	}
	backLeft, err := motor.FromDependencies(deps, config.BackLeft)
	if err != nil {
		return nil, errors.Wrap(err, "back_left motor not found")
	}
	backRight, err := motor.FromDependencies(deps, config.BackRight)
	if err != nil {
		return nil, errors.Wrap(err, "back_right motor not found")
	}

	base := &wheeledBase{
		widthMm:              config.WidthMM,
		wheelCircumferenceMm: config.WheelCircumferenceMM,
		spinSlipFactor:       config.SpinSlipFactor,
		left:                 []motor.Motor{frontLeft, backLeft},
		right:                []motor.Motor{frontRight, backRight},
	}

	if base.spinSlipFactor == 0 {
		base.spinSlipFactor = 1
	}

	base.allMotors = append(base.allMotors, base.left...)
	base.allMotors = append(base.allMotors, base.right...)

	return base, nil
}

// Config is how you configure a wheeled base.
type Config struct {
	WidthMM              int      `json:"width_mm"`
	WheelCircumferenceMM int      `json:"wheel_circumference_mm"`
	SpinSlipFactor       float64  `json:"spin_slip_factor,omitempty"`
	Left                 []string `json:"left"`
	Right                []string `json:"right"`
}

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) ([]string, error) {
	var deps []string

	if config.WidthMM == 0 {
		return nil, errors.New("need a width_mm for a wheeled base")
	}

	if config.WheelCircumferenceMM == 0 {
		return nil, errors.New("need a wheel_circumference_mm for a wheeled base")
	}

	if len(config.Left) == 0 || len(config.Right) == 0 {
		return nil, errors.New("need left and right motors")
	}

	if len(config.Left) != len(config.Right) {
		return nil, fmt.Errorf("left and right need to have the same number of motors, not %d vs %d", len(config.Left), len(config.Right))
	}

	deps = append(deps, config.Left...)
	deps = append(deps, config.Right...)

	return deps, nil
}

// CreateWheeledBase returns a new wheeled base defined by the given config.
func CreateWheeledBase(
	ctx context.Context,
	deps registry.Dependencies,
	config *Config,
	logger golog.Logger,
) (base.LocalBase, error) {
	base := &wheeledBase{
		widthMm:              config.WidthMM,
		wheelCircumferenceMm: config.WheelCircumferenceMM,
		spinSlipFactor:       config.SpinSlipFactor,
	}

	if base.spinSlipFactor == 0 {
		base.spinSlipFactor = 1
	}

	for _, name := range config.Left {
		m, err := motor.FromDependencies(deps, name)
		if err != nil {
			return nil, errors.Wrapf(err, "no left motor named (%s)", name)
		}
		base.left = append(base.left, m)
	}

	for _, name := range config.Right {
		m, err := motor.FromDependencies(deps, name)
		if err != nil {
			return nil, errors.Wrapf(err, "no right motor named (%s)", name)
		}
		base.right = append(base.right, m)
	}

	base.allMotors = append(base.allMotors, base.left...)
	base.allMotors = append(base.allMotors, base.right...)

	return base, nil
}
