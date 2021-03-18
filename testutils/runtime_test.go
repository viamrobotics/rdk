package testutils

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

func TestContextualMain(t *testing.T) {
	var captured []interface{}
	prevFatal := fatal
	defer func() {
		fatal = prevFatal
	}()
	fatal = func(t *testing.T, args ...interface{}) {
		captured = args
	}

	err1 := errors.New("whoops")
	mainWithArgs := func(ctx context.Context, args []string) error {
		return err1
	}
	exec := ContextualMain(mainWithArgs, nil)
	<-exec.Ready
	exec.Stop()
	test.That(t, <-exec.Done, test.ShouldEqual, err1)
	test.That(t, captured, test.ShouldHaveLength, 0)

	exec = ContextualMain(mainWithArgs, nil)
	<-exec.Ready
	exec.QuitSignal(t)
	exec.Stop()
	test.That(t, <-exec.Done, test.ShouldEqual, err1)
	test.That(t, captured, test.ShouldHaveLength, 1)
	test.That(t, captured[0], test.ShouldContainSubstring, "while")
	test.That(t, captured[0], test.ShouldContainSubstring, "whoops")
	captured = nil

	mainWithArgs = func(ctx context.Context, args []string) error {
		return nil
	}
	exec = ContextualMain(mainWithArgs, nil)
	<-exec.Ready
	exec.Stop()
	test.That(t, <-exec.Done, test.ShouldBeNil)
	test.That(t, captured, test.ShouldHaveLength, 0)

	exec = ContextualMain(mainWithArgs, nil)
	<-exec.Ready
	exec.QuitSignal(t)
	exec.Stop()
	test.That(t, <-exec.Done, test.ShouldBeNil)
	test.That(t, captured, test.ShouldHaveLength, 1)
	test.That(t, captured[0], test.ShouldContainSubstring, "while")
	test.That(t, captured[0], test.ShouldNotContainSubstring, "whoops")
	captured = nil

	var capturedArgs []string
	mainWithArgs = func(ctx context.Context, args []string) error {
		capturedArgs = args
		utils.ContextMainReadyFunc(ctx)()
		<-utils.ContextMainQuitSignal(ctx)
		return nil
	}

	exec = ContextualMain(mainWithArgs, []string{"1", "2", "3"})
	<-exec.Ready
	exec.QuitSignal(t)
	exec.Stop()
	test.That(t, <-exec.Done, test.ShouldBeNil)
	test.That(t, captured, test.ShouldHaveLength, 0)
	test.That(t, capturedArgs, test.ShouldResemble, []string{"main", "1", "2", "3"})
	captured = nil

	mainWithArgs = func(ctx context.Context, args []string) error {
		capturedArgs = args
		utils.ContextMainReadyFunc(ctx)()
		<-utils.ContextMainQuitSignal(ctx)
		return err1
	}

	exec = ContextualMain(mainWithArgs, []string{"1", "2", "3"})
	<-exec.Ready
	exec.QuitSignal(t)
	exec.Stop()
	test.That(t, <-exec.Done, test.ShouldEqual, err1)
	test.That(t, captured, test.ShouldHaveLength, 0)
	test.That(t, capturedArgs, test.ShouldResemble, []string{"main", "1", "2", "3"})
	captured = nil
}

