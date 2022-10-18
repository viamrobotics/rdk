package operation

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/multierr"
	"go.viam.com/utils"
)

// SingleOperationManager ensures only 1 operation is happening a time
// An operation can be nested, so if there is already an operation in progress,
// it can have sub-operations without an issue.
type SingleOperationManager struct {
	mu        sync.Mutex
	currentOp *anOp
}

// CancelRunning cancel's a current operation unless it's mine.
func (sm *SingleOperationManager) CancelRunning(ctx context.Context) {
	if ctx.Value(somCtxKeySingleOp) != nil {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.cancelInLock(ctx)
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
	// handle nested ops
	if ctx.Value(somCtxKeySingleOp) != nil {
		return ctx, func() {}
	}

	sm.mu.Lock()

	// first cancel any old operation
	sm.cancelInLock(ctx)

	theOp := &anOp{}

	ctx = context.WithValue(ctx, somCtxKeySingleOp, theOp)

	theOp.ctx, theOp.cancelFunc = context.WithCancel(ctx)
	sm.currentOp = theOp
	sm.mu.Unlock()

	return theOp.ctx, func() {
		if !theOp.closed {
			theOp.closed = true
		}
		sm.mu.Lock()
		if theOp == sm.currentOp {
			sm.currentOp = nil
		}
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
		var errStop error
		if errors.Is(ctx.Err(), context.Canceled) {
			sm.mu.Lock()
			oldOp := sm.currentOp == ctx.Value(somCtxKeySingleOp)
			sm.mu.Unlock()

			if oldOp || sm.currentOp == nil {
				errStop = stop(ctx, map[string]interface{}{})
			}
		}
		err = multierr.Combine(ctx.Err(), errStop)
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

func (sm *SingleOperationManager) cancelInLock(ctx context.Context) {
	myOp := ctx.Value(somCtxKeySingleOp)
	op := sm.currentOp

	if op == nil || myOp == op {
		return
	}

	op.cancelFunc()

	sm.currentOp = nil
}

type anOp struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
	closed     bool
}
