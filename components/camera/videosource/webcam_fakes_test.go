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
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var (
	errOpenFailed  = errors.New("fake open failed")
	errReadFailed  = errors.New("fake read failed")
	errCloseFailed = errors.New("fake close failed")
)

type fakeDriver struct {
	closeErr    error
	closes      atomic.Int32
	unavailable atomic.Bool
}

func (d *fakeDriver) Open() error               { return nil }
func (d *fakeDriver) Close() error              { d.closes.Add(1); return d.closeErr }
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

// gate holds callers of wait while blocked.
type gate struct{ ch atomic.Pointer[chan struct{}] }

func (g *gate) wait() {
	if ch := g.ch.Load(); ch != nil {
		<-*ch
	}
}

// block holds callers of wait until the returned func is called. Cleanup calls it too, so a failed assertion
// cannot leave Close waiting forever on a held call; call block after the webcam is created so it runs first.
func (g *gate) block(t *testing.T) func() {
	t.Helper()
	ch := make(chan struct{})
	g.ch.Store(&ch)
	unblock := sync.OnceFunc(func() {
		g.ch.Store(nil)
		close(ch)
	})
	t.Cleanup(unblock)
	return unblock
}

// fakeReader counts reads and, while blocked, holds each one until unblocked. The Nth read (1-based) is
// N pixels wide so tests can tell how many reads happened before a frame was buffered. While err is
// set reads fail with it. Each frame's release is tracked so tests can check none is leaked or doubled.
type fakeReader struct {
	gate
	reads          atomic.Int32
	frames         atomic.Int32
	releases       atomic.Int32
	doubleReleases atomic.Int32
	err            atomic.Pointer[error]
}

func (r *fakeReader) Read() (image.Image, func(), error) {
	n := int(r.reads.Add(1))
	r.wait()
	if err := r.err.Load(); err != nil && *err != nil {
		return nil, nil, *err
	}
	r.frames.Add(1)
	var released atomic.Bool
	return image.NewRGBA(image.Rect(0, 0, n, 1)), func() {
		if released.Swap(true) {
			r.doubleReleases.Add(1)
			return
		}
		r.releases.Add(1)
	}, nil
}

// failWith makes subsequent reads fail with err, or succeed again when err is nil.
func (r *fakeReader) failWith(err error) { r.err.Store(&err) }

// assertAllReleased asserts every frame handed out was released exactly once. Only meaningful once the
// webcam is closed, since the buffered frame is held until then.
func assertAllReleased(t *testing.T, r *fakeReader) {
	t.Helper()
	test.That(t, r.frames.Load(), test.ShouldBeGreaterThan, int32(0))
	test.That(t, r.releases.Load(), test.ShouldEqual, r.frames.Load())
	test.That(t, r.doubleReleases.Load(), test.ShouldEqual, int32(0))
}

// fakeOpener stands in for findReaderAndDriver during monitor reconnects. Opens fail while failing is set and,
// while blocked, are held until unblocked.
type fakeOpener struct {
	gate
	opens   atomic.Int32
	failing atomic.Bool
	reader  *fakeReader
	driver  *fakeDriver

	mu    sync.Mutex
	paths []string
}

func (o *fakeOpener) open(_ *WebcamConfig, path string, _ logging.Logger) (video.Reader, driverutils.Driver, string, error) {
	o.mu.Lock()
	o.paths = append(o.paths, path)
	o.mu.Unlock()
	o.opens.Add(1)
	o.wait()
	if o.failing.Load() {
		return nil, nil, "", errOpenFailed
	}
	return o.reader, o.driver, "", nil
}

func (o *fakeOpener) openedPaths() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.paths...)
}

// fakeCamera is a mediadevices camera adapter for tests that go through the real driver manager. Reporting
// availability.ErrNoDevice is what the real darwin driver does once a device is unplugged, so flipping
// available to false is how a test simulates a hot unplug. Set the other fields before the camera is used.
type fakeCamera struct {
	available  atomic.Bool
	props      []prop.Media
	openErr    error
	propsPanic bool
	recorded   atomic.Pointer[prop.Media]
}

