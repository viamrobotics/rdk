package videosource

import (
	"context"
	"errors"
	"image"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	driverutils "github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

type fakeDriver struct{ closes atomic.Int32 }

func (d *fakeDriver) Open() error                { return nil }
func (d *fakeDriver) Close() error               { d.closes.Add(1); return nil }
func (d *fakeDriver) Properties() []prop.Media   { return nil }
func (d *fakeDriver) ID() string                 { return "fake" }
func (d *fakeDriver) Info() driverutils.Info     { return driverutils.Info{Label: "fake"} }
func (d *fakeDriver) Status() driverutils.State  { return driverutils.StateRunning }
func (d *fakeDriver) IsAvailable() (bool, error) { return true, nil }

func fakeReader() video.Reader {
	return video.ReaderFunc(func() (image.Image, func(), error) {
		return image.NewRGBA(image.Rect(0, 0, 2, 2)), func() {}, nil
	})
}

// fakeOpener stands in for findReaderAndDriver. Each call blocks on gate (if set), then fails if
// fail is set, otherwise hands back a counting reader and the shared driver. The reader's Nth frame
// (1-based) is N pixels wide so tests can tell how many frames were consumed before one was buffered.
type fakeOpener struct {
	opens  atomic.Int32
	reads  atomic.Int32
	fail   atomic.Bool
	gate   chan struct{}
	driver *fakeDriver
}

func (o *fakeOpener) open(*WebcamConfig, string, logging.Logger) (video.Reader, driverutils.Driver, string, error) {
	o.opens.Add(1)
	if o.gate != nil {
		<-o.gate
	}
	if o.fail.Load() {
		return nil, nil, "", errors.New("camera busy")
	}
	reader := video.ReaderFunc(func() (image.Image, func(), error) {
		n := int(o.reads.Add(1))
		return image.NewRGBA(image.Rect(0, 0, n, 1)), func() {}, nil
	})
	return reader, o.driver, "fake", nil
}

const testIdleTimeoutMs = 100

func newFakeWebcam(t *testing.T, idleTimeoutMs int, first *fakeDriver, opener *fakeOpener) *webcam {
	t.Helper()
	conf := WebcamConfig{FrameRate: 100, IdleTimeoutMs: idleTimeoutMs}
	c := newWebcam(resource.NewName(camera.API, "cam"), conf, "fake", fakeReader(), first, opener.open, logging.NewTestLogger(t))
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

func idleStateOf(c *webcam) idleState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.idleState
}

func waitIdle(t *testing.T, c *webcam) {
	t.Helper()
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, idleStateOf(c), test.ShouldEqual, stateIdle)
	})
}

func waitFrame(t *testing.T, c *webcam) {
	t.Helper()
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		imgs, _, err := c.Images(context.Background(), nil, nil)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, imgs, test.ShouldHaveLength, 1)
	})
}

