package utils

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/edaniels/test"
	"go.uber.org/multierr"
)

func TestContextualMain(t *testing.T) {
	var captured []interface{}
	fatal = func(args ...interface{}) {
		captured = args
	}
	err1 := errors.New("whoops")
	mainWithArgs := func(ctx context.Context, args []string) error {
		return err1
	}
	ContextualMain(mainWithArgs)
	test.That(t, captured, test.ShouldResemble, []interface{}{err1})
	captured = nil
	mainWithArgs = func(ctx context.Context, args []string) error {
		return context.Canceled
	}
	ContextualMain(mainWithArgs)
	test.That(t, captured, test.ShouldBeNil)
	mainWithArgs = func(ctx context.Context, args []string) error {
		return multierr.Combine(context.Canceled, err1)
	}
	ContextualMain(mainWithArgs)
	test.That(t, captured, test.ShouldResemble, []interface{}{err1})
}

func TestContextualMainQuit(t *testing.T) {
	var captured []interface{}
	fatal = func(args ...interface{}) {
		captured = args
	}
	err1 := errors.New("whoops")
	mainWithArgs := func(ctx context.Context, args []string) error {
		return err1
	}
	ContextualMainQuit(mainWithArgs)
	test.That(t, captured, test.ShouldResemble, []interface{}{err1})
	captured = nil
	mainWithArgs = func(ctx context.Context, args []string) error {
		return context.Canceled
	}
	ContextualMainQuit(mainWithArgs)
	test.That(t, captured, test.ShouldBeNil)
	mainWithArgs = func(ctx context.Context, args []string) error {
		return multierr.Combine(context.Canceled, err1)
	}
	ContextualMainQuit(mainWithArgs)
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
