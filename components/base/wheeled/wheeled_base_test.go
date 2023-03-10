package wheeled

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
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
	cfg := config.Component{
		Name:  "test",
		Type:  base.Subtype.ResourceSubtype,
		Model: resource.Model{Name: "wheeled_base"},
		ConvertedAttributes: &AttrConfig{
			WidthMM:              100,
			WheelCircumferenceMM: 1000,
			Left:                 []string{"fl-m", "bl-m"},
			Right:                []string{"fr-m", "br-m"},
		},
	}
	deps, err := cfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	motorDeps := fakeMotorDependencies(t, deps)

	baseBase, err := createWheeledBase(context.Background(), motorDeps, cfg, logger)
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
		rpm, rotations := base.straightDistanceToMotorInputs(1000, 1000)
		test.That(t, rpm, test.ShouldEqual, 60.0)
		test.That(t, rotations, test.ShouldEqual, 1.0)

		rpm, rotations = base.straightDistanceToMotorInputs(-1000, 1000)
		test.That(t, rpm, test.ShouldEqual, 60.0)
		test.That(t, rotations, test.ShouldEqual, -1.0)

		rpm, rotations = base.straightDistanceToMotorInputs(1000, -1000)
		test.That(t, rpm, test.ShouldEqual, -60.0)
		test.That(t, rotations, test.ShouldEqual, 1.0)

		rpm, rotations = base.straightDistanceToMotorInputs(-1000, -1000)
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
	cfg := &AttrConfig{}
	_, err := cfg.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)

	// invalid config
	cfg = &AttrConfig{
		WidthMM:              100,
		WheelCircumferenceMM: 1000,
		Left:                 []string{"fl-m", "bl-m"},
		Right:                []string{"fr-m"},
	}
	_, err = cfg.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)

	// valid config
	compCfg := config.Component{
		Name:  "test",
		Type:  base.Subtype.ResourceSubtype,
		Model: resource.Model{Name: "wheeled_base"},
		ConvertedAttributes: &AttrConfig{
			WidthMM:              100,
			WheelCircumferenceMM: 1000,
			Left:                 []string{"fl-m", "bl-m"},
			Right:                []string{"fr-m", "br-m"},
		},
	}
	deps, err := compCfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	motorDeps := fakeMotorDependencies(t, deps)

	baseBase, err := createWheeledBase(ctx, motorDeps, compCfg, logger)
	test.That(t, err, test.ShouldBeNil)
	base, ok := baseBase.(*wheeledBase)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, len(base.left), test.ShouldEqual, 2)
	test.That(t, len(base.right), test.ShouldEqual, 2)
	test.That(t, len(base.allMotors), test.ShouldEqual, 4)
}

