package testutils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"testing"

	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"go.uber.org/zap/zaptest/observer"
)

// ContextualMainExecution reflects the execution of a main function
// that can have its lifecycel partially controlled.
type ContextualMainExecution struct {
	Ready       <-chan struct{}
	Done        <-chan error
	Start       func()
	Stop        func()
	QuitSignal  func(t *testing.T)             // reflects syscall.SIGQUIT
	ExpectIters func(t *testing.T, amount int) // expect a certain amount of iters
	WaitIters   func(t *testing.T)             // waits for iters defined by ExpectIters
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

	iterC := make(chan struct{})
	ctx = utils.ContextWithIterFunc(ctx, func() {
		iterC <- struct{}{}
	})
	doneC := make(chan error, 1)

	mainDone := make(chan struct{})
	var err error
	var startOnce sync.Once
	start := func() {
		startOnce.Do(func() {
			go func() {
				// if main is not a daemon like function or does not error out, just be "ready"
				// after execution is complete.
				defer readyF()
				defer close(mainDone)
				err = main(ctx, append([]string{"main"}, args...))
				doneC <- err
			}()
		})
	}
	earlyDone := func(t *testing.T) {
		t.Helper()
		if err != nil { // safe to check as we synchronize on mainDone
			fatalf(t, "main function completed while waiting with error: %v", err)
		} else {
			fatal(t, "main function completed while waiting")
		}
	}

	var waitIters chan struct{}
	var expectMu sync.Mutex
	closeDiscard := make(chan struct{})
	discardClosed := make(chan struct{})
	discardIter := func() {
		defer close(discardClosed)
		for {
			select {
			case <-closeDiscard:
				return
			default:
			}
			select {
			case <-closeDiscard:
			case <-mainDone:
				return
			case <-iterC:
			}
		}
	}
	go discardIter()

	return ContextualMainExecution{
		Ready: readyC,
		Done:  doneC,
		Start: start,
		Stop:  stop,
		QuitSignal: func(t *testing.T) {
			select {
			case <-mainDone:
				earlyDone(t)
				return
			case quitC <- syscall.SIGQUIT:
			}
		},
		ExpectIters: func(t *testing.T, amount int) {
			expectMu.Lock()
			started := make(chan struct{})
			close(closeDiscard)
			<-discardClosed
			go func() {
				defer expectMu.Unlock()
				waitIters = make(chan struct{})
				totalIters := 0
				expectingIters := amount
				close(started)

				for {
					select {
					case <-mainDone:
						earlyDone(t)
						return
					case <-iterC:
						totalIters++
					}
					if totalIters < expectingIters {
						continue
					}
					close(waitIters)

					// swap to discard
					closeDiscard = make(chan struct{})
					discardClosed = make(chan struct{})
					go discardIter()
					return
				}
			}()
			<-started
		},
		WaitIters: func(t *testing.T) {
			<-waitIters
		},
	}
}

// MainTestCase describes how to execute a main function and what
// to expect from it.
type MainTestCase struct {
	Name   string
	Args   []string
	Err    string
	Before func(t *testing.T, logger golog.Logger, exec *ContextualMainExecution)
	During func(ctx context.Context, t *testing.T, exec *ContextualMainExecution)
	After  func(t *testing.T, logs *observer.ObservedLogs)
}

var (
	completedBeforeExpected    = "main function completed before expected"
	errCompletedBeforeExpected = errors.New(completedBeforeExpected)
)

// TestMain tests a main function with a series of test cases in serial.
func TestMain(t *testing.T, mainWithArgs func(ctx context.Context, args []string) error, tcs []MainTestCase) {
	for i, tc := range tcs {
		testCaseName := tc.Name
		if testCaseName == "" {
			testCaseName = fmt.Sprintf("%d", i)
		}
		t.Run(testCaseName, func(t *testing.T) {
			var logs *observer.ObservedLogs
			exec := ContextualMain(mainWithArgs, tc.Args)
			logger, logs := golog.NewObservedTestLogger(t)
			if tc.Before != nil {
				tc.Before(t, logger, &exec)
			}
			exec.Start()
			<-exec.Ready
			done := make(chan error)
			var waitingInDuring bool
			var waitingInDuringFailed bool
			var waitMu sync.Mutex
			if tc.During != nil {
				waitingInDuring = true
			}
			cancelCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var doneErr error
			go func() {
				err := utils.FilterOutError(<-exec.Done, context.Canceled)
				doneErr = err
				waitMu.Lock()
				if waitingInDuring {
					waitingInDuringFailed = true
					cancel()
					if err == nil {
						t.Error(errCompletedBeforeExpected)
					} else {
						t.Error(fmt.Errorf("%s with error: %w", completedBeforeExpected, doneErr))
					}
				}
				waitMu.Unlock()
				done <- err
			}()
			if tc.During != nil {
				tc.During(cancelCtx, t, &exec)
				waitMu.Lock()
				if waitingInDuringFailed {
					// covers the case where the writer of During does not
					// error themselves.
					defer waitMu.Unlock()
					err := <-done
					if err == nil {
						fatal(t, errCompletedBeforeExpected)
					} else {
						fatal(t, fmt.Errorf("%s with error: %w", completedBeforeExpected, doneErr))
					}
				}
				waitingInDuring = false
				waitMu.Unlock()
			}
			exec.Stop()
			err := <-done
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

func fatalf(t *testing.T, format string, args ...interface{}) {
	fatal(t, fmt.Sprintf(format, args...))
}

var fatal = func(t *testing.T, args ...interface{}) {
	t.Fatal(args...)
}
