package baseimpl

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/base"
	"go.viam.com/core/component/motor"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

func init() {
	registry.RegisterBase("four-wheel", registry.Base{Constructor: CreateFourWheelBase})
	registry.RegisterBase("wheeled", registry.Base{Constructor: CreateWheeledBase})
}

type wheeledBase struct {
	widthMillis              int
	wheelCircumferenceMillis int
	spinSlipFactor           float64

	left      []motor.Motor
	right     []motor.Motor
	allMotors []motor.Motor
}

func (base *wheeledBase) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {

	// Spin math
	rpm, revolutions := base.spinMath(angleDeg, degsPerSec)

	// Send motor commands
	var err error
	for _, m := range base.left {
		err = multierr.Combine(err, m.GoFor(ctx, rpm, revolutions))
	}
	for _, m := range base.right {
		err = multierr.Combine(err, m.GoFor(ctx, -1*rpm, revolutions))
	}

	if err != nil {
		return multierr.Combine(err, base.Stop(ctx))
	}

	if !block {
		return nil
	}

	return base.WaitForMotorsToStop(ctx)
}

func (base *wheeledBase) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
	if distanceMillis == 0 && block {
		return errors.New("cannot block unless you have a distance")
	}

	// Straight math
	rpm, rotations := base.straightDistanceToMotorInfo(distanceMillis, millisPerSec)

	// Send motor commands
	for _, m := range base.allMotors {
		err := m.GoFor(ctx, rpm, rotations)
		if err != nil {
			return multierr.Combine(err, base.Stop(ctx))
		}
	}

	if !block {
		return nil
	}

	return base.WaitForMotorsToStop(ctx)
}

func (base *wheeledBase) MoveArc(ctx context.Context, distanceMillis int, millisPerSec float64, angleDeg float64, block bool) error {
	if millisPerSec == 0 && block {
		return errors.New("cannot block unless you have a speed")
	}

	// Arc math
	rpmLR, revLR := base.arcMath(distanceMillis, millisPerSec, angleDeg)

	// Send motor commands
	var err error
	for _, m := range base.left {
		err = multierr.Combine(err, m.GoFor(ctx, rpmLR[0], revLR[0]))
	}

	for _, m := range base.right {
		err = multierr.Combine(err, m.GoFor(ctx, rpmLR[1], revLR[1]))
	}

	if err != nil {
		return multierr.Combine(err, base.Stop(ctx))
	}

	if !block {
		return nil
	}

	return base.WaitForMotorsToStop(ctx)
}

// returns rpm, revolutions for spin motion
func (base *wheeledBase) spinMath(angleDeg float64, degsPerSec float64) (float64, float64) {
	wheelTravel := base.spinSlipFactor * float64(base.widthMillis) * math.Pi * angleDeg / 360.0
	revolutions := wheelTravel / float64(base.wheelCircumferenceMillis)

	// RPM = revolutions (unit) * deg/sec * (1 rot / 2pi deg) * (60 sec / 1 min) = rot/min
	rpm := revolutions * degsPerSec * 30 / math.Pi
	revolutions = math.Abs(revolutions)

	return rpm, revolutions
}

func (base *wheeledBase) arcMath(distanceMillis int, millisPerSec float64, angleDeg float64) ([]float64, []float64) {
	if distanceMillis == 0 {
		rpm, revolutions := base.spinMath(angleDeg, millisPerSec)
		rpms := []float64{rpm, -1 * rpm}
		rots := []float64{revolutions, revolutions}

		return rpms, rots
	}

	if distanceMillis < 0 {
		distanceMillis *= -1
		millisPerSec *= -1
	}

	// Base calculations
	v := millisPerSec
	t := float64(distanceMillis) / millisPerSec
	r := float64(base.wheelCircumferenceMillis) / (2.0 * math.Pi)
	l := float64(base.widthMillis)

	degsPerSec := angleDeg / 10 /// t
	w0 := degsPerSec / 180 * math.Pi
	wL := (v / r) + (l * w0 / (2 * r))
	wR := (v / r) - (l * w0 / (2 * r))

	// Determine directions of each wheel

	// Calculate # of rotations
	rotL := wL * t / (2 * math.Pi)
	rotR := wR * t / (2 * math.Pi)

	// RPM = revolutions (unit) * deg/sec * (1 rot / 2pi deg) * (60 sec / 1 min) = rot/min
	rpmL := (wL / (2 * math.Pi)) * 60
	rpmR := (wR / (2 * math.Pi)) * 60

	rpms := []float64{rpmL, rpmR}
	rots := []float64{rotL, rotR}

	return rpms, rots
}

