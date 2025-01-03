package motionplan

import (
	"context"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestEvaluateTrajectory(t *testing.T) {
	plan := Trajectory{
		referenceframe.FrameSystemInputs{"": {{1.}, {2.}, {3.}}},
		referenceframe.FrameSystemInputs{"": {{1.}, {2.}, {3.}}},
	}
	// Test no change
	score := plan.EvaluateCost(ik.FSConfigurationL2Distance)
	test.That(t, score, test.ShouldAlmostEqual, 0)

	// Test L2 for "", and nothing for plan with only one entry
	plan = append(plan, referenceframe.FrameSystemInputs{"": {{4.}, {5.}, {6.}}, "test": {{2.}, {3.}, {4.}}})
	score = plan.EvaluateCost(ik.FSConfigurationL2Distance)
	test.That(t, score, test.ShouldAlmostEqual, math.Sqrt(27))

	// Test cumulative L2 after returning to original inputs
	plan = append(plan, referenceframe.FrameSystemInputs{"": {{1.}, {2.}, {3.}}})
	score = plan.EvaluateCost(ik.FSConfigurationL2Distance)
	test.That(t, score, test.ShouldAlmostEqual, math.Sqrt(27)*2)

	// Test that the "test" inputs are properly evaluated after skipping a step
	plan = append(plan, referenceframe.FrameSystemInputs{"test": {{3.}, {5.}, {6.}}})
	score = plan.EvaluateCost(ik.FSConfigurationL2Distance)
	test.That(t, score, test.ShouldAlmostEqual, math.Sqrt(27)*2+3)

	// Evaluated with the tp-space metric, should be the sum of the distance values (third input) ignoring the first input step
	score = plan.EvaluateCost(tpspace.NewPTGDistanceMetric([]string{"", "test"}))
	test.That(t, score, test.ShouldAlmostEqual, 22)
}

func TestPlanStep(t *testing.T) {
	baseNameA := "my-base1"
	baseNameB := "my-base2"
	poseA := spatialmath.NewZeroPose()
	poseB := spatialmath.NewPose(r3.Vector{X: 100}, spatialmath.NewOrientationVector())

	protoAB := &pb.PlanStep{
		Step: map[string]*pb.ComponentState{
			baseNameA: {Pose: spatialmath.PoseToProtobuf(poseA)},
			baseNameB: {Pose: spatialmath.PoseToProtobuf(poseB)},
		},
	}
	stepAB := referenceframe.FrameSystemPoses{
		baseNameA: referenceframe.NewPoseInFrame(referenceframe.World, poseA),
		baseNameB: referenceframe.NewPoseInFrame(referenceframe.World, poseB),
	}

	t.Run("FrameSystemPosesFromProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       *pb.PlanStep
			result      referenceframe.FrameSystemPoses
			err         error
		}

		testCases := []testCase{
			{
				description: "nil pointer returns an error",
				input:       nil,
				result:      referenceframe.FrameSystemPoses{},
				err:         errors.New("received nil *pb.PlanStep"),
			},
			{
				description: "an empty *pb.PlanStep returns an empty referenceframe.FrameSystemPoses{}",
				input:       &pb.PlanStep{},
				result:      referenceframe.FrameSystemPoses{},
			},
			{
				description: "a full *pb.PlanStep returns an full referenceframe.FrameSystemPoses{}",
				input:       protoAB,
				result:      stepAB,
			},
		}
		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res, err := FrameSystemPosesFromProto(tc.input)
				if tc.err != nil {
					test.That(t, err, test.ShouldBeError, tc.err)
				} else {
					test.That(t, err, test.ShouldBeNil)
				}
				test.That(t, res, test.ShouldResemble, tc.result)
			})
		}
	})

	t.Run("FrameSystemPosesToProto()", func(t *testing.T) {
		type testCase struct {
			description string
			input       referenceframe.FrameSystemPoses
			result      *pb.PlanStep
		}

		testCases := []testCase{
			{
				description: "an nil referenceframe.FrameSystemPoses returns an empty *pb.PlanStep",
				input:       nil,
				result:      &pb.PlanStep{},
			},
			{
				description: "an empty referenceframe.FrameSystemPoses returns an empty *pb.PlanStep",
				input:       referenceframe.FrameSystemPoses{},
				result:      &pb.PlanStep{},
			},
			{
				description: "a full referenceframe.FrameSystemPoses{} returns an full *pb.PlanStep",
				input:       stepAB,
				result:      protoAB,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res := FrameSystemPosesToProto(tc.input)
				test.That(t, res, test.ShouldResemble, tc.result)
			})
		}
	})
}

