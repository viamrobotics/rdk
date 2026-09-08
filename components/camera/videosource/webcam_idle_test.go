package videosource

import (
	"context"
	"errors"
	"image"
	"sync/atomic"
	"testing"
	"time"

	driverutils "github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/testutils"

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

// newFakeWebcam mirrors NewWebcam without touching hardware.
func newFakeWebcam(t *testing.T, idleTimeout time.Duration, first *fakeDriver) *webcam {
	t.Helper()
	c := &webcam{
		Named:   resource.NewName(resource.APINamespaceRDK.WithComponentType("camera"), "cam").AsNamed(),
		logger:  logging.NewTestLogger(t),
		workers: goutils.NewBackgroundStoppableWorkers(),
		buffer:  newWebcamBuffer(),
		reader:  fakeReader(),
		driver:  first,
		conf:    WebcamConfig{FrameRate: 100},
	}
	c.idleTimeout = idleTimeout
	c.lastAccess = time.Now()
	c.wakeCh = make(chan struct{}, 1)
	return c
}

func TestWebcamIdleTimeout(t *testing.T) {
	ctx := context.Background()
	first := &fakeDriver{}
	c := newFakeWebcam(t, 100*time.Millisecond, first)

	var opens atomic.Int32
	second := &fakeDriver{}
	var openErr atomic.Bool
	c.openCamera = func(*WebcamConfig, string, logging.Logger) (video.Reader, driverutils.Driver, string, error) {
		opens.Add(1)
		if openErr.Load() {
			return nil, nil, "", errors.New("camera busy")
		}
		return fakeReader(), second, "fake", nil
	}
	c.startMonitorWorker()
	c.startBufferWorker()

	// Streams while recently accessed.
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		imgs, _, err := c.Images(ctx, nil, nil)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, imgs, test.ShouldHaveLength, 1)
	})

	// Goes idle and closes the camera once unused (driver close happens after idle is set).
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		c.mu.Lock()
		idle := c.idle
		c.mu.Unlock()
		test.That(tb, idle, test.ShouldBeTrue)
		test.That(tb, first.closes.Load(), test.ShouldEqual, int32(1))
	})
	test.That(t, opens.Load(), test.ShouldEqual, int32(0))

	// Next request reopens and returns a frame synchronously.
	imgs, _, err := c.Images(ctx, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, imgs, test.ShouldHaveLength, 1)
	test.That(t, opens.Load(), test.ShouldEqual, int32(1))
	c.mu.Lock()
	test.That(t, c.idle, test.ShouldBeFalse)
	test.That(t, c.driver, test.ShouldEqual, second)
	c.mu.Unlock()

	// A failed reopen surfaces to the caller and leaves the camera idle for the next attempt.
	openErr.Store(true)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		c.mu.Lock()
		idle := c.idle
		c.mu.Unlock()
		test.That(tb, idle, test.ShouldBeTrue)
	})
	_, _, err = c.Images(ctx, nil, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "failed to reopen idle camera")
	openErr.Store(false)
	imgs, _, err = c.Images(ctx, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, imgs, test.ShouldHaveLength, 1)

	// Caller context cancellation is honored while waiting on a reopen.
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		c.mu.Lock()
		idle := c.idle
		c.mu.Unlock()
		test.That(tb, idle, test.ShouldBeTrue)
	})
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, _, err = c.Images(cancelled, nil, nil)
	test.That(t, errors.Is(err, context.Canceled), test.ShouldBeTrue)

	test.That(t, c.Close(ctx), test.ShouldBeNil)
	_, _, err = c.Images(ctx, nil, nil)
	test.That(t, errors.Is(err, errClosed), test.ShouldBeTrue)
}

func TestWebcamIdleTimeoutDisabled(t *testing.T) {
	ctx := context.Background()
	first := &fakeDriver{}
	c := newFakeWebcam(t, 0, first)
	var opens atomic.Int32
	c.openCamera = func(*WebcamConfig, string, logging.Logger) (video.Reader, driverutils.Driver, string, error) {
		opens.Add(1)
		return fakeReader(), &fakeDriver{}, "fake", nil
	}
	c.startMonitorWorker()
	c.startBufferWorker()

	time.Sleep(300 * time.Millisecond)
	c.mu.Lock()
	test.That(t, c.idle, test.ShouldBeFalse)
	c.mu.Unlock()
	test.That(t, first.closes.Load(), test.ShouldEqual, int32(0))

	imgs, _, err := c.Images(ctx, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, imgs, test.ShouldHaveLength, 1)
	test.That(t, c.Close(ctx), test.ShouldBeNil)
	test.That(t, first.closes.Load(), test.ShouldEqual, int32(1))
	test.That(t, opens.Load(), test.ShouldEqual, int32(0))
}
