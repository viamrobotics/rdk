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
	t.Run("Cancel, ensure stop", func(t *testing.T) {
		fakeMotor := &mockMotor{Name: "testMotor"}
		som.SetStop(fakeMotor)
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)

		ctx, finished := som.New(ctx)
		cancel()
		finished()
		defer test.That(t, fakeMotor.stopCount, test.ShouldEqual, 1)
		test.That(t, ctx.Err(), test.ShouldNotBeNil)
	})

	t.Run("Cancel, ensure stop old stop", func(t *testing.T) {
		fakeMotor := &mockMotorOld{Name: "testMotorOld"}
		som.SetStop(nil)

		som.SetOldStop(fakeMotor)
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)

		ctx, finished := som.New(ctx)
		cancel()
		finished()
		defer test.That(t, fakeMotor.stopCount, test.ShouldEqual, 1)
		test.That(t, ctx.Err(), test.ShouldNotBeNil)
	})
	t.Run("ensure op that completes successfully does not call stop", func(t *testing.T) {
		count := int64(0)
		fakeMotor := &mockMotor{Name: "testMotor"}
		som.SetStop(fakeMotor)
		ctx := context.Background()

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
		defer test.That(t, fakeMotor.stopCount, test.ShouldEqual, 0)
		test.That(t, ctx.Err(), test.ShouldBeNil)
	})
}

func (m *mockMotor) Stop(ctx context.Context, extra map[string]interface{}) error {
	m.stopCount++
	m.extra = extra
	return nil
}

type mockMotor struct {
	Name      string
	stopCount int
	extra     map[string]interface{}
}

func (m *mockMotorOld) Stop(ctx context.Context) error {
	m.stopCount++
	return nil
}

type mockMotorOld struct {
	Name      string
	stopCount int
}
