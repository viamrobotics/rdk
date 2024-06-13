package motion

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/motionplan"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
)

var defaultMotionCfg = MotionConfiguration{
	ObstacleDetectors: []ObstacleDetectorName{},
}

func TestPlanWithStatus(t *testing.T) {
	planID := uuid.New()
	executionID := uuid.New()

	baseName := base.Named("my-base1")
	poseA := spatialmath.NewZeroPose()
	poseB := spatialmath.NewPose(r3.Vector{X: 100}, spatialmath.NewOrientationVector())

	timestamp := time.Now().UTC()
	timestampb := timestamppb.New(timestamp)
	reason := "some reason"

	plan := PlanWithMetadata{
		ID:            planID,
		ExecutionID:   executionID,
		ComponentName: baseName,
		Plan: motionplan.NewSimplePlan(
			[]motionplan.PathStep{
				{baseName.ShortName(): referenceframe.NewPoseInFrame(referenceframe.World, poseA)},
				{baseName.ShortName(): referenceframe.NewPoseInFrame(referenceframe.World, poseB)},
			},
			nil,
		),
	}

	protoPlan := &pb.Plan{
		Id:            planID.String(),
		ExecutionId:   executionID.String(),
		ComponentName: rprotoutils.ResourceNameToProto(baseName),
		Steps: []*pb.PlanStep{
			{
				Step: map[string]*pb.ComponentState{
					baseName.ShortName(): {Pose: spatialmath.PoseToProtobuf(poseA)},
				},
			},
			{
				Step: map[string]*pb.ComponentState{
					baseName.ShortName(): {Pose: spatialmath.PoseToProtobuf(poseB)},
				},
			},
		},
	}

	t.Run("planWithStatusFromProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       *pb.PlanWithStatus
			result      PlanWithStatus
			err         error
		}

		testCases := []testCase{
			{
				description: "nil pointer returns error",
				input:       nil,
				result:      PlanWithStatus{},
				err:         errors.New("received nil *pb.PlanWithStatus"),
			},
			{
				description: "empty plan returns an error",
				input:       &pb.PlanWithStatus{},
				result:      PlanWithStatus{},
				err:         errors.New("received nil *pb.Plan"),
			},
			{
				description: "empty status returns an error",
				input:       &pb.PlanWithStatus{Plan: PlanWithMetadata{}.ToProto()},
				result:      PlanWithStatus{},
				err:         errors.New("received nil *pb.PlanStatus"),
			},
			{
				description: "nil pointers in the status history returns an error",
				input: &pb.PlanWithStatus{
					Plan:          PlanWithMetadata{}.ToProto(),
					Status:        PlanStatus{}.ToProto(),
					StatusHistory: []*pb.PlanStatus{nil},
				},
				result: PlanWithStatus{},
				err:    errors.New("received nil *pb.PlanStatus"),
			},
			{
				description: "empty *pb.PlanWithStatus status returns an empty PlanWithStatus",
				input: &pb.PlanWithStatus{
					Plan:   PlanWithMetadata{}.ToProto(),
					Status: PlanStatus{}.ToProto(),
				},
				result: PlanWithStatus{
					Plan:          PlanWithMetadata{},
					StatusHistory: []PlanStatus{{}},
				},
			},
			{
				description: "full *pb.PlanWithStatus status returns a full PlanWithStatus",
				input: &pb.PlanWithStatus{
					Plan: protoPlan,
					Status: &pb.PlanStatus{
						State:     pb.PlanState_PLAN_STATE_FAILED,
						Timestamp: timestampb,
						Reason:    &reason,
					},
					StatusHistory: []*pb.PlanStatus{
						{
							State:     pb.PlanState_PLAN_STATE_IN_PROGRESS,
							Timestamp: timestampb,
						},
					},
				},
				result: PlanWithStatus{
					Plan: plan,
					StatusHistory: []PlanStatus{
						{State: PlanStateFailed, Timestamp: timestamp, Reason: &reason},
						{State: PlanStateInProgress, Timestamp: timestamp},
					},
				},
			},
		}
		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res, err := planWithStatusFromProto(tc.input)
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
			input       PlanWithStatus
			result      *pb.PlanWithStatus
		}

		testCases := []testCase{
			{
				description: "an empty PlanWithStatus returns an empty *pb.PlanWithStatus",
				input:       PlanWithStatus{},
				result:      &pb.PlanWithStatus{Plan: PlanWithMetadata{}.ToProto()},
			},
			{
				description: "full PlanWithStatus without status history returns a full *pb.PlanWithStatus",
				input: PlanWithStatus{
					Plan: plan,
					StatusHistory: []PlanStatus{
						{State: PlanStateInProgress, Timestamp: timestamp},
					},
				},
				result: &pb.PlanWithStatus{
					Plan: protoPlan,
					Status: &pb.PlanStatus{
						State:     pb.PlanState_PLAN_STATE_IN_PROGRESS,
						Timestamp: timestampb,
					},
				},
			},
			{
				description: "full PlanWithStatus with status history returns a full *pb.PlanWithStatus",
				input: PlanWithStatus{
					Plan: plan,
					StatusHistory: []PlanStatus{
						{State: PlanStateFailed, Timestamp: timestamp, Reason: &reason},
						{State: PlanStateInProgress, Timestamp: timestamp},
					},
				},
				result: &pb.PlanWithStatus{
					Plan: protoPlan,
					Status: &pb.PlanStatus{
						State:     pb.PlanState_PLAN_STATE_FAILED,
						Timestamp: timestampb,
						Reason:    &reason,
					},
					StatusHistory: []*pb.PlanStatus{
						{
							State:     pb.PlanState_PLAN_STATE_IN_PROGRESS,
							Timestamp: timestampb,
						},
					},
				},
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

func TestPlanState(t *testing.T) {
	t.Run("planStateFromProto", func(t *testing.T) {
		type testCase struct {
			input    pb.PlanState
			expected PlanState
		}

		testCases := []testCase{
			{input: pb.PlanState_PLAN_STATE_IN_PROGRESS, expected: PlanStateInProgress},
			{input: pb.PlanState_PLAN_STATE_STOPPED, expected: PlanStateStopped},
			{input: pb.PlanState_PLAN_STATE_SUCCEEDED, expected: PlanStateSucceeded},
			{input: pb.PlanState_PLAN_STATE_FAILED, expected: PlanStateFailed},
			{input: pb.PlanState_PLAN_STATE_UNSPECIFIED, expected: PlanStateUnspecified},
			{input: 50, expected: PlanStateUnspecified},
		}
		for _, tc := range testCases {
			test.That(t, planStateFromProto(tc.input), test.ShouldEqual, tc.expected)
		}
	})

	t.Run("ToProto()", func(t *testing.T) {
		type testCase struct {
			input    PlanState
			expected pb.PlanState
		}

		testCases := []testCase{
			{input: PlanStateInProgress, expected: pb.PlanState_PLAN_STATE_IN_PROGRESS},
			{input: PlanStateStopped, expected: pb.PlanState_PLAN_STATE_STOPPED},
			{input: PlanStateSucceeded, expected: pb.PlanState_PLAN_STATE_SUCCEEDED},
			{input: PlanStateFailed, expected: pb.PlanState_PLAN_STATE_FAILED},
			{input: PlanStateUnspecified, expected: pb.PlanState_PLAN_STATE_UNSPECIFIED},
			{input: 60, expected: pb.PlanState_PLAN_STATE_UNSPECIFIED},
		}

		for _, tc := range testCases {
			test.That(t, tc.input.ToProto(), test.ShouldEqual, tc.expected)
		}
	})

	t.Run("String()", func(t *testing.T) {
		type testCase struct {
			input    PlanState
			expected string
		}

		testCases := []testCase{
			{input: PlanStateInProgress, expected: "in progress"},
			{input: PlanStateStopped, expected: "stopped"},
			{input: PlanStateSucceeded, expected: "succeeded"},
			{input: PlanStateFailed, expected: "failed"},
			{input: PlanStateUnspecified, expected: "unspecified"},
			{input: 60, expected: "unknown"},
		}

		for _, tc := range testCases {
			test.That(t, tc.input.String(), test.ShouldEqual, tc.expected)
		}
	})
}

func TestPlanStatusWithID(t *testing.T) {
	t.Run("planStatusWithIDFromProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       *pb.PlanStatusWithID
			result      PlanStatusWithID
			err         error
		}

		id := uuid.New()

		mybase := base.Named("mybase")
		timestamp := time.Now().UTC()
		timestampb := timestamppb.New(timestamp)
		reason := "some reason"

		testCases := []testCase{
			{
				description: "nil pointer returns error",
				input:       nil,
				result:      PlanStatusWithID{},
				err:         errors.New("received nil *pb.PlanStatusWithID"),
			},
			{
				description: "non uuid PlanID returns error",
				input:       &pb.PlanStatusWithID{PlanId: "not a uuid"},
				result:      PlanStatusWithID{},
				err:         errors.New("invalid UUID length: 10"),
			},
			{
				description: "non uuid ExecutionID returns error",
				input:       &pb.PlanStatusWithID{PlanId: id.String(), ExecutionId: "not a uuid"},
				result:      PlanStatusWithID{},
				err:         errors.New("invalid UUID length: 10"),
			},
			{
				description: "nil status returns error",
				input:       &pb.PlanStatusWithID{PlanId: id.String(), ExecutionId: id.String()},
				result:      PlanStatusWithID{},
				err:         errors.New("received nil *pb.PlanStatus"),
			},
			{
				description: "no component name returns error",
				input:       &pb.PlanStatusWithID{PlanId: id.String(), ExecutionId: id.String(), Status: &pb.PlanStatus{}},
				result:      PlanStatusWithID{},
				err:         errors.New("received nil *commonpb.ResourceName"),
			},
			{
				description: "success case with a failed plan status & reason",
				input: &pb.PlanStatusWithID{
					ComponentName: rprotoutils.ResourceNameToProto(mybase),
					ExecutionId:   id.String(),
					PlanId:        id.String(),
					Status:        &pb.PlanStatus{State: pb.PlanState_PLAN_STATE_FAILED, Timestamp: timestampb, Reason: &reason},
				},
				result: PlanStatusWithID{
					ComponentName: mybase,
					ExecutionID:   id,
					PlanID:        id,
					Status:        PlanStatus{State: PlanStateFailed, Timestamp: timestamp, Reason: &reason},
				},
			},
			{
				description: "success case with a in progress plan status",
				input: &pb.PlanStatusWithID{
					ComponentName: rprotoutils.ResourceNameToProto(mybase),
					ExecutionId:   id.String(),
					PlanId:        id.String(),
					Status:        &pb.PlanStatus{State: pb.PlanState_PLAN_STATE_IN_PROGRESS, Timestamp: timestampb},
				},
				result: PlanStatusWithID{
					ComponentName: mybase,
					ExecutionID:   id,
					PlanID:        id,
					Status:        PlanStatus{State: PlanStateInProgress, Timestamp: timestamp},
				},
			},
		}
		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res, err := planStatusWithIDFromProto(tc.input)
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
			input       PlanStatusWithID
			result      *pb.PlanStatusWithID
		}

		id := uuid.New()

		mybase := base.Named("mybase")
		timestamp := time.Now().UTC()
		timestampb := timestamppb.New(timestamp)
		reason := "some reason"

		testCases := []testCase{
			{
				description: "an empty PlanStatusWithID returns an empty *pb.PlanStatusWithID",
				input:       PlanStatusWithID{},
				result: &pb.PlanStatusWithID{
					PlanId:        uuid.Nil.String(),
					ExecutionId:   uuid.Nil.String(),
					ComponentName: rprotoutils.ResourceNameToProto(resource.Name{}),
					Status:        PlanStatus{}.ToProto(),
				},
			},
			{
				description: "a full PlanStatusWithID with a failed state & reason returns a full *pb.PlanStatusWithID",
				input: PlanStatusWithID{
					ComponentName: mybase,
					ExecutionID:   id,
					PlanID:        id,
					Status:        PlanStatus{State: PlanStateFailed, Timestamp: timestamp, Reason: &reason},
				},
				result: &pb.PlanStatusWithID{
					ComponentName: rprotoutils.ResourceNameToProto(mybase),
					ExecutionId:   id.String(),
					PlanId:        id.String(),
					Status:        &pb.PlanStatus{State: pb.PlanState_PLAN_STATE_FAILED, Timestamp: timestampb, Reason: &reason},
				},
			},
			{
				description: "a full PlanStatusWithID with an in progres state & nil reason returns a full *pb.PlanStatusWithID",
				input: PlanStatusWithID{
					ComponentName: mybase,
					ExecutionID:   id,
					PlanID:        id,
					Status:        PlanStatus{State: PlanStateInProgress, Timestamp: timestamp},
				},
				result: &pb.PlanStatusWithID{
					ComponentName: rprotoutils.ResourceNameToProto(mybase),
					ExecutionId:   id.String(),
					PlanId:        id.String(),
					Status:        &pb.PlanStatus{State: pb.PlanState_PLAN_STATE_IN_PROGRESS, Timestamp: timestampb},
				},
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

func TestPlanStatus(t *testing.T) {
	t.Run("planStatusFromProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       *pb.PlanStatus
			result      PlanStatus
			err         error
		}

		timestamp := time.Now().UTC()
		timestampb := timestamppb.New(timestamp)
		reason := "some reason"

		testCases := []testCase{
			{
				description: "nil pointer returns error",
				input:       nil,
				result:      PlanStatus{},
				err:         errors.New("received nil *pb.PlanStatus"),
			},
			{
				description: "success case with a failed plan state & reason",
				input:       &pb.PlanStatus{State: pb.PlanState_PLAN_STATE_FAILED, Timestamp: timestampb, Reason: &reason},
				result:      PlanStatus{State: PlanStateFailed, Timestamp: timestamp, Reason: &reason},
			},
			{
				description: "success case with a stopped plan state",
				input:       &pb.PlanStatus{State: pb.PlanState_PLAN_STATE_STOPPED, Timestamp: timestampb},
				result:      PlanStatus{State: PlanStateStopped, Timestamp: timestamp},
			},
			{
				description: "success case with a succeeded plan state",
				input:       &pb.PlanStatus{State: pb.PlanState_PLAN_STATE_SUCCEEDED, Timestamp: timestampb},
				result:      PlanStatus{State: PlanStateSucceeded, Timestamp: timestamp},
			},
			{
				description: "success case with an unspecified plan state",
				input:       &pb.PlanStatus{State: pb.PlanState_PLAN_STATE_UNSPECIFIED, Timestamp: timestampb},
				result:      PlanStatus{State: PlanStateUnspecified, Timestamp: timestamp},
			},
			{
				description: "success case with a in progress plan status",
				input:       &pb.PlanStatus{State: pb.PlanState_PLAN_STATE_IN_PROGRESS, Timestamp: timestampb},
				result:      PlanStatus{State: PlanStateInProgress, Timestamp: timestamp},
			},
		}
		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res, err := planStatusFromProto(tc.input)
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
			input       PlanStatus
			result      *pb.PlanStatus
		}

		timestamp := time.Now().UTC()
		timestampb := timestamppb.New(timestamp)
		reason := "some reason"

		testCases := []testCase{
			{
				description: "an empty PlanStatus returns an empty *pb.PlanStatus",
				input:       PlanStatus{},
				result:      &pb.PlanStatus{Timestamp: timestamppb.New(time.Time{})},
			},
			{
				description: "success case with a failed plan state & reason",
				input:       PlanStatus{State: PlanStateFailed, Timestamp: timestamp, Reason: &reason},
				result:      &pb.PlanStatus{State: pb.PlanState_PLAN_STATE_FAILED, Timestamp: timestampb, Reason: &reason},
			},
			{
				description: "success case with a stopped plan state",
				input:       PlanStatus{State: PlanStateStopped, Timestamp: timestamp},
				result:      &pb.PlanStatus{State: pb.PlanState_PLAN_STATE_STOPPED, Timestamp: timestampb},
			},
			{
				description: "success case with a succeeded plan state",
				input:       PlanStatus{State: PlanStateSucceeded, Timestamp: timestamp},
				result:      &pb.PlanStatus{State: pb.PlanState_PLAN_STATE_SUCCEEDED, Timestamp: timestampb},
			},
			{
				description: "success case with an unspecified plan state",
				input:       PlanStatus{State: PlanStateUnspecified, Timestamp: timestamp},
				result:      &pb.PlanStatus{State: pb.PlanState_PLAN_STATE_UNSPECIFIED, Timestamp: timestampb},
			},
			{
				description: "success case with a in progress plan status",
				input:       PlanStatus{State: PlanStateInProgress, Timestamp: timestamp},
				result:      &pb.PlanStatus{State: pb.PlanState_PLAN_STATE_IN_PROGRESS, Timestamp: timestampb},
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

func TestPlan(t *testing.T) {
	planID := uuid.New()
	executionID := uuid.New()

	baseName := base.Named("my-base1")
	poseA := spatialmath.NewZeroPose()
	poseB := spatialmath.NewPose(r3.Vector{X: 100}, spatialmath.NewOrientationVector())

	protoAB := &pb.Plan{
		Id:            planID.String(),
		ExecutionId:   executionID.String(),
		ComponentName: rprotoutils.ResourceNameToProto(baseName),
		Steps: []*pb.PlanStep{
			{
				Step: map[string]*pb.ComponentState{
					baseName.ShortName(): {Pose: spatialmath.PoseToProtobuf(poseA)},
				},
			},
			{
				Step: map[string]*pb.ComponentState{
					baseName.ShortName(): {Pose: spatialmath.PoseToProtobuf(poseB)},
				},
			},
		},
	}
	planAB := PlanWithMetadata{
		ID:            planID,
		ExecutionID:   executionID,
		ComponentName: baseName,
		Plan: motionplan.NewSimplePlan(
			[]motionplan.PathStep{
				{baseName.ShortName(): referenceframe.NewPoseInFrame(referenceframe.World, poseA)},
				{baseName.ShortName(): referenceframe.NewPoseInFrame(referenceframe.World, poseB)},
			},
			nil,
		),
	}

	t.Run("planFromProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       *pb.Plan
			result      PlanWithMetadata
			err         error
		}

		testCases := []testCase{
			{
				description: "nil pointer returns error",
				input:       nil,
				result:      PlanWithMetadata{},
				err:         errors.New("received nil *pb.Plan"),
			},
			{
				description: "empty PlanID in *pb.Plan{} returns an error",
				input:       &pb.Plan{},
				result:      PlanWithMetadata{},
				err:         errors.New("invalid UUID length: 0"),
			},
			{
				description: "empty ExecutionID in *pb.Plan{} returns an error",
				input:       &pb.Plan{Id: planID.String()},
				result:      PlanWithMetadata{},
				err:         errors.New("invalid UUID length: 0"),
			},
			{
				description: "empty ComponentName in *pb.Plan{} returns an error",
				input:       &pb.Plan{Id: planID.String(), ExecutionId: executionID.String()},
				result:      PlanWithMetadata{},
				err:         errors.New("received nil *pb.ResourceName"),
			},
			{
				description: "a nil *pb.PlanStep{} returns an error",
				input: &pb.Plan{
					Id:            planID.String(),
					ExecutionId:   executionID.String(),
					ComponentName: rprotoutils.ResourceNameToProto(resource.Name{}),
					Steps:         []*pb.PlanStep{nil},
				},
				result: PlanWithMetadata{},
				err:    errors.New("received nil *pb.PlanStep"),
			},
			{
				description: "success case for empty steps",
				input: &pb.Plan{
					Id:            planID.String(),
					ExecutionId:   executionID.String(),
					ComponentName: rprotoutils.ResourceNameToProto(resource.Name{}),
				},
				result: PlanWithMetadata{
					ID:            planID,
					ExecutionID:   executionID,
					ComponentName: resource.Name{},
				},
			},
			{
				description: "success case for full steps",
				input:       protoAB,
				result:      planAB,
			},
		}
		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res, err := planFromProto(tc.input)
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
			input       PlanWithMetadata
			result      *pb.Plan
		}

		testCases := []testCase{
			{
				description: "an empty Plan returns an empty *pb.Plan",
				input:       PlanWithMetadata{},
				result: &pb.Plan{
					Id:            uuid.Nil.String(),
					ComponentName: rprotoutils.ResourceNameToProto(resource.Name{}),
					ExecutionId:   uuid.Nil.String(),
				},
			},
			{
				description: "full Plan returns full *pb.Plan",
				input:       planAB,
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

func TestConfiguration(t *testing.T) {
	visionCameraPairs := [][]resource.Name{
		{vision.Named("vision service 1"), camera.Named("camera 1")},
		{vision.Named("vision service 2"), camera.Named("camera 2")},
	}
	obstacleDetectorsPB := []*pb.ObstacleDetector{}
	obstacleDetectors := []ObstacleDetectorName{}
	for _, pair := range visionCameraPairs {
		obstacleDetectors = append(obstacleDetectors, ObstacleDetectorName{
			VisionServiceName: pair[0],
			CameraName:        pair[1],
		})
		obstacleDetectorsPB = append(obstacleDetectorsPB, &pb.ObstacleDetector{
			VisionService: rprotoutils.ResourceNameToProto(pair[0]),
			Camera:        rprotoutils.ResourceNameToProto(pair[1]),
		})
	}

	t.Run("configurationFromProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       *pb.MotionConfiguration
			result      *MotionConfiguration
		}
		linearMPerSec := 1.
		angularDegsPerSec := 2.
		planDeviationMM := 3000.
		planDeviationM := planDeviationMM / 1000
		positionPollingFreqHz := 4.
		obstaclePollingFreqHz := 5.

		testCases := []testCase{
			{
				description: "when passed a nil pointer returns default MotionConfiguration struct",
				input:       nil,
				result:      &defaultMotionCfg,
			},
			{
				description: "when passed an empty struct returns default MotionConfiguration struct",
				input:       &pb.MotionConfiguration{},
				result:      &defaultMotionCfg,
			},
			{
				description: "when passed a full struct returns a full struct",
				input: &pb.MotionConfiguration{
					ObstacleDetectors:          obstacleDetectorsPB,
					LinearMPerSec:              &linearMPerSec,
					AngularDegsPerSec:          &angularDegsPerSec,
					PlanDeviationM:             &planDeviationM,
					PositionPollingFrequencyHz: &positionPollingFreqHz,
					ObstaclePollingFrequencyHz: &obstaclePollingFreqHz,
				},
				result: &MotionConfiguration{
					ObstacleDetectors:     obstacleDetectors,
					LinearMPerSec:         linearMPerSec,
					AngularDegsPerSec:     angularDegsPerSec,
					PlanDeviationMM:       planDeviationMM,
					PositionPollingFreqHz: positionPollingFreqHz,
					ObstaclePollingFreqHz: obstaclePollingFreqHz,
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res := configurationFromProto(tc.input)
				test.That(t, res, test.ShouldResemble, tc.result)
			})
		}
	})

	t.Run("toProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       *MotionConfiguration
			result      *pb.MotionConfiguration
		}

		linearMPerSec := 1.
		angularDegsPerSec := 2.
		planDeviationMM := 3000.
		planDeviationM := planDeviationMM / 1000
		positionPollingFreqHz := 4.
		obstaclePollingFreqHz := 5.
		zero := 0.

		testCases := []testCase{
			{
				description: "when passed an empty struct returns mostly empty struct",
				input:       &MotionConfiguration{},
				result:      &pb.MotionConfiguration{PlanDeviationM: &zero},
			},
			{
				description: "when passed a full struct returns a full struct",
				input: &MotionConfiguration{
					ObstacleDetectors:     obstacleDetectors,
					LinearMPerSec:         linearMPerSec,
					AngularDegsPerSec:     angularDegsPerSec,
					PlanDeviationMM:       planDeviationMM,
					PositionPollingFreqHz: positionPollingFreqHz,
					ObstaclePollingFreqHz: obstaclePollingFreqHz,
				},
				result: &pb.MotionConfiguration{
					ObstacleDetectors:          obstacleDetectorsPB,
					LinearMPerSec:              &linearMPerSec,
					AngularDegsPerSec:          &angularDegsPerSec,
					PlanDeviationM:             &planDeviationM,
					PositionPollingFrequencyHz: &positionPollingFreqHz,
					ObstaclePollingFrequencyHz: &obstaclePollingFreqHz,
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res := tc.input.toProto()
				test.That(t, res, test.ShouldResemble, tc.result)
			})
		}
	})
}

func TestMoveOnGlobeReq(t *testing.T) {
	name := "somename"
	dst := geo.NewPoint(1, 2)

	t.Run("String()", func(t *testing.T) {
		s := "motion.MoveOnGlobeReq{ComponentName: " +
			"rdk:component:base/my-base, Destination: " +
			"&{lat:1 lng:2}, Heading: 0.500000, MovementSensorName: " +
			"rdk:component:movement_sensor/my-movementsensor, " +
			"Obstacles: [], BoundingRegions: [], MotionCfg: &motion.MotionConfiguration{" +
			"ObstacleDetectors:[]motion.ObstacleDetectorName{" +
			"motion.ObstacleDetectorName{VisionServiceName:resource.Name{" +
			"API:resource.API{Type:resource.APIType{Namespace:\"rdk\", Name:\"service\"}, " +
			"SubtypeName:\"vision\"}, Remote:\"\", Name:\"vision service 1\"}, " +
			"CameraName:resource.Name{API:resource.API{Type:resource.APIType{" +
			"Namespace:\"rdk\", Name:\"component\"}, SubtypeName:\"camera\"}, " +
			"Remote:\"\", Name:\"camera 1\"}}, motion.ObstacleDetectorName{" +
			"VisionServiceName:resource.Name{API:resource.API{Type:resource.APIType{" +
			"Namespace:\"rdk\", Name:\"service\"}, SubtypeName:\"vision\"}, " +
			"Remote:\"\", Name:\"vision service 2\"}, CameraName:resource.Name{" +
			"API:resource.API{Type:resource.APIType{Namespace:\"rdk\", " +
			"Name:\"component\"}, SubtypeName:\"camera\"}, Remote:\"\", " +
			"Name:\"camera 2\"}}}, PositionPollingFreqHz:4, ObstaclePollingFreqHz:5, " +
			"PlanDeviationMM:3, LinearMPerSec:1, AngularDegsPerSec:2}, Extra: map[]}"
		test.That(t, validMoveOnGlobeRequest().String(), test.ShouldResemble, s)
	})

	t.Run("toProto", func(t *testing.T) {
		t.Run("error due to nil destination", func(t *testing.T) {
			mogReq := validMoveOnGlobeRequest()
			mogReq.Destination = nil
			_, err := mogReq.toProto(name)
			test.That(t, err, test.ShouldBeError, errors.New("must provide a destination"))
		})

		t.Run("sets heading to nil if set to NaN", func(t *testing.T) {
			mogReq := validMoveOnGlobeRequest()
			mogReq.Heading = math.NaN()
			req, err := mogReq.toProto(name)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, req.Heading, test.ShouldBeNil)
		})

		t.Run("success", func(t *testing.T) {
			mogReq := validMoveOnGlobeRequest()
			req, err := mogReq.toProto(name)

			test.That(t, err, test.ShouldBeNil)
			test.That(t, req.Name, test.ShouldResemble, "somename")
			test.That(t, req.ComponentName.Name, test.ShouldResemble, "my-base")
			test.That(t, req.Destination.Latitude, test.ShouldAlmostEqual, dst.Lat())
			test.That(t, req.Destination.Longitude, test.ShouldAlmostEqual, dst.Lng())
			test.That(t, req.Heading, test.ShouldNotBeNil)
			test.That(t, *req.Heading, test.ShouldAlmostEqual, 0.5)
			test.That(t, req.MovementSensorName.Name, test.ShouldResemble, "my-movementsensor")
			test.That(t, req.Obstacles, test.ShouldBeEmpty)
			test.That(t, req.MotionConfiguration, test.ShouldResemble, mogReq.MotionCfg.toProto())
			test.That(t, req.Extra.AsMap(), test.ShouldBeEmpty)
		})

		t.Run("allows nil motion config", func(t *testing.T) {
			mogReq := validMoveOnGlobeRequest()
			mogReq.MotionCfg = nil
			req, err := mogReq.toProto(name)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, req.Name, test.ShouldResemble, "somename")
			test.That(t, req.ComponentName.Name, test.ShouldResemble, "my-base")
			test.That(t, req.Destination.Latitude, test.ShouldAlmostEqual, dst.Lat())
			test.That(t, req.Destination.Longitude, test.ShouldAlmostEqual, dst.Lng())
			test.That(t, req.Heading, test.ShouldNotBeNil)
			test.That(t, *req.Heading, test.ShouldAlmostEqual, 0.5)
			test.That(t, req.MovementSensorName.Name, test.ShouldResemble, "my-movementsensor")
			test.That(t, req.Obstacles, test.ShouldBeEmpty)
			test.That(t, req.MotionConfiguration, test.ShouldBeNil)
			test.That(t, req.Extra.AsMap(), test.ShouldBeEmpty)
		})
	})

	visionCameraPairs := [][]resource.Name{
		{vision.Named("vision service 1"), camera.Named("camera 1")},
		{vision.Named("vision service 2"), camera.Named("camera 2")},
	}
	obstacleDetectorsPB := []*pb.ObstacleDetector{}
	obstacleDetectors := []ObstacleDetectorName{}
	for _, pair := range visionCameraPairs {
		obstacleDetectors = append(obstacleDetectors, ObstacleDetectorName{
			VisionServiceName: pair[0],
			CameraName:        pair[1],
		})
		obstacleDetectorsPB = append(obstacleDetectorsPB, &pb.ObstacleDetector{
			VisionService: rprotoutils.ResourceNameToProto(pair[0]),
			Camera:        rprotoutils.ResourceNameToProto(pair[1]),
		})
	}

	t.Run("moveOnGlobeRequestFromProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       *pb.MoveOnGlobeRequest
			result      MoveOnGlobeReq
			err         error
		}

		heading := 1.
		linearMPerSec := 1.
		angularDegsPerSec := 2.
		planDeviationMM := 3000.
		planDeviationM := planDeviationMM / 1000
		positionPollingFreqHz := 4.
		obstaclePollingFreqHz := 5.
		sphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 1, "sphere")
		test.That(t, err, test.ShouldBeNil)

		mybase := base.Named("my-base")

		testCases := []testCase{
			{
				description: "an nil *pb.MoveOnGlobeRequest returns an error",
				input:       nil,
				result:      MoveOnGlobeReq{},
				err:         errors.New("received nil *pb.MoveOnGlobeRequest"),
			},
			{
				description: "an empty destination returns an error",
				input:       &pb.MoveOnGlobeRequest{},
				result:      MoveOnGlobeReq{},
				err:         errors.New("must provide a destination"),
			},
			{
				description: "an empty compnent name returns an error",
				input: &pb.MoveOnGlobeRequest{
					Destination: &commonpb.GeoPoint{Latitude: 1, Longitude: 2},
				},
				result: MoveOnGlobeReq{},
				err:    errors.New("received nil *commonpb.ResourceName"),
			},
			{
				description: "an empty movement sensor name returns an error",
				input: &pb.MoveOnGlobeRequest{
					Destination:   &commonpb.GeoPoint{Latitude: 1, Longitude: 2},
					ComponentName: rprotoutils.ResourceNameToProto(mybase),
				},
				result: MoveOnGlobeReq{},
				err:    errors.New("received nil *commonpb.ResourceName"),
			},
			{
				description: "an empty *pb.MoveOnGlobeRequest returns an empty MoveOnGlobeReq",
				input: &pb.MoveOnGlobeRequest{
					Heading:            &heading,
					Destination:        &commonpb.GeoPoint{Latitude: 1, Longitude: 2},
					ComponentName:      rprotoutils.ResourceNameToProto(mybase),
					MovementSensorName: rprotoutils.ResourceNameToProto(movementsensor.Named("my-movementsensor")),
				},
				result: MoveOnGlobeReq{
					Heading:            heading,
					Destination:        geo.NewPoint(1, 2),
					ComponentName:      mybase,
					MovementSensorName: movementsensor.Named("my-movementsensor"),
					MotionCfg:          &defaultMotionCfg,
					Obstacles:          []*spatialmath.GeoGeometry{},
					BoundingRegions:    []*spatialmath.GeoGeometry{},
					Extra:              map[string]interface{}{},
				},
			},
			{
				description: "a full *pb.MoveOnGlobeRequest returns a full MoveOnGlobeReq",
				input: &pb.MoveOnGlobeRequest{
					Heading:            &heading,
					Destination:        &commonpb.GeoPoint{Latitude: 1, Longitude: 2},
					ComponentName:      rprotoutils.ResourceNameToProto(mybase),
					MovementSensorName: rprotoutils.ResourceNameToProto(movementsensor.Named("my-movementsensor")),
					Obstacles: []*commonpb.GeoGeometry{
						{
							Location: &commonpb.GeoPoint{Latitude: 3, Longitude: 4},
							Geometries: []*commonpb.Geometry{
								{
									Center: &commonpb.Pose{OZ: 1},
									GeometryType: &commonpb.Geometry_Sphere{
										Sphere: &commonpb.Sphere{
											RadiusMm: 1,
										},
									},
									Label: "sphere",
								},
							},
						},
					},
					BoundingRegions: []*commonpb.GeoGeometry{
						{
							Location: &commonpb.GeoPoint{Latitude: 1, Longitude: 2},
							Geometries: []*commonpb.Geometry{
								{
									Center: &commonpb.Pose{OZ: 1},
									GeometryType: &commonpb.Geometry_Sphere{
										Sphere: &commonpb.Sphere{
											RadiusMm: 1,
										},
									},
									Label: "sphere",
								},
							},
						},
					},
					MotionConfiguration: &pb.MotionConfiguration{
						ObstacleDetectors:          obstacleDetectorsPB,
						LinearMPerSec:              &linearMPerSec,
						AngularDegsPerSec:          &angularDegsPerSec,
						PlanDeviationM:             &planDeviationM,
						PositionPollingFrequencyHz: &positionPollingFreqHz,
						ObstaclePollingFrequencyHz: &obstaclePollingFreqHz,
					},
				},
				result: MoveOnGlobeReq{
					Heading:            heading,
					Destination:        dst,
					ComponentName:      mybase,
					MovementSensorName: movementsensor.Named("my-movementsensor"),
					BoundingRegions: []*spatialmath.GeoGeometry{
						spatialmath.NewGeoGeometry(geo.NewPoint(1, 2), []spatialmath.Geometry{sphere}),
					},
					Obstacles: []*spatialmath.GeoGeometry{
						spatialmath.NewGeoGeometry(geo.NewPoint(3, 4), []spatialmath.Geometry{sphere}),
					},
					MotionCfg: &MotionConfiguration{
						ObstacleDetectors:     obstacleDetectors,
						LinearMPerSec:         linearMPerSec,
						AngularDegsPerSec:     angularDegsPerSec,
						PlanDeviationMM:       planDeviationMM,
						PositionPollingFreqHz: positionPollingFreqHz,
						ObstaclePollingFreqHz: obstaclePollingFreqHz,
					},
					Extra: map[string]interface{}{},
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res, err := moveOnGlobeRequestFromProto(tc.input)

				if tc.err != nil {
					test.That(t, err, test.ShouldBeError, tc.err)
				} else {
					test.That(t, err, test.ShouldBeNil)
				}
				test.That(t, res, test.ShouldResemble, tc.result)
			})
		}

		t.Run("nil heading is converted into a NaN heading", func(t *testing.T) {
			input := &pb.MoveOnGlobeRequest{
				Destination:        &commonpb.GeoPoint{Latitude: 1, Longitude: 2},
				ComponentName:      rprotoutils.ResourceNameToProto(mybase),
				MovementSensorName: rprotoutils.ResourceNameToProto(movementsensor.Named("my-movementsensor")),
			}
			res, err := moveOnGlobeRequestFromProto(input)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, math.IsNaN(res.Heading), test.ShouldBeTrue)
		})
	})
}

