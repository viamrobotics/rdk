package operation

import (
	"context"
	"sync"
	"time"

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
	theOp.waitCh = make(chan bool)
	sm.currentOp = theOp
	sm.mu.Unlock()

	return theOp.ctx, func() {
		if !theOp.closed {
			close(theOp.waitCh)
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

// IsPoweredInterface is a utility so can wait on IsPowered easily.
type IsPoweredInterface interface {
	IsPowered(ctx context.Context) (bool, error)
}

// WaitTillNotPowered waits until IsPowered returns false.
func (sm *SingleOperationManager) WaitTillNotPowered(ctx context.Context, pollTime time.Duration, powered IsPoweredInterface) error {
	return sm.WaitForSuccess(
		ctx,
		pollTime,
		func(ctx context.Context) (bool, error) {
			res, err := powered.IsPowered(ctx)
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
	<-op.waitCh

	sm.currentOp = nil
}

type anOp struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
	waitCh     chan bool
	closed     bool
}

