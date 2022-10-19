package operation

import (
	"context"
	"errors"
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

	t.Run("WaitForSuccess-error", func(t *testing.T) {
		count := int64(0)

		err := som.WaitForSuccess(
			ctx,
			time.Millisecond,
			func(ctx context.Context) (bool, error) {
				if atomic.AddInt64(&count, 1) == 5 {
					return false, errors.New("blah")
				}
				return false, nil
			},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, count, test.ShouldEqual, int64(5))
	})

	t.Run("don't cancel myself", func(t *testing.T) {
		ctx, done := som.New(context.Background())
		defer done()

		som.CancelRunning(ctx)
		test.That(t, ctx.Err(), test.ShouldBeNil)
	})

	t.Run("cancel race", func(t *testing.T) {
		ctx, done := som.New(context.Background())
		defer done()

		c := make(chan bool)

		go func() {
			c <- true
			_, done := som.New(context.Background())
			defer done()
		}()

		<-c

		som.CancelRunning(ctx)
		test.That(t, ctx.Err(), test.ShouldNotBeNil)
	})
	t.Run("Ensure stop called on cancelled context", func(t *testing.T) {
		ctx, done := som.New(context.Background())
		mock := &mock{stopCount: 0}
		defer done()
		ctx, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup

		wg.Add(1)

		go func() {
			som.WaitTillNotPowered(ctx, 5*time.Second, mock, mock.stop)
			wg.Done()
		}()

		cancel()
		wg.Wait()
		test.That(t, ctx.Err(), test.ShouldNotBeNil)
		test.That(t, mock.stopCount, test.ShouldEqual, 1)
	})
	t.Run("Ensure error contains stop and cancel errors", func(t *testing.T) {
		ctx := context.Background()
		mock := &mock{stopCount: 0}
		ctx, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup

		wg.Add(1)
		var errRet error
		go func(errRet *error) {
			*errRet = som.WaitTillNotPowered(ctx, 5*time.Second, mock, mock.stopFail)
			wg.Done()
		}(&errRet)

		cancel()
		wg.Wait()
		test.That(t, errRet.Error(), test.ShouldEqual, "context canceled; Stop failed")
	})
	t.Run("Ensure stop not called on old context when new context is spawned", func(t *testing.T) {
		ctx, done := som.New(context.Background())
		mock := &mock{stopCount: 0}
		defer done()
		var wg sync.WaitGroup

		wg.Add(1)

		go func() {
			som.WaitTillNotPowered(ctx, 5*time.Second, mock, mock.stop)
			wg.Done()
		}()
		som.New(context.Background())
		wg.Wait()
		test.That(t, ctx.Err(), test.ShouldNotBeNil)
		test.That(t, mock.stopCount, test.ShouldEqual, 0)
	})
}

type mock struct {
	stopCount int
}

func (m *mock) stop(ctx context.Context, extra map[string]interface{}) error {
	m.stopCount++
	return nil
}

func (m *mock) stopFail(ctx context.Context, extra map[string]interface{}) error {
	return errors.New("Stop failed")
}

func (m *mock) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	return false, 0, nil
}
