package motionplan

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/motionplan/tpspace"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

func TestFixOvIncrement(t *testing.T) {
	pos1 := commonpb.Pose{
		X:     -66,
		Y:     -133,
		Z:     372,
		Theta: 15,
		OX:    0,
		OY:    1,
		OZ:    0,
	}
	pos2 := commonpb.Pose{
		X:     pos1.X,
		Y:     pos1.Y,
		Z:     pos1.Z,
		Theta: pos1.Theta,
		OX:    pos1.OX,
		OY:    pos1.OY,
		OZ:    pos1.OZ,
	}

	// Increment, but we're not pointing at Z axis, so should do nothing
	pos2.OX = -0.1
	outpos := fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos, test.ShouldResemble, spatialmath.NewPoseFromProtobuf(&pos2))

	// point at positive Z axis, decrement OX, should subtract 180
	pos1.OZ = 1
	pos2.OZ = 1
	pos1.OY = 0
	pos2.OY = 0
	outpos = fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos.Orientation().OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, -165)

	// Spatial translation is incremented, should do nothing
	pos2.X -= 0.1
	outpos = fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos, test.ShouldResemble, spatialmath.NewPoseFromProtobuf(&pos2))

	// Point at -Z, increment OY
	pos2.X += 0.1
	pos2.OX += 0.1
	pos1.OZ = -1
	pos2.OZ = -1
	pos2.OY = 0.1
	outpos = fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos.Orientation().OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 105)

	// OX and OY are both incremented, should do nothing
	pos2.OX += 0.1
	outpos = fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos, test.ShouldResemble, spatialmath.NewPoseFromProtobuf(&pos2))
}

