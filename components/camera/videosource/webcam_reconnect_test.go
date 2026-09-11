//go:build darwin

// Note: reconnecting by name is only needed on darwin

package videosource

import (
	"context"
	"image"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/driver/availability"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// fakeCamera is a mediadevices video adapter whose availability the test controls. Reporting
// availability.ErrNoDevice is what the real darwin driver does once a device is unplugged, so flipping
// available to false is how a test simulates a hot unplug.
type fakeCamera struct {
	available atomic.Bool
}

func newFakeCamera(available bool) *fakeCamera {
	f := &fakeCamera{}
	f.available.Store(available)
	return f
}

func (f *fakeCamera) Open() error  { return nil }
func (f *fakeCamera) Close() error { return nil }

func (f *fakeCamera) Properties() []prop.Media {
	return []prop.Media{{Video: prop.Video{Width: 640, Height: 480, FrameRate: 30, FrameFormat: frame.FormatI420}}}
}

func (f *fakeCamera) VideoRecord(p prop.Media) (video.Reader, error) {
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

// registerFakeCamera registers a fakeCamera in the global mediadevices driver manager under the given Label
// and Name and returns both the fake and the registered driver. The registration is removed when the test ends.
// Labels must not contain path separators so that findReaderAndDriver's filepath.Base leaves them unchanged.
func registerFakeCamera(t *testing.T, label, name string, available bool) (*fakeCamera, driver.Driver) {
	t.Helper()
	fake := newFakeCamera(available)
	manager := driver.GetManager()
	err := manager.Register(fake, driver.Info{Label: label, Name: name, DeviceType: driver.Camera})
	test.That(t, err, test.ShouldBeNil)

	drivers := manager.Query(labelFilter(label, false, false))
	test.That(t, drivers, test.ShouldHaveLength, 1)
	d := drivers[0]

	t.Cleanup(func() {
		if d.Status() != driver.StateClosed {
			test.That(t, d.Close(), test.ShouldBeNil)
		}
		manager.Delete(d.ID())
	})
	return fake, d
}

// The monitor worker ticks every 500ms; three ticks is long enough to be sure a reconnect that was going to
// happen would have happened.
const monitorSettleTime = 1500 * time.Millisecond

// newTestWebcam opens the given registered driver and wires it into a webcam with only the monitor worker
// running. It bypasses NewWebcam because that starts the AVFoundation observer, which is not available in a
// test process and is not what these tests exercise.
func newTestWebcam(t *testing.T, d driver.Driver) *webcam {
	t.Helper()
	logger := logging.NewTestLogger(t)
	label := d.Info().Label
	conf := WebcamConfig{Path: label, FrameRate: defaultFrameRate}

	reader, opened, err := getReaderAndDriver(labelFilter(label, true, false), label, makeConstraints(&conf, logger), logger)
	test.That(t, err, test.ShouldBeNil)

	c := &webcam{
		Named:      resource.NewName(camera.API, "test-webcam").AsNamed(),
		logger:     logger,
		workers:    goutils.NewBackgroundStoppableWorkers(),
		buffer:     newWebcamBuffer(),
		reader:     reader,
		driver:     opened,
		targetPath: label,
		targetName: d.Info().Name,
		conf:       conf,
	}
	t.Cleanup(func() {
		test.That(t, c.Close(context.Background()), test.ShouldBeNil)
	})
	c.startMonitorWorker()
	return c
}

type webcamSnapshot struct {
	disconnected     bool
	sawOtherSameName bool
	targetPath       string
	driverLabel      string
}

func snapshot(c *webcam) webcamSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := webcamSnapshot{
		disconnected:     c.disconnected,
		sawOtherSameName: c.sawOtherSameName,
		targetPath:       c.targetPath,
	}
	if c.driver != nil {
		s.driverLabel = c.driver.Info().Label
	}
	return s
}

// unplug mimics what the darwin observer does when a device disappears: the device stops reporting as
// available and its registration is removed from the driver manager.
func unplug(fake *fakeCamera, d driver.Driver) {
	fake.available.Store(false)
	driver.GetManager().Delete(d.ID())
}

func waitForDisconnect(t *testing.T, c *webcam) {
	t.Helper()
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, snapshot(c).disconnected, test.ShouldBeTrue)
	})
}

func waitForReconnect(t *testing.T, c *webcam, wantLabel string) {
	t.Helper()
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		s := snapshot(c)
		test.That(tb, s.disconnected, test.ShouldBeFalse)
		test.That(tb, s.targetPath, test.ShouldEqual, wantLabel)
		test.That(tb, s.driverLabel, test.ShouldEqual, wantLabel)
	})
}

func TestMonitorReconnectsByNameWhenPathChanges(t *testing.T) {
	fakeA, a := registerFakeCamera(t, "rdk-test-replug-a", "rdk-test-replug-cam", true)
	c := newTestWebcam(t, a)

	unplug(fakeA, a)
	waitForDisconnect(t, c)

	// The same camera comes back under a new UID, as happens when it is plugged into a different port.
	registerFakeCamera(t, "rdk-test-replug-a2", "rdk-test-replug-cam", true)
	waitForReconnect(t, c, "rdk-test-replug-a2")
}

func TestMonitorSkipsNameFallbackAfterSeeingDuplicate(t *testing.T) {
	fakeA, a := registerFakeCamera(t, "rdk-test-dup-a", "rdk-test-dup-cam", true)
	_, b := registerFakeCamera(t, "rdk-test-dup-b", "rdk-test-dup-cam", true)
	c := newTestWebcam(t, a)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, snapshot(c).sawOtherSameName, test.ShouldBeTrue)
	})

	// Removing the duplicate must not re-enable the fallback.
	driver.GetManager().Delete(b.ID())
	unplug(fakeA, a)
	waitForDisconnect(t, c)

	registerFakeCamera(t, "rdk-test-dup-a2", "rdk-test-dup-cam", true)
	time.Sleep(monitorSettleTime)
	s := snapshot(c)
	test.That(t, s.disconnected, test.ShouldBeTrue)
	test.That(t, s.driverLabel, test.ShouldEqual, "")

	// Reconnecting by the original path is unaffected by the flag.
	registerFakeCamera(t, "rdk-test-dup-a", "rdk-test-dup-cam", true)
	waitForReconnect(t, c, "rdk-test-dup-a")
}

func TestMonitorRefusesNameFallbackWhenMultipleAppear(t *testing.T) {
	fakeA, a := registerFakeCamera(t, "rdk-test-multi-a", "rdk-test-multi-cam", true)
	c := newTestWebcam(t, a)

	unplug(fakeA, a)
	waitForDisconnect(t, c)

	// Both candidates are registered unavailable first so that a monitor tick landing between the two
	// registrations cannot open the lone candidate; they are only made available once both exist.
	fakeA2, _ := registerFakeCamera(t, "rdk-test-multi-a2", "rdk-test-multi-cam", false)
	fakeA3, a3 := registerFakeCamera(t, "rdk-test-multi-a3", "rdk-test-multi-cam", false)
	fakeA2.available.Store(true)
	fakeA3.available.Store(true)

	time.Sleep(monitorSettleTime)
	s := snapshot(c)
	test.That(t, s.disconnected, test.ShouldBeTrue)
	test.That(t, s.driverLabel, test.ShouldEqual, "")

	// Once only one candidate remains the fallback proceeds, since no duplicate was ever seen while connected.
	driver.GetManager().Delete(a3.ID())
	waitForReconnect(t, c, "rdk-test-multi-a2")
}