func newFakeCamera(available bool) *fakeCamera {
	f := &fakeCamera{props: []prop.Media{media(640, 480, 30, frame.FormatI420)}}
	f.available.Store(available)
	return f
}

func media(width, height int, frameRate float32, format frame.Format) prop.Media {
	return prop.Media{Video: prop.Video{Width: width, Height: height, FrameRate: frameRate, FrameFormat: format}}
}

func (f *fakeCamera) Open() error  { return f.openErr }
func (f *fakeCamera) Close() error { return nil }

func (f *fakeCamera) Properties() []prop.Media {
	if f.propsPanic {
		panic("fake camera properties")
	}
	return f.props
}

func (f *fakeCamera) VideoRecord(p prop.Media) (video.Reader, error) {
	f.recorded.Store(&p)
	img := image.NewRGBA(image.Rect(0, 0, p.Width, p.Height))
	return video.ReaderFunc(func() (image.Image, func(), error) {
		return img, func() {}, nil
	}), nil
}

func (f *fakeCamera) IsAvailable() (bool, error) {
	if !f.available.Load() {
		return false, availability.ErrNoDevice
	}
	return true, nil
}

// registerCamera registers a in the global mediadevices driver manager and removes it when the test ends.
// Labels must be unique and free of path separators so that findReaderAndDriver's filepath.Base leaves them unchanged.
func registerCamera(t *testing.T, a driverutils.Adapter, info driverutils.Info) driverutils.Driver {
	t.Helper()
	info.DeviceType = driverutils.Camera
	manager := driverutils.GetManager()
	test.That(t, manager.Register(a, info), test.ShouldBeNil)

	drivers := manager.Query(labelFilter(info.Label, false, false))
	test.That(t, drivers, test.ShouldHaveLength, 1)
	d := drivers[0]

	t.Cleanup(func() {
		if d.Status() != driverutils.StateClosed {
			test.That(t, d.Close(), test.ShouldBeNil)
		}
		manager.Delete(d.ID())
	})
	return d
}

func registerFakeCamera(t *testing.T, label, name string, available bool) (*fakeCamera, driverutils.Driver) {
	t.Helper()
	f := newFakeCamera(available)
	return f, registerCamera(t, f, driverutils.Info{Label: label, Name: name})
}

const (
	testIdleTimeoutMs   = 100
	testMonitorInterval = 10 * time.Millisecond
)

// startFakeWebcam wraps the fakes in a webcam with a fast monitor tick, starts its workers, and closes it
// when the test ends.
func startFakeWebcam(
	t *testing.T,
	conf WebcamConfig,
	logger logging.Logger,
	reader *fakeReader,
	driver *fakeDriver,
	opener *fakeOpener,
) *webcam {
	t.Helper()
	c := newWebcam(resource.NewName(camera.API, "cam"), conf, "fake", reader, driver, opener.open, logger)
	c.monitorInterval = testMonitorInterval
	c.start()
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

func newFakeWebcam(t *testing.T, idleTimeoutMs int, reader *fakeReader, driver *fakeDriver, opener *fakeOpener) *webcam {
	t.Helper()
	conf := WebcamConfig{FrameRate: 100, IdleTimeoutMs: idleTimeoutMs}
	return startFakeWebcam(t, conf, logging.NewTestLogger(t), reader, driver, opener)
}

func idleStateOf(c *webcam) idleState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.idleState
}

func isDisconnected(c *webcam) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.disconnected
}

// usesDriver reports whether c holds d. Compared here rather than with test.ShouldEqual, whose failure
// output reads the fakes' atomic counters non-atomically.
func usesDriver(c *webcam, d *fakeDriver) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.driver == d
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
