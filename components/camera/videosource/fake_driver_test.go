package videosource

import (
	"context"
	"image"
	"sync"
	"testing"

	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/driver/availability"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// fakeDriver is an in-memory mediadevices camera adapter. Registering it with the driver
// manager lets the real webcam and query code run against it without hardware.
//
// Every field is guarded by mu because the webcam's monitor and buffer workers call into
// the driver from their own goroutines while tests mutate it.
type fakeDriver struct {
	mu        sync.Mutex
	props     []prop.Media
	frameSize image.Rectangle
	available bool
	openErr   error
	readErr   error
	opens     int
	closes    int
}

// newFakeDriver advertises and produces width x height frames.
func newFakeDriver(width, height int) *fakeDriver {
	return &fakeDriver{
		available: true,
		frameSize: image.Rect(0, 0, width, height),
		props: []prop.Media{{
			Video: prop.Video{
				Width:       width,
				Height:      height,
				FrameRate:   30,
				FrameFormat: frame.FormatI420,
			},
		}},
	}
}

func (f *fakeDriver) Open() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.openErr != nil {
		return f.openErr
	}
	f.opens++
	return nil
}

func (f *fakeDriver) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closes++
	return nil
}

func (f *fakeDriver) Properties() []prop.Media {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.props
}

// IsAvailable reports ErrNoDevice when unplugged, which is the signal isCameraConnected and
// queryDriverProperties use to treat a camera as gone.
func (f *fakeDriver) IsAvailable() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.available {
		return true, nil
	}
	return false, availability.ErrNoDevice
}

// VideoRecord ignores the requested media, like hardware that cannot honor a resolution,
// and produces frames of the driver's frameSize.
func (f *fakeDriver) VideoRecord(_ prop.Media) (video.Reader, error) {
	return video.ReaderFunc(func() (image.Image, func(), error) {
		f.mu.Lock()
		err := f.readErr
		size := f.frameSize
		f.mu.Unlock()
		if err != nil {
			return nil, nil, err
		}
		return image.NewRGBA(size), func() {}, nil
	}), nil
}

// setFrameSize makes produced frames differ from the advertised properties.
func (f *fakeDriver) setFrameSize(width, height int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.frameSize = image.Rect(0, 0, width, height)
}

func (f *fakeDriver) setAvailable(available bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.available = available
}

func (f *fakeDriver) setOpenErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.openErr = err
}

func (f *fakeDriver) setReadErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.readErr = err
}

func (f *fakeDriver) counts() (opens, closes int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.opens, f.closes
}

// registerFakeDriver registers f with the mediadevices manager under label and returns the
// wrapped driver the manager hands out. The registration is removed when the test ends.
func registerFakeDriver(t *testing.T, label string, f *fakeDriver) driver.Driver {
	t.Helper()
	manager := driver.GetManager()
	err := manager.Register(f, driver.Info{Label: label, DeviceType: driver.Camera})
	test.That(t, err, test.ShouldBeNil)

	drivers := manager.Query(labelFilter(label, false))
	test.That(t, drivers, test.ShouldHaveLength, 1)
	d := drivers[0]
	t.Cleanup(func() { manager.Delete(d.ID()) })
	return d
}

// isolateDriverRegistry stops the webcam from syncing the driver manager with real hardware for
// the rest of the test. Both the linux/windows re-scan and the darwin observer delete every
// registered video driver they do not recognize, fakes included.
func isolateDriverRegistry(t *testing.T) {
	t.Helper()
	origRefresh, origObserver := refreshDriverRegistry, startObserver
	refreshDriverRegistry = func() {}
	startObserver = func(logging.Logger) {}
	t.Cleanup(func() { refreshDriverRegistry, startObserver = origRefresh, origObserver })
}

// skipUnlessOnlyFakesRegistered skips tests that select a camera without a path. On a developer
// machine mediadevices registers real cameras at init, and such a test could open one.
func skipUnlessOnlyFakesRegistered(t *testing.T, fakes ...driver.Driver) {
	t.Helper()
	registered := driver.GetManager().Query(getVideoFilterBase())
	if len(registered) != len(fakes) {
		t.Skipf("host has %d real video drivers registered; skipping to avoid opening hardware", len(registered)-len(fakes))
	}
}

func newTestWebcamConfig(name string, conf WebcamConfig) resource.Config {
	rc := resource.NewEmptyConfig(resource.NewName(camera.API, name), ModelWebcam)
	rc.ConvertedAttributes = &conf
	return rc
}

// newTestWebcam constructs a webcam from conf. The caller registers whatever fake drivers the
// config should resolve to. A successfully built webcam is closed when the test ends.
func newTestWebcam(t *testing.T, conf WebcamConfig, logger logging.Logger) (*webcam, error) {
	t.Helper()
	isolateDriverRegistry(t)
	cam, err := NewWebcam(context.Background(), nil, newTestWebcamConfig("cam1", conf), logger)
	if err != nil {
		return nil, err
	}
	wc, ok := cam.(*webcam)
	test.That(t, ok, test.ShouldBeTrue)
	t.Cleanup(func() {
		// Tests that close explicitly make this a no-op that returns errClosed.
		_ = wc.Close(context.Background())
	})
	return wc, nil
}