func TestMoveOnMapReq(t *testing.T) {
	visionCameraPairs := [][]resource.Name{
		{vision.Named("vision service 1"), camera.Named("camera 1")},
		{vision.Named("vision service 2"), camera.Named("camera 2")},
	}
	obstacleDetectors := []ObstacleDetectorName{}
	for _, pair := range visionCameraPairs {
		obstacleDetectors = append(obstacleDetectors, ObstacleDetectorName{
			VisionServiceName: pair[0],
			CameraName:        pair[1],
		})
	}
	myBase := base.Named("mybase")
	mySlam := slam.Named(("mySlam"))
	motionCfg := &MotionConfiguration{
		ObstacleDetectors:     obstacleDetectors,
		LinearMPerSec:         1,
		AngularDegsPerSec:     2,
		PlanDeviationMM:       3,
		PositionPollingFreqHz: 4,
		ObstaclePollingFreqHz: 5,
	}

	validMoveOnMapReq := MoveOnMapReq{
		ComponentName: myBase,
		Destination:   spatialmath.NewZeroPose(),
		SlamName:      mySlam,
		MotionCfg:     motionCfg,
		Obstacles:     []spatialmath.Geometry{},
		Extra:         map[string]interface{}{},
	}

	validPbMoveOnMapRequest := &pb.MoveOnMapRequest{
		Name:                "bloop",
		Destination:         spatialmath.PoseToProtobuf(spatialmath.NewZeroPose()),
		ComponentName:       rprotoutils.ResourceNameToProto(myBase),
		SlamServiceName:     rprotoutils.ResourceNameToProto(mySlam),
		MotionConfiguration: motionCfg.toProto(),
		Extra:               &structpb.Struct{},
	}

	t.Run("String()", func(t *testing.T) {
		s := fmt.Sprintf(
			"motion.MoveOnMapReq{ComponentName: %s, SlamName: %s, Destination: %+v, "+
				"MotionCfg: %#v, Obstacles: %s, Extra: %s}",
			validMoveOnMapReq.ComponentName,
			validMoveOnMapReq.SlamName,
			spatialmath.PoseToProtobuf(validMoveOnMapReq.Destination),
			validMoveOnMapReq.MotionCfg,
			validMoveOnMapReq.Obstacles,
			validMoveOnMapReq.Extra)
		test.That(t, validMoveOnMapReq.String(), test.ShouldEqual, s)
	})

	t.Run("toProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       MoveOnMapReq
			name        string
			result      *pb.MoveOnMapRequest
			err         error
		}

		testCases := []testCase{
			{
				description: "empty struct fails due to nil destination",
				input:       MoveOnMapReq{},
				name:        "bloop",
				result:      nil,
				err:         errors.New("must provide a destination"),
			},
			{
				description: "success",
				input:       validMoveOnMapReq,
				name:        "bloop",
				result:      validPbMoveOnMapRequest,
				err:         nil,
			},
			{
				description: "allows nil motion cfg",
				input: MoveOnMapReq{
					ComponentName: myBase,
					Destination:   spatialmath.NewZeroPose(),
					SlamName:      mySlam,
				},
				name: "bloop",

				result: &pb.MoveOnMapRequest{
					Name:            "bloop",
					Destination:     spatialmath.PoseToProtobuf(spatialmath.NewZeroPose()),
					ComponentName:   rprotoutils.ResourceNameToProto(myBase),
					SlamServiceName: rprotoutils.ResourceNameToProto(mySlam),
					Extra:           &structpb.Struct{},
				},
				err: nil,
			},
			{
				description: "allows non-nil obstacles",
				input: MoveOnMapReq{
					ComponentName: myBase,
					Destination:   spatialmath.NewZeroPose(),
					SlamName:      mySlam,
					Obstacles:     []spatialmath.Geometry{spatialmath.NewPoint(r3.Vector{2, 2, 2}, "pt")},
				},
				name: "bloop",

				result: &pb.MoveOnMapRequest{
					Name:            "bloop",
					Destination:     spatialmath.PoseToProtobuf(spatialmath.NewZeroPose()),
					ComponentName:   rprotoutils.ResourceNameToProto(myBase),
					SlamServiceName: rprotoutils.ResourceNameToProto(mySlam),
					Obstacles:       spatialmath.NewGeometriesToProto([]spatialmath.Geometry{spatialmath.NewPoint(r3.Vector{2, 2, 2}, "pt")}),
					Extra:           &structpb.Struct{},
				},
				err: nil,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res, err := tc.input.toProto(tc.name)
				if tc.err != nil {
					test.That(t, err, test.ShouldBeError, tc.err)
				} else {
					test.That(t, err, test.ShouldBeNil)
				}
				test.That(t, res, test.ShouldResemble, tc.result)
			})
		}
	})

	t.Run("moveOnMapRequestFromProto", func(t *testing.T) {
		type testCase struct {
			description string

			input  *pb.MoveOnMapRequest
			result MoveOnMapReq
			err    error
		}

		testCases := []testCase{
			{
				description: "nil request fails",
				input:       nil,
				result:      MoveOnMapReq{},
				err:         errors.New("received nil *pb.MoveOnMapRequest"),
			},
			{
				description: "nil destination causes failure",

				input:  &pb.MoveOnMapRequest{},
				result: MoveOnMapReq{},
				err:    errors.New("received nil *commonpb.Pose for destination"),
			},
			{
				description: "nil componentName causes failure",

				input: &pb.MoveOnMapRequest{
					Destination: spatialmath.PoseToProtobuf(spatialmath.NewZeroPose()),
				},
				result: MoveOnMapReq{},
				err:    errors.New("received nil *commonpb.ResourceName for component name"),
			},
			{
				description: "nil SlamName causes failure",

				input: &pb.MoveOnMapRequest{
					Destination:   spatialmath.PoseToProtobuf(spatialmath.NewZeroPose()),
					ComponentName: rprotoutils.ResourceNameToProto(myBase),
				},
				result: MoveOnMapReq{},
				err:    errors.New("received nil *commonpb.ResourceName for SlamService name"),
			},
			{
				description: "success",
				input:       validPbMoveOnMapRequest,
				result:      validMoveOnMapReq,
				err:         nil,
			},
			{
				description: "success - allow nil motionCfg",

				input: &pb.MoveOnMapRequest{
					Destination:     spatialmath.PoseToProtobuf(spatialmath.NewPoseFromPoint(r3.Vector{2700, 0, 0})),
					ComponentName:   rprotoutils.ResourceNameToProto(myBase),
					SlamServiceName: rprotoutils.ResourceNameToProto(mySlam),
				},
				result: MoveOnMapReq{
					ComponentName: myBase,
					Destination:   spatialmath.NewPoseFromPoint(r3.Vector{2700, 0, 0}),
					SlamName:      mySlam,
					MotionCfg: &MotionConfiguration{
						ObstacleDetectors:     []ObstacleDetectorName{},
						PositionPollingFreqHz: 0,
						ObstaclePollingFreqHz: 0,
						PlanDeviationMM:       0,
						LinearMPerSec:         0,
						AngularDegsPerSec:     0,
					},
					Obstacles: []spatialmath.Geometry{},
					Extra:     map[string]interface{}{},
				},
				err: nil,
			},
			{
				description: "success - allow non-nil obstacles",
				input: &pb.MoveOnMapRequest{
					Destination:     spatialmath.PoseToProtobuf(spatialmath.NewPoseFromPoint(r3.Vector{2700, 0, 0})),
					ComponentName:   rprotoutils.ResourceNameToProto(myBase),
					SlamServiceName: rprotoutils.ResourceNameToProto(mySlam),
					Obstacles:       spatialmath.NewGeometriesToProto([]spatialmath.Geometry{spatialmath.NewPoint(r3.Vector{2, 2, 2}, "pt")}),
				},
				result: MoveOnMapReq{
					ComponentName: myBase,
					Destination:   spatialmath.NewPoseFromPoint(r3.Vector{2700, 0, 0}),
					SlamName:      mySlam,
					MotionCfg: &MotionConfiguration{
						ObstacleDetectors:     []ObstacleDetectorName{},
						PositionPollingFreqHz: 0,
						ObstaclePollingFreqHz: 0,
						PlanDeviationMM:       0,
						LinearMPerSec:         0,
						AngularDegsPerSec:     0,
					},
					Obstacles: []spatialmath.Geometry{spatialmath.NewPoint(r3.Vector{2, 2, 2}, "pt")},
					Extra:     map[string]interface{}{},
				},
				err: nil,
			},
			{
				description: "fail - inconvertible geometry",
				input: &pb.MoveOnMapRequest{
					Destination:     spatialmath.PoseToProtobuf(spatialmath.NewPoseFromPoint(r3.Vector{2700, 0, 0})),
					ComponentName:   rprotoutils.ResourceNameToProto(myBase),
					SlamServiceName: rprotoutils.ResourceNameToProto(mySlam),
					Obstacles:       []*commonpb.Geometry{{GeometryType: nil}},
				},
				result: MoveOnMapReq{},
				err:    errors.New("cannot convert obstacles into geometries: cannot have nil pose for geometry"),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res, err := moveOnMapRequestFromProto(tc.input)
				if tc.err != nil {
					test.That(t, err, test.ShouldBeError, tc.err)
				} else {
					test.That(t, err, test.ShouldBeNil)
				}
				test.That(t, res, test.ShouldResemble, tc.result)
			})
		}
	})
}

