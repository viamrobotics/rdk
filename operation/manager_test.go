package operation

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.viam.com/test"
)

func TestSingleOperationManager(t *testing.T) {
	ctx := context.Background()
	som := SingleOperationManager{}

	test.That(t, som.NewTimedWaitOp(ctx, time.Millisecond), test.ShouldBeTrue)

	t.Run("nested operation does not cancel parent", func(t *testing.T) {
		ctx1, close1 := som.New(ctx)
		defer close1()
		_, close2 := som.New(ctx1)
		defer close2()
		test.That(t, ctx1.Err(), test.ShouldBeNil)
	})

	t.Run("cancelling on different context works", func(t *testing.T) {
		res := int32(0)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			x := som.NewTimedWaitOp(context.Background(), 10*time.Second)
			if x {
				atomic.StoreInt32(&res, 1)
			}
		}()

		for !som.OpRunning() {
			time.Sleep(time.Millisecond)
		}

		test.That(t, som.NewTimedWaitOp(ctx, time.Millisecond), test.ShouldBeTrue)

		wg.Wait()
		test.That(t, res, test.ShouldEqual, 0)
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
