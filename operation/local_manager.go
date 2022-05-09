package operation

import (
	"context"
	"sync"
	"time"

	"go.viam.com/utils"
)

// LocalCallManager ensures only 1 operation is happening a time
// An operation can be nested, so if there is already an operation in progress,
// it can have sub-operations without an issue.
type LocalCallManager struct {
	mu sync.Mutex
	op *basic
}

// CancelRunning cancel's a current operation.
func (cm *LocalCallManager) CancelRunning() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cancelInLock()
}

// OpRunning returns if there is a current operation.
func (cm *LocalCallManager) OpRunning() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.op != nil
}

type myContextKeyType string

const theContextKey = myContextKeyType("opkey")

// New creates a new operation, cancels previous, returns a new context and function to call when done.
func (cm *LocalCallManager) New(ctx context.Context) (context.Context, func()) {
	if ctx.Value(theContextKey) != nil {
		return ctx, func() {}
	}

	cm.mu.Lock()

	// first cancel any old operation
	cm.cancelInLock()

	theOp := &basic{}

	ctx = context.WithValue(ctx, theContextKey, theOp)

	theOp.theContext, theOp.cancelFunc = context.WithCancel(ctx)
	theOp.waitCh = make(chan bool)
	cm.op = theOp
	cm.mu.Unlock()

	return theOp.theContext, func() {
		close(theOp.waitCh)
		cm.mu.Lock()
		if theOp == cm.op {
			cm.op = nil
		}
		cm.mu.Unlock()
	}
}

// TimedWait returns true if it finished, false if cancelled.
func (cm *LocalCallManager) TimedWait(ctx context.Context, dur time.Duration) bool {
	ctx, finish := cm.New(ctx)
	defer finish()

	return utils.SelectContextOrWait(ctx, dur)
}

// WaitForSuccess will call testFunc ever pollTime until it returns true or an error.
func (cm *LocalCallManager) WaitForSuccess(
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

func (cm *LocalCallManager) cancelInLock() {
	op := cm.op

	if op == nil {
		return
	}
	op.cancelFunc()
	<-op.waitCh

	cm.op = nil
}

// ---

type basic struct {
	theContext context.Context
	cancelFunc context.CancelFunc
	waitCh     chan bool
}

