package baseimpl

import (
	"context"
	"testing"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/core/config"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/rlog"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/testutils/inject"

	"go.viam.com/test"
)

func TestFourWheelBase1(t *testing.T) {
	ctx := context.Background()

	fakeRobot := &inject.Robot{}

	fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		return &fake.Motor{}, true
	}

	_, err := CreateFourWheelBase(context.Background(), fakeRobot, config.Component{}, rlog.Logger)

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
	baseBase, err := CreateFourWheelBase(context.Background(), fakeRobot, cfg, rlog.Logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, baseBase, test.ShouldNotBeNil)
	base, ok := baseBase.(*FourWheelBase)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("basics", func(t *testing.T) {
		temp, err := base.WidthMillis(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, temp, test.ShouldEqual, 100)
	})

	t.Run("math_straight", func(t *testing.T) {
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
	// Spin tests
	t.Run("spin math", func(t *testing.T) {
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
	// Arc tests
	t.Run("arc math", func(t *testing.T) {

		directions, rpms, rotations := base.arcMath(10, 1000, 1000)
		test.That(t, directions[0], test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
		test.That(t, rpms[0], test.ShouldAlmostEqual, 60.524, 0.01)
		test.That(t, rotations[0], test.ShouldAlmostEqual, 1.008, .01)
		test.That(t, directions[1], test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
		test.That(t, rpms[1], test.ShouldAlmostEqual, 59.476, 0.01)
		test.That(t, rotations[1], test.ShouldAlmostEqual, 0.9913, .01)

		directions, _, rotations = base.arcMath(10, -10000, 1000)
		test.That(t, directions[0], test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
		test.That(t, rpms[0], test.ShouldAlmostEqual, 60.524, 0.01)
		test.That(t, rotations[0], test.ShouldAlmostEqual, 0.999, .01)
		test.That(t, directions[1], test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
		test.That(t, rpms[1], test.ShouldAlmostEqual, 59.476, 0.01)
		test.That(t, rotations[1], test.ShouldAlmostEqual, 1.001, .01)

	})

	t.Run("arc math zero speed", func(t *testing.T) {

		directions, _, rotations := base.arcMath(10, 0, 90)
		test.That(t, directions[0], test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
		test.That(t, rotations[0], test.ShouldAlmostEqual, .0785, .001)
		test.That(t, directions[1], test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
		test.That(t, rotations[1], test.ShouldAlmostEqual, .0785, .001)

		directions, _, rotations = base.arcMath(10, 0, -90)
		test.That(t, directions[0], test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
		test.That(t, rotations[0], test.ShouldAlmostEqual, .0785, .001)
		test.That(t, directions[1], test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
		test.That(t, rotations[1], test.ShouldAlmostEqual, .0785, .001)

	})

	t.Run("arc math zero angle", func(t *testing.T) {

		directions, rpms, rotations := base.arcMath(0, 1000, 1000)
		test.That(t, directions[0], test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
		test.That(t, rpms[0], test.ShouldEqual, 60.0)
		test.That(t, rotations[0], test.ShouldEqual, 1.0)
		test.That(t, directions[1], test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
		test.That(t, rpms[1], test.ShouldEqual, 60.0)
		test.That(t, rotations[1], test.ShouldEqual, 1.0)

		directions, rpms, rotations = base.arcMath(0, -1000, 1000)
		test.That(t, directions[0], test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
		test.That(t, rpms[0], test.ShouldEqual, 60.0)
		test.That(t, rotations[0], test.ShouldEqual, 1.0)
		test.That(t, directions[1], test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
		test.That(t, rpms[1], test.ShouldEqual, 60.0)
		test.That(t, rotations[1], test.ShouldEqual, 1.0)

	})

	t.Run("MoveArc test with block and zero distance", func(t *testing.T) {
		distanceMillis, err := base.MoveArc(ctx, 0, 1, 1, true)
		test.That(t, distanceMillis, test.ShouldEqual, 0)
		test.That(t, err, test.ShouldBeError, errors.New("cannot block unless you have a distance"))
	})

}
