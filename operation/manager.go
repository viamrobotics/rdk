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

// CancelRunning cancel's a current operation.
func (sm *SingleOperationManager) CancelRunning() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.cancelInLock()
}

// OpRunning returns if there is a current operation.
func (sm *SingleOperationManager) OpRunning() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.currentOp != nil
}

// New creates a new operation, cancels previous, returns a new context and function to call when done.
func (sm *SingleOperationManager) New(ctx context.Context) (context.Context, func()) {
	type ctxKey byte
	const ctxKeySingleOp = ctxKey(iota)

	// handle nested ops
	if ctx.Value(ctxKeySingleOp) != nil {
		return ctx, func() {}
	}

	sm.mu.Lock()

	// first cancel any old operation
	sm.cancelInLock()

	theOp := &anOp{}

	ctx = context.WithValue(ctx, ctxKeySingleOp, theOp)

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

// WaitForSuccess will call testFunc every pollTime until it returns true or an error.
func (sm *SingleOperationManager) WaitForSuccess(
	ctx context.Context,
	pollTime time.Duration,
	testFunc func(ctx context.Context) (bool, error)) error {
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

func (sm *SingleOperationManager) cancelInLock() {
	op := sm.currentOp

	if op == nil {
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