func TestEvaluate(t *testing.T) {
	plan := Plan{
		map[string][]frame.Input{"": {{1.}, {2.}, {3.}}},
	}
	score := plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Inf(1))

	// Test no change
	plan = append(plan, map[string][]frame.Input{"": {{1.}, {2.}, {3.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, 0)

	// Test L2 for "", and nothing for plan with only one entry
	plan = append(plan, map[string][]frame.Input{"": {{4.}, {5.}, {6.}}, "test": {{2.}, {3.}, {4.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Sqrt(27))

	// Test cumulative L2 after returning to original inputs
	plan = append(plan, map[string][]frame.Input{"": {{1.}, {2.}, {3.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Sqrt(27)*2)

	// Test that the "test" inputs are properly evaluated after skipping a step
	plan = append(plan, map[string][]frame.Input{"test": {{3.}, {5.}, {6.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Sqrt(27)*2+3)

	// Evaluated with the tp-space metric, should be the sum of the distance values (third input) ignoring the first input set for each
	// named input set
	score = plan.Evaluate(tpspace.PTGSegmentMetric)
	test.That(t, score, test.ShouldEqual, 18)
}

func TestPlanToPlanStepsAndGeoPoses(t *testing.T) {
	t.Skip() // TODO: ordering of PTGs should not be assumed to be stable, and so hardcoding inputs like this is not guaranteed to work.
	logger := logging.NewTestLogger(t)
	sphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "base")
	test.That(t, err, test.ShouldBeNil)
	baseName := base.Named("myBase")
	kinematicFrame, err := tpspace.NewPTGFrameFromKinematicOptions(
		"itsabase",
		logger,
		200, 60, 0, 1000,
		2,
		[]spatialmath.Geometry{sphere},
		false,
	)
	test.That(t, err, test.ShouldBeNil)
	goal := spatialmath.NewPoseFromPoint(r3.Vector{X: 1000, Y: 8000, Z: 0})
	baseFS := frame.NewEmptyFrameSystem("baseFS")
	err = baseFS.AddFrame(kinematicFrame, baseFS.World())
	test.That(t, err, test.ShouldBeNil)
	planRequest := &PlanRequest{
		Logger:             logger,
		Goal:               frame.NewPoseInFrame(frame.World, goal),
		Frame:              kinematicFrame,
		FrameSystem:        baseFS,
		StartConfiguration: frame.StartPositions(baseFS),
		Options:            map[string]interface{}{"smooth_iter": 0},
	}
	plan := Plan{
		map[string][]frame.Input{
			"itsabase": {{0}, {0}, {0}},
			"world":    {},
		},
		map[string][]frame.Input{
			"itsabase": {{5}, {-0.8480042879579467}, {999.9999726352631}},
			"world":    {},
		},
		map[string][]frame.Input{
			"itsabase": {{5}, {-0.8490767023454777}, {999.9999726346474}},
			"world":    {},
		},
		map[string][]frame.Input{
			"itsabase": {{1}, {-0.21209039180842176}, {1000}},
			"world":    {},
		},
		map[string][]frame.Input{
			"itsabase": {{0}, {0.00044190456001478516}, {1000}},
			"world":    {},
		},
		map[string][]frame.Input{
			"itsabase": {{1}, {-0.9318038198426468}, {28.154612436699026}},
			"world":    {},
		},
		map[string][]frame.Input{
			"itsabase": {{1}, {2.8385178994421265}, {27.694382041674338}},
			"world":    {},
		},
		map[string][]frame.Input{
			"itsabase": {{6}, {0.11216682798779984}, {999.9999973004774}},
			"world":    {},
		},
		map[string][]frame.Input{
			"itsabase": {{6}, {0.011555036037605201}, {999.999997300387}},
			"world":    {},
		},
		map[string][]frame.Input{
			"itsabase": {{0}, {-0.15509986951180685}, {527.4237036450636}},
			"world":    {},
		},
		map[string][]frame.Input{
			"itsabase": {{0}, {0.3104320634486429}, {406.313687289847}},
			"world":    {},
		},
		map[string][]frame.Input{
			"itsabase": {{6}, {0.1619719027689384}, {835.8675981179792}},
			"world":    {},
		},
		map[string][]frame.Input{
			"itsabase": {{6}, {-0.16177935464497809}, {280.3182867874354}},
			"world":    {},
		},
		map[string][]frame.Input{
			"itsabase": {{0}, {0}, {0}},
			"world":    {},
		},
	}

	expectedPBPoses := []commonpb.Pose{
		{X: 0, Y: 0, Z: 0, OX: 0, OY: 0, OZ: 1, Theta: 0},
		{X: 33.823716950666906, Y: 995.1906975055888, Z: 0, OX: 0, OY: 0, OZ: 1, Theta: 0},
		{X: 67.73174762780747, Y: 1990.363235889872, Z: 0, OX: 0, OY: 0, OZ: 1, Theta: 0},
		{X: 242.64104911326686, Y: 2969.667800846239, Z: 0, OX: 0, OY: 0, OZ: 1, Theta: -20.25314054316429},
		{X: 588.3947613604477, Y: 3907.992630880257, Z: 0, OX: 0, OY: 0, OZ: 1, Theta: -20.22782127692785},
		{X: 598.7036921283044, Y: 3934.1896129113657, Z: 0, OX: 0, OY: 0, OZ: 1.0000000000000002, Theta: -22.733038691228344},
		{X: 607.7043545069445, Y: 3960.359640622657, Z: 0, OX: 0, OY: 0, OZ: 0.9999999999999997, Theta: -15.226241234582744},
		{X: 896.0541805207657, Y: 4917.846783458451, Z: 0, OX: 0, OY: 0, OZ: 1.0000000000000002, Theta: -16.832912695850446},
		{X: 1188.3866528623023, Y: 5874.1634878309915, Z: 0, OX: 0, OY: 0, OZ: 1.0000000000000004, Theta: -16.998426395119534},
		{X: 1418.5742933393453, Y: 6348.532364022327, Z: 0, OX: 0, OY: 0, OZ: 0.9999999999999999, Theta: -25.884994321175864},
		{X: 1475.7704606006394, Y: 6750.48664214989, Z: 0, OX: 0, OY: 0, OZ: 1, Theta: -8.098547260031237},
		{X: 1624.2929955269274, Y: 7572.94373132755, Z: 0, OX: 0, OY: 0, OZ: 1.0000000000000002, Theta: -10.418623867122113},
		{X: 1666.406401015521, Y: 7849.98041000934, Z: 0, OX: 0, OY: 0, OZ: 1.0000000000000002, Theta: -8.10130530874526},
		{X: 1666.406401015521, Y: 7849.98041000934, Z: 0, OX: 0, OY: 0, OZ: 1.0000000000000002, Theta: -8.10130530874526},
	}
	expectedPoses := []spatialmath.Pose{}
	for i := range expectedPBPoses {
		expectedPoses = append(expectedPoses, spatialmath.NewPoseFromProtobuf(&expectedPBPoses[i]))
	}

	type testCase struct {
		msg         string
		origin      spatialmath.GeoPose
		expectedGPs []spatialmath.GeoPose
	}

	//nolint:dupl
	tcs := []testCase{
		{
			msg:    "null island origin & north heading",
			origin: *spatialmath.NewGeoPose(geo.NewPoint(0, 0), 0),
			expectedGPs: []spatialmath.GeoPose{
				*spatialmath.NewGeoPose(geo.NewPoint(0, 0), 0),
				*spatialmath.NewGeoPose(geo.NewPoint(8.949964962761077e-06, 3.0418397506967887e-07), 0),
				*spatialmath.NewGeoPose(geo.NewPoint(1.7899766616620807e-05, 6.091261943754186e-07), 0),
				*spatialmath.NewGeoPose(geo.NewPoint(2.670686415702184e-05, 2.1821234034117234e-06), 20.253140543164307),
				*spatialmath.NewGeoPose(geo.NewPoint(3.514542208721792e-05, 5.291561187530965e-06), 20.22782127692784),
				*spatialmath.NewGeoPose(geo.NewPoint(3.538101720672235e-05, 5.384271659385257e-06), 22.73303869122833),
				*spatialmath.NewGeoPose(geo.NewPoint(3.561636992020488e-05, 5.465216521584129e-06), 15.226241234582744),
				*spatialmath.NewGeoPose(geo.NewPoint(4.4227258669621044e-05, 8.058408857357672e-06), 16.83291269585044),
				*spatialmath.NewGeoPose(geo.NewPoint(5.282762141305311e-05, 1.0687417902843484e-05), 16.99842639511951),
				*spatialmath.NewGeoPose(geo.NewPoint(5.7093723208395436e-05, 1.275754507036777e-05), 25.88499432117584),
				*spatialmath.NewGeoPose(geo.NewPoint(6.070858487751126e-05, 1.3271922591671265e-05), 8.09854726003124),
				*spatialmath.NewGeoPose(geo.NewPoint(6.810511917989601e-05, 1.4607617852590454e-05), 10.418623867122108),
				*spatialmath.NewGeoPose(geo.NewPoint(7.059656988760095e-05, 1.498635280806064e-05), 8.101305308745282),
				*spatialmath.NewGeoPose(geo.NewPoint(7.059656988760095e-05, 1.498635280806064e-05), 8.101305308745282),
			},
		},
		{
			msg:    "null island origin & east heading",
			origin: *spatialmath.NewGeoPose(geo.NewPoint(0, 0), 90),
			expectedGPs: []spatialmath.GeoPose{
				*spatialmath.NewGeoPose(geo.NewPoint(0, 0), 90.),
				*spatialmath.NewGeoPose(geo.NewPoint(-3.0418399446214297e-07, 8.949964948781307e-06), 90.),
				*spatialmath.NewGeoPose(geo.NewPoint(-6.091262404832106e-07, 1.7899766646051922e-05), 90.),
				*spatialmath.NewGeoPose(geo.NewPoint(-2.1821233795034082e-06, 2.670686413187438e-05), 110.2531405431643),
				*spatialmath.NewGeoPose(geo.NewPoint(-5.291561217008132e-06, 3.514542204475805e-05), 110.22782127692784),
				*spatialmath.NewGeoPose(geo.NewPoint(-5.384271658742659e-06, 3.538101718001091e-05), 112.73303869122833),
				*spatialmath.NewGeoPose(geo.NewPoint(-5.465216560189554e-06, 3.561636988066373e-05), 105.22624123458274),
				*spatialmath.NewGeoPose(geo.NewPoint(-8.058408846160474e-06, 4.4227258693742326e-05), 106.83291269585044),
				*spatialmath.NewGeoPose(geo.NewPoint(-1.0687417931043593e-05, 5.282762144838041e-05), 106.99842639511951),
				*spatialmath.NewGeoPose(geo.NewPoint(-1.2757545116007572e-05, 5.709372325862088e-05), 115.88499432117584),
				*spatialmath.NewGeoPose(geo.NewPoint(-1.3271922605945433e-05, 6.0708584870059856e-05), 98.09854726003124),
				*spatialmath.NewGeoPose(geo.NewPoint(-1.4607617852194795e-05, 6.810511917864972e-05), 100.41862386712211),
				*spatialmath.NewGeoPose(geo.NewPoint(-1.498635280674151e-05, 7.059656989571761e-05), 98.10130530874528),
				*spatialmath.NewGeoPose(geo.NewPoint(-1.498635280674151e-05, 7.059656989571761e-05), 98.10130530874528),
			},
		},
		{
			msg:    "central park origin & west heading",
			origin: *spatialmath.NewGeoPose(geo.NewPoint(40.770190, -73.977192), 270),
			expectedGPs: []spatialmath.GeoPose{
				*spatialmath.NewGeoPose(geo.NewPoint(40.77019, -73.97719199999997), 270),
				*spatialmath.NewGeoPose(geo.NewPoint(40.770190304183394, -73.97720381771077), 270),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77019060912384, -73.97721563520601), 270),
				*spatialmath.NewGeoPose(geo.NewPoint(40.770192182118016, -73.97722726427267), 290.2531405431643),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77019529155192, -73.97723840671397), 290.22782127692784),
				*spatialmath.NewGeoPose(geo.NewPoint(40.770195384262244, -73.97723871779857), 292.73303869122833),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77019546520702, -73.97723902856299), 285.22624123458274),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77019805839413, -73.97725039855398), 286.83291269585044),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77020068739694, -73.97726175464722), 286.9984263951195),
				*spatialmath.NewGeoPose(geo.NewPoint(40.770202757520586, -73.97726738769566), 295.88499432117584),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77020327189487, -73.97727216083203), 278.09854726003124),
				*spatialmath.NewGeoPose(geo.NewPoint(40.770204607582954, -73.97728192736587), 280.4186238671221),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77020498631531, -73.97728521712797), 278.1013053087453),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77020498631531, -73.97728521712797), 278.1013053087453),
			},
		},
		{
			msg:    "central park origin & south heading",
			origin: *spatialmath.NewGeoPose(geo.NewPoint(40.770190, -73.977192), 180),
			expectedGPs: []spatialmath.GeoPose{
				*spatialmath.NewGeoPose(geo.NewPoint(40.77019, -73.97719199999997), 180),
				*spatialmath.NewGeoPose(geo.NewPoint(40.770181050035035, -73.97719240165048), 180),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77017210023339, -73.97719280430209), 180),
				*spatialmath.NewGeoPose(geo.NewPoint(40.770163293135816, -73.97719488131769), 200.2531405431643),
				*spatialmath.NewGeoPose(geo.NewPoint(40.7701548545777, -73.97719898707842), 200.22782127692784),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77015461898258, -73.97719910949496), 202.73303869122833),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77015438362986, -73.97719921637616), 195.22624123458274),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77014577274085, -73.97720264047533), 196.83291269585044),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77013717237774, -73.97720611186675), 196.9984263951195),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77013290627557, -73.97720884530038), 205.88499432117584),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77012929141379, -73.97720952449302), 188.09854726003124),
				*spatialmath.NewGeoPose(geo.NewPoint(40.77012189487922, -73.97721128816768), 190.4186238671221),
				*spatialmath.NewGeoPose(geo.NewPoint(40.770119403428424, -73.97721178825562), 188.10130530874528),
				*spatialmath.NewGeoPose(geo.NewPoint(40.770119403428424, -73.97721178825562), 188.10130530874528),
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.msg, func(t *testing.T) {
			ps, gps, err := PlanToPlanStepsAndGeoPoses(plan, baseName, tc.origin, *planRequest)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(ps), test.ShouldEqual, len(expectedPoses))
			for i, step := range ps {
				if len(step) != 1 {
					t.Logf("iteration %d expected steps with a single component\n", i)
					t.FailNow()
				}

				pose, ok := step[baseName]
				if !ok {
					t.Logf("iteration %d expected component %s in the step\n", i, baseName.Name)
					t.FailNow()
				}

				test.That(t, spatialmath.PoseAlmostEqual(pose, expectedPoses[i]), test.ShouldBeTrue)
			}

			test.That(t, len(gps), test.ShouldEqual, len(tc.expectedGPs))
			for i, gp := range gps {
				test.That(t, gp.Location().Lat(), test.ShouldAlmostEqual, tc.expectedGPs[i].Location().Lat())
				test.That(t, gp.Location().Lng(), test.ShouldAlmostEqual, tc.expectedGPs[i].Location().Lng())
				test.That(t, gp.Heading(), test.ShouldAlmostEqual, tc.expectedGPs[i].Heading())
			}
		})
	}
}
