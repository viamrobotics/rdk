package motion_test

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"

	// registers all components.

	"go.viam.com/test"

	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/services/motion"
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

	t.Run("returns polls until a termianl state is reached", func(t *testing.T) {
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
