package videosource

import (
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func labelPrefix(prefix string) driver.FilterFn {
	return func(d driver.Driver) bool { return strings.HasPrefix(d.Info().Label, prefix) }
}

func labelsOf(drivers []driver.Driver) []string {
	labels := make([]string, 0, len(drivers))
	for _, d := range drivers {
		labels = append(labels, d.Info().Label)
	}
	slices.Sort(labels)
	return labels
}

// withoutAvailability hides IsAvailable, so the manager reports availability.ErrUnimplemented for the camera.
func withoutAvailability(f *fakeCamera) driver.Adapter {
	return struct {
		driver.Adapter
		driver.VideoRecorder
	}{f, f}
}

// unregisterAllCameras empties the driver manager so a search of every camera only sees the test's fakes.
// Real devices are not restored; no test in this package uses them.
func unregisterAllCameras() {
	manager := driver.GetManager()
	for _, d := range manager.Query(driver.FilterVideoRecorder()) {
		manager.Delete(d.ID())
	}
}

func TestSelectVideoDriver(t *testing.T) {
	logger := logging.NewTestLogger(t)
	for label, props := range map[string]prop.Media{
		"rdk-test-select-vga":      media(640, 480, 30, frame.FormatI420),
		"rdk-test-select-vga-60":   media(640, 480, 60, frame.FormatI420),
		"rdk-test-select-hd-mjpeg": media(1280, 720, 30, frame.FormatMJPEG),
	} {
		f, _ := registerFakeCamera(t, label, "", true)
		f.props = []prop.Media{props}
	}
	filter := labelPrefix("rdk-test-select-")

	for _, tc := range []struct {
		name string
		conf WebcamConfig
		want string
	}{
		{"unset config prefers 640x480 at 30fps", WebcamConfig{}, "rdk-test-select-vga"},
		{"frame rate", WebcamConfig{FrameRate: 60}, "rdk-test-select-vga-60"},
		{"width", WebcamConfig{Width: 1280}, "rdk-test-select-hd-mjpeg"},
		{"format", WebcamConfig{Format: frame.FormatMJPEG}, "rdk-test-select-hd-mjpeg"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, _, err := getUserVideoDriver(makeConstraints(&tc.conf, logger), filter, "", logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, d.Info().Label, test.ShouldEqual, tc.want)
		})
	}

	t.Run("no camera satisfies the config", func(t *testing.T) {
		_, _, err := getUserVideoDriver(makeConstraints(&WebcamConfig{Width: 1920}, logger), filter, "", logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "matches the config constraints")
		test.That(t, err.Error(), test.ShouldContainSubstring, "rdk-test-select-vga")
	})

	t.Run("no queryable cameras", func(t *testing.T) {
		constraints := makeConstraints(&WebcamConfig{}, logger)
		_, _, err := getUserVideoDriver(constraints, labelPrefix("rdk-test-missing"), "rdk-test-missing", logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "check if the device")
		_, _, err = getUserVideoDriver(constraints, labelPrefix("rdk-test-missing"), "", logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldNotContainSubstring, "check if the device")
	})

	t.Run("priority outweighs a slightly worse fit", func(t *testing.T) {
		registerFakeCamera(t, "rdk-test-priority-normal", "", true)
		high := newFakeCamera(true)
		high.props = []prop.Media{media(640, 480, 33, frame.FormatI420)}
		registerCamera(t, high, driver.Info{Label: "rdk-test-priority-high", Priority: driver.PriorityHigh})

		d, _, err := getUserVideoDriver(makeConstraints(&WebcamConfig{}, logger), labelPrefix("rdk-test-priority-"), "", logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, d.Info().Label, test.ShouldEqual, "rdk-test-priority-high")
	})
}

func TestQueryDriverProperties(t *testing.T) {
	registerFakeCamera(t, "rdk-test-query-ok", "", true)
	registerFakeCamera(t, "rdk-test-query-unplugged", "", false)
	openFails, _ := registerFakeCamera(t, "rdk-test-query-open-fails", "", true)
	openFails.openErr = errOpenFailed
	panics, _ := registerFakeCamera(t, "rdk-test-query-panics", "", true)
	panics.propsPanic = true
	registerCamera(t, withoutAvailability(newFakeCamera(true)), driver.Info{Label: "rdk-test-query-unknown"})

	filter := labelPrefix("rdk-test-query-")
	props := queryDriverProperties(filter, logging.NewTestLogger(t))
	test.That(t, labelsOf(slices.Collect(maps.Keys(props))), test.ShouldResemble,
		[]string{"rdk-test-query-ok", "rdk-test-query-unknown"})

	for _, d := range driver.GetManager().Query(filter) {
		test.That(t, d.Status(), test.ShouldEqual, driver.StateClosed)
	}
}

