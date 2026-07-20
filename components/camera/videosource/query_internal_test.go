package videosource

import (
	"testing"

	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"
)

// fakeDriver is a minimal driver.Driver used to exercise label matching.
type fakeDriver struct {
	label string
}

func (d *fakeDriver) Open() error              { return nil }
func (d *fakeDriver) Close() error             { return nil }
func (d *fakeDriver) Properties() []prop.Media { return nil }
func (d *fakeDriver) ID() string               { return d.label }
func (d *fakeDriver) Info() driver.Info        { return driver.Info{Label: d.label} }
func (d *fakeDriver) Status() driver.State     { return driver.StateClosed }

// TestLabelFilterMatchesDeviceIDAndPath documents that both halves of a Linux
// "<stable device_id>;<device node>" label are matchable. device_id selection
// relies on the by-id/by-path half (labels[0]) matching here; video_path
// selection relies on the device-node half (labels[1]).
func TestLabelFilterMatchesDeviceIDAndPath(t *testing.T) {
	// Linux-style label: stable id ; device node.
	linux := &fakeDriver{label: "usb-046d_webcam-video-index0;video4"}
	// macOS/Windows-style label: a single identifier, no separator.
	single := &fakeDriver{label: "0x14200000046d0825"}

	t.Run("device_id half matches", func(t *testing.T) {
		f := labelFilter("usb-046d_webcam-video-index0", true)
		test.That(t, f(linux), test.ShouldBeTrue)
	})

	t.Run("video_path half matches", func(t *testing.T) {
		f := labelFilter("video4", true)
		test.That(t, f(linux), test.ShouldBeTrue)
	})

	t.Run("non-matching target is rejected", func(t *testing.T) {
		f := labelFilter("video9", true)
		test.That(t, f(linux), test.ShouldBeFalse)
	})

	t.Run("single-part label (macOS/windows) matches device_id", func(t *testing.T) {
		f := labelFilter("0x14200000046d0825", true)
		test.That(t, f(single), test.ShouldBeTrue)
	})
}
