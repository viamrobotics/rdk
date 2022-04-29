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

	t.Run("sub", func(t *testing.T) {
		ctx1, close1 := lcm.New(ctx)
		defer close1()
		_, close2 := lcm.New(ctx1)
		defer close2()
		test.That(t, ctx1.Err(), test.ShouldBeNil)
	})

	t.Run("child", func(t *testing.T) {
		go func() {
			test.That(t, lcm.TimedWait(context.Background(), 10*time.Second), test.ShouldBeFalse)
		}()

		for !lcm.OpRunning() {
			time.Sleep(time.Millisecond)
		}

		test.That(t, lcm.TimedWait(ctx, time.Millisecond), test.ShouldBeTrue)
	})
}