func TestWebcamIdleTimeout(t *testing.T) {
	ctx := context.Background()

	t.Run("reopens on request", func(t *testing.T) {
		first, second := &fakeDriver{}, &fakeDriver{}
		opener := &fakeOpener{driver: second}
		c := newFakeWebcam(t, testIdleTimeoutMs, first, opener)

		waitFrame(t, c)
		waitIdle(t, c)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, first.closes.Load(), test.ShouldEqual, int32(1))
		})
		test.That(t, opener.opens.Load(), test.ShouldEqual, int32(0))

		imgs, _, err := c.Images(ctx, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imgs, test.ShouldHaveLength, 1)
		test.That(t, opener.opens.Load(), test.ShouldEqual, int32(1))
		test.That(t, idleStateOf(c), test.ShouldEqual, stateStreaming)
		c.mu.Lock()
		test.That(t, c.driver, test.ShouldEqual, second)
		c.mu.Unlock()

		test.That(t, c.Close(ctx), test.ShouldBeNil)
		test.That(t, second.closes.Load(), test.ShouldEqual, int32(1))
		_, _, err = c.Images(ctx, nil, nil)
		test.That(t, errors.Is(err, errClosed), test.ShouldBeTrue)
	})

	t.Run("discards warmup frames after wake", func(t *testing.T) {
		opener := &fakeOpener{driver: &fakeDriver{}}
		c := newFakeWebcam(t, testIdleTimeoutMs, &fakeDriver{}, opener)
		waitIdle(t, c)

		imgs, _, err := c.Images(ctx, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imgs, test.ShouldHaveLength, 1)
		img, err := imgs[0].Image(ctx)
		test.That(t, err, test.ShouldBeNil)
		// The buffer worker may have read further by now, so only assert the discarded frames never surfaced.
		test.That(t, img.Bounds().Dx(), test.ShouldBeGreaterThan, defaultWakeDiscardFrames)
		test.That(t, opener.reads.Load(), test.ShouldBeGreaterThanOrEqualTo, int32(defaultWakeDiscardFrames+1))
	})

	t.Run("concurrent callers share one reopen", func(t *testing.T) {
		opener := &fakeOpener{driver: &fakeDriver{}, gate: make(chan struct{})}
		c := newFakeWebcam(t, testIdleTimeoutMs, &fakeDriver{}, opener)
		waitIdle(t, c)

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
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, opener.opens.Load(), test.ShouldEqual, int32(1))
		})
		close(opener.gate)
		wg.Wait()
		for _, err := range errs {
			test.That(t, err, test.ShouldBeNil)
		}
		test.That(t, opener.opens.Load(), test.ShouldEqual, int32(1))
	})

	t.Run("failed reopen hands recovery to the monitor", func(t *testing.T) {
		opener := &fakeOpener{driver: &fakeDriver{}}
		c := newFakeWebcam(t, testIdleTimeoutMs, &fakeDriver{}, opener)
		waitIdle(t, c)

		opener.fail.Store(true)
		_, _, err := c.Images(ctx, nil, nil)
		test.That(t, errors.Is(err, errDisconnected), test.ShouldBeTrue)
		c.mu.Lock()
		test.That(t, c.idleState, test.ShouldEqual, stateStreaming)
		test.That(t, c.disconnected, test.ShouldBeTrue)
		c.mu.Unlock()

		opener.fail.Store(false)
		waitFrame(t, c)
		test.That(t, opener.opens.Load(), test.ShouldBeGreaterThanOrEqualTo, int32(2))

		// Idles again once the reconnected camera goes unused.
		waitIdle(t, c)
	})

	t.Run("wake timeout", func(t *testing.T) {
		opener := &fakeOpener{driver: &fakeDriver{}, gate: make(chan struct{})}
		c := newFakeWebcam(t, testIdleTimeoutMs, &fakeDriver{}, opener)
		c.mu.Lock()
		c.wakeTimeout = 50 * time.Millisecond
		c.mu.Unlock()
		waitIdle(t, c)

		_, _, err := c.Images(ctx, nil, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "timed out waiting for idle camera")
		close(opener.gate)
		waitFrame(t, c)
	})

	t.Run("caller context cancellation", func(t *testing.T) {
		opener := &fakeOpener{driver: &fakeDriver{}, gate: make(chan struct{})}
		c := newFakeWebcam(t, testIdleTimeoutMs, &fakeDriver{}, opener)
		waitIdle(t, c)

		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		_, _, err := c.Images(cancelled, nil, nil)
		test.That(t, errors.Is(err, context.Canceled), test.ShouldBeTrue)
		close(opener.gate)
	})

	t.Run("disabled", func(t *testing.T) {
		first := &fakeDriver{}
		opener := &fakeOpener{driver: &fakeDriver{}}
		c := newFakeWebcam(t, 0, first, opener)

		time.Sleep(3 * testIdleTimeoutMs * time.Millisecond)
		test.That(t, idleStateOf(c), test.ShouldEqual, stateStreaming)
		test.That(t, first.closes.Load(), test.ShouldEqual, int32(0))

		imgs, _, err := c.Images(ctx, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imgs, test.ShouldHaveLength, 1)
		test.That(t, c.Close(ctx), test.ShouldBeNil)
		test.That(t, first.closes.Load(), test.ShouldEqual, int32(1))
		test.That(t, opener.opens.Load(), test.ShouldEqual, int32(0))
	})
}
