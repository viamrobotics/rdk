package wheeled

import (
	"context"
	"math"
	"strconv"
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
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
)

var testCfg config.Component = config.Component{
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
	deps, err := testCfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	motorDeps := fakeMotorDependencies(t, deps)

	baseBase, err := createWheeledBase(context.Background(), motorDeps, testCfg, logger)
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
	deps, err := testCfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	motorDeps := fakeMotorDependencies(t, deps)

	baseBase, err := createWheeledBase(ctx, motorDeps, testCfg, logger)
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

func TestSpinWithMSMath(t *testing.T) {
	// tests getTurnState, if we're atTarget, overshot or travelled the minimum amount
	for _, stops := range []struct {
		curr     float64
		start    float64
		target   float64
		dir      float64
		angleDeg float64
		atTarget [3]bool
	}{
		{1.0, 0, 10, 1, 10, [3]bool{false, false, false}},
		{10, 0, 10, 1, 10, [3]bool{true, true, true}},
		{719, 10, 730, 1, 720, [3]bool{false, false, true}},
		{745, 10, 730, 1, 720, [3]bool{false, true, true}},
	} {
		at, over, min := getTurnState(
			stops.curr, stops.start, stops.target, stops.dir, stops.angleDeg)
		test.That(t, at, test.ShouldEqual, stops.atTarget[0])
		test.That(t, over, test.ShouldEqual, stops.atTarget[1])
		test.That(t, min, test.ShouldEqual, stops.atTarget[2])
	}

	// test angleBewteen calculations
	for _, bound := range []struct {
		angle  float64
		bound1 float64
		bound2 float64
	}{
		{15, 0, 270},
		{0, -15, 15},
		{0, -15, 15},
		{-15, -270, 0},
	} {
		test.That(t, angleBetween(bound.angle, bound.bound1, bound.bound2), test.ShouldBeTrue)
	}

	// test addAnglesInDomain calculation
	for _, add := range []struct {
		angle1   float64
		angle2   float64
		expected float64
	}{
		{15, 15, 30},
		{15, -30, 345},
		{110, 120, 230},
		{50, 350, 40},
		{-90, 0, 270},
		{-60, 0, 300},
	} {
		test.That(t, addAnglesInDomain(add.angle1, add.angle2), test.ShouldEqual, add.expected)
	}

	// test getCurrentYaw
	ctx := context.Background()
	ms := &inject.MovementSensor{
		OrientationFunc: func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
			if extra != nil {
				if yaw, ok := extra["yaw"].(float64); ok {
					return &spatialmath.EulerAngles{Yaw: yaw}, nil
				}
			}
			return &spatialmath.EulerAngles{}, nil
		},
	}
	extra := make(map[string]interface{})
	yaws := []float64{
		math.Pi / 18, math.Pi / 3, math.Pi / 9, math.Pi / 6, math.Pi / 3, -math.Pi, -3 * math.Pi / 4,
	}
	for _, yaw := range yaws {
		extra["yaw"] = yaw
		calcYaw := addAnglesInDomain(rdkutils.RadToDeg(yaw), 0)
		measYaw, err := getCurrentYaw(ctx, ms, extra)
		test.That(t, measYaw, test.ShouldEqual, calcYaw)
		test.That(t, measYaw > 0, test.ShouldBeTrue)
		test.That(t, calcYaw > 0, test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)
	}

	// test findSpin Parameters calculations
	for _, params := range []struct {
		added float64
		speed float64
		start float64
		dir   float64
		goal  float64
	}{
		{-15, -20, 0, 1, 0},
		{20, -20, 360, -1, 0},
		{90, 10, 10, 1, 0},
	} {
		params.goal = addAnglesInDomain(params.start, params.added)
		goal, dir := findSpinParams(params.added, params.speed, params.start)
		test.That(t, goal, test.ShouldAlmostEqual, params.goal)
		test.That(t, dir, test.ShouldAlmostEqual, params.dir)
	}
}

func TestHasOverShot(t *testing.T) {
	dirCases := []dirInfo{
		{"ccw", 1.0},
		{"cw", -1},
	}

	addCases := []addInfo{
		{"acute", 3},
		{"acute", 20},
		{"right", 90},
		{"obtuse", 110},
		{"straight", 180},
		{"reflex", 200},
		{"reflexright", 270},
		{"reflexplus", 345},
		{"complete", 357},
	}

	startCases := []float64{
		// TODO RSDK- refine overshot cases, add 3 around 360 range
		5,
		12,
		15,
		45,
		90,
		135,
		180,
		225,
		270,
		315,
		260,
		270,
		345,
		355,
		359,
	}

	/*
			definition of quadrants and directions
			q2	 	|		q1	  <-| ccw (+ve)
				+ve	|  +ve			|
		180 ________|________ 0
					|
				-ve	|  -ve			|
			q3		|		q4	  <-| cw (-ve)
	*/

	for _, dirCase := range dirCases {
		for _, addCase := range addCases {
			for _, start := range startCases {
				condition := makeCondition(addCase, dirCase, start)

				start := condition.Start
				target := condition.Target
				dir := condition.Direction
				added := addCase.Value

				// test a few cases in range ensure were not falsely overshooting
				notovers := map[string]float64{
					"start":    addAnglesInDomain(start, 0),
					"under:+":  addAnglesInDomain(start, dir),
					"under:++": addAnglesInDomain(start, dir*added/2),
					"under:--": addAnglesInDomain(target, -dir*added/2),
					"under:-":  addAnglesInDomain(target, -dir),
					"end:":     addAnglesInDomain(target, 0),
					// TODO: RSDK- refine overshot cases, test end and cw failure
					"over:": addAnglesInDomain(target, dir),
				}
				for key, angle := range notovers {
					noStr := "[" + strconv.FormatFloat(angle, 'f', 1, 64) + "]"
					t.Run(condition.Name+noStr+key, func(t *testing.T) {
						if key == "end:" || key == "over:" {
							// skipped edge case
							if key == "end:" && dirCase.Name == "cw" && target == 0.0 {
								t.Skip()
							} else {
								test.That(t,
									hasOverShot(angle, start, target, dir),
									test.ShouldBeTrue)
							}
						} else {
							test.That(t,
								hasOverShot(angle, start, target, dir),
								test.ShouldBeFalse)
						}
					})
				}
			}
		}
	}
}

type TestCase struct {
	Name      string
	Start     float64
	Target    float64
	Over      float64
	Direction float64
}

func makeCondition(addI addInfo, dirI dirInfo, startI float64) TestCase {
	target := addAnglesInDomain(startI, dirI.Value*addI.Value)

	a2Str := func(number float64) string {
		return strconv.FormatFloat(number, 'f', 1, 64)
	}

	sQ := findQuadrant(startI)
	tQ := findQuadrant(target)
	behaviour := sQ + "-to-" + tQ + "-" + dirI.Name + "-" + addI.AngleType
	s2t := "(" + a2Str(startI) + "->" + a2Str(target) + ")"
	name := behaviour + s2t

	return TestCase{
		Name:      name,
		Start:     startI,
		Target:    target,
		Direction: dirI.Value,
	}
}

type dirInfo struct {
	Name  string
	Value float64
}
type addInfo struct {
	AngleType string
	Value     float64
}

const (
	q1 = "q1"
	q2 = "q2"
	q3 = "q3"
	q4 = "q4"
)

func findQuadrant(value float64) string {
	switch {
	case 0 <= value && value < 90:
		return q1
	case 90 <= value && value < 180:
		return q2
	case 180 <= value && value < 270:
		return q3
	case 270 <= value && value < 360:
		return q4
	default:
		return "none"
	}
}
