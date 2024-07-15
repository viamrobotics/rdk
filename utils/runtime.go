package utils

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/edaniels/golog"
	"go.uber.org/goleak"
)

const (
	ctxKeyQuitSignaler = ctxKey(iota)
	ctxKeyReadyFunc
)

// Logger is used various parts of the package for informational/debugging purposes.
var Logger = golog.Global()

// Debug is helpful to turn on when the library isn't working quite right.
var Debug = false

// ILogger is a basic logging interface.
type ILogger interface {
	Debug(...interface{})
	Info(...interface{})
	Warn(...interface{})
	Fatal(...interface{})
}

// FindGoroutineLeaks finds any goroutine leaks after a program is done running. This
// should be used at the end of a main test run or a top-level process run.
func FindGoroutineLeaks(options ...goleak.Option) error {
	optsCopy := make([]goleak.Option, len(options))
	copy(optsCopy, options)
	optsCopy = append(optsCopy,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("github.com/desertbit/timer.timerRoutine"),              // gRPC uses this
		goleak.IgnoreTopFunction("github.com/letsencrypt/pebble/va.VAImpl.processTasks"), // no way to stop it,
	)
	return goleak.Find(optsCopy...)
}

// ContextualMain calls a main entry point function with a cancellable
// context via SIGTERM. This should be called once per process so as
// to not clobber the signals from Notify.
func ContextualMain[L ILogger](main func(ctx context.Context, args []string, logger L) error, logger L) {
	// This will only run on a successful exit due to the fatal error
	// logic in contextualMain.
	defer func() {
		if err := FindGoroutineLeaks(); err != nil {
			fmt.Fprintf(os.Stderr, "goroutine leak(s) detected: %v\n", err)
		}
	}()
	contextualMain(main, false, logger)
}

// ContextualMainQuit is the same as ContextualMain but catches quit signals into the provided
// context accessed via ContextMainQuitSignal.
func ContextualMainQuit[L ILogger](main func(ctx context.Context, args []string, logger L) error, logger L) {
	contextualMain(main, true, logger)
}

func contextualMain[L ILogger](main func(ctx context.Context, args []string, logger L) error, quitSignal bool, logger L) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if quitSignal {
		quitC := make(chan os.Signal, 1)
		signal.Notify(quitC, syscall.SIGQUIT)
		ctx = ContextWithQuitSignal(ctx, quitC)
	}
	usr1C := make(chan os.Signal, 1)
	notifySignals(usr1C)

	var signalWatcher sync.WaitGroup
	signalWatcher.Add(1)
	defer signalWatcher.Wait()
	defer stop()
	ManagedGo(func() {
		for {
			if !SelectContextOrWaitChan(ctx, usr1C) {
				return
			}
			buf := make([]byte, 1024)
			for {
				n := runtime.Stack(buf, true)
				if n < len(buf) {
					buf = buf[:n]
					break
				}
				buf = make([]byte, 2*len(buf))
			}
			logger.Warn(string(buf))
		}
	}, signalWatcher.Done)

	readyC := make(chan struct{})
	readyCtx := ContextWithReadyFunc(ctx, readyC)
	if err := FilterOutError(main(readyCtx, os.Args, logger), context.Canceled); err != nil {
		fatal(logger, err)
	}
}

// ContextWithQuitSignal attaches a quit signaler to the given context.
func ContextWithQuitSignal(ctx context.Context, c <-chan os.Signal) context.Context {
	return context.WithValue(ctx, ctxKeyQuitSignaler, c)
}

var fatal = func(logger ILogger, args ...interface{}) {
	logger.Fatal(args...)
}

const waitDur = 3 * time.Second

func PanicCapturingGoWithCallback(f func(), callback func(err interface{})) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				debug.PrintStack()
				golog.Global().Errorw("panic while running function", "error", err)
				if callback == nil {
					return
				}
				golog.Global().Infow("waiting a bit to call callback", "wait", waitDur.String())
				time.Sleep(waitDur)
				callback(err)
			}
		}()
		f()
	}()
}

// ManagedGo keeps the given function alive in the background until
// it terminates normally.
func ManagedGo(f, onComplete func()) {
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

// ContextWithReadyFunc attaches a ready signaler to the given context.
func ContextWithReadyFunc(ctx context.Context, c chan<- struct{}) context.Context {
	closeOnce := sync.Once{}
	return context.WithValue(ctx, ctxKeyReadyFunc, func() {
		closeOnce.Do(func() {
			close(c)
		})
	})
}

// SelectContextOrWaitChan either terminates because the given context is done
// or the given time channel is received on. It returns true if the channel
// was received on.
func SelectContextOrWaitChan[T any](ctx context.Context, c <-chan T) bool {
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
