package videosource

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/logging"
)

func TestWebcamMonitor(t *testing.T) {
	ctx := context.Background()

	t.Run("reconnects once the camera can be reopened", func(t *testing.T) {
		firstReader, secondReader := &fakeReader{}, &fakeReader{}
		firstDriver, secondDriver := &fakeDriver{}, &fakeDriver{}
		opener := &fakeOpener{reader: secondReader, driver: secondDriver}
		opener.failing.Store(true)
		c := newFakeWebcam(t, 0, firstReader, firstDriver, opener)
		waitFrame(t, c)

		firstDriver.unavailable.Store(true)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, isDisconnected(c), test.ShouldBeTrue)
			test.That(tb, opener.opens.Load(), test.ShouldBeGreaterThan, int32(2))
		})
		_, _, err := c.Images(ctx, nil, nil)
		test.That(t, errors.Is(err, errDisconnected), test.ShouldBeTrue)
		_, err = c.Properties(ctx)
		test.That(t, errors.Is(err, errDisconnected), test.ShouldBeTrue)
		// Every attempt detaches the current camera, but only the first attempt has one to close.
		test.That(t, firstDriver.closes.Load(), test.ShouldEqual, int32(1))

		opener.failing.Store(false)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, usesDriver(c, secondDriver), test.ShouldBeTrue)
		})
		waitFrame(t, c)
		for _, path := range opener.openedPaths() {
			test.That(t, path, test.ShouldEqual, "fake")
		}

		test.That(t, c.Close(ctx), test.ShouldBeNil)
		assertAllReleased(t, firstReader)
		assertAllReleased(t, secondReader)
	})

	t.Run("drops the unplugged camera's frame", func(t *testing.T) {
		firstDriver, secondDriver, secondReader := &fakeDriver{}, &fakeDriver{}, &fakeReader{}
		c := newFakeWebcam(t, 0, &fakeReader{}, firstDriver, &fakeOpener{reader: secondReader, driver: secondDriver})
		waitFrame(t, c)
		secondReader.block(t)

		firstDriver.unavailable.Store(true)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, usesDriver(c, secondDriver), test.ShouldBeTrue)
		})
		_, _, err := c.Images(ctx, nil, nil)
		test.That(t, errors.Is(err, errNoFrames), test.ShouldBeTrue)
	})

	t.Run("survives failing to close the old driver", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		firstDriver, secondDriver := &fakeDriver{closeErr: errCloseFailed}, &fakeDriver{}
		opener := &fakeOpener{reader: &fakeReader{}, driver: secondDriver}
		c := startFakeWebcam(t, WebcamConfig{FrameRate: 100}, logger, &fakeReader{}, firstDriver, opener)

		firstDriver.unavailable.Store(true)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, usesDriver(c, secondDriver), test.ShouldBeTrue)
		})
		test.That(t, logs.FilterMessage("failed to close current camera").Len(), test.ShouldEqual, 1)
		waitFrame(t, c)
	})

	t.Run("close stops reconnect attempts", func(t *testing.T) {
		firstDriver := &fakeDriver{}
		opener := &fakeOpener{}
		opener.failing.Store(true)
		c := newFakeWebcam(t, 0, &fakeReader{}, firstDriver, opener)

		firstDriver.unavailable.Store(true)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, opener.opens.Load(), test.ShouldBeGreaterThan, int32(1))
		})

		test.That(t, c.Close(ctx), test.ShouldBeNil)
		test.That(t, firstDriver.closes.Load(), test.ShouldEqual, int32(1))
	})

	t.Run("does not pause for idle while disconnected", func(t *testing.T) {
		firstDriver := &fakeDriver{}
		opener := &fakeOpener{}
		opener.failing.Store(true)
		c := newFakeWebcam(t, int(time.Hour/time.Millisecond), &fakeReader{}, firstDriver, opener)

		firstDriver.unavailable.Store(true)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, isDisconnected(c), test.ShouldBeTrue)
		})
		c.mu.Lock()
		c.lastAccess = time.Now().Add(-2 * time.Hour)
		c.mu.Unlock()

		// Several buffer worker ticks at 100fps, each past the idle timeout.
		time.Sleep(100 * time.Millisecond)
		test.That(t, idleStateOf(c), test.ShouldEqual, stateStreaming)
	})
}
