//go:build linux

package videosource

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/pion/mediadevices/pkg/driver"
	mediadevicescamera "github.com/pion/mediadevices/pkg/driver/camera"
	"go.viam.com/test"
)

// Adapted from github.com/pion/mediadevices/pkg/driver/camera/camera_linux_test.go TestDiscover.
// It only exercises path/symlink handling and driver registration; the temp files are not real
// V4L2 devices, so webcam.Open fails and name/bus info are empty, exactly as upstream.
func TestDiscoverV4L2(t *testing.T) {
	const (
		shortName  = "unittest-video0"
		shortName2 = "unittest-video1"
		longName   = "unittest-long-device-name:0:1:2:3"
	)

	dir := t.TempDir()
	byPathDir := filepath.Join(dir, "v4l", "by-path")
	test.That(t, os.MkdirAll(byPathDir, 0o755), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dir, shortName), []byte{}, 0o644), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dir, shortName2), []byte{}, 0o644), test.ShouldBeNil)
	test.That(t, os.Symlink(filepath.Join(dir, shortName), filepath.Join(byPathDir, longName)), test.ShouldBeNil)

	discovered := make(map[string]struct{})
	discoverV4L2(discovered, filepath.Join(byPathDir, "*"))
	discoverV4L2(discovered, filepath.Join(dir, "unittest-video*"))

	drvs := driver.GetManager().Query(func(d driver.Driver) bool {
		// Ignore real cameras.
		return d.Info().DeviceType == driver.Camera && strings.Contains(d.Info().Label, "unittest")
	})
	defer func() {
		for _, d := range drvs {
			driver.GetManager().Delete(d.ID())
		}
	}()
	test.That(t, drvs, test.ShouldHaveLength, 2)

	labels := []string{drvs[0].Info().Label, drvs[1].Info().Label}
	// Returned drivers are unordered. Sort to get static result.
	sort.Strings(labels)

	// The symlinked device is labeled "<link name>;<target name>"; the plain device "<name>;<name>".
	test.That(t, labels[0], test.ShouldEqual, longName+mediadevicescamera.LabelSeparator+shortName)
	test.That(t, labels[1], test.ShouldEqual, shortName2+mediadevicescamera.LabelSeparator+shortName2)
}

func TestV4L2BufferCount(t *testing.T) {
	t.Setenv(v4l2BufferCountEnv, "")
	os.Unsetenv(v4l2BufferCountEnv)
	test.That(t, v4l2BufferCount(), test.ShouldEqual, defaultV4L2BufferCount)

	t.Setenv(v4l2BufferCountEnv, "2")
	test.That(t, v4l2BufferCount(), test.ShouldEqual, 2)

	t.Setenv(v4l2BufferCountEnv, "0")
	test.That(t, v4l2BufferCount(), test.ShouldEqual, defaultV4L2BufferCount)

	t.Setenv(v4l2BufferCountEnv, "notanumber")
	test.That(t, v4l2BufferCount(), test.ShouldEqual, defaultV4L2BufferCount)
}

func TestCalcFramerate(t *testing.T) {
	fr, err := calcFramerate(1, 30)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fr, test.ShouldEqual, float32(30))

	fr, err = calcFramerate(1001, 30000)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fr, test.ShouldEqual, float32(29.97))

	_, err = calcFramerate(1, 0)
	test.That(t, err, test.ShouldNotBeNil)
}