func (base *wheeledBase) straightDistanceToMotorInfo(distanceMillis int, millisPerSec float64) (float64, float64) {

	rotations := float64(distanceMillis) / float64(base.wheelCircumferenceMillis)

	rotationsPerSec := millisPerSec / float64(base.wheelCircumferenceMillis)
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
			isOn, err := m.IsOn(ctx)
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
			return base.Stop(ctx)
		}
	}
}

func (base *wheeledBase) Stop(ctx context.Context) error {
	var err error
	for _, m := range base.allMotors {
		err = multierr.Combine(err, m.Off(ctx))
	}
	return err
}

func (base *wheeledBase) Close() error {
	return base.Stop(context.Background())
}

func (base *wheeledBase) WidthMillis(ctx context.Context) (int, error) {
	return base.widthMillis, nil
}

// CreateFourWheelBase returns a new four wheel base defined by the given config.
func CreateFourWheelBase(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (base.Base, error) {
	frontLeft, ok := r.MotorByName(config.Attributes.String("frontLeft"))
	if !ok {
		return nil, errors.New("frontLeft motor not found")
	}
	frontRight, ok := r.MotorByName(config.Attributes.String("frontRight"))
	if !ok {
		return nil, errors.New("frontRight motor not found")
	}
	backLeft, ok := r.MotorByName(config.Attributes.String("backLeft"))
	if !ok {
		return nil, errors.New("backLeft motor not found")
	}
	backRight, ok := r.MotorByName(config.Attributes.String("backRight"))
	if !ok {
		return nil, errors.New("backRight motor not found")
	}

	base := &wheeledBase{
		widthMillis:              config.Attributes.Int("widthMillis", 0),
		wheelCircumferenceMillis: config.Attributes.Int("wheelCircumferenceMillis", 0),
		spinSlipFactor:           config.Attributes.Float64("spinSlipFactor", 1.0),
		left:                     []motor.Motor{frontLeft, backLeft},
		right:                    []motor.Motor{frontRight, backRight},
	}

	if base.widthMillis == 0 {
		return nil, errors.New("need a widthMillis for a four-wheel base")
	}

	if base.wheelCircumferenceMillis == 0 {
		return nil, errors.New("need a wheelCircumferenceMillis for a four-wheel base")
	}

	base.allMotors = append(base.allMotors, base.left...)
	base.allMotors = append(base.allMotors, base.right...)

	return base, nil
}

// CreateWheeledBase returns a new wheeled base defined by the given config.
func CreateWheeledBase(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (base.Base, error) {

	base := &wheeledBase{
		widthMillis:              config.Attributes.Int("widthMillis", 0),
		wheelCircumferenceMillis: config.Attributes.Int("wheelCircumferenceMillis", 0),
		spinSlipFactor:           config.Attributes.Float64("spinSlipFactor", 1.0),
	}

	if base.widthMillis == 0 {
		return nil, errors.New("need a widthMillis for a wheeled base")
	}

	if base.wheelCircumferenceMillis == 0 {
		return nil, errors.New("need a wheelCircumferenceMillis for a wheeled base")
	}

	for _, name := range config.Attributes.StringSlice("left") {
		m, ok := r.MotorByName(name)
		if !ok {
			return nil, fmt.Errorf("no left motor named (%s)", name)
		}
		base.left = append(base.left, m)
	}

	for _, name := range config.Attributes.StringSlice("right") {
		m, ok := r.MotorByName(name)
		if !ok {
			return nil, fmt.Errorf("no right motor named (%s)", name)
		}
		base.right = append(base.right, m)
	}

	if len(base.left) == 0 {
		return nil, errors.New("need left and right motors")
	}

	if len(base.left) != len(base.right) {
		return nil, fmt.Errorf("left and right need to have the same number of motors, not %d vs %d", len(base.left), len(base.right))
	}

	base.allMotors = append(base.allMotors, base.left...)
	base.allMotors = append(base.allMotors, base.right...)

	return base, nil
}
