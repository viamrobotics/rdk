package utils

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-errors/errors"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/test"
)

func TestContextualMain(t *testing.T) {
	var captured []interface{}
	fatal = func(logger golog.Logger, args ...interface{}) {
		captured = args
	}
	err1 := errors.New("whoops")
	mainWithArgs := func(ctx context.Context, args []string, logger golog.Logger) error {
		return err1
	}
	logger := golog.NewTestLogger(t)
	ContextualMain(mainWithArgs, logger)
	test.That(t, captured, test.ShouldResemble, []interface{}{err1})
	captured = nil
	mainWithArgs = func(ctx context.Context, args []string, logger golog.Logger) error {
		return context.Canceled
	}
	ContextualMain(mainWithArgs, logger)
	test.That(t, captured, test.ShouldBeNil)
	mainWithArgs = func(ctx context.Context, args []string, logger golog.Logger) error {
		return multierr.Combine(context.Canceled, err1)
	}
	ContextualMain(mainWithArgs, logger)
	test.That(t, captured, test.ShouldResemble, []interface{}{err1})
}

func TestContextualMainQuit(t *testing.T) {
	var captured []interface{}
	fatal = func(logger golog.Logger, args ...interface{}) {
		captured = args
	}
	err1 := errors.New("whoops")
	mainWithArgs := func(ctx context.Context, args []string, logger golog.Logger) error {
		return err1
	}
	logger := golog.NewTestLogger(t)
	ContextualMainQuit(mainWithArgs, logger)
	test.That(t, captured, test.ShouldResemble, []interface{}{err1})
	captured = nil
	mainWithArgs = func(ctx context.Context, args []string, logger golog.Logger) error {
		return context.Canceled
	}
	ContextualMainQuit(mainWithArgs, logger)
	test.That(t, captured, test.ShouldBeNil)
	mainWithArgs = func(ctx context.Context, args []string, logger golog.Logger) error {
		return multierr.Combine(context.Canceled, err1)
	}
	ContextualMainQuit(mainWithArgs, logger)
	test.That(t, captured, test.ShouldResemble, []interface{}{err1})
}

func TestContextWithQuitSignal(t *testing.T) {
	ctx := context.Background()
	sig := make(chan os.Signal, 1)
	ctx = ContextWithQuitSignal(ctx, sig)
	sig2 := ContextMainQuitSignal(context.Background())
	test.That(t, sig2, test.ShouldBeNil)
	sig2 = ContextMainQuitSignal(ctx)
	test.That(t, sig2, test.ShouldEqual, (<-chan os.Signal)(sig))
}

func TestContextWithReadyFunc(t *testing.T) {
	ctx := context.Background()
	sig := make(chan struct{}, 1)
	ctx = ContextWithReadyFunc(ctx, sig)
	func1 := ContextMainReadyFunc(context.Background())
	func1()
	var ok bool
	select {
	case <-sig:
		ok = true
	default:
	}
	test.That(t, ok, test.ShouldBeFalse)
	func1 = ContextMainReadyFunc(ctx)
	func1()
	select {
	case <-sig:
		ok = true
	default:
	}
	test.That(t, ok, test.ShouldBeTrue)
	func1()
	func1()
	select {
	case <-sig:
		ok = true
	default:
	}
	test.That(t, ok, test.ShouldBeTrue)
}

func TestContextWithIterFunc(t *testing.T) {
	ctx := context.Background()
	sig := make(chan struct{}, 1)
	ctx = ContextWithIterFunc(ctx, func() {
		sig <- struct{}{}
	})
	func1 := ContextMainIterFunc(context.Background())
	func1()
	var ok bool
	select {
	case <-sig:
		ok = true
	default:
	}
	test.That(t, ok, test.ShouldBeFalse)
	func1 = ContextMainIterFunc(ctx)
	go func1()
	<-sig
	go func1()
	go func1()
	<-sig
}

func TestPanicCapturingGo(t *testing.T) {
	running := make(chan struct{})
	PanicCapturingGo(func() {
		close(running)
		panic("dead")
	})
	<-running
	time.Sleep(time.Second)
}

func TestPanicCapturingGoWithCallback(t *testing.T) {
	running := make(chan struct{})
	errCh := make(chan interface{})
	PanicCapturingGoWithCallback(func() {
		close(running)
		panic("dead")
	}, func(err interface{}) {
		errCh <- err
	})
	<-running
	test.That(t, <-errCh, test.ShouldEqual, "dead")
}

func TestSelectContextOrWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ok := SelectContextOrWait(ctx, time.Hour)
	test.That(t, ok, test.ShouldBeFalse)

	ctx, cancel = context.WithCancel(context.Background())
	go func() {
		time.Sleep(time.Second)
		cancel()
	}()
	ok = SelectContextOrWait(ctx, time.Hour)
	test.That(t, ok, test.ShouldBeFalse)

	ok = SelectContextOrWait(context.Background(), time.Second)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestSelectContextOrWaitChan(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	timer := time.NewTimer(time.Second)
	timer.Stop()
	ok := SelectContextOrWaitChan(ctx, timer.C)
	test.That(t, ok, test.ShouldBeFalse)

	ctx, cancel = context.WithCancel(context.Background())
	go func() {
		time.Sleep(time.Second)
		cancel()
	}()
	ok = SelectContextOrWaitChan(ctx, timer.C)
	test.That(t, ok, test.ShouldBeFalse)

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	ok = SelectContextOrWaitChan(context.Background(), timer.C)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestManagedGo(t *testing.T) {
	dieCount := 3
	done := make(chan struct{})
	ManagedGo(func() {
		time.Sleep(50 * time.Millisecond)
		if dieCount == 0 {
			return
		}
		dieCount--
		panic(dieCount)
	}, func() {
		close(done)
	})
	<-done
}