func TestTestMain(t *testing.T) {
	var captured []interface{}
	prevFatal := fatal
	defer func() {
		fatal = prevFatal
	}()
	fatal = func(t *testing.T, args ...interface{}) {
		captured = args
	}

	err1 := errors.New("whoops")
	var capturedArgs []string
	mainWithArgs := func(ctx context.Context, args []string) error {
		capturedArgs = args
		return err1
	}

	TestMain(t, mainWithArgs, []MainTestCase{
		{
			Name: "",
			Args: []string{"1", "2", "3"},
			Err:  err1.Error(),
			Before: func(t *testing.T, logger golog.Logger) {
				captured = nil
				test.That(t, logger, test.ShouldNotBeNil)
			},
			During: func(ctx context.Context, t *testing.T, exec *ContextualMainExecution) {
				test.That(t, capturedArgs, test.ShouldResemble, []string{"main", "1", "2", "3"})
				test.That(t, exec, test.ShouldNotBeNil)
			},
			After: func(t *testing.T, logs *observer.ObservedLogs) {
				test.That(t, captured, test.ShouldBeNil)
				test.That(t, logs, test.ShouldNotBeNil)
			},
		},
		{
			Name: "next",
			Args: []string{"1", "2", "3"},
			Err:  err1.Error(),
			Before: func(t *testing.T, logger golog.Logger) {
				captured = nil
				test.That(t, logger, test.ShouldNotBeNil)
				logger.Info("hi")
			},
			During: func(ctx context.Context, t *testing.T, exec *ContextualMainExecution) {
				<-exec.Ready
				exec.QuitSignal(t)
				exec.Stop()
				test.That(t, capturedArgs, test.ShouldResemble, []string{"main", "1", "2", "3"})
				test.That(t, exec, test.ShouldNotBeNil)
			},
			After: func(t *testing.T, logs *observer.ObservedLogs) {
				test.That(t, captured, test.ShouldHaveLength, 1)
				test.That(t, captured[0], test.ShouldContainSubstring, "while")
				test.That(t, captured[0], test.ShouldContainSubstring, "whoops")
				test.That(t, logs, test.ShouldNotBeNil)
				test.That(t, logs.FilterMessage("hi").All(), test.ShouldHaveLength, 1)
			},
		},
	})

	mainWithArgs = func(ctx context.Context, args []string) error {
		capturedArgs = args
		utils.ContextMainReadyFunc(ctx)()
		<-utils.ContextMainQuitSignal(ctx)
		return err1
	}
	TestMain(t, mainWithArgs, []MainTestCase{
		{
			Name: "",
			Args: []string{"1", "2", "3"},
			Err:  err1.Error(),
			Before: func(t *testing.T, logger golog.Logger) {
				captured = nil
				test.That(t, logger, test.ShouldNotBeNil)
			},
			During: func(ctx context.Context, t *testing.T, exec *ContextualMainExecution) {
				<-exec.Ready
				exec.QuitSignal(t)
				exec.Stop()
				test.That(t, capturedArgs, test.ShouldResemble, []string{"main", "1", "2", "3"})
				test.That(t, exec, test.ShouldNotBeNil)
			},
			After: func(t *testing.T, logs *observer.ObservedLogs) {
				test.That(t, captured, test.ShouldBeNil)
				test.That(t, logs, test.ShouldNotBeNil)
			},
		},
		{
			Name: "next",
			Args: []string{"1", "2", "3"},
			Err:  err1.Error(),
			Before: func(t *testing.T, logger golog.Logger) {
				captured = nil
				test.That(t, logger, test.ShouldNotBeNil)
				logger.Info("hi")
			},
			During: func(ctx context.Context, t *testing.T, exec *ContextualMainExecution) {
				<-exec.Ready
				exec.QuitSignal(t)
				exec.Stop()
				test.That(t, capturedArgs, test.ShouldResemble, []string{"main", "1", "2", "3"})
				test.That(t, exec, test.ShouldNotBeNil)
			},
			After: func(t *testing.T, logs *observer.ObservedLogs) {
				test.That(t, captured, test.ShouldHaveLength, 0)
				test.That(t, logs, test.ShouldNotBeNil)
				test.That(t, logs.FilterMessage("hi").All(), test.ShouldHaveLength, 1)
			},
		},
	})

	mainWithArgs = func(ctx context.Context, args []string) error {
		capturedArgs = args
		utils.ContextMainReadyFunc(ctx)()
		<-utils.ContextMainQuitSignal(ctx)
		return nil
	}
	TestMain(t, mainWithArgs, []MainTestCase{
		{
			Name: "",
			Args: []string{"1", "2", "3"},
			Err:  "",
			Before: func(t *testing.T, logger golog.Logger) {
				captured = nil
				test.That(t, logger, test.ShouldNotBeNil)
			},
			During: func(ctx context.Context, t *testing.T, exec *ContextualMainExecution) {
				<-exec.Ready
				exec.QuitSignal(t)
				exec.Stop()
				test.That(t, capturedArgs, test.ShouldResemble, []string{"main", "1", "2", "3"})
				test.That(t, exec, test.ShouldNotBeNil)
			},
			After: func(t *testing.T, logs *observer.ObservedLogs) {
				test.That(t, captured, test.ShouldBeNil)
				test.That(t, logs, test.ShouldNotBeNil)
			},
		},
		{
			Name: "next",
			Args: []string{"1", "2", "3"},
			Err:  "",
			Before: func(t *testing.T, logger golog.Logger) {
				captured = nil
				test.That(t, logger, test.ShouldNotBeNil)
				logger.Info("hi")
			},
			During: func(ctx context.Context, t *testing.T, exec *ContextualMainExecution) {
				<-exec.Ready
				exec.QuitSignal(t)
				exec.Stop()
				test.That(t, capturedArgs, test.ShouldResemble, []string{"main", "1", "2", "3"})
				test.That(t, exec, test.ShouldNotBeNil)
			},
			After: func(t *testing.T, logs *observer.ObservedLogs) {
				test.That(t, captured, test.ShouldHaveLength, 0)
				test.That(t, logs, test.ShouldNotBeNil)
				test.That(t, logs.FilterMessage("hi").All(), test.ShouldHaveLength, 1)
			},
		},
	})
}

func TestWaitOrFail(t *testing.T) {
	WaitOrFail(context.Background(), t, time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var captured []interface{}
	prevFatal := fatal
	defer func() {
		fatal = prevFatal
	}()
	fatal = func(t *testing.T, args ...interface{}) {
		captured = args
	}
	WaitOrFail(ctx, t, time.Second)
	test.That(t, captured, test.ShouldResemble, []interface{}{context.Canceled})
}
