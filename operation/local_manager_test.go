package operation

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"
)

func TestLocalCallManager(t *testing.T) {
	ctx := context.Background()

	lcm := LocalCallManager{}

	test.That(t, lcm.TimedWait(ctx, time.Millisecond), test.ShouldBeTrue)
}
