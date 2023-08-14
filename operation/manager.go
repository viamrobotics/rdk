package operation

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/multierr"
	"go.viam.com/utils"
)

type anOp struct {
	// cancelAndWaitFunc waits until the `SingleOperationManager.currentOp` is empty. This will
	// interrupt any existing operations as necessary.
	cancelAndWaitFunc func()
	// Cancels the context of what's currently running an operation.
	interruptFunc context.CancelFunc
}

// SingleOperationManager ensures only 1 operation is happening at a time.
// An operation can be nested, so if there is already an operation in progress,
// it can have sub-operations without an issue.
type SingleOperationManager struct {
	mu         sync.Mutex
	opDoneCond *sync.Cond
	currentOp  *anOp
}

func NewSingleOperationManager() *SingleOperationManager {
	ret := &SingleOperationManager{}
	ret.opDoneCond = sync.NewCond(&ret.mu)
	return ret
}

// CancelRunning cancels a current operation unless it's mine.
func (sm *SingleOperationManager) CancelRunning(ctx context.Context) {
	if ctx.Value(somCtxKeySingleOp) != nil {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.currentOp != nil {
		sm.currentOp.cancelAndWaitFunc()
	}
}

// OpRunning returns if there is a current operation.
func (sm *SingleOperationManager) OpRunning() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.currentOp != nil
}

type somCtxKey byte

const somCtxKeySingleOp = somCtxKey(iota)

// New creates a new operation, cancels previous, returns a new context and function to call when done.
func (sm *SingleOperationManager) New(ctx context.Context) (context.Context, func()) {
	// Handle nested ops. Note an operation set on a context by one `SingleOperationManager` can be
	// observed on a different instance of a `SingleOperationManager`.
	if ctx.Value(somCtxKeySingleOp) != nil {
		return ctx, func() {}
	}

	sm.mu.Lock()

	// Cancel any existing operation. This blocks until the operation is completed.
	if sm.currentOp != nil {
		sm.currentOp.cancelAndWaitFunc()
	}

	theOp := &anOp{}

	ctx = context.WithValue(ctx, somCtxKeySingleOp, theOp)

	var newUserCtx context.Context
	newUserCtx, theOp.interruptFunc = context.WithCancel(ctx)
	theOp.cancelAndWaitFunc = func() {
		// Precondition: Caller must be holding `sm.mu`.
		//
		// If there are two threads competing to win a race, it's not sufficient to return once the
		// condition variable is signaled. We must re-check that a new operation didn't beat us to
		// getting the next operation slot.
		//
		// Ironically, "winning the race" in this scenario just means the "loser" is going to
		// immediately interrupt the winner. A future optimization could avoid this unnecessary
		// starting/stopping.
		for sm.currentOp != nil {
			sm.currentOp.interruptFunc()
			sm.opDoneCond.Wait()
		}
	}
	sm.currentOp = theOp
	sm.mu.Unlock()

	return newUserCtx, func() {
		sm.mu.Lock()
		sm.opDoneCond.Broadcast()
		sm.currentOp = nil
		sm.mu.Unlock()
	}
}

// NewTimedWaitOp returns true if it finished, false if cancelled.
// If there are other operations pending, this will cancel them.
func (sm *SingleOperationManager) NewTimedWaitOp(ctx context.Context, dur time.Duration) bool {
	ctx, finish := sm.New(ctx)
	defer finish()

	return utils.SelectContextOrWait(ctx, dur)
}

// IsPoweredInterface is a utility so can wait on IsPowered easily. It returns whether it is
// powered, the power percent (between 0 and 1, or between -1 and 1 for motors that support
// negative power), and any error that occurred while obtaining these.
type IsPoweredInterface interface {
	IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error)
}

// WaitTillNotPowered waits until IsPowered returns false.
func (sm *SingleOperationManager) WaitTillNotPowered(ctx context.Context, pollTime time.Duration, powered IsPoweredInterface,
	stop func(context.Context, map[string]interface{}) error,
) (err error) {
	// Defers a function that will stop and clean up if the context errors
	defer func(ctx context.Context) {
		if errors.Is(ctx.Err(), context.Canceled) {
			err = multierr.Combine(ctx.Err(), stop(ctx, map[string]interface{}{}))
		} else {
			err = ctx.Err()
		}
	}(ctx)
	return sm.WaitForSuccess(
		ctx,
		pollTime,
		func(ctx context.Context) (res bool, err error) {
			res, _, err = powered.IsPowered(ctx, nil)
			return !res, err
		},
	)
}

// WaitForSuccess will call testFunc every pollTime until it returns true or an error.
func (sm *SingleOperationManager) WaitForSuccess(
	ctx context.Context,
	pollTime time.Duration,
	testFunc func(ctx context.Context) (bool, error),
) error {
	ctx, finish := sm.New(ctx)
	defer finish()

	for {
		res, err := testFunc(ctx)
		if err != nil {
			return err
		}
		if res {
			return nil
		}

		if !utils.SelectContextOrWait(ctx, pollTime) {
			return ctx.Err()
		}
	}
}
