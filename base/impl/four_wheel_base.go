// Package baseimpl defines implementations of a base.
package baseimpl

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/rlog"
	"go.viam.com/core/robot"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

func init() {
	registry.RegisterBase("four-wheel", CreateFourWheelBase)
}

type fourWheelBase struct {
	widthMillis              int
	wheelCircumferenceMillis int
	spinSlipFactor           float64

	frontLeft, frontRight, backRight, backLeft board.Motor
	allMotors                                  []board.Motor
}

// return direction, rpm, rotations
func (base *fourWheelBase) straightDistanceToMotorInfo(distanceMillis int, millisPerSec float64) (pb.DirectionRelative, float64, float64) {
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

func (base *fourWheelBase) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
	if distanceMillis == 0 && block {
		return 0, errors.New("cannot block unless you have a distance")
	}

	d, rpm, rotations := base.straightDistanceToMotorInfo(distanceMillis, millisPerSec)

	for _, m := range base.allMotors {
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
	return distanceMillis, base.waitForMotorsToStop(ctx)
}

// return left direction, rpm, revolutions
func (base *fourWheelBase) spinMath(angleDeg float64, degsPerSec float64) (pb.DirectionRelative, float64, float64) {
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

func (base *fourWheelBase) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
	leftDirection, rpm, revolutions := base.spinMath(angleDeg, degsPerSec)
	rightDirection := board.FlipDirection(leftDirection)

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
	return angleDeg, base.waitForMotorsToStop(ctx)
}

func (base *fourWheelBase) waitForMotorsToStop(ctx context.Context) error {
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

func (base *fourWheelBase) Stop(ctx context.Context) error {
	return multierr.Combine(
		base.frontLeft.Off(ctx),
		base.frontRight.Off(ctx),
		base.backLeft.Off(ctx),
		base.backRight.Off(ctx),
	)
}

// Reconfigure replaces this base with the given base.
func (base *fourWheelBase) Reconfigure(newBase base.Base) {
	actual, ok := newBase.(*fourWheelBase)
	if !ok {
		panic(fmt.Errorf("expected new base to be %T but got %T", actual, newBase))
	}
	if err := base.Close(); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	*base = *actual
}

func (base *fourWheelBase) Close() error {
	return base.Stop(context.Background())
}

func (base *fourWheelBase) WidthMillis(ctx context.Context) (int, error) {
	return base.widthMillis, nil
}

// CreateFourWheelBase returns a new four wheel base defined by the given config.
func CreateFourWheelBase(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (base.Base, error) {
	board := r.BoardByName(config.Attributes.String("board"))
	if board == nil {
		return nil, errors.Errorf("need a board for four-wheel, named (%v)", config.Attributes["board"])
	}

	base := &fourWheelBase{
		widthMillis:              config.Attributes.Int("widthMillis", 0),
		wheelCircumferenceMillis: config.Attributes.Int("wheelCircumferenceMillis", 0),
		spinSlipFactor:           config.Attributes.Float64("spinSlipFactor", 1.0),
		frontLeft:                board.Motor(config.Attributes.String("frontLeft")),
		frontRight:               board.Motor(config.Attributes.String("frontRight")),
		backLeft:                 board.Motor(config.Attributes.String("backLeft")),
		backRight:                board.Motor(config.Attributes.String("backRight")),
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

	base.allMotors = append(base.allMotors, base.frontLeft)
	base.allMotors = append(base.allMotors, base.frontRight)
	base.allMotors = append(base.allMotors, base.backLeft)
	base.allMotors = append(base.allMotors, base.backRight)

	return base, nil
}
