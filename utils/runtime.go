package utils

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/edaniels/golog"
)

// ContextualMain calls a main entry point function with a cancellable
// context via SIGTERM. This should be called once per process so as
// to not clobber the signals from Notify.
func ContextualMain(main func(ctx context.Context, args []string) error) {
	contextualMain(main, false)
}

// ContextualMainQuit is the same as ContextualMain but catches quit signals into the provided
// context accessed via ContextMainQuitSignal.
func ContextualMainQuit(main func(ctx context.Context, args []string) error) {
	contextualMain(main, true)
}

func contextualMain(main func(ctx context.Context, args []string) error, quitSignal bool) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if quitSignal {
		quitC := make(chan os.Signal, 1)
		signal.Notify(quitC, syscall.SIGQUIT)
		ctx = ContextWithQuitSignal(ctx, quitC)
	}
	readyC := make(chan struct{})
	ctx = ContextWithReadyFunc(ctx, readyC)
	if err := main(ctx, os.Args); err != nil {
		golog.Global.Fatal(err)
	}
}

type ctxKey int

const (
	ctxKeyQuitSignaler = ctxKey(iota)
	ctxKeyReadyFunc
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

// ContextMainReadyFunc returns a signal channel for indicating readiness. This
// is intended for main functions that block forever (e.g. daemons). It may
// be nil if the value was never set.
func ContextMainReadyFunc(ctx context.Context) func() {
	signaler := ctx.Value(ctxKeyReadyFunc)
	if signaler == nil {
		return func() {}
	}
	return signaler.(func())
}
