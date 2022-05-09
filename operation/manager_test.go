package operation

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.viam.com/test"
)

func TestSingleOperationManager(t *testing.T) {
	ctx := context.Background()
	som := SingleOperationManager{}

	test.That(t, som.NewTimedWaitOp(ctx, time.Millisecond), test.ShouldBeTrue)

	t.Run("sub", func(t *testing.T) {
		ctx1, close1 := som.New(ctx)
		defer close1()
		_, close2 := som.New(ctx1)
		defer close2()
		test.That(t, ctx1.Err(), test.ShouldBeNil)
	})

	t.Run("child", func(t *testing.T) {
		go func() {
			test.That(t, som.NewTimedWaitOp(context.Background(), 10*time.Second), test.ShouldBeFalse)
		}()

		for !som.OpRunning() {
			time.Sleep(time.Millisecond)
		}

		test.That(t, som.NewTimedWaitOp(ctx, time.Millisecond), test.ShouldBeTrue)
	})

	t.Run("WaitForSuccess", func(t *testing.T) {
		count := int64(0)

		err := som.WaitForSuccess(
			ctx,
			time.Millisecond,
			func(ctx context.Context) (bool, error) {
				if atomic.AddInt64(&count, 1) == 5 {
					return true, nil
				}
				return false, nil
			},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, count, test.ShouldEqual, int64(5))
	})
}
