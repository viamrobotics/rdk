package baseimpl

import (
	"context"
	"math"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/component/motor"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

func init() {
	registry.RegisterBase("four-wheel", registry.Base{Constructor: CreateFourWheelBase})
}

// FourWheelBase implements a four wheeled base
type FourWheelBase struct {
	widthMillis              int
	wheelCircumferenceMillis int
	spinSlipFactor           float64

	frontLeft, frontRight, backRight, backLeft motor.Motor
	AllMotors                                  []motor.Motor
}

// Spin spins the base a specified angle (subset of Move Arc)
func (base *FourWheelBase) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {

	// Spin math
	leftDirection, rpm, revolutions := base.spinMath(angleDeg, degsPerSec)
	rightDirection := board.FlipDirection(leftDirection)

	// Send motor commands
	err := multierr.Combine(
		base.frontLeft.GoFor(ctx, leftDirection, rpm, revolutions),
		base.frontRight.GoFor(ctx, rightDirection, rpm, revolutions),
		base.backLeft.GoFor(ctx, leftDirection, rpm, revolutions),
		base.backRight.GoFor(ctx, rightDirection, rpm, revolutions),
	)

	if err != nil {
		return math.NaN(), multierr.Combine(err, base.Stop(ctx))
	}

	if !block {
		// TODO(erh): return how much it actually spun
		return angleDeg, nil
	}

	// TODO(erh): return how much it actually spun
	return angleDeg, base.WaitForMotorsToStop(ctx)
}

// MoveStraight moves the base a specified distance (subset of Move Arc)
func (base *FourWheelBase) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
	if distanceMillis == 0 && block {
		return 0, errors.New("cannot block unless you have a distance")
	}

	// Straight math
	d, rpm, rotations := base.straightDistanceToMotorInfo(distanceMillis, millisPerSec)

	// Send motor commands
	for _, m := range base.AllMotors {
		err := m.GoFor(ctx, d, rpm, rotations)
		if err != nil {
			// TODO(erh): return how much it actually moved
			return 0, multierr.Combine(err, base.Stop(ctx))
		}
	}

	if !block {
		// TODO(erh): return how much it actually moved
		return distanceMillis, nil
	}

	// TODO(erh): return how much it actually moved
	return distanceMillis, base.WaitForMotorsToStop(ctx)
}

// MoveArc moves the base a specified distance at a set speed and degs per sec
func (base *FourWheelBase) MoveArc(ctx context.Context, distanceMillis int, millisPerSec float64, angleDeg float64, block bool) (int, error) {
	if distanceMillis == 0 && block {
		return 0, errors.New("cannot block unless you have a distance")
	}

	// Arc math
	dirLR, rpmLR, revLR := base.arcMath(angleDeg, millisPerSec, distanceMillis)

	// Send motor commands
	err := multierr.Combine(
		base.frontLeft.GoFor(ctx, dirLR[0], rpmLR[0], revLR[0]),
		base.frontRight.GoFor(ctx, dirLR[1], rpmLR[1], revLR[1]),
		base.backLeft.GoFor(ctx, dirLR[0], rpmLR[0], revLR[0]),
		base.backRight.GoFor(ctx, dirLR[1], rpmLR[1], revLR[1]),
	)

	if err != nil {
		return 0, multierr.Combine(err, base.Stop(ctx))
	}

	if !block {
		// TODO(erh): return how much it actually moved
		return distanceMillis, nil
	}

	// TODO(erh): return how much it actually moved
	return distanceMillis, base.WaitForMotorsToStop(ctx)
}

// SpinMath returns direction, rpm, revolutions for spin motion
func (base *FourWheelBase) spinMath(angleDeg float64, degsPerSec float64) (pb.DirectionRelative, float64, float64) {
	leftDirection := pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
	if angleDeg < 0 {
		leftDirection = board.FlipDirection(leftDirection)
		angleDeg *= -1
	}

	wheelTravel := base.spinSlipFactor * float64(base.widthMillis) * math.Pi * angleDeg / 360.0
	revolutions := wheelTravel / float64(base.wheelCircumferenceMillis)

	// RPM = revolutions (unit) * deg/sec * (1 rot / 2pi deg) * (60 sec / 1 min) = rot/min
	rpm := revolutions * degsPerSec * 30 / math.Pi

	return leftDirection, rpm, revolutions
}

