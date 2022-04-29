package operation

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/multierr"
	
	"go.viam.com/utils"
)

type NBCallManager struct {
	mu sync.Mutex
	op *basic
}

func (cm *NBCallManager) CancelRunning() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cancelInLock()
}

func (cm *NBCallManager) NewTimed(ctx context.Context, dur time.Duration, done func(), canceled func() ) NonBlockingReturn {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// first cancel any old operation
	cm.cancelInLock()

	theOp := &basic{}
	theOp.theContext, theOp.cancelFunc = context.WithCancel(context.Background())
	
	utils.PanicCapturingGo(func() {
		if utils.SelectContextOrWait(theOp.theContext, dur) {
			done()
		} else {
			canceled()
		}

		theOp.cancelFunc()

		cm.mu.Lock()
		defer cm.mu.Unlock()
		cm.op = nil
	})

	cm.op = theOp
	return cm.op
}

func (cm *NBCallManager) NewChecked(ctx context.Context, check func(ctx context.Context) (bool, error), canceled func() ) NonBlockingReturn {
	panic(1)
}

func (cm *NBCallManager) cancelInLock() {
	if cm.op == nil {
		return
	}
	cm.op.Cancel()
}

// ---

type NonBlockingReturn interface {
	Block(ctx context.Context) error
	Cancel()
}

type NoopNonBlockingReturn struct {}
func (*NoopNonBlockingReturn) Block(ctx context.Context) error{
	return nil
}
func (*NoopNonBlockingReturn) Cancel() {
}

// ---

func NewFuncNonBlockingReturn(ctx context.Context, f func(ctx context.Context) error) NonBlockingReturn {
	ctx, cancel := context.WithCancel(ctx)
	b := &basic{theContext: ctx, cancelFunc: cancel}
	go func() {
		b.err.Store(f(ctx))
	}()
	
	return b
}

// -----

type basic struct {
	theContext context.Context
	cancelFunc context.CancelFunc
	err atomic.Value
}

func (b *basic) getError() error {
	e := b.err.Load()
	if e == nil {
		return nil
	}
	return e.(error)
}

func (b *basic) Block(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return multierr.Combine(ctx.Err(), b.getError())
	case <-b.theContext.Done():
		return b.getError()
	}
}

func (b *basic) Cancel() {
	b.cancelFunc()
}
