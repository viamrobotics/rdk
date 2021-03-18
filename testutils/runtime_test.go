package testutils

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/test"
)

func TestWaitOrFail(t *testing.T) {
	WaitOrFail(context.Background(), t, time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var captured []interface{}
	fatal = func(t *testing.T, args ...interface{}) {
		captured = args
	}
	WaitOrFail(ctx, t, time.Second)
	test.That(t, captured, test.ShouldResemble, []interface{}{context.Canceled})
}
