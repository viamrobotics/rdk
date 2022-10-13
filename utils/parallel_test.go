package utils

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.viam.com/test"
	gutils "go.viam.com/utils"
)

// This test may be flaky due to timing-based tests.
func TestRunInParallel(t *testing.T) {
	wait200ms := func(ctx context.Context) error {
		gutils.SelectContextOrWait(ctx, 200*time.Millisecond)
		return ctx.Err()
	}

	elapsed, err := RunInParallel(context.Background(), []SimpleFunc{wait200ms, wait200ms})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, elapsed, test.ShouldBeLessThan, 300*time.Millisecond)
	test.That(t, elapsed, test.ShouldBeGreaterThan, 180*time.Millisecond)

	errFunc := func(ctx context.Context) error {
		return errors.New("bad")
	}

	elapsed, err = RunInParallel(context.Background(), []SimpleFunc{wait200ms, wait200ms, errFunc})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, elapsed, test.ShouldBeLessThan, 50*time.Millisecond)

	panicFunc := func(ctx context.Context) error {
		panic(1)
	}

	_, err = RunInParallel(context.Background(), []SimpleFunc{panicFunc})
	test.That(t, err, test.ShouldNotBeNil)
}