func TestPlanHistoryReq(t *testing.T) {
	t.Run("toProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       PlanHistoryReq
			name        string
			result      *pb.GetPlanRequest
			err         error
		}

		executionID := uuid.New()
		mybase := base.Named("mybase")
		executionIDStr := executionID.String()

		testCases := []testCase{
			{
				description: "empty struct returns an empty struct",
				input:       PlanHistoryReq{},
				name:        "some name",
				result: &pb.GetPlanRequest{
					Name:          "some name",
					ComponentName: rprotoutils.ResourceNameToProto(resource.Name{}),
					Extra:         &structpb.Struct{},
				},
			},
			{
				description: "full struct returns a full struct",
				input: PlanHistoryReq{
					ComponentName: mybase,
					ExecutionID:   executionID,
					LastPlanOnly:  true,
				},
				name: "some name",
				result: &pb.GetPlanRequest{
					Name:          "some name",
					ComponentName: rprotoutils.ResourceNameToProto(mybase),
					ExecutionId:   &executionIDStr,
					LastPlanOnly:  true,
					Extra:         &structpb.Struct{},
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res, err := tc.input.toProto(tc.name)

				if tc.err != nil {
					test.That(t, err, test.ShouldBeError, tc.err)
				} else {
					test.That(t, err, test.ShouldBeNil)
				}
				test.That(t, res, test.ShouldResemble, tc.result)
			})
		}
	})

	t.Run("getPlanRequestFromProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       *pb.GetPlanRequest
			result      PlanHistoryReq
			err         error
		}

		executionID := uuid.New()
		mybase := base.Named("mybase")
		executionIDStr := executionID.String()

		testCases := []testCase{
			{
				description: "returns an error if component name is nil",
				input:       &pb.GetPlanRequest{},
				result:      PlanHistoryReq{},
				err:         errors.New("received nil *commonpb.ResourceName"),
			},
			{
				description: "empty struct returns an empty struct",
				input: &pb.GetPlanRequest{
					ComponentName: rprotoutils.ResourceNameToProto(resource.Name{}),
				},
				result: PlanHistoryReq{Extra: map[string]interface{}{}},
			},
			{
				description: "full struct returns a full struct",
				input: &pb.GetPlanRequest{
					Name:          "some name",
					ComponentName: rprotoutils.ResourceNameToProto(mybase),
					ExecutionId:   &executionIDStr,
					LastPlanOnly:  true,
					Extra:         &structpb.Struct{},
				},
				result: PlanHistoryReq{
					ComponentName: mybase,
					ExecutionID:   executionID,
					LastPlanOnly:  true,
					Extra:         map[string]interface{}{},
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res, err := getPlanRequestFromProto(tc.input)

				if tc.err != nil {
					test.That(t, err, test.ShouldBeError, tc.err)
				} else {
					test.That(t, err, test.ShouldBeNil)
				}
				test.That(t, res, test.ShouldResemble, tc.result)
			})
		}
	})
}

func validMoveOnGlobeRequest() MoveOnGlobeReq {
	dst := geo.NewPoint(1, 2)
	visionCameraPairs := [][]resource.Name{
		{vision.Named("vision service 1"), camera.Named("camera 1")},
		{vision.Named("vision service 2"), camera.Named("camera 2")},
	}
	obstacleDetectors := []ObstacleDetectorName{}
	for _, pair := range visionCameraPairs {
		obstacleDetectors = append(obstacleDetectors, ObstacleDetectorName{
			VisionServiceName: pair[0],
			CameraName:        pair[1],
		})
	}
	return MoveOnGlobeReq{
		ComponentName:      base.Named("my-base"),
		Destination:        dst,
		Heading:            0.5,
		MovementSensorName: movementsensor.Named("my-movementsensor"),
		Obstacles:          nil,
		MotionCfg: &MotionConfiguration{
			ObstacleDetectors:     obstacleDetectors,
			LinearMPerSec:         1,
			AngularDegsPerSec:     2,
			PlanDeviationMM:       3,
			PositionPollingFreqHz: 4,
			ObstaclePollingFreqHz: 5,
		},
		Extra: nil,
	}
}
