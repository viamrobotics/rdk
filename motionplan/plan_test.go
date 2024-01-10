package motionplan

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

func TestEvaluateTrajectory(t *testing.T) {
	plan := Trajectory{
		map[string][]referenceframe.Input{"": {{1.}, {2.}, {3.}}},
	}
	score := plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Inf(1))

	// Test no change
	plan = append(plan, map[string][]referenceframe.Input{"": {{1.}, {2.}, {3.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, 0)

	// Test L2 for "", and nothing for plan with only one entry
	plan = append(plan, map[string][]referenceframe.Input{"": {{4.}, {5.}, {6.}}, "test": {{2.}, {3.}, {4.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Sqrt(27))

	// Test cumulative L2 after returning to original inputs
	plan = append(plan, map[string][]referenceframe.Input{"": {{1.}, {2.}, {3.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Sqrt(27)*2)

	// Test that the "test" inputs are properly evaluated after skipping a step
	plan = append(plan, map[string][]referenceframe.Input{"test": {{3.}, {5.}, {6.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Sqrt(27)*2+3)

	// Evaluated with the tp-space metric, should be the sum of the distance values (third input) ignoring the first input set for each
	// named input set
	score = plan.Evaluate(tpspace.PTGSegmentMetric)
	test.That(t, score, test.ShouldEqual, 18)
}

func TestPlanStep(t *testing.T) {
	baseNameA := base.Named("my-base1")
	baseNameB := base.Named("my-base2")
	poseA := spatialmath.NewZeroPose()
	poseB := spatialmath.NewPose(r3.Vector{X: 100}, spatialmath.NewOrientationVector())

	protoAB := &pb.PlanStep{
		Step: map[string]*pb.ComponentState{
			baseNameA.String(): {Pose: spatialmath.PoseToProtobuf(poseA)},
			baseNameB.String(): {Pose: spatialmath.PoseToProtobuf(poseB)},
		},
	}
	stepAB := PathStep{
		baseNameA.ShortName(): referenceframe.NewPoseInFrame(referenceframe.World, poseA),
		baseNameB.ShortName(): referenceframe.NewPoseInFrame(referenceframe.World, poseB),
	}

	t.Run("pathStepFromProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       *pb.PlanStep
			result      PathStep
			err         error
		}

		testCases := []testCase{
			{
				description: "nil pointer returns an error",
				input:       nil,
				result:      PathStep{},
				err:         errors.New("received nil *pb.PlanStep"),
			},
			{
				description: "returns an error if any of the step resource names are invalid",
				input: &pb.PlanStep{
					Step: map[string]*pb.ComponentState{
						baseNameA.String():       {Pose: spatialmath.PoseToProtobuf(poseA)},
						"invalid component name": {Pose: spatialmath.PoseToProtobuf(poseB)},
					},
				},
				result: PathStep{},
				err:    errors.New("string \"invalid component name\" is not a valid resource name"),
			},
			{
				description: "an empty *pb.PlanStep returns an empty PathStep{}",
				input:       &pb.PlanStep{},
				result:      PathStep{},
			},
			{
				description: "a full *pb.PlanStep returns an full PathStep{}",
				input:       protoAB,
				result:      stepAB,
			},
		}
		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res, err := PathStepFromProto(tc.input)
				if tc.err != nil {
					test.That(t, err, test.ShouldBeError, tc.err)
				} else {
					test.That(t, err, test.ShouldBeNil)
				}
				test.That(t, res, test.ShouldResemble, tc.result)
			})
		}
	})

	t.Run("ToProto()", func(t *testing.T) {
		type testCase struct {
			description string
			input       PathStep
			result      *pb.PlanStep
		}

		testCases := []testCase{
			{
				description: "an nil PathStep returns an empty *pb.PlanStep",
				input:       nil,
				result:      &pb.PlanStep{},
			},
			{
				description: "an empty PathStep returns an empty *pb.PlanStep",
				input:       PathStep{},
				result:      &pb.PlanStep{},
			},
			{
				description: "a full PathStep{} returns an full *pb.PlanStep",
				input:       stepAB,
				result:      protoAB,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res := tc.input.ToProto()
				test.That(t, res, test.ShouldResemble, tc.result)
			})
		}
	})
}
