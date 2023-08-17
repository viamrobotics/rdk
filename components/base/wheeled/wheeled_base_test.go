package wheeled

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
)

func newTestCfg() resource.Config {
	return resource.Config{
		Name:  "test",
		API:   base.API,
		Model: resource.Model{Name: "wheeled_base"},
		ConvertedAttributes: &Config{
			WidthMM:              100,
			WheelCircumferenceMM: 1000,
			Left:                 []string{"fl-m", "bl-m"},
			Right:                []string{"fr-m", "br-m"},
		},
	}
}

func fakeMotorDependencies(t *testing.T, deps []string) resource.Dependencies {
	t.Helper()
	logger := golog.NewTestLogger(t)

	result := make(resource.Dependencies)
	for _, dep := range deps {
		result[motor.Named(dep)] = &fake.Motor{
			Named:  motor.Named(dep).AsNamed(),
			MaxRPM: 60,
			OpMgr:  operation.NewSingleOperationManager(),
			Logger: logger,
		}
	}
	return result
}

func TestWheelBaseMath(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	testCfg := newTestCfg()
	deps, err := testCfg.Validate("path", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)
	motorDeps := fakeMotorDependencies(t, deps)

	newBase, err := createWheeledBase(context.Background(), motorDeps, testCfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, newBase, test.ShouldNotBeNil)
	wb, ok := newBase.(*wheeledBase)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("basics", func(t *testing.T) {
		props, err := wb.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, props.WidthMeters, test.ShouldEqual, 100*0.001)

		geometries, err := wb.Geometries(ctx, nil)
		test.That(t, geometries, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)

		err = wb.SetVelocity(ctx, r3.Vector{X: 0, Y: 10, Z: 0}, r3.Vector{X: 0, Y: 0, Z: 10}, nil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "0 RPM")

		err = wb.SetVelocity(ctx, r3.Vector{X: 0, Y: 100, Z: 0}, r3.Vector{X: 0, Y: 0, Z: 100}, nil)
		test.That(t, err, test.ShouldBeNil)

		moving, err := wb.IsMoving(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeTrue)

		err = wb.SetPower(ctx, r3.Vector{X: 0, Y: 10, Z: 0}, r3.Vector{X: 0, Y: 0, Z: 10}, nil)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, wb.Stop(ctx, nil), test.ShouldBeNil)

		moving, err = wb.IsMoving(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
	})

	t.Run("math_straight", func(t *testing.T) {
		rpm, rotations := wb.straightDistanceToMotorInputs(1000, 1000)
		test.That(t, rpm, test.ShouldEqual, 60.0)
		test.That(t, rotations, test.ShouldEqual, 1.0)

		rpm, rotations = wb.straightDistanceToMotorInputs(-1000, 1000)
		test.That(t, rpm, test.ShouldEqual, 60.0)
		test.That(t, rotations, test.ShouldEqual, -1.0)

		rpm, rotations = wb.straightDistanceToMotorInputs(1000, -1000)
		test.That(t, rpm, test.ShouldEqual, -60.0)
		test.That(t, rotations, test.ShouldEqual, 1.0)

		rpm, rotations = wb.straightDistanceToMotorInputs(-1000, -1000)
		test.That(t, rpm, test.ShouldEqual, -60.0)
		test.That(t, rotations, test.ShouldEqual, -1.0)
	})

	t.Run("straight no speed", func(t *testing.T) {
		err := wb.MoveStraight(ctx, 1000, 0, nil)
		test.That(t, err, test.ShouldBeNil)

		err = waitForMotorsToStop(ctx, wb)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range wb.allMotors {
			isOn, powerPct, err := m.IsPowered(context.Background(), nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
			test.That(t, powerPct, test.ShouldEqual, 0.0)
		}
	})

	t.Run("straight no distance", func(t *testing.T) {
		err := wb.MoveStraight(ctx, 0, 1000, nil)
		test.That(t, err, test.ShouldBeNil)

		err = waitForMotorsToStop(ctx, wb)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range wb.allMotors {
			isOn, powerPct, err := m.IsPowered(context.Background(), nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
			test.That(t, powerPct, test.ShouldEqual, 0.0)
		}
	})

	t.Run("waitForMotorsToStop", func(t *testing.T) {
		err := wb.Stop(ctx, nil)
		test.That(t, err, test.ShouldBeNil)

		err = wb.allMotors[0].SetPower(ctx, 1, nil)
		test.That(t, err, test.ShouldBeNil)
		isOn, powerPct, err := wb.allMotors[0].IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isOn, test.ShouldBeTrue)
		test.That(t, powerPct, test.ShouldEqual, 1.0)

		err = waitForMotorsToStop(ctx, wb)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range wb.allMotors {
			isOn, powerPct, err := m.IsPowered(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
			test.That(t, powerPct, test.ShouldEqual, 0.0)
		}

		err = waitForMotorsToStop(ctx, wb)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range wb.allMotors {
			isOn, powerPct, err := m.IsPowered(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
			test.That(t, powerPct, test.ShouldEqual, 0.0)
		}
	})

	test.That(t, wb.Close(context.Background()), test.ShouldBeNil)
	t.Run("go block", func(t *testing.T) {
		go func() {
			time.Sleep(time.Millisecond * 10)
			err = wb.Stop(ctx, nil)
			if err != nil {
				panic(err)
			}
		}()

		err := wb.MoveStraight(ctx, 10000, 1000, nil)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range wb.allMotors {
			isOn, powerPct, err := m.IsPowered(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
			test.That(t, powerPct, test.ShouldEqual, 0.0)
		}
	})
	// Spin tests
	t.Run("spin math", func(t *testing.T) {
		rpms, rotations := wb.spinMath(90, 10)
		test.That(t, rpms, test.ShouldAlmostEqual, 0.523, 0.001)
		test.That(t, rotations, test.ShouldAlmostEqual, 0.0785, 0.001)

		rpms, rotations = wb.spinMath(-90, 10)
		test.That(t, rpms, test.ShouldAlmostEqual, -0.523, 0.001)
		test.That(t, rotations, test.ShouldAlmostEqual, 0.0785, 0.001)

		rpms, rotations = wb.spinMath(90, -10)
		test.That(t, rpms, test.ShouldAlmostEqual, -0.523, 0.001)
		test.That(t, rotations, test.ShouldAlmostEqual, 0.0785, 0.001)

		rpms, rotations = wb.spinMath(-90, -10)
		test.That(t, rpms, test.ShouldAlmostEqual, 0.523, 0.001)
		test.That(t, rotations, test.ShouldAlmostEqual, 0.0785, 0.001)

		rpms, rotations = wb.spinMath(60, 10)
		test.That(t, rpms, test.ShouldAlmostEqual, 0.523, 0.001)
		test.That(t, rotations, test.ShouldAlmostEqual, 0.0523, 0.001)

		rpms, rotations = wb.spinMath(30, 10)
		test.That(t, rpms, test.ShouldAlmostEqual, 0.523, 0.001)
		test.That(t, rotations, test.ShouldAlmostEqual, 0.0261, 0.001)
	})
	t.Run("spin block", func(t *testing.T) {
		go func() {
			time.Sleep(time.Millisecond * 10)
			err := wb.Stop(ctx, nil)
			if err != nil {
				panic(err)
			}
		}()

		err := wb.Spin(ctx, 5, 5, nil)
		test.That(t, err, test.ShouldBeNil)

		for _, m := range wb.allMotors {
			isOn, powerPct, err := m.IsPowered(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isOn, test.ShouldBeFalse)
			test.That(t, powerPct, test.ShouldEqual, 0.0)
		}
	})

	// Velocity tests
	t.Run("velocity math curved", func(t *testing.T) {
		l, r := wb.velocityMath(1000, 10)
		test.That(t, l, test.ShouldAlmostEqual, 59.476, 0.01)
		test.That(t, r, test.ShouldAlmostEqual, 60.523, 0.01)

		l, r = wb.velocityMath(-1000, -10)
		test.That(t, l, test.ShouldAlmostEqual, -59.476, 0.01)
		test.That(t, r, test.ShouldAlmostEqual, -60.523, 0.01)

		l, r = wb.velocityMath(1000, -10)
		test.That(t, l, test.ShouldAlmostEqual, 60.523, 0.01)
		test.That(t, r, test.ShouldAlmostEqual, 59.476, 0.01)

		l, r = wb.velocityMath(-1000, 10)
		test.That(t, l, test.ShouldAlmostEqual, -60.523, 0.01)
		test.That(t, r, test.ShouldAlmostEqual, -59.476, 0.01)
	})

	t.Run("arc math zero angle", func(t *testing.T) {
		l, r := wb.velocityMath(1000, 0)
		test.That(t, l, test.ShouldEqual, 60.0)
		test.That(t, r, test.ShouldEqual, 60.0)

		l, r = wb.velocityMath(-1000, 0)
		test.That(t, l, test.ShouldEqual, -60.0)
		test.That(t, r, test.ShouldEqual, -60.0)
	})

	t.Run("differentialDrive", func(t *testing.T) {
		var fwdL, fwdR, revL, revR float64

		// Go straight (↑)
		t.Logf("Go straight (↑)")
		fwdL, fwdR = wb.differentialDrive(1, 0)
		test.That(t, fwdL, test.ShouldBeGreaterThan, 0)
		test.That(t, fwdR, test.ShouldBeGreaterThan, 0)
		test.That(t, math.Abs(fwdL), test.ShouldEqual, math.Abs(fwdR))

		// Go forward-left (↰)
		t.Logf("Go forward-left (↰)")
		fwdL, fwdR = wb.differentialDrive(1, 1)
		test.That(t, fwdL, test.ShouldBeGreaterThan, 0)
		test.That(t, fwdR, test.ShouldBeGreaterThan, 0)
		test.That(t, math.Abs(fwdL), test.ShouldBeLessThan, math.Abs(fwdR))

		// Go reverse-left (↲)
		t.Logf("Go reverse-left (↲)")
		revL, revR = wb.differentialDrive(-1, 1)
		test.That(t, revL, test.ShouldBeLessThan, 0)
		test.That(t, revR, test.ShouldBeLessThan, 0)
		test.That(t, math.Abs(revL), test.ShouldBeLessThan, math.Abs(revR))

		// Ensure motor powers are mirrored going left.
		test.That(t, math.Abs(fwdL), test.ShouldEqual, math.Abs(revL))
		test.That(t, math.Abs(fwdR), test.ShouldEqual, math.Abs(revR))

		// Go forward-right (↱)
		t.Logf("Go forward-right (↱)")
		fwdL, fwdR = wb.differentialDrive(1, -1)
		test.That(t, fwdL, test.ShouldBeGreaterThan, 0)
		test.That(t, fwdR, test.ShouldBeGreaterThan, 0)
		test.That(t, math.Abs(fwdL), test.ShouldBeGreaterThan, math.Abs(revL))

		// Go reverse-right (↳)
		t.Logf("Go reverse-right (↳)")
		revL, revR = wb.differentialDrive(-1, -1)
		test.That(t, revL, test.ShouldBeLessThan, 0)
		test.That(t, revR, test.ShouldBeLessThan, 0)
		test.That(t, math.Abs(revL), test.ShouldBeGreaterThan, math.Abs(revR))

		// Ensure motor powers are mirrored going right.
		test.That(t, math.Abs(fwdL), test.ShouldEqual, math.Abs(revL))
		test.That(t, math.Abs(fwdR), test.ShouldEqual, math.Abs(revR))

		// Test spin left (↺)
		t.Logf("Test spin left (↺)")
		spinL, spinR := wb.differentialDrive(0, 1)
		test.That(t, spinL, test.ShouldBeLessThanOrEqualTo, 0)
		test.That(t, spinR, test.ShouldBeGreaterThan, 0)

		// Test spin right (↻)
		t.Logf("Test spin right (↻)")
		spinL, spinR = wb.differentialDrive(0, -1)
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
	testCfg := newTestCfg()
	deps, err := testCfg.Validate("path", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)
	motorDeps := fakeMotorDependencies(t, deps)

	newBase, err := createWheeledBase(ctx, motorDeps, testCfg, logger)
	test.That(t, err, test.ShouldBeNil)
	wb, ok := newBase.(*wheeledBase)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, len(wb.left), test.ShouldEqual, 2)
	test.That(t, len(wb.right), test.ShouldEqual, 2)
	test.That(t, len(wb.allMotors), test.ShouldEqual, 4)
}

func TestWheeledBaseReconfigure(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	// valid config
	testCfg := newTestCfg()
	deps, err := testCfg.Validate("path", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)
	motorDeps := fakeMotorDependencies(t, deps)

	newBase, err := createWheeledBase(ctx, motorDeps, testCfg, logger)
	test.That(t, err, test.ShouldBeNil)
	wb, ok := newBase.(*wheeledBase)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, len(wb.left), test.ShouldEqual, 2)
	test.That(t, len(wb.right), test.ShouldEqual, 2)
	test.That(t, len(wb.allMotors), test.ShouldEqual, 4)

	// invert the motors to confirm that Reconfigure still occurs when array order/naming changes
	newTestConf := newTestCfg()
	newTestConf.ConvertedAttributes = &Config{
		WidthMM:              100,
		WheelCircumferenceMM: 1000,
		Left:                 []string{"fr-m", "br-m"},
		Right:                []string{"fl-m", "bl-m"},
	}
	deps, err = newTestConf.Validate("path", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)
	motorDeps = fakeMotorDependencies(t, deps)
	test.That(t, wb.Reconfigure(ctx, motorDeps, newTestConf), test.ShouldBeNil)

	// Add a new motor to Left only to confirm that Reconfigure is impossible because cfg validation fails
	newerTestCfg := newTestCfg()
	newerTestCfg.ConvertedAttributes = &Config{
		WidthMM:              100,
		WheelCircumferenceMM: 1000,
		Left:                 []string{"fl-m", "bl-m", "ml-m"},
		Right:                []string{"fr-m", "br-m"},
	}

	deps, err = newerTestCfg.Validate("path", resource.APITypeComponentName)
	test.That(t, err.Error(), test.ShouldContainSubstring, "left and right need to have the same number of motors")
	test.That(t, deps, test.ShouldBeNil)

	// Add a motor to Right so Left and Right are now the same size again, confirm that Reconfigure
	// occurs after the motor array size change
	newestTestCfg := newTestCfg()
	newestTestCfg.ConvertedAttributes = &Config{
		WidthMM:              100,
		WheelCircumferenceMM: 1000,
		Left:                 []string{"fl-m", "bl-m", "ml-m"},
		Right:                []string{"fr-m", "br-m", "mr-m"},
	}

	deps, err = newestTestCfg.Validate("path", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)
	motorDeps = fakeMotorDependencies(t, deps)
	test.That(t, wb.Reconfigure(ctx, motorDeps, newestTestCfg), test.ShouldBeNil)
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

// waitForMotorsToStop polls all motors to see if they're on, used only for testing.
func waitForMotorsToStop(ctx context.Context, wb *wheeledBase) error {
	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		}

		anyOn := false
		anyOff := false

		for _, m := range wb.allMotors {
			isOn, _, err := m.IsPowered(ctx, nil)
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
			return wb.Stop(ctx, nil)
		}
	}
}