func TestOpenDriver(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*fakeCamera, driver.Driver)
		wantErr string
	}{
		{"unavailable", func(f *fakeCamera, _ driver.Driver) { f.available.Store(false) }, "video driver not available"},
		{"in use", func(_ *fakeCamera, d driver.Driver) { _ = d.Open() }, "video driver in use"},
		{"open fails", func(f *fakeCamera, _ driver.Driver) { f.openErr = errOpenFailed }, "cannot open video driver"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, d := registerFakeCamera(t, "rdk-test-open", "", true)
			tc.prepare(f, d)
			err := openDriver(d)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.wantErr)
		})
	}

	t.Run("availability unimplemented", func(t *testing.T) {
		d := registerCamera(t, withoutAvailability(newFakeCamera(true)), driver.Info{Label: "rdk-test-open-unknown"})
		test.That(t, openDriver(d), test.ShouldBeNil)
	})
}

func TestFindRegisteredReaderAndDriver(t *testing.T) {
	logger := logging.NewTestLogger(t)
	conf := &WebcamConfig{}

	t.Run("by one of the label's paths", func(t *testing.T) {
		f, d := registerFakeCamera(t, "rdk-test-find-by-id;rdk-test-find-video9", "", true)

		_, got, path, err := findRegisteredReaderAndDriver(conf, "rdk-test-find-video9", logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, path, test.ShouldEqual, "rdk-test-find-video9")
		test.That(t, got.ID(), test.ShouldEqual, d.ID())
		recorded := f.recorded.Load()
		test.That(t, recorded.Width, test.ShouldEqual, 640)
		test.That(t, recorded.Height, test.ShouldEqual, 480)
		test.That(t, recorded.DiscardFramesOlderThan, test.ShouldEqual, time.Second)
	})

	t.Run("through a symlink", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("windows matches the full path and symlinks need elevated privileges")
		}
		dir := t.TempDir()
		target := filepath.Join(dir, "rdk-test-find-symlinked")
		test.That(t, os.WriteFile(target, nil, 0o600), test.ShouldBeNil)
		link := filepath.Join(dir, "video0")
		test.That(t, os.Symlink(target, link), test.ShouldBeNil)
		_, d := registerFakeCamera(t, "rdk-test-find-symlinked", "", true)

		_, got, path, err := findRegisteredReaderAndDriver(conf, link, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got.ID(), test.ShouldEqual, d.ID())
		resolved, err := filepath.EvalSymlinks(target)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, path, test.ShouldEqual, resolved)
	})

	t.Run("any camera returns its first path", func(t *testing.T) {
		unregisterAllCameras()
		registerFakeCamera(t, "rdk-test-find-any;rdk-test-find-any-id", "", true)

		_, _, path, err := findRegisteredReaderAndDriver(conf, "", logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, path, test.ShouldEqual, "rdk-test-find-any")
	})

	t.Run("no cameras", func(t *testing.T) {
		unregisterAllCameras()
		_, _, _, err := findRegisteredReaderAndDriver(conf, "", logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "found no webcams")
	})
}

func TestRefreshWindowsDrivers(t *testing.T) {
	unregisterAllCameras()
	manager := driver.GetManager()
	filter := labelPrefix("rdk-test-win-")
	t.Cleanup(func() {
		for _, d := range manager.Query(filter) {
			manager.Delete(d.ID())
		}
	})
	_, held := registerFakeCamera(t, "rdk-test-win-held", "", true)
	test.That(t, held.Open(), test.ShouldBeNil)
	registerFakeCamera(t, "rdk-test-win-unplugged", "", true)

	refreshWindowsDrivers(func() {
		// Like the windows driver, registers every connected camera again, including the open one.
		for _, label := range []string{"rdk-test-win-held", "rdk-test-win-new"} {
			test.That(t, manager.Register(newFakeCamera(true), driver.Info{Label: label, DeviceType: driver.Camera}), test.ShouldBeNil)
		}
	})

	test.That(t, labelsOf(manager.Query(filter)), test.ShouldResemble, []string{"rdk-test-win-held", "rdk-test-win-new"})
	test.That(t, manager.Query(driver.FilterID(held.ID())), test.ShouldHaveLength, 1)
}