func TestValidate(t *testing.T) {
	cfg := &AttrConfig{}
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

func TestAngleCalculations(t *testing.T) {
	for _, tc := range []struct {
		Condition string
		Added     float64
		Current   float64
		Target    float64
		Over      float64 // Tests check against an overshoot of 10 degrees
		Dir       float64
	}{
		/*
			quadrants
			q2	 	|		q1	  <-| ccw (+ve)
				+ve	|  +ve			|
			________|________
					|
				-ve	|  -ve			|
			q3		|		q4	  <-| cw (-ve)
		*/

		// acute angle additions
		// eight possibilities counterclockwise
		{"acute-CCW-quadrant1-to-quadrant1", 10, 10, 20, 35, 1},
		{"acute-CCW-quadrant1-to-quadrant2", 10, 85, 95, 110, 1},
		{"acute-CCW-quadrant2-to-quadrant2", 10, 95, 105, 120, 1},
		{"acute-CCW-quadrant2-to-quadrant3", 10, 175, -175, -160, 1},
		{"acute-CCW-quadrant3-to-quadrant3", 10, -105, -95, -80, 1},
		{"acute-CCW-quadrant4-to-quadrant4", 10, -35, -25, -10, 1},
		{"acute-CCW-quadrant4-to-quadrant1", 10, -15, -5, 10, 1},

		// eight possibilities clockwise
		{"acute-CW-quadrant1-to-quadrant1", -20, 40, 20, 5, -1},
		{"acute-CW-quadrant1-to-quadrant4", -20, 10, -10, -25, -1},
		{"acute-CW-quadrant4-to-quadrant4", -20, -10, -30, -45, -1},
		{"acute-CW-quadrant4-to-quadrant3", -20, -80, -100, -115, -1},
		{"acute-CW-quadrant3-to-quadrant3", -20, -100, -120, -135, -1},
		{"acute-CW-quadrant3-to-quadrant2", -20, -170, 170, 155, -1},
		{"acute-CW-quadrant2-to-quadrant1", -20, 100, 80, 65, -1},

		// obtuse angle additions,
		// eight possibilities counterclockwise
		{"obtuse-CCW-quadrant1-to-quadrant3", 110, 80, -170, -155, 1},
		{"obtuse-CCW-quadrant1-to-quadrant2", 110, 10, 120, 135, 1},
		{"obtuse-CCW-quadrant2-to-quadrant3", 110, 95, -155, -140, 1},
		{"obtuse-CCW-quadrant2-to-quadrant4", 110, 170, -80, -65, 1},
		{"obtuse-CCW-quadrant3-to-quadrant1", 110, -80, 30, 45, 1},
		{"obtuse-CCW-quadrant3-to-quadrant4", 110, -170, -60, -45, 1},
		{"obtuse-CCW-quadrant4-to-quadrant2", 110, -10, 100, 115, 1},
		{"obtuse-CCW-quadrant4-to-quadrant1", 110, -80, 30, 45, 1},

		// eight possibilities clockwise
		{"obtuse-CW-quadrant1-to-quadrant4", -110, 80, -30, -45, -1},
		{"obtuse-CW-quadrant1-to-quadrant3", -110, 10, -100, -115, -1},
		{"obtuse-CW-quadrant2-to-quadrant1", -110, 170, 60, 45, -1},
		{"obtuse-CW-quadrant2-to-quadrant4", -110, 95, -15, -30, -1},
		{"obtuse-CW-quadrant3-to-quadrant2", -110, -150, 100, 85, -1},
		{"obtuse-CW-quadrant4-to-quadrant2", -110, -80, 170, 155, -1},
		{"obtuse-CW-quadrant4-to-quadrant3", -110, -10, -120, -135, -1},
		{"obtuse-CW-quadrant3-to-quadrant1", -110, -170, 80, 65, -1},

		// reflex angle additions,
		// eight possibilities counterclockwise
		{"reflex-CCW-quadrant1-to-quadrant4", 200, 80, -80, -65, 1},
		{"reflex-CCW-quadrant1-to-quadrant3", 200, 10, -150, -135, 1},
		{"reflex-CCW-quadrant2-to-quadrant3", 200, 95, -65, -50, 1},
		{"reflex-CCW-quadrant2-to-quadrant1", 200, 170, 10, 25, 1},
		{"reflex-CCW-quadrant3-to-quadrant2", 200, -80, 120, 135, 1},
		{"reflex-CCW-quadrant3-to-quadrant1", 200, -170, 30, 45, 1},
		{"reflex-CCW-quadrant4-to-quadrant2", 200, -10, -170, -155, 1},
		{"reflex-CCW-quadrant4-to-quadrant1", 200, -80, 120, 135, 1},

		// eight possibilities clockwise
		{"reflex-CW-quadrant1-to-quadrant2", -200, 10, 170, 155, -1},
		{"reflex-CW-quadrant1-to-quadrant3", -200, 80, -120, -135, -1},
		{"reflex-CW-quadrant2-to-quadrant3", -200, 100, -100, -115, -1},
		{"reflex-CW-quadrant2-to-quadrant4", -200, 170, -30, -45, -1},
		{"reflex-CW-quadrant3-to-quadrant2", -200, -100, 60, 45, -1},
		{"reflex-CW-quadrant3-to-quadrant4", -200, -170, -10, -25, -1},
		{"reflex-CW-quadrant4-to-quadrant1", -200, -80, 80, 65, -1},
		{"reflex-CW-quadrant4-to-quadrant3", -200, -10, 150, 135, -1},

		// quadrant boundary cases
		{"ninetey-CCW-quadrant1-to-quadrant2", 90, 0, 90, 105, 1},
		{"ninetey-CCW-quadrant2-to-quadrant3", 90, 90, 179.9, -165.1, 1},
		{"ninetey-CCW-quadrant3-to-quadrant4", 90, 180, -90, -75, 1},
		{"ninetey-CCW-quadrant4-to-quadrant1", 90, -90, 0, 15, 1},
		{"ninetey-CW-quadrant1-to-quadrant4", -90, 0, -90, -105, -1},
		{"ninetey-CW-quadrant2-to-quadrant1", -90, 90, 0, -15, -1},
		{"ninetey-CW-quadrant3-to-quadrant2", -90, 180, 90, 75, -1},
		{"ninetey-CW-quadrant4-to-quadrant3", -90, -90, 179.9, 164.9, -1},
		{"oneeighty-CCW-zero-to-oneeighty", 180, 0, 179.9, -165.1, 1},
		{"oneeighty-CCW-quadrant1-to-quadrant3", 180, 10, -170, -155, 1},
		{"oneeighty-CCW-quadrant2-to-quadrant4", 180, 90, -90, -75, 1},
		{"oneeighty-CW-quadrant3-to-quadrant2", -10, -170, 179.9, 164.9, -1},
	} {
		t.Run(tc.Condition, func(t *testing.T) {
			t.Run(tc.Condition+" calculation", func(t *testing.T) {
				target := addAnglesInDomain(tc.Added, tc.Current)
				test.That(t, target, test.ShouldAlmostEqual, tc.Target)
				overshoot := addAnglesInDomain(tc.Target, tc.Dir*allowableAngle)
				test.That(t, overshoot, test.ShouldAlmostEqual, tc.Over)
			})
		})
		t.Run(tc.Condition+" overshoot", func(t *testing.T) {
			start := tc.Current
			target := tc.Target
			dir := tc.Dir

			over := tc.Over
			test.That(t,
				hasBaseOvershot(over, target, start, dir),
				test.ShouldBeTrue)

			notover := addAnglesInDomain(target, -1*dir*1)
			test.That(t,
				hasBaseOvershot(notover, target, start, dir),
				test.ShouldBeFalse)
		})
	}
}

type added struct {
	AngleType string // acute, ninety, obtuse, oneeighty, reflex twoseventy, twoseventyplus, threesixty
	Added     float64
	Over      float64
}

type start struct {
	Quadrant  string // quadrant 1, quadrant2, quadrant3 quadrant4
	Direction string
	Value     float64
}
type TestCase struct {
	Condition string
	Start     float64
	Target    float64
	Over      float64
	Direction float64
}

func TestAngleCalculations2(t *testing.T) {

}

var angleTypes = map[string]float64{
	"acute":          20,
	"ninety":         90,
	"obtuse":         110,
	"oneeighty":      180,
	"reflex":         200,
	"twoseventy":     270,
	"twoseventyplus": 325,
	"threesixty":     360}
var dirs = map[string]float64{"cw": -1, "ccw": 1}
