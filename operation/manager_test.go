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

func TestNestedOperatioDoesNotCancelParent(t *testing.T) {
	som := SingleOperationManager{}
	ctx := context.Background()
	test.That(t, som.NewTimedWaitOp(ctx, time.Millisecond), test.ShouldBeTrue)

	ctx1, close1 := som.New(ctx)
	defer close1()
	_, close2 := som.New(ctx1)
	defer close2()
	test.That(t, ctx1.Err(), test.ShouldBeNil)
}

func TestCallOnDifferentContext(t *testing.T) {
	som := SingleOperationManager{}
	ctx := context.Background()
	test.That(t, som.NewTimedWaitOp(ctx, time.Millisecond), test.ShouldBeTrue)

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
}

func TestWaitForSuccess(t *testing.T) {
	som := SingleOperationManager{}
	ctx := context.Background()
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
}

func TestWaitForError(t *testing.T) {
	som := SingleOperationManager{}
	count := int64(0)

	err := som.WaitForSuccess(
		context.Background(),
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
}

func TestDontCancel(t *testing.T) {
	som := SingleOperationManager{}
	ctx, done := som.New(context.Background())
	defer done()

	som.CancelRunning(ctx)
	test.That(t, ctx.Err(), test.ShouldBeNil)
}

func TestCancelRace(t *testing.T) {
	som := SingleOperationManager{}
	ctx, done := som.New(context.Background())
	defer done()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		_, done := som.New(context.Background())
		wg.Done()
		defer done()
	}()

	som.CancelRunning(ctx)
	wg.Wait()
	test.That(t, ctx.Err(), test.ShouldNotBeNil)
}

func TestStopCalled(t *testing.T) {
	som := SingleOperationManager{}
	ctx, done := som.New(context.Background())
	defer done()
	mock := &mock{stopCount: 0}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		som.WaitTillNotPowered(ctx, time.Second, mock, mock.stop)
		wg.Done()
	}()

	cancel()
	wg.Wait()
	test.That(t, ctx.Err(), test.ShouldNotBeNil)
	test.That(t, mock.stopCount, test.ShouldEqual, 1)
}

func TestErrorContainsStopAndCancel(t *testing.T) {
	som := SingleOperationManager{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mock := &mock{stopCount: 0}
	var wg sync.WaitGroup

	wg.Add(1)
	var errRet error
	go func(errRet *error) {
		*errRet = som.WaitTillNotPowered(ctx, time.Second, mock, mock.stopFail)
		wg.Done()
	}(&errRet)

	cancel()
	wg.Wait()
	test.That(t, errRet.Error(), test.ShouldEqual, "context canceled; Stop failed")
}

func TestStopNotCalledOnOldContext(t *testing.T) {
	som := SingleOperationManager{}
	ctx, done := som.New(context.Background())
	defer done()
	mock := &mock{stopCount: 0}
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		som.WaitTillNotPowered(ctx, time.Second, mock, mock.stop)
		wg.Done()
	}()
	som.New(context.Background())
	wg.Wait()
	test.That(t, ctx.Err(), test.ShouldNotBeNil)
	test.That(t, mock.stopCount, test.ShouldEqual, 0)
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
	return true, 1, nil
}
