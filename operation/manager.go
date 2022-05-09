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
func (cm *SingleOperationManager) CancelRunning() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cancelInLock()
}

// OpRunning returns if there is a current operation.
func (cm *SingleOperationManager) OpRunning() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.currentOp != nil
}

// New creates a new operation, cancels previous, returns a new context and function to call when done.
func (cm *SingleOperationManager) New(ctx context.Context) (context.Context, func()) {
	type ctxKey byte
	const ctxKeySingleOp = ctxKey(iota)

	// handle nested ops
	if ctx.Value(ctxKeySingleOp) != nil {
		return ctx, func() {}
	}

	cm.mu.Lock()

	// first cancel any old operation
	cm.cancelInLock()

	theOp := &anOp{}

	ctx = context.WithValue(ctx, ctxKeySingleOp, theOp)

	theOp.ctx, theOp.cancelFunc = context.WithCancel(ctx)
	theOp.waitCh = make(chan bool)
	cm.currentOp = theOp
	cm.mu.Unlock()

	return theOp.ctx, func() {
		if !theOp.closed {
			close(theOp.waitCh)
			theOp.closed = true
		}
		cm.mu.Lock()
		if theOp == cm.currentOp {
			cm.currentOp = nil
		}
		cm.mu.Unlock()
	}
}

// NewTimedWaitOp returns true if it finished, false if cancelled.
// If there are other operations pending, this will cancel them.
func (cm *SingleOperationManager) NewTimedWaitOp(ctx context.Context, dur time.Duration) bool {
	ctx, finish := cm.New(ctx)
	defer finish()

	return utils.SelectContextOrWait(ctx, dur)
}

// WaitForSuccess will call testFunc every pollTime until it returns true or an error.
func (cm *SingleOperationManager) WaitForSuccess(
	ctx context.Context,
	pollTime time.Duration,
	testFunc func(ctx context.Context) (bool, error)) error {
	ctx, finish := cm.New(ctx)
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

func (cm *SingleOperationManager) cancelInLock() {
	op := cm.currentOp

	if op == nil {
		return
	}
	op.cancelFunc()
	<-op.waitCh

	cm.currentOp = nil
}

type anOp struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
	waitCh     chan bool
	closed     bool
}