func TestNewGeoPlan(t *testing.T) {
	sphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "base")
	test.That(t, err, test.ShouldBeNil)
	baseName := "myBase"
	geoms := []spatialmath.Geometry{sphere}
	kinematicFrame, err := tpspace.NewPTGFrameFromKinematicOptions(baseName, logger, 200./60., 2, geoms, false, true)
	test.That(t, err, test.ShouldBeNil)
	baseFS := referenceframe.NewEmptyFrameSystem("baseFS")
	err = baseFS.AddFrame(kinematicFrame, baseFS.World())
	test.That(t, err, test.ShouldBeNil)

	goal := spatialmath.NewPoseFromPoint(r3.Vector{X: 1000, Y: 8000, Z: 0})
	plan, err := Replan(context.Background(), &PlanRequest{
		Logger: logging.NewTestLogger(t),
		StartState: &PlanState{
			poses:         referenceframe.FrameSystemPoses{kinematicFrame.Name(): referenceframe.NewZeroPoseInFrame(referenceframe.World)},
			configuration: referenceframe.NewZeroInputs(baseFS),
		},
		Goals: []*PlanState{{
			poses: referenceframe.FrameSystemPoses{kinematicFrame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, goal)},
		}},
		FrameSystem: baseFS,
	}, nil, math.NaN())
	test.That(t, err, test.ShouldBeNil)

	// test Path gets constructed correctly
	test.That(t, len(plan.Path()), test.ShouldBeGreaterThan, 1)
	test.That(t, spatialmath.PoseAlmostEqual(plan.Path()[0][baseName].Pose(), spatialmath.NewZeroPose()), test.ShouldBeTrue)
	test.That(t, spatialmath.PoseAlmostCoincidentEps(plan.Path()[len(plan.Path())-1][baseName].Pose(), goal, 10), test.ShouldBeTrue)

	type testCase struct {
		name        string
		origin      *geo.Point
		expectedGPs []spatialmath.GeoPose
	}

	tcs := []testCase{
		{
			name:   "null island origin",
			origin: geo.NewPoint(0, 0),
			expectedGPs: []spatialmath.GeoPose{
				*spatialmath.NewGeoPose(geo.NewPoint(0, 0), 0),
				*spatialmath.NewGeoPose(geo.NewPoint(7.059656988760095e-05, 1.498635280806064e-05), 8.101305308745282),
			},
		},
		{
			name:   "NE USA origin",
			origin: geo.NewPoint(40, -74),
			expectedGPs: []spatialmath.GeoPose{
				*spatialmath.NewGeoPose(geo.NewPoint(40, -74), 0),
				*spatialmath.NewGeoPose(geo.NewPoint(40+7.059656988760095e-05, -74+1.498635280806064e-05), 278.1013053087453),
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// test Path gets converted to a GeoPlan correctly
			gps := NewGeoPlan(plan, tc.origin)
			test.That(t, err, test.ShouldBeNil)
			pose := gps.Path()[0][baseName].Pose()
			pt := pose.Point()
			heading := utils.RadToDeg(pose.Orientation().EulerAngles().Yaw)
			heading = math.Mod(math.Abs(heading-360), 360)
			test.That(t, pt.X, test.ShouldAlmostEqual, tc.expectedGPs[0].Location().Lng(), 1e-6)
			test.That(t, pt.Y, test.ShouldAlmostEqual, tc.expectedGPs[0].Location().Lat(), 1e-6)
			test.That(t, heading, test.ShouldAlmostEqual, tc.expectedGPs[0].Heading(), 1e-3)

			pose = gps.Path()[len(gps.Path())-1][baseName].Pose()
			pt = pose.Point()
			test.That(t, pt.X, test.ShouldAlmostEqual, tc.expectedGPs[1].Location().Lng(), 1e-3)
			test.That(t, pt.Y, test.ShouldAlmostEqual, tc.expectedGPs[1].Location().Lat(), 1e-3)
		})
	}
}
