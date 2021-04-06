package robot

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

func init() {
	api.RegisterBase("four-wheel", CreateFourWheelBase)
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

func (base *fourWheelBase) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
	if distanceMillis == 0 && block {
		return fmt.Errorf("cannot block unless you have a distance")
	}

	d, rpm, rotations := base.straightDistanceToMotorInfo(distanceMillis, millisPerSec)

	for _, m := range base.allMotors {
		err := m.GoFor(ctx, d, rpm, rotations)
		if err != nil {
			return multierr.Combine(err, base.Stop(ctx))
		}
	}

	if !block {
		return nil
	}

	return base.waitForMotorsToStop(ctx)
}

// return left direction, rpm, rotations
func (base *fourWheelBase) spinMath(angleDeg float64, speed int) (pb.DirectionRelative, float64, float64) {
	leftDirection := pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
	if angleDeg < 0 {
		leftDirection = board.FlipDirection(leftDirection)
		angleDeg *= -1
	}

	wheelTravel := base.spinSlipFactor * float64(base.widthMillis) * math.Pi * angleDeg / 360.0
	rotations := wheelTravel / float64(base.wheelCircumferenceMillis)

	// TODO(erh): spin use speed "correctly"
	// for now, assume we want to turn in 1 seconds
	rpm := rotations * 60

	return leftDirection, rpm, rotations
}

func (base *fourWheelBase) Spin(ctx context.Context, angleDeg float64, speed int, block bool) error {
	leftDirection, rpm, rotations := base.spinMath(angleDeg, speed)
	rightDirection := board.FlipDirection(leftDirection)

	err := multierr.Combine(
		base.frontLeft.GoFor(ctx, leftDirection, rpm, rotations),
		base.frontRight.GoFor(ctx, rightDirection, rpm, rotations),
		base.backLeft.GoFor(ctx, leftDirection, rpm, rotations),
		base.backRight.GoFor(ctx, rightDirection, rpm, rotations),
	)

	if err != nil {
		return multierr.Combine(err, base.Stop(ctx))
	}

	if !block {
		return nil
	}

	return base.waitForMotorsToStop(ctx)
}

func (base *fourWheelBase) waitForMotorsToStop(ctx context.Context) error {
	for {
		time.Sleep(10 * time.Millisecond)

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

func (base *fourWheelBase) Close(ctx context.Context) error {
	return base.Stop(ctx)
}

func (base *fourWheelBase) WidthMillis(ctx context.Context) (int, error) {
	return base.widthMillis, nil
}

func CreateFourWheelBase(ctx context.Context, r api.Robot, config api.Component, logger golog.Logger) (api.Base, error) {
	board := r.BoardByName(config.Attributes.GetString("board"))
	if board == nil {
		return nil, fmt.Errorf("need a board for four-wheel, named (%v)", config.Attributes["board"])
	}

	base := &fourWheelBase{
		widthMillis:              config.Attributes.GetInt("widthMillis", 0),
		wheelCircumferenceMillis: config.Attributes.GetInt("wheelCircumferenceMillis", 0),
		spinSlipFactor:           config.Attributes.GetFloat64("spinSlipFactor", 1.0),
		frontLeft:                board.Motor(config.Attributes.GetString("frontLeft")),
		frontRight:               board.Motor(config.Attributes.GetString("frontRight")),
		backLeft:                 board.Motor(config.Attributes.GetString("backLeft")),
		backRight:                board.Motor(config.Attributes.GetString("backRight")),
	}

	if base.widthMillis == 0 {
		return nil, fmt.Errorf("need a widthMillis for a four-wheel base")
	}

	if base.wheelCircumferenceMillis == 0 {
		return nil, fmt.Errorf("need a wheelCircumferenceMillis for a four-wheel base")
	}

	if base.frontLeft == nil || base.frontRight == nil || base.backLeft == nil || base.backRight == nil {
		return nil, fmt.Errorf("need valid motors for frontLeft, frontRight, backLeft, backRight")
	}

	base.allMotors = append(base.allMotors, base.frontLeft)
	base.allMotors = append(base.allMotors, base.frontRight)
	base.allMotors = append(base.allMotors, base.backLeft)
	base.allMotors = append(base.allMotors, base.backRight)

	return base, nil
}
