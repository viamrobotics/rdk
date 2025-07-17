package motiontypes

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
				res, err := referenceframe.FrameSystemPosesFromProto(tc.input)
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
				res := referenceframe.FrameSystemPosesToProto(tc.input)
				test.That(t, res, test.ShouldResemble, tc.result)
			})
		}
	})
}
