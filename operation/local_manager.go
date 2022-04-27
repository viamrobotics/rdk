package operation

import (
	"context"
	"sync"
	"time"

	"go.viam.com/utils"
)

type LocalCallManager struct {
	mu sync.Mutex
	op *basic
}

func (cm *LocalCallManager) CancelRunning() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cancelInLock()
}

func (cm *LocalCallManager) New(ctx context.Context) (context.Context, func()) {

	cm.mu.Lock()

	// first cancel any old operation
	cm.cancelInLock()

	theOp := &basic{}
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


// return true if it finished, false if cancelled
func (cm *LocalCallManager) TimedWait(ctx context.Context, dur time.Duration) bool {
	ctx, finish := cm.New(ctx)
	defer finish()
	
	return utils.SelectContextOrWait(ctx, dur)
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
	waitCh chan bool
}

