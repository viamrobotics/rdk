package testutils

import (
	"context"
	"os"
	"syscall"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/robotcore/utils"
)

// ContextualMainExecution reflects the execution of a main function
// that can have its lifecycel partially controlled.
type ContextualMainExecution struct {
	Ready      <-chan struct{}
	Done       <-chan error
	Stop       func()
	QuitSignal func() // reflects syscall.SIGQUIT
}

// ContextualMain calls a main entry point function with a cancellable
// context via the returned execution struct. The main function is run
// in a separate goroutine.
func ContextualMain(main func(ctx context.Context, args []string) error, args []string) ContextualMainExecution {
	return contextualMain(main, args)
}

func contextualMain(main func(ctx context.Context, args []string) error, args []string) ContextualMainExecution {
	ctx, stop := context.WithCancel(context.Background())
	quitC := make(chan os.Signal)
	ctx = utils.ContextWithQuitSignal(ctx, quitC)
	readyC := make(chan struct{}, 1)
	ctx = utils.ContextWithReadyFunc(ctx, readyC)
	readyF := utils.ContextMainReadyFunc(ctx)
	doneC := make(chan error, 1)

	go func() {
		// if main is not a daemon like function or does not error out, just be "ready"
		// after execution is complete.
		defer readyF()
		doneC <- main(ctx, append([]string{"main"}, args...))
	}()
	return ContextualMainExecution{
		Ready: readyC,
		Done:  doneC,
		Stop:  stop,
		QuitSignal: func() {
			quitC <- syscall.SIGQUIT
		},
	}
}

// MainTestCase describes how to execute a main function and what
// to expect from it.
type MainTestCase struct {
	Name   string
	Args   []string
	Err    string
	Before func(logger golog.Logger)
	During func(exec *ContextualMainExecution)
	After  func(t *testing.T, logs *observer.ObservedLogs)
}

// TestMain tests a main function with a series of test cases in serial.
func TestMain(t *testing.T, mainWithArgs func(ctx context.Context, args []string) error, tcs []MainTestCase) {
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			var logs *observer.ObservedLogs
			logger, logs := golog.NewObservedTestLogger(t)
			if tc.Before != nil {
				tc.Before(logger)
			}
			exec := ContextualMain(mainWithArgs, tc.Args)
			<-exec.Ready

			if tc.During != nil {
				tc.During(&exec)
			}
			exec.Stop()
			err := <-exec.Done
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
			if tc.After != nil {
				tc.After(t, logs)
			}
		})
	}
}
