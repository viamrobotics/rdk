package videosource

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"
)

func TestWebcamIdleTimeout(t *testing.T) {
	ctx := context.Background()

	t.Run("pauses reads and resumes on request", func(t *testing.T) {
		reader, driver := &fakeReader{}, &fakeDriver{}
		c := newFakeWebcam(t, testIdleTimeoutMs, reader, driver, &fakeOpener{})

		waitFrame(t, c)
		waitIdle(t, c)
		c.mu.Lock()
		frame := c.buffer.frame
		c.mu.Unlock()
		test.That(t, frame, test.ShouldBeNil)
		test.That(t, usesDriver(c, driver), test.ShouldBeTrue)

		readsAtPause := reader.reads.Load()
		time.Sleep(3 * testIdleTimeoutMs * time.Millisecond)
		test.That(t, reader.reads.Load(), test.ShouldEqual, readsAtPause)
		test.That(t, driver.closes.Load(), test.ShouldEqual, int32(0))

		imgs, _, err := c.Images(ctx, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imgs, test.ShouldHaveLength, 1)
		img, err := imgs[0].Image(ctx)
		test.That(t, err, test.ShouldBeNil)
		// The buffer worker may have read further by now, so only assert the stale frames never surfaced.
		test.That(t, img.Bounds().Dx(), test.ShouldBeGreaterThan, int(readsAtPause)+idleStaleFrames)
		test.That(t, idleStateOf(c), test.ShouldEqual, stateStreaming)

		test.That(t, c.Close(ctx), test.ShouldBeNil)
		test.That(t, driver.closes.Load(), test.ShouldEqual, int32(1))
		_, _, err = c.Images(ctx, nil, nil)
		test.That(t, errors.Is(err, errClosed), test.ShouldBeTrue)
		assertAllReleased(t, reader)
	})

	t.Run("requests keep the camera streaming", func(t *testing.T) {
		c := newFakeWebcam(t, 3*testIdleTimeoutMs, &fakeReader{}, &fakeDriver{}, &fakeOpener{})
		waitFrame(t, c)

		// Twice the idle timeout in total, so this only passes if each request restarts the timer.
		for range 12 {
			time.Sleep(testIdleTimeoutMs * time.Millisecond / 2)
			test.That(t, idleStateOf(c), test.ShouldEqual, stateStreaming)
			_, _, err := c.Images(ctx, nil, nil)
			test.That(t, err, test.ShouldBeNil)
		}
	})

	t.Run("concurrent callers share one wake", func(t *testing.T) {
		reader := &fakeReader{}
		c := newFakeWebcam(t, testIdleTimeoutMs, reader, &fakeDriver{}, &fakeOpener{})
		waitIdle(t, c)
		unblock := reader.block(t)
		readsAtPause := reader.reads.Load()

		const callers = 3
		var wg sync.WaitGroup
		errs := make([]error, callers)
		for i := range callers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, errs[i] = c.Images(ctx, nil, nil)
			}()
		}
		// Exactly one read is in flight (held by block) no matter how many callers are waiting.
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, reader.reads.Load(), test.ShouldEqual, readsAtPause+1)
		})
		time.Sleep(50 * time.Millisecond)
		test.That(t, reader.reads.Load(), test.ShouldEqual, readsAtPause+1)

		unblock()
		wg.Wait()
		for _, err := range errs {
			test.That(t, err, test.ShouldBeNil)
		}
	})

	t.Run("reconnects while paused", func(t *testing.T) {
		first, second := &fakeDriver{}, &fakeDriver{}
		opener := &fakeOpener{reader: &fakeReader{}, driver: second}
		c := newFakeWebcam(t, testIdleTimeoutMs, &fakeReader{}, first, opener)
		waitIdle(t, c)

		first.unavailable.Store(true)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, opener.opens.Load(), test.ShouldEqual, int32(1))
			test.That(tb, first.closes.Load(), test.ShouldEqual, int32(1))
		})
		test.That(t, idleStateOf(c), test.ShouldEqual, stateIdle)

		waitFrame(t, c)
		test.That(t, opener.reader.reads.Load(), test.ShouldBeGreaterThan, int32(idleStaleFrames))
		test.That(t, usesDriver(c, second), test.ShouldBeTrue)
	})

	t.Run("wake timeout", func(t *testing.T) {
		reader := &fakeReader{}
		c := newFakeWebcam(t, testIdleTimeoutMs, reader, &fakeDriver{}, &fakeOpener{})
		c.mu.Lock()
		c.wakeTimeout = 50 * time.Millisecond
		c.mu.Unlock()
		waitIdle(t, c)
		unblock := reader.block(t)

		_, _, err := c.Images(ctx, nil, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "timed out waiting for idle camera")
		unblock()
		waitFrame(t, c)
	})

	t.Run("caller context cancellation", func(t *testing.T) {
		reader := &fakeReader{}
		c := newFakeWebcam(t, testIdleTimeoutMs, reader, &fakeDriver{}, &fakeOpener{})
		waitIdle(t, c)
		reader.block(t)

		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		_, _, err := c.Images(cancelled, nil, nil)
		test.That(t, errors.Is(err, context.Canceled), test.ShouldBeTrue)
	})

	t.Run("disabled", func(t *testing.T) {
		reader, driver := &fakeReader{}, &fakeDriver{}
		c := newFakeWebcam(t, 0, reader, driver, &fakeOpener{})

		time.Sleep(3 * testIdleTimeoutMs * time.Millisecond)
		test.That(t, idleStateOf(c), test.ShouldEqual, stateStreaming)
		before := reader.reads.Load()
		time.Sleep(50 * time.Millisecond)
		test.That(t, reader.reads.Load(), test.ShouldBeGreaterThan, before)

		imgs, _, err := c.Images(ctx, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imgs, test.ShouldHaveLength, 1)
		test.That(t, c.Close(ctx), test.ShouldBeNil)
		test.That(t, driver.closes.Load(), test.ShouldEqual, int32(1))
	})
}
