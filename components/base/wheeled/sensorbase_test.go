package wheeled

import (
	"context"
	"math"
	"strconv"
	"sync"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
)

func TestSpinWithMSMath(t *testing.T) {
	// tests getTurnState, if we're atTarget, overshot or travelled the minimum amount
	for _, stops := range []struct {
		name     string
		curr     float64
		start    float64
		target   float64
		dir      float64
		angleDeg float64
		atTarget [3]bool
	}{
		{"one turn near start", 1.0, 0, 10, 1, 10, [3]bool{false, false, false}},
		{"one turn near end", 10, 0, 10, 1, 10, [3]bool{true, true, true}},
		{"three turns near end", 719, 10, 730, 1, 720, [3]bool{false, false, true}},
		{"three turns over end", 745, 10, 730, 1, 720, [3]bool{false, true, true}},
	} {
		t.Run(stops.name, func(t *testing.T) {
			at, over, min := getTurnState(
				stops.curr, stops.start, stops.target, stops.dir, stops.angleDeg, errTarget)
			test.That(t, at, test.ShouldEqual, stops.atTarget[0])
			test.That(t, over, test.ShouldEqual, stops.atTarget[1])
			test.That(t, min, test.ShouldEqual, stops.atTarget[2])
		})
	}

	// test angleBewteen calculations
	for _, bound := range []struct {
		name   string
		angle  float64
		bound1 float64
		bound2 float64
	}{
		{"in +ve", 15, 0, 270},
		{"in cross", 0, -15, 15},
		{"in zero", 0, -15, 15},
		{"in -ve", -15, -270, 0},
	} {
		t.Run(bound.name, func(t *testing.T) {
			test.That(t, angleBetween(bound.angle, bound.bound1, bound.bound2), test.ShouldBeTrue)
		})
	}

	// test addAnglesInDomain calculation
	for _, add := range []struct {
		name     string
		angle1   float64
		angle2   float64
		expected float64
	}{
		{"+ve", 15, 15, 30},
		{"+ve ccw cross zero", 15, -30, 345},
		{"+ve ccw", 110, 120, 230},
		{"+ve ccw cross zero", 50, 350, 40},
		{"-ve cw cross zero", -90, 0, 270},
		{"-ve cw cross zero", -60, 0, 300},
	} {
		t.Run(add.name, func(t *testing.T) {
			test.That(t, addAnglesInDomain(add.angle1, add.angle2), test.ShouldEqual, add.expected)
		})
	}

	// test getCurrentYaw
	ctx := context.Background()

	yaws := []float64{
		math.Pi / 18, math.Pi / 3, math.Pi / 9, math.Pi / 6, math.Pi / 3, -math.Pi, -3 * math.Pi / 4,
	}

	for _, yaw := range yaws {
		ms := &inject.MovementSensor{
			OrientationFunc: func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
				return &spatialmath.EulerAngles{Yaw: yaw}, nil
			},
		}

		ori, err := ms.Orientation(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		calcYaw := addAnglesInDomain(rdkutils.RadToDeg(ori.EulerAngles().Yaw), 0)
		measYaw, err := getCurrentYaw(ctx, ms)
		test.That(t, measYaw, test.ShouldEqual, calcYaw)
		test.That(t, measYaw > 0, test.ShouldBeTrue)
		test.That(t, calcYaw > 0, test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)
	}

	// test findSpin Parameters calculations
	for _, params := range []struct {
		name  string
		added float64
		speed float64
		start float64
		dir   float64
		goal  float64
		turns int
	}{
		{"angle- speed- start0", -15, -20, 0, 1, 0, 0},
		{"angle+ speed- start360 turn+1", 365, 20, 0, 1, 0, 1},
		{"angle+ speed+ start10", 90, 10, 10, 1, 0, 0},
	} {
		t.Run(params.name, func(t *testing.T) {
			params.goal = addAnglesInDomain(params.start, params.added)
			goal, dir, turns := findSpinParams(params.added, params.speed, params.start)
			test.That(t, goal, test.ShouldAlmostEqual, params.goal)
			test.That(t, dir, test.ShouldAlmostEqual, params.dir)
			test.That(t, turns, test.ShouldAlmostEqual, params.turns)
		})
	}
}

