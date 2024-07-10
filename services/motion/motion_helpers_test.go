package motion_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"

	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

func TestPollHistoryUntilSuccessOrError(t *testing.T) {
	ctx := context.Background()
	ms := inject.NewMotionService("my motion")
	t.Run("returns error if context is cancelled", func(t *testing.T) {
		cancelledCtx, cancelFn := context.WithCancel(context.Background())
		cancelFn()
		ms.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			t.Error("should not be called")
			t.FailNow()
			return nil, nil
		}
		err := motion.PollHistoryUntilSuccessOrError(cancelledCtx, ms, time.Millisecond, motion.PlanHistoryReq{})
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	t.Run("returns error if PlanHistory returns an error", func(t *testing.T) {
		errExpected := errors.New("some error")
		ms.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			return nil, errExpected
		}
		err := motion.PollHistoryUntilSuccessOrError(ctx, ms, time.Millisecond, motion.PlanHistoryReq{})
		test.That(t, err, test.ShouldBeError, errExpected)
	})

	t.Run("returns an error if PlanHistory returns a most recent plan which is in an invalid state", func(t *testing.T) {
		errExpected := errors.New("invalid plan state 0")
		ms.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			return []motion.PlanWithStatus{{StatusHistory: []motion.PlanStatus{{State: motion.PlanStateUnspecified}}}}, nil
		}
		err := motion.PollHistoryUntilSuccessOrError(ctx, ms, time.Millisecond, motion.PlanHistoryReq{})
		test.That(t, err, test.ShouldBeError, errExpected)
	})

	t.Run("returns an error if PlanHistory returns a most recent plan which is in Stopped state", func(t *testing.T) {
		errExpected := errors.New("plan stopped")
		ms.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			return []motion.PlanWithStatus{{StatusHistory: []motion.PlanStatus{{State: motion.PlanStateStopped}}}}, nil
		}
		err := motion.PollHistoryUntilSuccessOrError(ctx, ms, time.Millisecond, motion.PlanHistoryReq{})
		test.That(t, err, test.ShouldBeError, errExpected)
	})

	t.Run("returns an error with reason if PlanHistory returns a most recent plan which is in Failed state", func(t *testing.T) {
		reason := "this is the fail reason"
		errExpected := errors.Wrap(errors.New("plan failed"), reason)
		ms.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			return []motion.PlanWithStatus{{StatusHistory: []motion.PlanStatus{{State: motion.PlanStateFailed, Reason: &reason}}}}, nil
		}
		err := motion.PollHistoryUntilSuccessOrError(ctx, ms, time.Millisecond, motion.PlanHistoryReq{})
		test.That(t, err, test.ShouldBeError, errExpected)
	})

	t.Run("returns nil if PlanHistory returns a most recent plan which is in Succeeded state", func(t *testing.T) {
		ms.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			return []motion.PlanWithStatus{{StatusHistory: []motion.PlanStatus{{State: motion.PlanStateSucceeded}}}}, nil
		}
		err := motion.PollHistoryUntilSuccessOrError(ctx, ms, time.Millisecond, motion.PlanHistoryReq{})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("returns polls until a terminal state is reached", func(t *testing.T) {
		var callCount int
		ms.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			callCount++
			switch callCount {
			case 1:
				return []motion.PlanWithStatus{{StatusHistory: []motion.PlanStatus{{State: motion.PlanStateInProgress}}}}, nil
			case 2:
				return []motion.PlanWithStatus{{StatusHistory: []motion.PlanStatus{{State: motion.PlanStateSucceeded}}}}, nil
			default:
				t.Error("should not be called")
				t.FailNow()
				return nil, errors.New("should not happen")
			}
		}
		err := motion.PollHistoryUntilSuccessOrError(ctx, ms, time.Millisecond, motion.PlanHistoryReq{})
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestOOBArmMotion(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg := resource.Config{
		Name:  arm.API.String(),
		Model: resource.DefaultModelFamily.WithModel("ur5e"),
		ConvertedAttributes: &fake.Config{
			ArmModel: "ur5e",
		},
	}

	// instantiate out of bounds arm
	notReal, err := fake.NewArm(context.Background(), nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	injectedArm := &inject.Arm{
		Arm: notReal,
		JointPositionsFunc: func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
			return &pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 720}}, nil
		},
	}

	t.Run("EndPosition works when OOB", func(t *testing.T) {
		jPositions := pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 720}}
		pose, err := motionplan.ComputeOOBPosition(injectedArm.ModelFrame(), &jPositions)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pose, test.ShouldNotBeNil)
	})

	t.Run("MoveArm fails when OOB", func(t *testing.T) {
		pose := spatialmath.NewPoseFromPoint(r3.Vector{200, 200, 200})
		err := motion.MoveArm(context.Background(), logger, injectedArm, pose)
		test.That(t, err.Error(), test.ShouldContain, referenceframe.OOBErrString)
	})

	t.Run("MoveToJointPositions fails when OOB", func(t *testing.T) {
		err := injectedArm.MoveToJointPositions(context.Background(), &pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 0}}, nil)
		test.That(t, err.Error(), test.ShouldContain, referenceframe.OOBErrString)
	})
}
