package wheeled

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/registry"
)

func fakeMotorDependencies(t *testing.T, deps []string) registry.Dependencies {
	t.Helper()
	logger := golog.NewTestLogger(t)

	result := make(registry.Dependencies)
	for _, dep := range deps {
		result[motor.Named(dep)] = &fake.Motor{
			MaxRPM: 60,
			Logger: logger,
		}
	}
	return result
}

func TestWheelBaseMath(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg := &Config{
		WidthMM:              100,
		WheelCircumferenceMM: 1000,
		Left:                 []string{"fl-m", "bl-m"},
		Right:                []string{"fr-m", "br-m"},
	}
	deps, err := cfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	motorDeps := fakeMotorDependencies(t, deps)

	baseBase, err := CreateWheeledBase(context.Background(), motorDeps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, baseBase, test.ShouldNotBeNil)
	base, ok := baseBase.(*wheeledBase)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("basics", func(t *testing.T) {
		temp, err := base.Width(ctx)
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
		err := base.MoveStraight(ctx, 1000, 0, nil)
		test.That(t, err, test.ShouldBeNil)

		err = base.WaitForMotorsToStop(ctx)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.allMotors {
			isOn, powerPct, err := m.IsPowered(context.Background(), nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
			test.That(t, powerPct, test.ShouldEqual, 0.0)
		}
	})

	t.Run("straight no distance", func(t *testing.T) {
		err := base.MoveStraight(ctx, 0, 1000, nil)
		test.That(t, err, test.ShouldBeNil)

		err = base.WaitForMotorsToStop(ctx)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.allMotors {
			isOn, powerPct, err := m.IsPowered(context.Background(), nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
			test.That(t, powerPct, test.ShouldEqual, 0.0)
		}
	})

	t.Run("WaitForMotorsToStop", func(t *testing.T) {
		err := base.Stop(ctx, nil)
		test.That(t, err, test.ShouldBeNil)

		err = base.allMotors[0].SetPower(ctx, 1, nil)
		test.That(t, err, test.ShouldBeNil)
		isOn, powerPct, err := base.allMotors[0].IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isOn, test.ShouldBeTrue)
		test.That(t, powerPct, test.ShouldEqual, 1.0)

		err = base.WaitForMotorsToStop(ctx)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.allMotors {
			isOn, powerPct, err := m.IsPowered(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
			test.That(t, powerPct, test.ShouldEqual, 0.0)
		}

		err = base.WaitForMotorsToStop(ctx)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.allMotors {
			isOn, powerPct, err := m.IsPowered(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
			test.That(t, powerPct, test.ShouldEqual, 0.0)
		}
	})

	test.That(t, base.Close(context.Background()), test.ShouldBeNil)
	t.Run("go block", func(t *testing.T) {
		go func() {
			time.Sleep(time.Millisecond * 10)
			err = base.Stop(ctx, nil)
			if err != nil {
				panic(err)
			}
		}()

		err := base.MoveStraight(ctx, 10000, 1000, nil)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.allMotors {
			isOn, powerPct, err := m.IsPowered(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
			test.That(t, powerPct, test.ShouldEqual, 0.0)
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
			err := base.Stop(ctx, nil)
			if err != nil {
				panic(err)
			}
		}()

		err := base.Spin(ctx, 5, 5, nil)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range base.allMotors {
			isOn, powerPct, err := m.IsPowered(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
			test.That(t, powerPct, test.ShouldEqual, 0.0)
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

	t.Run("differentialDrive", func(t *testing.T) {
		var fwdL, fwdR, revL, revR float64

		// Go straight (↑)
		t.Logf("Go straight (↑)")
		fwdL, fwdR = base.differentialDrive(1, 0)
		test.That(t, fwdL, test.ShouldBeGreaterThan, 0)
		test.That(t, fwdR, test.ShouldBeGreaterThan, 0)
		test.That(t, math.Abs(fwdL), test.ShouldEqual, math.Abs(fwdR))

		// Go forward-left (↰)
		t.Logf("Go forward-left (↰)")
		fwdL, fwdR = base.differentialDrive(1, 1)
		test.That(t, fwdL, test.ShouldBeGreaterThan, 0)
		test.That(t, fwdR, test.ShouldBeGreaterThan, 0)
		test.That(t, math.Abs(fwdL), test.ShouldBeLessThan, math.Abs(fwdR))

		// Go reverse-left (↲)
		t.Logf("Go reverse-left (↲)")
		revL, revR = base.differentialDrive(-1, 1)
		test.That(t, revL, test.ShouldBeLessThan, 0)
		test.That(t, revR, test.ShouldBeLessThan, 0)
		test.That(t, math.Abs(revL), test.ShouldBeLessThan, math.Abs(revR))

		// Ensure motor powers are mirrored going left.
		test.That(t, math.Abs(fwdL), test.ShouldEqual, math.Abs(revL))
		test.That(t, math.Abs(fwdR), test.ShouldEqual, math.Abs(revR))

		// Go forward-right (↱)
		t.Logf("Go forward-right (↱)")
		fwdL, fwdR = base.differentialDrive(1, -1)
		test.That(t, fwdL, test.ShouldBeGreaterThan, 0)
		test.That(t, fwdR, test.ShouldBeGreaterThan, 0)
		test.That(t, math.Abs(fwdL), test.ShouldBeGreaterThan, math.Abs(revL))

		// Go reverse-right (↳)
		t.Logf("Go reverse-right (↳)")
		revL, revR = base.differentialDrive(-1, -1)
		test.That(t, revL, test.ShouldBeLessThan, 0)
		test.That(t, revR, test.ShouldBeLessThan, 0)
		test.That(t, math.Abs(revL), test.ShouldBeGreaterThan, math.Abs(revR))

		// Ensure motor powers are mirrored going right.
		test.That(t, math.Abs(fwdL), test.ShouldEqual, math.Abs(revL))
		test.That(t, math.Abs(fwdR), test.ShouldEqual, math.Abs(revR))

		// Test spin left (↺)
		t.Logf("Test spin left (↺)")
		spinL, spinR := base.differentialDrive(0, 1)
		test.That(t, spinL, test.ShouldBeLessThanOrEqualTo, 0)
		test.That(t, spinR, test.ShouldBeGreaterThan, 0)

		// Test spin right (↻)
		t.Logf("Test spin right (↻)")
		spinL, spinR = base.differentialDrive(0, -1)
		test.That(t, spinL, test.ShouldBeGreaterThan, 0)
		test.That(t, spinR, test.ShouldBeLessThanOrEqualTo, 0)
	})
}

func TestWheeledBaseConstructor(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

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

	baseBase, err := CreateWheeledBase(ctx, motorDeps, cfg, logger)
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
	test.That(t, err.Error(), test.ShouldContainSubstring, "\"width_mm\" is required")

	cfg.WidthMM = 100
	deps, err = cfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "\"wheel_circumference_mm\" is required")

	cfg.WheelCircumferenceMM = 1000
	deps, err = cfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "\"left\" is required")

	cfg.Left = []string{"fl-m", "bl-m"}
	deps, err = cfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "\"right\" is required")

	cfg.Right = []string{"fr-m"}
	deps, err = cfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "left and right need to have the same number of motors, not 2 vs 1")

	cfg.Right = append(cfg.Right, "br-m")
	deps, err = cfg.Validate("path")
	test.That(t, deps, test.ShouldResemble, []string{"fl-m", "bl-m", "fr-m", "br-m"})
	test.That(t, err, test.ShouldBeNil)
}
