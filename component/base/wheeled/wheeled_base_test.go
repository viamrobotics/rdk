package wheeled

import (
	"context"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/component/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/testutils/inject"
)

func TestFourWheelBase1(t *testing.T) {
	ctx := context.Background()

	fakeRobot := &inject.Robot{}

	fakeRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return &fake.Motor{}, nil
	}

	_, err := CreateFourWheelBase(context.Background(), fakeRobot, config.Component{}, rlog.Logger)
	test.That(t, err, test.ShouldNotBeNil)

	cfg := config.Component{
		Attributes: config.AttributeMap{
			"width_mm":               100,
			"wheel_circumference_mm": 1000,
			"front_right":            "fr-m",
			"front_left":             "fl-m",
			"back_right":             "br-m",
			"back_left":              "bl-m",
		},
	}
	baseBase, err := CreateFourWheelBase(context.Background(), fakeRobot, cfg, rlog.Logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, baseBase, test.ShouldNotBeNil)
	base, ok := baseBase.(*wheeledBase)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("basics", func(t *testing.T) {
		temp, err := base.GetWidth(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, temp, test.ShouldEqual, 100)
	})

	t.Run("math_straight", func(t *testing.T) {
		rpm, rotations := base.straightDistanceToMotorInfo(1000, 1000)
		test.That(t, rpm, test.ShouldEqual, 60.0)
		test.That(t, rotations, test.ShouldEqual, 1.0)

		rpm, rotations = base.straightDistanceToMotorInfo(-1000, 1000)
		test.That(t, rpm, test.ShouldEqual, 60.0)
		test.That(t, rotations, test.ShouldEqual, -1.0)

		rpm, rotations = base.straightDistanceToMotorInfo(1000, -1000)
		test.That(t, rpm, test.ShouldEqual, -60.0)
		test.That(t, rotations, test.ShouldEqual, 1.0)

		rpm, rotations = base.straightDistanceToMotorInfo(-1000, -1000)
		test.That(t, rpm, test.ShouldEqual, -60.0)
		test.That(t, rotations, test.ShouldEqual, -1.0)
	})

	t.Run("straight no speed", func(t *testing.T) {
		err := base.MoveStraight(ctx, 1000, 0)
		test.That(t, err, test.ShouldBeNil)

		err = base.WaitForMotorsToStop(ctx)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.allMotors {
			isOn, err := m.IsPowered(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
		}
	})

	t.Run("straight no distance", func(t *testing.T) {
		err := base.MoveStraight(ctx, 0, 1000)
		test.That(t, err, test.ShouldBeNil)

		err = base.WaitForMotorsToStop(ctx)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.allMotors {
			isOn, err := m.IsPowered(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
		}
	})

	t.Run("WaitForMotorsToStop", func(t *testing.T) {
		err := base.Stop(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = base.allMotors[0].SetPower(ctx, 1)
		test.That(t, err, test.ShouldBeNil)
		isOn, err := base.allMotors[0].IsPowered(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isOn, test.ShouldBeTrue)

		err = base.WaitForMotorsToStop(ctx)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.allMotors {
			isOn, err := m.IsPowered(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
		}

		err = base.WaitForMotorsToStop(ctx)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.allMotors {
			isOn, err := m.IsPowered(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
		}
	})

	test.That(t, base.Close(context.Background()), test.ShouldBeNil)
	t.Run("go block", func(t *testing.T) {
		go func() {
			time.Sleep(time.Millisecond * 10)
			err = base.Stop(ctx)
			if err != nil {
				panic(err)
			}
		}()

		err := base.MoveStraight(ctx, 10000, 1000)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.allMotors {
			isOn, err := m.IsPowered(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
		}
	})
	// Spin tests
	t.Run("spin math", func(t *testing.T) {
		rpms, rotations := base.spinMath(90, 10)
		test.That(t, rpms, test.ShouldAlmostEqual, 7.5, 0.001)
		test.That(t, rotations, test.ShouldAlmostEqual, 0.0785, 0.001)

		rpms, rotations = base.spinMath(-90, 10)
		test.That(t, rpms, test.ShouldAlmostEqual, -7.5, 0.001)
		test.That(t, rotations, test.ShouldAlmostEqual, 0.0785, 0.001)

		rpms, rotations = base.spinMath(90, -10)
		test.That(t, rpms, test.ShouldAlmostEqual, -7.5, 0.001)
		test.That(t, rotations, test.ShouldAlmostEqual, 0.0785, 0.001)

		rpms, rotations = base.spinMath(-90, -10)
		test.That(t, rpms, test.ShouldAlmostEqual, 7.5, 0.001)
		test.That(t, rotations, test.ShouldAlmostEqual, 0.0785, 0.001)
	})
	t.Run("spin block", func(t *testing.T) {
		go func() {
			time.Sleep(time.Millisecond * 10)
			err := base.Stop(ctx)
			if err != nil {
				panic(err)
			}
		}()

		err := base.Spin(ctx, 5, 5)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.allMotors {
			isOn, err := m.IsPowered(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
		}
	})

	// Velocity tests
	t.Run("velocity math curved", func(t *testing.T) {
		l, r := base.velocityMath(1000, 10)
		test.That(t, l, test.ShouldAlmostEqual, 59.476, 0.01)
		test.That(t, r, test.ShouldAlmostEqual, 60.523, 0.01)

		l, r = base.velocityMath(-1000, -10)
		test.That(t, l, test.ShouldAlmostEqual, -59.476, 0.01)
		test.That(t, r, test.ShouldAlmostEqual, -60.523, 0.01)

		l, r = base.velocityMath(1000, -10)
		test.That(t, l, test.ShouldAlmostEqual, 60.523, 0.01)
		test.That(t, r, test.ShouldAlmostEqual, 59.476, 0.01)

		l, r = base.velocityMath(-1000, 10)
		test.That(t, l, test.ShouldAlmostEqual, -60.523, 0.01)
		test.That(t, r, test.ShouldAlmostEqual, -59.476, 0.01)
	})

	t.Run("arc math zero angle", func(t *testing.T) {
		l, r := base.velocityMath(1000, 0)
		test.That(t, l, test.ShouldEqual, 60.0)
		test.That(t, r, test.ShouldEqual, 60.0)

		l, r = base.velocityMath(-1000, 0)
		test.That(t, l, test.ShouldEqual, -60.0)
		test.That(t, r, test.ShouldEqual, -60.0)
	})

	t.Run("setPowerMath", func(t *testing.T) {
		l, r := base.setPowerMath(r3.Vector{Y: 1}, r3.Vector{})
		test.That(t, l, test.ShouldEqual, 1)
		test.That(t, r, test.ShouldEqual, 1)

		l, r = base.setPowerMath(r3.Vector{Y: -1}, r3.Vector{})
		test.That(t, l, test.ShouldEqual, -1)
		test.That(t, r, test.ShouldEqual, -1)

		l, r = base.setPowerMath(r3.Vector{}, r3.Vector{Z: 1})
		test.That(t, l, test.ShouldEqual, -1)
		test.That(t, r, test.ShouldEqual, 1)

		l, r = base.setPowerMath(r3.Vector{}, r3.Vector{Z: -1})
		test.That(t, l, test.ShouldEqual, 1)
		test.That(t, r, test.ShouldEqual, -1)

		l, r = base.setPowerMath(r3.Vector{Y: 1}, r3.Vector{Z: 1})
		test.That(t, l, test.ShouldAlmostEqual, 0, .001)
		test.That(t, r, test.ShouldEqual, 1)
	})
}

func TestWheeledBaseConstructor(t *testing.T) {
	ctx := context.Background()

	fakeRobot := &inject.Robot{}
	fakeRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return &fake.Motor{}, nil
	}

	_, err := CreateWheeledBase(context.Background(), fakeRobot, &Config{}, rlog.Logger)
	test.That(t, err, test.ShouldNotBeNil)

	cfg := &Config{
		WidthMM:              100,
		WheelCircumferenceMM: 1000,
		Left:                 []string{"fl-m", "bl-m"},
		Right:                []string{"fr-m"},
	}
	_, err = CreateWheeledBase(ctx, fakeRobot, cfg, rlog.Logger)
	test.That(t, err, test.ShouldNotBeNil)

	cfg = &Config{
		WidthMM:              100,
		WheelCircumferenceMM: 1000,
		Left:                 []string{"fl-m", "bl-m"},
		Right:                []string{"fr-m", "br-m"},
	}
	baseBase, err := CreateWheeledBase(ctx, fakeRobot, cfg, rlog.Logger)
	test.That(t, err, test.ShouldBeNil)
	base, ok := baseBase.(*wheeledBase)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, len(base.left), test.ShouldEqual, 2)
	test.That(t, len(base.right), test.ShouldEqual, 2)
	test.That(t, len(base.allMotors), test.ShouldEqual, 4)
}
