package baseimpl_test

import (
	"context"
	"testing"
	"time"

	baseimpl "go.viam.com/core/base/impl"
	"go.viam.com/core/config"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/rlog"
	robotimpl "go.viam.com/core/robot/impl"

	"go.viam.com/test"
)

func TestFourWheelBase1(t *testing.T) {
	ctx := context.Background()
	r, err := robotimpl.New(ctx,
		&config.Config{
			Components: []config.Component{
				{
					Name:                "fr-m",
					Model:               "fake",
					Type:                config.ComponentTypeMotor,
					ConvertedAttributes: &motor.Config{},
				},
				{
					Name:                "fl-m",
					Model:               "fake",
					Type:                config.ComponentTypeMotor,
					ConvertedAttributes: &motor.Config{},
				},
				{
					Name:                "br-m",
					Model:               "fake",
					Type:                config.ComponentTypeMotor,
					ConvertedAttributes: &motor.Config{},
				},
				{
					Name:                "bl-m",
					Model:               "fake",
					Type:                config.ComponentTypeMotor,
					ConvertedAttributes: &motor.Config{},
				},
			},
		},
		rlog.Logger,
	)
	test.That(t, err, test.ShouldBeNil)
	defer test.That(t, r.Close(), test.ShouldBeNil)

	_, err = baseimpl.CreateFourWheelBase(context.Background(), r, config.Component{}, rlog.Logger)
	test.That(t, err, test.ShouldNotBeNil)

	cfg := config.Component{
		Attributes: config.AttributeMap{
			"widthMillis":              100,
			"wheelCircumferenceMillis": 1000,
			"frontRight":               "fr-m",
			"frontLeft":                "fl-m",
			"backRight":                "br-m",
			"backLeft":                 "bl-m",
		},
	}
	baseBase, err := baseimpl.CreateFourWheelBase(context.Background(), r, cfg, rlog.Logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, baseBase, test.ShouldNotBeNil)
	base, ok := baseBase.(*baseimpl.FourWheelBase)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("basics", func(t *testing.T) {
		temp, err := base.WidthMillis(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, temp, test.ShouldEqual, 100)
	})

	t.Run("math", func(t *testing.T) {
		d, rpm, rotations := base.StraightDistanceToMotorInfo(1000, 1000)
		test.That(t, d, test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
		test.That(t, rpm, test.ShouldEqual, 60.0)
		test.That(t, rotations, test.ShouldEqual, 1.0)

		d, rpm, rotations = base.StraightDistanceToMotorInfo(-1000, 1000)
		test.That(t, d, test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
		test.That(t, rpm, test.ShouldEqual, 60.0)
		test.That(t, rotations, test.ShouldEqual, 1.0)

		d, rpm, rotations = base.StraightDistanceToMotorInfo(-1000, -1000)
		test.That(t, d, test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
		test.That(t, rpm, test.ShouldEqual, 60.0)
		test.That(t, rotations, test.ShouldEqual, 1.0)
	})

	t.Run("WaitForMotorsToStop", func(t *testing.T) {
		err := base.Stop(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = base.AllMotors[0].Go(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1)
		test.That(t, err, test.ShouldBeNil)
		isOn, err := base.AllMotors[0].IsOn(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isOn, test.ShouldBeTrue)

		err = base.WaitForMotorsToStop(ctx)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.AllMotors {
			isOn, err := m.IsOn(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
		}

		err = base.WaitForMotorsToStop(ctx)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.AllMotors {
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

		for _, m := range base.AllMotors {
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

		for _, m := range base.AllMotors {
			isOn, err := m.IsOn(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
		}

	})

	t.Run("spin math", func(t *testing.T) {
		// i'm only testing pieces that are correct

		leftDirection, _, rotations := base.SpinMath(90, 10)
		test.That(t, leftDirection, test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
		test.That(t, rotations, test.ShouldAlmostEqual, .0785, .001)

		leftDirection, _, rotations = base.SpinMath(-90, 10)
		test.That(t, leftDirection, test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
		test.That(t, rotations, test.ShouldAlmostEqual, .0785, .001)

	})

	t.Run("spin no block", func(t *testing.T) {
		spun, err := base.Spin(ctx, 5, 5, false)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spun, test.ShouldEqual, float64(5))

		for _, m := range base.AllMotors {
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

		for _, m := range base.AllMotors {
			isOn, err := m.IsOn(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
		}

	})

}
