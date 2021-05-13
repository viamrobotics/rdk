package baseimpl

import (
	"context"
	"testing"
	"time"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/rlog"
	robotimpl "go.viam.com/core/robot/impl"

	"go.viam.com/test"
)

func TestFourWheelBase1(t *testing.T) {
	ctx := context.Background()
	r, err := robotimpl.New(ctx,
		&config.Config{
			Boards: []board.Config{
				{
					Name:  "local",
					Model: "fake",
					Motors: []board.MotorConfig{
						{Name: "fr-m"},
						{Name: "fl-m"},
						{Name: "br-m"},
						{Name: "bl-m"},
					},
				},
			},
		},
		rlog.Logger,
	)
	test.That(t, err, test.ShouldBeNil)

	_, err = CreateFourWheelBase(context.Background(), r, config.Component{}, rlog.Logger)
	test.That(t, err, test.ShouldNotBeNil)

	cfg := config.Component{
		Attributes: config.AttributeMap{
			"widthMillis":              100,
			"wheelCircumferenceMillis": 1000,
			"board":                    "local",
			"frontRight":               "fr-m",
			"frontLeft":                "fl-m",
			"backRight":                "br-m",
			"backLeft":                 "bl-m",
		},
	}
	baseBase, err := CreateFourWheelBase(context.Background(), r, cfg, rlog.Logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, baseBase, test.ShouldNotBeNil)
	base, ok := baseBase.(*fourWheelBase)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("basics", func(t *testing.T) {
		temp, err := base.WidthMillis(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, temp, test.ShouldEqual, 100)
	})

	t.Run("math", func(t *testing.T) {
		d, rpm, rotations := base.straightDistanceToMotorInfo(1000, 1000)
		test.That(t, d, test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
		test.That(t, rpm, test.ShouldEqual, 60.0)
		test.That(t, rotations, test.ShouldEqual, 1.0)

		d, rpm, rotations = base.straightDistanceToMotorInfo(-1000, 1000)
		test.That(t, d, test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
		test.That(t, rpm, test.ShouldEqual, 60.0)
		test.That(t, rotations, test.ShouldEqual, 1.0)

		d, rpm, rotations = base.straightDistanceToMotorInfo(-1000, -1000)
		test.That(t, d, test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
		test.That(t, rpm, test.ShouldEqual, 60.0)
		test.That(t, rotations, test.ShouldEqual, 1.0)
	})

	t.Run("waitForMotorsToStop", func(t *testing.T) {
		err := base.Stop(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = base.allMotors[0].Go(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1)
		test.That(t, err, test.ShouldBeNil)
		isOn, err := base.allMotors[0].IsOn(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isOn, test.ShouldBeTrue)

		err = base.waitForMotorsToStop(ctx)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.allMotors {
			isOn, err := m.IsOn(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
		}

		err = base.waitForMotorsToStop(ctx)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.allMotors {
			isOn, err := m.IsOn(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
		}

	})

	test.That(t, base.Close(), test.ShouldBeNil)

	t.Run("go no block", func(t *testing.T) {
		moved, err := base.MoveStraight(ctx, 10000, 1000, false)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moved, test.ShouldEqual, 10000)

		for _, m := range base.allMotors {
			isOn, err := m.IsOn(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeTrue)
		}

		err = base.Stop(ctx)
		test.That(t, err, test.ShouldBeNil)

	})

	t.Run("go block", func(t *testing.T) {
		go func() {
			time.Sleep(time.Millisecond * 10)
			err = base.Stop(ctx)
			if err != nil {
				panic(err)
			}
		}()

		moved, err := base.MoveStraight(ctx, 10000, 1000, true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moved, test.ShouldEqual, 10000)

		for _, m := range base.allMotors {
			isOn, err := m.IsOn(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
		}

	})

	t.Run("spin math", func(t *testing.T) {
		// i'm only testing pieces that are correct

		leftDirection, _, rotations := base.spinMath(90, 10)
		test.That(t, leftDirection, test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
		test.That(t, rotations, test.ShouldAlmostEqual, .0785, .001)

		leftDirection, _, rotations = base.spinMath(-90, 10)
		test.That(t, leftDirection, test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
		test.That(t, rotations, test.ShouldAlmostEqual, .0785, .001)

	})

	t.Run("spin no block", func(t *testing.T) {
		spun, err := base.Spin(ctx, 5, 5, false)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spun, test.ShouldEqual, float64(5))

		for _, m := range base.allMotors {
			isOn, err := m.IsOn(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeTrue)
		}

		err = base.Stop(ctx)
		test.That(t, err, test.ShouldBeNil)

	})

	t.Run("spin block", func(t *testing.T) {
		go func() {
			time.Sleep(time.Millisecond * 10)
			err := base.Stop(ctx)
			if err != nil {
				panic(err)
			}
		}()

		spun, err := base.Spin(ctx, 5, 5, true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spun, test.ShouldEqual, float64(5))

		for _, m := range base.allMotors {
			isOn, err := m.IsOn(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
		}

	})

}