// ArcMath performs calculations for arcing motion
func (base *FourWheelBase) arcMath(degsPerSec float64, millisPerSec float64, distanceMillis int) ([]pb.DirectionRelative, []float64, []float64) {

	if math.Abs(millisPerSec) < 0.01 {
		leftDirection, rpm, revolutions := base.spinMath(float64(distanceMillis), degsPerSec)

		dirs := []pb.DirectionRelative{leftDirection, board.FlipDirection(leftDirection)}
		rpms := []float64{rpm, rpm}
		rots := []float64{revolutions, revolutions}

		return dirs, rpms, rots
	}

	// Base calculations
	v := millisPerSec
	w0 := degsPerSec / 180 * math.Pi
	t := float64(distanceMillis) / math.Abs(millisPerSec)
	r := float64(base.wheelCircumferenceMillis) / (2.0 * math.Pi)
	l := float64(base.widthMillis)

	wL := (v / r) + (l * w0 / (2 * r))
	wR := (v / r) - (l * w0 / (2 * r))

	// Determine directions of each wheel
	dirL := pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
	if wL < 0 {
		dirL = board.FlipDirection(dirL)
		wL *= -1
	}

	dirR := pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
	if wR < 0 {
		dirR = board.FlipDirection(dirR)
		wR *= -1
	}

	// Calculate # of rotations
	rotL := wL * t / (2 * math.Pi)
	rotR := wR * t / (2 * math.Pi)

	// RPM = revolutions (unit) * deg/sec * (1 rot / 2pi deg) * (60 sec / 1 min) = rot/min
	rpmL := (wL / (2 * math.Pi)) * 60
	rpmR := (wR / (2 * math.Pi)) * 60

	dirs := []pb.DirectionRelative{dirL, dirR}
	rpms := []float64{rpmL, rpmR}
	rots := []float64{rotL, rotR}

	return dirs, rpms, rots
}

// StraightDistanceToMotorInfo performs calculations to determeine direction, rpm and # of revolutions
func (base *FourWheelBase) straightDistanceToMotorInfo(distanceMillis int, millisPerSec float64) (pb.DirectionRelative, float64, float64) {
	var d pb.DirectionRelative = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
	if millisPerSec < 0 {
		d = board.FlipDirection(d)
		millisPerSec *= -1
	}

	if distanceMillis < 0 {
		d = board.FlipDirection(d)
		distanceMillis *= -1
	}

	rotations := float64(distanceMillis) / float64(base.wheelCircumferenceMillis)

	rotationsPerSec := millisPerSec / float64(base.wheelCircumferenceMillis)
	rpm := 60 * rotationsPerSec

	return d, rpm, rotations
}

// WaitForMotorsToStop waits for motors to stop
func (base *FourWheelBase) WaitForMotorsToStop(ctx context.Context) error {
	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		}

		anyOn := false
		anyOff := false

		for _, m := range base.AllMotors {
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

// Stop stops motors
func (base *FourWheelBase) Stop(ctx context.Context) error {
	return multierr.Combine(
		base.frontLeft.Off(ctx),
		base.frontRight.Off(ctx),
		base.backLeft.Off(ctx),
		base.backRight.Off(ctx),
	)
}

// Close closes out background processes
func (base *FourWheelBase) Close() error {
	return base.Stop(context.Background())
}

// WidthMillis returns width of base
func (base *FourWheelBase) WidthMillis(ctx context.Context) (int, error) {
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

	base := &FourWheelBase{
		widthMillis:              config.Attributes.Int("widthMillis", 0),
		wheelCircumferenceMillis: config.Attributes.Int("wheelCircumferenceMillis", 0),
		spinSlipFactor:           config.Attributes.Float64("spinSlipFactor", 1.0),
		frontLeft:                frontLeft,
		frontRight:               frontRight,
		backLeft:                 backLeft,
		backRight:                backRight,
	}

	if base.widthMillis == 0 {
		return nil, errors.New("need a widthMillis for a four-wheel base")
	}

	if base.wheelCircumferenceMillis == 0 {
		return nil, errors.New("need a wheelCircumferenceMillis for a four-wheel base")
	}

	if base.frontLeft == nil || base.frontRight == nil || base.backLeft == nil || base.backRight == nil {
		return nil, errors.New("need valid motors for frontLeft, frontRight, backLeft, backRight")
	}

	base.AllMotors = append(base.AllMotors, base.frontLeft)
	base.AllMotors = append(base.AllMotors, base.frontRight)
	base.AllMotors = append(base.AllMotors, base.backLeft)
	base.AllMotors = append(base.AllMotors, base.backRight)

	return base, nil
}
