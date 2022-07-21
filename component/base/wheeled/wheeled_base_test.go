package wheeled

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/motor/fake"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rlog"
)

func fakeMotorDependencies(t *testing.T, deps []string) registry.Dependencies {
	t.Helper()
	logger := golog.NewTestLogger(t)

	result := make(registry.Dependencies)
	for _, dep := range deps {
		result[motor.Named(dep)] = &fake.Motor{
			Encoder:           fake.Encoder{Valid: true},
			PositionReporting: true,
			TicksPerRotation:  1,
			MaxRPM:            60,
			Logger:            logger,
		}
	}
	return result
}

func TestFourWheelBase1(t *testing.T) {
	ctx := context.Background()

	cfg := &FourWheelConfig{}
	_, err := cfg.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)

	cfg = &FourWheelConfig{
		WidthMM:              100,
		WheelCircumferenceMM: 1000,
		FrontRight:           "fr-m",
		FrontLeft:            "fl-m",
		BackRight:            "br-m",
		BackLeft:             "bl-m",
	}
	deps, err := cfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	motorDeps := fakeMotorDependencies(t, deps)

	baseBase, err := CreateFourWheelBase(context.Background(), motorDeps, cfg, rlog.Logger)
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

	// empty config
	cfg := &Config{}
	_, err := cfg.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)

	// invalid config
	cfg = &Config{
		WidthMM:              100,
		WheelCircumferenceMM: 1000,
		Left:                 []string{"fl-m", "bl-m"},
		Right:                []string{"fr-m"},
	}
	_, err = cfg.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)

	// valid config
	cfg = &Config{
		WidthMM:              100,
		WheelCircumferenceMM: 1000,
		Left:                 []string{"fl-m", "bl-m"},
		Right:                []string{"fr-m", "br-m"},
	}
	deps, err := cfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	motorDeps := fakeMotorDependencies(t, deps)

	baseBase, err := CreateWheeledBase(ctx, motorDeps, cfg, rlog.Logger)
	test.That(t, err, test.ShouldBeNil)
	base, ok := baseBase.(*wheeledBase)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, len(base.left), test.ShouldEqual, 2)
	test.That(t, len(base.right), test.ShouldEqual, 2)
	test.That(t, len(base.allMotors), test.ShouldEqual, 4)
}

func TestValidate(t *testing.T) {
	cfg := &Config{}
	deps, err := cfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "need a width_mm for a wheeled base")

	cfg.WidthMM = 100
	deps, err = cfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "need a wheel_circumference_mm for a wheeled base")

	cfg.WheelCircumferenceMM = 1000
	deps, err = cfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "need left and right motors")

	cfg.Left = []string{"fl-m", "bl-m"}
	deps, err = cfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "need left and right motors")

	cfg.Right = []string{"fr-m"}
	deps, err = cfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "left and right need to have the same number of motors, not 2 vs 1")

	cfg.Right = append(cfg.Right, "br-m")
	deps, err = cfg.Validate("path")
	test.That(t, deps, test.ShouldResemble, []string{"fl-m", "bl-m", "fr-m", "br-m"})
	test.That(t, err, test.ShouldBeNil)
}
