package utils

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.viam.com/test"
	gutils "go.viam.com/utils"
)

func TestRunInParallel(t *testing.T) {
	wait100ms := func(ctx context.Context) error {
		gutils.SelectContextOrWait(ctx, 100*time.Millisecond)
		return ctx.Err()
	}

	elapsed, err := RunInParallel(context.Background(), []SimpleFunc{wait100ms, wait100ms})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, elapsed, test.ShouldBeLessThan, 110*time.Millisecond)
	test.That(t, elapsed, test.ShouldBeGreaterThan, 90*time.Millisecond)

	errFunc := func(ctx context.Context) error {
		return errors.New("bad")
	}

	elapsed, err = RunInParallel(context.Background(), []SimpleFunc{wait100ms, wait100ms, errFunc})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, elapsed, test.ShouldBeLessThan, 10*time.Millisecond)

	panicFunc := func(ctx context.Context) error {
		panic(1)
	}

	_, err = RunInParallel(context.Background(), []SimpleFunc{panicFunc})
	test.That(t, err, test.ShouldNotBeNil)
}
