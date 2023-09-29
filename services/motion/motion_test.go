package motion

import (
	"math"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/movementsensor"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
)

func TestPlanWithStatus(t *testing.T) {
	planID, err := uuid.NewUUID()
	test.That(t, err, test.ShouldBeNil)

	executionID, err := uuid.NewUUID()
	test.That(t, err, test.ShouldBeNil)

	baseName := base.Named("my-base1")
	poseA := spatialmath.NewZeroPose()
	poseB := spatialmath.NewPose(r3.Vector{X: 100}, spatialmath.NewOrientationVector())

	timestamp := time.Now().UTC()
	timestampb := timestamppb.New(timestamp)
	reason := "some reason"

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
				input:       &pb.PlanWithStatus{Plan: Plan{}.ToProto()},
				result:      PlanWithStatus{},
				err:         errors.New("received nil *pb.PlanStatus"),
			},
			{
				description: "nil pointers in the status history returns an error",
				input: &pb.PlanWithStatus{
					Plan:          Plan{}.ToProto(),
					Status:        PlanStatus{}.ToProto(),
					StatusHistory: []*pb.PlanStatus{nil},
				},
				result: PlanWithStatus{},
				err:    errors.New("received nil *pb.PlanStatus"),
			},
			{
				description: "empty *pb.PlanWithStatus status returns an empty PlanWithStatus",
				input: &pb.PlanWithStatus{
					Plan:   Plan{}.ToProto(),
					Status: PlanStatus{}.ToProto(),
				},
				result: PlanWithStatus{
					Plan:          Plan{},
					StatusHistory: []PlanStatus{{}},
				},
			},
			{
				description: "full *pb.PlanWithStatus status returns a full PlanWithStatus",
				input: &pb.PlanWithStatus{
					Plan: &pb.Plan{
						Id:            planID.String(),
						ExecutionId:   executionID.String(),
						ComponentName: rprotoutils.ResourceNameToProto(baseName),
						Steps: []*pb.PlanStep{
							{
								Step: map[string]*pb.ComponentState{
									baseName.String(): {Pose: spatialmath.PoseToProtobuf(poseA)},
								},
							},
							{
								Step: map[string]*pb.ComponentState{
									baseName.String(): {Pose: spatialmath.PoseToProtobuf(poseB)},
								},
							},
						},
					},
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
					Plan: Plan{
						ID:            planID,
						ExecutionID:   executionID,
						ComponentName: baseName,
						Steps: []PlanStep{
							map[resource.Name]spatialmath.Pose{baseName: poseA},
							map[resource.Name]spatialmath.Pose{baseName: poseB},
						},
					},
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
				result:      &pb.PlanWithStatus{Plan: Plan{}.ToProto()},
			},
			{
				description: "full PlanWithStatus without status history returns a full *pb.PlanWithStatus",
				input: PlanWithStatus{
					Plan: Plan{
						ID:            planID,
						ExecutionID:   executionID,
						ComponentName: baseName,
						Steps: []PlanStep{
							map[resource.Name]spatialmath.Pose{baseName: poseA},
							map[resource.Name]spatialmath.Pose{baseName: poseB},
						},
					},
					StatusHistory: []PlanStatus{
						{State: PlanStateInProgress, Timestamp: timestamp},
					},
				},
				result: &pb.PlanWithStatus{
					Plan: &pb.Plan{
						Id:            planID.String(),
						ExecutionId:   executionID.String(),
						ComponentName: rprotoutils.ResourceNameToProto(baseName),
						Steps: []*pb.PlanStep{
							{
								Step: map[string]*pb.ComponentState{
									baseName.String(): {Pose: spatialmath.PoseToProtobuf(poseA)},
								},
							},
							{
								Step: map[string]*pb.ComponentState{
									baseName.String(): {Pose: spatialmath.PoseToProtobuf(poseB)},
								},
							},
						},
					},
					Status: &pb.PlanStatus{
						State:     pb.PlanState_PLAN_STATE_IN_PROGRESS,
						Timestamp: timestampb,
					},
				},
			},
			{
				description: "full PlanWithStatus with status history returns a full *pb.PlanWithStatus",
				input: PlanWithStatus{
					Plan: Plan{
						ID:            planID,
						ExecutionID:   executionID,
						ComponentName: baseName,
						Steps: []PlanStep{
							map[resource.Name]spatialmath.Pose{baseName: poseA},
							map[resource.Name]spatialmath.Pose{baseName: poseB},
						},
					},
					StatusHistory: []PlanStatus{
						{State: PlanStateFailed, Timestamp: timestamp, Reason: &reason},
						{State: PlanStateInProgress, Timestamp: timestamp},
					},
				},
				result: &pb.PlanWithStatus{
					Plan: &pb.Plan{
						Id:            planID.String(),
						ExecutionId:   executionID.String(),
						ComponentName: rprotoutils.ResourceNameToProto(baseName),
						Steps: []*pb.PlanStep{
							{
								Step: map[string]*pb.ComponentState{
									baseName.String(): {Pose: spatialmath.PoseToProtobuf(poseA)},
								},
							},
							{
								Step: map[string]*pb.ComponentState{
									baseName.String(): {Pose: spatialmath.PoseToProtobuf(poseB)},
								},
							},
						},
					},
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

		id, err := uuid.NewUUID()
		test.That(t, err, test.ShouldBeNil)

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

		id, err := uuid.NewUUID()
		test.That(t, err, test.ShouldBeNil)

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
	t.Run("planFromProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       *pb.Plan
			result      Plan
			err         error
		}

		planID, err := uuid.NewUUID()
		test.That(t, err, test.ShouldBeNil)

		executionID, err := uuid.NewUUID()
		test.That(t, err, test.ShouldBeNil)

		baseName := base.Named("my-base1")
		poseA := spatialmath.NewZeroPose()
		poseB := spatialmath.NewPose(r3.Vector{X: 100}, spatialmath.NewOrientationVector())

		testCases := []testCase{
			{
				description: "nil pointer returns error",
				input:       nil,
				result:      Plan{},
				err:         errors.New("received nil *pb.Plan"),
			},
			{
				description: "empty PlanID in *pb.Plan{} returns an error",
				input:       &pb.Plan{},
				result:      Plan{},
				err:         errors.New("invalid UUID length: 0"),
			},
			{
				description: "empty ExecutionID in *pb.Plan{} returns an error",
				input:       &pb.Plan{Id: planID.String()},
				result:      Plan{},
				err:         errors.New("invalid UUID length: 0"),
			},
			{
				description: "empty ComponentName in *pb.Plan{} returns an error",
				input:       &pb.Plan{Id: planID.String(), ExecutionId: executionID.String()},
				result:      Plan{},
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
				result: Plan{},
				err:    errors.New("received nil *pb.PlanStep"),
			},
			{
				description: "success case for empty steps",
				input: &pb.Plan{
					Id:            planID.String(),
					ExecutionId:   executionID.String(),
					ComponentName: rprotoutils.ResourceNameToProto(resource.Name{}),
				},
				result: Plan{
					ID:            planID,
					ExecutionID:   executionID,
					ComponentName: resource.Name{},
				},
			},
			{
				description: "success case for full steps",
				input: &pb.Plan{
					Id:            planID.String(),
					ExecutionId:   executionID.String(),
					ComponentName: rprotoutils.ResourceNameToProto(baseName),
					Steps: []*pb.PlanStep{
						{
							Step: map[string]*pb.ComponentState{
								baseName.String(): {Pose: spatialmath.PoseToProtobuf(poseA)},
							},
						},
						{
							Step: map[string]*pb.ComponentState{
								baseName.String(): {Pose: spatialmath.PoseToProtobuf(poseB)},
							},
						},
					},
				},
				result: Plan{
					ID:            planID,
					ExecutionID:   executionID,
					ComponentName: baseName,
					Steps: []PlanStep{
						map[resource.Name]spatialmath.Pose{baseName: poseA},
						map[resource.Name]spatialmath.Pose{baseName: poseB},
					},
				},
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
			input       Plan
			result      *pb.Plan
		}

		planID, err := uuid.NewUUID()
		test.That(t, err, test.ShouldBeNil)

		executionID, err := uuid.NewUUID()
		test.That(t, err, test.ShouldBeNil)

		baseName := base.Named("my-base1")
		poseA := spatialmath.NewZeroPose()
		poseB := spatialmath.NewPose(r3.Vector{X: 100}, spatialmath.NewOrientationVector())

		testCases := []testCase{
			{
				description: "an empty Plan returns an empty *pb.Plan",
				input:       Plan{},
				result: &pb.Plan{
					Id:            uuid.Nil.String(),
					ComponentName: rprotoutils.ResourceNameToProto(resource.Name{}),
					ExecutionId:   uuid.Nil.String(),
				},
			},
			{
				description: "full Plan returns full *pb.Plan",
				input: Plan{
					ID:            planID,
					ExecutionID:   executionID,
					ComponentName: baseName,
					Steps: []PlanStep{
						map[resource.Name]spatialmath.Pose{baseName: poseA},
						map[resource.Name]spatialmath.Pose{baseName: poseB},
					},
				},
				result: &pb.Plan{
					Id:            planID.String(),
					ExecutionId:   executionID.String(),
					ComponentName: rprotoutils.ResourceNameToProto(baseName),
					Steps: []*pb.PlanStep{
						{
							Step: map[string]*pb.ComponentState{
								baseName.String(): {Pose: spatialmath.PoseToProtobuf(poseA)},
							},
						},
						{
							Step: map[string]*pb.ComponentState{
								baseName.String(): {Pose: spatialmath.PoseToProtobuf(poseB)},
							},
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

func TestPlanStep(t *testing.T) {
	baseNameA := base.Named("my-base1")
	baseNameB := base.Named("my-base2")
	poseA := spatialmath.NewZeroPose()
	poseB := spatialmath.NewPose(r3.Vector{X: 100}, spatialmath.NewOrientationVector())

	t.Run("planStepFromProto", func(t *testing.T) {
		type testCase struct {
			description string
			input       *pb.PlanStep
			result      PlanStep
			err         error
		}

		testCases := []testCase{
			{
				description: "nil pointer returns an error",
				input:       nil,
				result:      PlanStep{},
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
				result: PlanStep{},
				err:    errors.New("string \"invalid component name\" is not a valid resource name"),
			},
			{
				description: "an empty *pb.PlanStep returns an empty PlanStep{}",
				input:       &pb.PlanStep{},
				result:      PlanStep{},
			},
			{
				description: "a full *pb.PlanStep returns an full PlanStep{}",
				input: &pb.PlanStep{
					Step: map[string]*pb.ComponentState{
						baseNameA.String(): {Pose: spatialmath.PoseToProtobuf(poseA)},
						baseNameB.String(): {Pose: spatialmath.PoseToProtobuf(poseB)},
					},
				},
				result: map[resource.Name]spatialmath.Pose{
					baseNameA: poseA,
					baseNameB: poseB,
				},
			},
		}
		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				res, err := planStepFromProto(tc.input)
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
			input       PlanStep
			result      *pb.PlanStep
		}

		testCases := []testCase{
			{
				description: "an nil PlanStep returns an empty *pb.PlanStep",
				input:       nil,
				result:      &pb.PlanStep{},
			},
			{
				description: "an empty PlanStep returns an empty *pb.PlanStep",
				input:       PlanStep{},
				result:      &pb.PlanStep{},
			},
			{
				description: "a full PlanStep{} returns an full *pb.PlanStep",
				input: map[resource.Name]spatialmath.Pose{
					baseNameA: poseA,
					baseNameB: poseB,
				},
				result: &pb.PlanStep{
					Step: map[string]*pb.ComponentState{
						baseNameA.String(): {Pose: spatialmath.PoseToProtobuf(poseA)},
						baseNameB.String(): {Pose: spatialmath.PoseToProtobuf(poseB)},
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

func TestConfiguration(t *testing.T) {
	t.Run("configurationFromProto", func(t *testing.T) {
		t.Run("returns mostly empty struct when passed a nil pointer", func(t *testing.T) {
			test.That(t, configurationFromProto(nil), test.ShouldResemble, &MotionConfiguration{VisionServices: []resource.Name{}})
		})

		t.Run("returns non empty struct", func(t *testing.T) {
			test.That(t, configurationFromProto(nil), test.ShouldResemble, &MotionConfiguration{VisionServices: []resource.Name{}})
		})
	})

	t.Run("toProto", func(t *testing.T) {
		visionServices := []resource.Name{vision.Named("vision service 1"), vision.Named("vision service 2")}
		cfg := &MotionConfiguration{
			VisionServices:        visionServices,
			LinearMPerSec:         1,
			AngularDegsPerSec:     2,
			PlanDeviationMM:       3,
			PositionPollingFreqHz: 4,
			ObstaclePollingFreqHz: 5,
		}
		mCfg := cfg.toProto()

		test.That(t, len(mCfg.VisionServices), test.ShouldEqual, 2)
		test.That(t, mCfg.VisionServices[0].Name, test.ShouldResemble, "vision service 1")
		test.That(t, mCfg.VisionServices[1].Name, test.ShouldResemble, "vision service 2")
		test.That(t, mCfg.LinearMPerSec, test.ShouldNotBeNil)
		test.That(t, *mCfg.LinearMPerSec, test.ShouldEqual, 1)
		test.That(t, mCfg.AngularDegsPerSec, test.ShouldNotBeNil)
		test.That(t, *mCfg.AngularDegsPerSec, test.ShouldEqual, 2)
		test.That(t, mCfg.PlanDeviationM, test.ShouldNotBeNil)
		test.That(t, *mCfg.PlanDeviationM, test.ShouldAlmostEqual, 0.003)
		test.That(t, mCfg.PositionPollingFrequencyHz, test.ShouldNotBeNil)
		test.That(t, *mCfg.PositionPollingFrequencyHz, test.ShouldEqual, 4)
		test.That(t, mCfg.ObstaclePollingFrequencyHz, test.ShouldNotBeNil)
		test.That(t, *mCfg.ObstaclePollingFrequencyHz, test.ShouldEqual, 5)
	})
}

func TestMoveOnGlobeRequest(t *testing.T) {
	name := "somename"
	//nolint:dupl
	t.Run("toProto", func(t *testing.T) {
		t.Run("error due to nil destination", func(t *testing.T) {
			mogReq := validMoveOnGlobeRequest()
			mogReq.Destination = nil
			_, err := mogReq.ToProto(name)
			test.That(t, err, test.ShouldBeError, errors.New("Must provide a destination"))
		})

		t.Run("error due to nil motion config", func(t *testing.T) {
			mogReq := validMoveOnGlobeRequest()
			mogReq.MotionCfg = nil
			_, err := mogReq.ToProto(name)
			test.That(t, err, test.ShouldBeError, errors.New("Must provide a non nil motion configuration"))
		})

		t.Run("sets heading to nil if set to NaN", func(t *testing.T) {
			mogReq := validMoveOnGlobeRequest()
			mogReq.Heading = math.NaN()
			req, err := mogReq.ToProto(name)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, req.Heading, test.ShouldBeNil)
		})

		t.Run("success", func(t *testing.T) {
			mogReq := validMoveOnGlobeRequest()
			req, err := mogReq.ToProto(name)

			test.That(t, err, test.ShouldBeNil)
			test.That(t, req.Name, test.ShouldResemble, "somename")
			test.That(t, req.ComponentName.Name, test.ShouldResemble, "my-base")
			test.That(t, req.Destination.Latitude, test.ShouldAlmostEqual, 0)
			test.That(t, req.Destination.Longitude, test.ShouldAlmostEqual, 1e-5)
			test.That(t, req.Heading, test.ShouldNotBeNil)
			test.That(t, *req.Heading, test.ShouldAlmostEqual, 0.5)
			test.That(t, req.MovementSensorName.Name, test.ShouldResemble, "my-movementsensor")
			test.That(t, req.Obstacles, test.ShouldBeEmpty)
			test.That(t, req.MotionConfiguration, test.ShouldResemble, mogReq.MotionCfg.toProto())
			test.That(t, req.Extra.AsMap(), test.ShouldBeEmpty)
		})
	})

	//nolint:dupl
	t.Run("toProtoNew", func(t *testing.T) {
		t.Run("error due to nil destination", func(t *testing.T) {
			mogReq := validMoveOnGlobeRequest()
			mogReq.Destination = nil
			_, err := mogReq.ToProtoNew(name)
			test.That(t, err, test.ShouldBeError, errors.New("Must provide a destination"))
		})

		t.Run("error due to nil motion config", func(t *testing.T) {
			mogReq := validMoveOnGlobeRequest()
			mogReq.MotionCfg = nil
			_, err := mogReq.ToProtoNew(name)
			test.That(t, err, test.ShouldBeError, errors.New("Must provide a non nil motion configuration"))
		})

		t.Run("sets heading to nil if set to NaN", func(t *testing.T) {
			mogReq := validMoveOnGlobeRequest()
			mogReq.Heading = math.NaN()
			req, err := mogReq.ToProtoNew(name)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, req.Heading, test.ShouldBeNil)
		})

		t.Run("success", func(t *testing.T) {
			mogReq := validMoveOnGlobeRequest()
			req, err := mogReq.ToProtoNew(name)

			test.That(t, err, test.ShouldBeNil)
			test.That(t, req.Name, test.ShouldResemble, "somename")
			test.That(t, req.ComponentName.Name, test.ShouldResemble, "my-base")
			test.That(t, req.Destination.Latitude, test.ShouldAlmostEqual, 0)
			test.That(t, req.Destination.Longitude, test.ShouldAlmostEqual, 1e-5)
			test.That(t, req.Heading, test.ShouldNotBeNil)
			test.That(t, *req.Heading, test.ShouldAlmostEqual, 0.5)
			test.That(t, req.MovementSensorName.Name, test.ShouldResemble, "my-movementsensor")
			test.That(t, req.Obstacles, test.ShouldBeEmpty)
			test.That(t, req.MotionConfiguration, test.ShouldResemble, mogReq.MotionCfg.toProto())

			test.That(t, req.Extra.AsMap(), test.ShouldBeEmpty)
		})
	})
}

func validMoveOnGlobeRequest() MoveOnGlobeReq {
	gpsPoint := geo.NewPoint(0, 0)
	dst := geo.NewPoint(gpsPoint.Lat(), gpsPoint.Lng()+1e-5)

	visionServices := []resource.Name{vision.Named("vision service 1"), vision.Named("vision service 2")}
	return MoveOnGlobeReq{
		ComponentName:      base.Named("my-base"),
		Destination:        dst,
		Heading:            0.5,
		MovementSensorName: movementsensor.Named("my-movementsensor"),
		Obstacles:          nil,
		MotionCfg: &MotionConfiguration{
			VisionServices:        visionServices,
			LinearMPerSec:         1,
			AngularDegsPerSec:     2,
			PlanDeviationMM:       3,
			PositionPollingFreqHz: 4,
			ObstaclePollingFreqHz: 5,
		},
		Extra: nil,
	}
}
