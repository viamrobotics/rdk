// Package utils contains all utility functions that currently have no better home than here.
package utils

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/rlog"
)

// ContextualMain calls a main entry point function with a cancellable
// context via SIGTERM. This should be called once per process so as
// to not clobber the signals from Notify.
func ContextualMain(main func(ctx context.Context, args []string, logger golog.Logger) error, logger golog.Logger) {
	contextualMain(main, false, logger)
}

// ContextualMainQuit is the same as ContextualMain but catches quit signals into the provided
// context accessed via ContextMainQuitSignal.
func ContextualMainQuit(main func(ctx context.Context, args []string, logger golog.Logger) error, logger golog.Logger) {
	contextualMain(main, true, logger)
}

func contextualMain(main func(ctx context.Context, args []string, logger golog.Logger) error, quitSignal bool, logger golog.Logger) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if quitSignal {
		quitC := make(chan os.Signal, 1)
		signal.Notify(quitC, syscall.SIGQUIT)
		ctx = ContextWithQuitSignal(ctx, quitC)
	}
	readyC := make(chan struct{})
	ctx = ContextWithReadyFunc(ctx, readyC)
	if err := FilterOutError(main(ctx, os.Args, logger), context.Canceled); err != nil {
		fatal(logger, err)
	}
}

var fatal = func(logger golog.Logger, args ...interface{}) {
	logger.Fatal(args...)
}

type ctxKey int

const (
	ctxKeyQuitSignaler = ctxKey(iota)
	ctxKeyReadyFunc
	ctxKeyIterFunc
)

// ContextWithQuitSignal attaches a quit signaler to the given context.
func ContextWithQuitSignal(ctx context.Context, c <-chan os.Signal) context.Context {
	return context.WithValue(ctx, ctxKeyQuitSignaler, c)
}

// ContextMainQuitSignal returns a signal channel for quits. It may
// be nil if the value was never set.
func ContextMainQuitSignal(ctx context.Context) <-chan os.Signal {
	signaler := ctx.Value(ctxKeyQuitSignaler)
	if signaler == nil {
		return nil
	}
	return signaler.(<-chan os.Signal)
}

// ContextWithReadyFunc attaches a ready signaler to the given context.
func ContextWithReadyFunc(ctx context.Context, c chan<- struct{}) context.Context {
	closeOnce := sync.Once{}
	return context.WithValue(ctx, ctxKeyReadyFunc, func() {
		closeOnce.Do(func() {
			close(c)
		})
	})
}

// ContextMainReadyFunc returns a function for indicating readiness. This
// is intended for main functions that block forever (e.g. daemons).
func ContextMainReadyFunc(ctx context.Context) func() {
	signaler := ctx.Value(ctxKeyReadyFunc)
	if signaler == nil {
		return func() {}
	}
	return signaler.(func())
}

// ContextWithIterFunc attaches an iteration func to the given context.
func ContextWithIterFunc(ctx context.Context, f func()) context.Context {
	return context.WithValue(ctx, ctxKeyIterFunc, f)
}

// ContextMainIterFunc returns a function for indicating an iteration of the
// program has completed.
func ContextMainIterFunc(ctx context.Context) func() {
	iterFunc := ctx.Value(ctxKeyIterFunc)
	if iterFunc == nil {
		return func() {}
	}
	return iterFunc.(func())
}

// PanicCapturingGo spawns a goroutine to run the given function and captures
// any panic that occurs and logs it.
func PanicCapturingGo(f func()) {
	PanicCapturingGoWithCallback(f, nil)
}

const waitDur = 3 * time.Second

// PanicCapturingGoWithCallback spawns a goroutine to run the given function and captures
// any panic that occurs, logs it, and calls the given callback. The callback can be
// used for restart functionality.
func PanicCapturingGoWithCallback(f func(), callback func(err interface{})) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				rlog.Logger.Errorw("panic while running function", "error", err)
				if callback == nil {
					return
				}
				rlog.Logger.Infow("waiting a bit to call callback", "wait", waitDur.String())
				time.Sleep(waitDur)
				callback(err)
			}
		}()
		f()
	}()
}

// ManagedGo keeps the given function alive in the background until
// it terminates normally.
func ManagedGo(f func(), onComplete func()) {
	PanicCapturingGoWithCallback(func() {
		defer func() {
			if err := recover(); err == nil && onComplete != nil {
				onComplete()
			} else if err != nil {
				// re-panic
				panic(err)
			}
		}()
		f()
	}, func(_ interface{}) {
		ManagedGo(f, onComplete)
	})
}

// SelectContextOrWait either terminates because the given context is done
// or the given duration elapses. It returns true if the duration elapsed.
func SelectContextOrWait(ctx context.Context, dur time.Duration) bool {
	timer := time.NewTimer(dur)
	defer timer.Stop()
	return SelectContextOrWaitChan(ctx, timer.C)
}

// SelectContextOrWaitChan either terminates because the given context is done
// or the given time channel is received on. It returns true if the channel
// was received on.
func SelectContextOrWaitChan(ctx context.Context, c <-chan time.Time) bool {
	select {
	case <-ctx.Done():
		return false
	default:
	}
	select {
	case <-ctx.Done():
		return false
	case <-c:
	}
	return true
}