func TestHasOverShot(t *testing.T) {
	/*
			definition of quadrants and directions
			q2	 	|		q1	  <-| ccw (+ve)
				+ve	|  +ve			|
		180 ________|________ 0
					|
				-ve	|  -ve			|
			q3		|		q4	  <-| cw (-ve)
	*/

	a2Str := func(number float64) string {
		return strconv.FormatFloat(number, 'f', 1, 64)
	}

	const (
		q1 = "q1"
		q2 = "q2"
		q3 = "q3"
		q4 = "q4"
	)

	findQuadrant := func(value float64) string {
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

	for _, dirCase := range []struct {
		name  string
		value float64
	}{
		{"ccw", 1.0},
		{"cw", -1},
	} {
		for _, addCase := range []struct {
			angleType string
			value     float64
		}{
			{"acute", 3},
			{"acute", 20},
			{"right", 90},
			{"obtuse", 110},
			{"straight", 180},
			{"reflex", 200},
			{"reflexright", 270},
			{"reflexplus", 345},
			{"complete", 357},
		} {
			for _, start := range []float64{
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
			} {
				start := start
				target := addAnglesInDomain(start, dirCase.value*addCase.value)
				dir := dirCase.value
				added := addCase.value

				sQ := findQuadrant(start)
				tQ := findQuadrant(target)
				behaviour := sQ + "-to-" + tQ + "-" + dirCase.name + "-" + addCase.angleType
				s2t := "(" + a2Str(start) + "->" + a2Str(target) + ")"
				condition := behaviour + s2t

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
					t.Run(condition+noStr+key, func(t *testing.T) {
						if key == "end:" || key == "over:" {
							// skipped edge case
							if key == "end:" && dirCase.name == "cw" && target == 0.0 {
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

func TestSpinWithMovementSensor(t *testing.T) {
	m := inject.Motor{
		GoForFunc: func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
			return nil
		},
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error {
			return nil
		},
	}
	ms := &inject.MovementSensor{
		OrientationFunc: func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
			return &spatialmath.EulerAngles{Yaw: 1}, nil
		},
	}

	logger := golog.NewDebugLogger("loggie")

	ctx := context.Background()
	sensorCtx, sensorCancel := context.WithCancel(ctx)
	base := wheeledBase{
		Unimplemented:        generic.Unimplemented{},
		widthMm:              1,
		wheelCircumferenceMm: 1,
		spinSlipFactor:       0,
		left:                 []motor.Motor{&m},
		right:                []motor.Motor{&m},
		allMotors:            []motor.Motor{&m},

		opMgr:             operation.SingleOperationManager{},
		logger:            logger,
		name:              "basie",
		collisionGeometry: nil,
	}

	sensorBase := &sensorBase{
		Unimplemented: generic.Unimplemented{},
		base:          &base,
		sensorMu:      sync.Mutex{},
		sensorDone:    sensorCancel,
		allSensors:    []movementsensor.MovementSensor{ms},
		orientation:   ms,
	}

	err := sensorBase.stopSpinWithSensor(sensorCtx, 10, 50)
	test.That(t, err, test.ShouldBeNil)
	// we have no way of stopping the sensor in this little test
	// so we stop runnign goroutines manually and test our function
	// sensorBase.stopSensors()
	sensorBase.setPolling(false)
	sensorBase.sensorDone()
}

var sConfig config.Component = config.Component{
	Name:  "test",
	Type:  base.Subtype.ResourceSubtype,
	Model: resource.Model{Name: "wheeled_base"},
	ConvertedAttributes: &Config{
		WidthMM:              100,
		WheelCircumferenceMM: 1000,
		Left:                 []string{"fl-m", "bl-m"},
		Right:                []string{"fr-m", "br-m"},
		MovementSensor:       []string{"ms"},
	},
}

func TestSensorBase(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	deps, err := testCfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	motorDeps := fakeMotorDependencies(t, deps)
	wheeled, err := createWheeledBase(context.Background(), motorDeps, testCfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, wheeled, test.ShouldNotBeNil)

	msDeps := make(registry.Dependencies)
	msDeps[movementsensor.Named("ms")] = &inject.MovementSensor{
		PropertiesFuncExtraCap: map[string]interface{}{},
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
			return &movementsensor.Properties{OrientationSupported: true}, nil
		},
	}
	sensorBase, err := makeBaseWithSensors(ctx, wheeled, msDeps, sConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sensorBase, test.ShouldNotBeNil)
}
