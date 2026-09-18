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
	"github.com/pion/mediadevices/pkg/driver/availability"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

type fakeDriver struct {
	closes      atomic.Int32
	unavailable atomic.Bool
}

func (d *fakeDriver) Open() error               { return nil }
func (d *fakeDriver) Close() error              { d.closes.Add(1); return nil }
func (d *fakeDriver) Properties() []prop.Media  { return nil }
func (d *fakeDriver) ID() string                { return "fake" }
func (d *fakeDriver) Info() driverutils.Info    { return driverutils.Info{Label: "fake"} }
func (d *fakeDriver) Status() driverutils.State { return driverutils.StateRunning }
func (d *fakeDriver) IsAvailable() (bool, error) {
	if d.unavailable.Load() {
		return false, availability.ErrNoDevice
	}
	return true, nil
}

// fakeReader counts reads and blocks each one on gate while gate is set. The Nth frame (1-based) is
// N pixels wide so tests can tell how many frames were consumed before one was buffered.
type fakeReader struct {
	reads atomic.Int32
	gate  atomic.Pointer[chan struct{}]
}

func (r *fakeReader) Read() (image.Image, func(), error) {
	n := int(r.reads.Add(1))
	if gate := r.gate.Load(); gate != nil {
		<-*gate
	}
	return image.NewRGBA(image.Rect(0, 0, n, 1)), func() {}, nil
}

// fakeOpener stands in for findReaderAndDriver during monitor reconnects.
type fakeOpener struct {
	opens  atomic.Int32
	reader *fakeReader
	driver *fakeDriver
}

func (o *fakeOpener) open(*WebcamConfig, string, logging.Logger) (video.Reader, driverutils.Driver, string, error) {
	o.opens.Add(1)
	return o.reader, o.driver, "fake", nil
}

const testIdleTimeoutMs = 100

func newFakeWebcam(t *testing.T, idleTimeoutMs int, reader *fakeReader, driver *fakeDriver, opener *fakeOpener) *webcam {
	t.Helper()
	conf := WebcamConfig{FrameRate: 100, IdleTimeoutMs: idleTimeoutMs}
	c := newWebcam(resource.NewName(camera.API, "cam"), conf, "fake", reader, driver, opener.open, logging.NewTestLogger(t))
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

	t.Run("pauses reads and resumes on request", func(t *testing.T) {
		reader, driver := &fakeReader{}, &fakeDriver{}
		c := newFakeWebcam(t, testIdleTimeoutMs, reader, driver, &fakeOpener{})

		waitFrame(t, c)
		waitIdle(t, c)
		c.mu.Lock()
		frame, cur := c.buffer.frame, c.driver
		c.mu.Unlock()
		test.That(t, frame, test.ShouldBeNil)
		test.That(t, cur, test.ShouldEqual, driver)

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
	})

	t.Run("concurrent callers share one wake", func(t *testing.T) {
		reader := &fakeReader{}
		c := newFakeWebcam(t, testIdleTimeoutMs, reader, &fakeDriver{}, &fakeOpener{})
		waitIdle(t, c)
		gate := make(chan struct{})
		reader.gate.Store(&gate)
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
		// Exactly one read is in flight (blocked on the gate) no matter how many callers are waiting.
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(tb, reader.reads.Load(), test.ShouldEqual, readsAtPause+1)
		})
		time.Sleep(50 * time.Millisecond)
		test.That(t, reader.reads.Load(), test.ShouldEqual, readsAtPause+1)

		close(gate)
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
		c.mu.Lock()
		cur := c.driver
		c.mu.Unlock()
		test.That(t, cur, test.ShouldEqual, second)
	})

	t.Run("wake timeout", func(t *testing.T) {
		reader := &fakeReader{}
		c := newFakeWebcam(t, testIdleTimeoutMs, reader, &fakeDriver{}, &fakeOpener{})
		c.mu.Lock()
		c.wakeTimeout = 50 * time.Millisecond
		c.mu.Unlock()
		waitIdle(t, c)
		gate := make(chan struct{})
		reader.gate.Store(&gate)

		_, _, err := c.Images(ctx, nil, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "timed out waiting for idle camera")
		reader.gate.Store(nil)
		close(gate)
		waitFrame(t, c)
	})

	t.Run("caller context cancellation", func(t *testing.T) {
		reader := &fakeReader{}
		c := newFakeWebcam(t, testIdleTimeoutMs, reader, &fakeDriver{}, &fakeOpener{})
		waitIdle(t, c)
		gate := make(chan struct{})
		reader.gate.Store(&gate)

		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		_, _, err := c.Images(cancelled, nil, nil)
		test.That(t, errors.Is(err, context.Canceled), test.ShouldBeTrue)
		reader.gate.Store(nil)
		close(gate)
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
