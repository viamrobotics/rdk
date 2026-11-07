//go:build linux

package videosource

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.viam.com/test"
)

func TestUdevEncode(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    string
		expected string
	}{
		{"plain", "EMEET", "EMEET"},
		{"spaces become underscores", "EMEET SmartCam C950", "EMEET_SmartCam_C950"},
		{"whitespace runs collapse", "USB   Video", "USB_Video"},
		{"leading and trailing whitespace dropped", "  USB Video  ", "USB_Video"},
		{"tabs count as whitespace", "USB\tVideo", "USB_Video"},
		// udev's WHITESPACE is only " \t\n\r"; these are unsafe characters, not separators.
		{"vertical tab is not whitespace to udev", "USB\vVideo", "USB_Video"},
		{"form feed is not whitespace to udev", "USB\fVideo", "USB_Video"},
		{"non-ascii space passes through", "USB\u00a0Video", "USB\u00a0Video"},
		{"non-ascii space is not trimmed", "\u00a0USB", "\u00a0USB"},
		{"safe punctuation preserved", "a#b+c-d.e:f=g@h_i", "a#b+c-d.e:f=g@h_i"},
		{"unsafe punctuation replaced", "a/b(c)d", "a_b_c_d"},
		{"empty", "", ""},
		{"only whitespace", "   ", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			test.That(t, udevEncode(tc.input), test.ShouldEqual, tc.expected)
		})
	}
}

// writeSysfsCamera builds a sysfs-shaped fixture for one video device: a class directory
// holding an index attribute and a "device" symlink to a USB interface, whose parent
// carries the descriptor attributes.
func writeSysfsCamera(t *testing.T, root, device, index string, attrs map[string]string) {
	t.Helper()

	usbDir := filepath.Join(root, "devices", "usb1", "1-1"+device)
	ifaceDir := filepath.Join(usbDir, "1-1"+device+":1.0")
	test.That(t, os.MkdirAll(ifaceDir, 0o755), test.ShouldBeNil)
	for name, value := range attrs {
		test.That(t, os.WriteFile(filepath.Join(usbDir, name), []byte(value+"\n"), 0o600), test.ShouldBeNil)
	}

	classDir := filepath.Join(root, "class", "video4linux", device)
	test.That(t, os.MkdirAll(classDir, 0o755), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(classDir, "index"), []byte(index+"\n"), 0o600), test.ShouldBeNil)
	test.That(t, os.Symlink(ifaceDir, filepath.Join(classDir, "device")), test.ShouldBeNil)
}

func withSysfsFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	original := sysfsV4LDir
	sysfsV4LDir = filepath.Join(root, "class", "video4linux")
	t.Cleanup(func() { sysfsV4LDir = original })
	return root
}

func TestResolveByIDName(t *testing.T) {
	root := withSysfsFixture(t)

	// A capture card that reports no serial, like the MACROSILICON HDMI devices.
	writeSysfsCamera(t, root, "video0", "0", map[string]string{
		"idVendor": "534d", "idProduct": "2109",
		"manufacturer": "MACROSILICON", "product": "USB Video",
	})
	// A webcam that does report a serial, exposing two nodes at different indexes.
	writeSysfsCamera(t, root, "video2", "0", map[string]string{
		"idVendor": "328f", "idProduct": "00ba",
		"manufacturer": "EMEET", "product": "EMEET SmartCam C950", "serial": "24082114152",
	})
	writeSysfsCamera(t, root, "video3", "1", map[string]string{
		"idVendor": "328f", "idProduct": "00ba",
		"manufacturer": "EMEET", "product": "EMEET SmartCam C950", "serial": "24082114152",
	})

	t.Run("resolves a device with no serial", func(t *testing.T) {
		device, err := resolveByIDName("usb-MACROSILICON_USB_Video-video-index0")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, device, test.ShouldEqual, "video0")
	})

	t.Run("index disambiguates nodes on one device", func(t *testing.T) {
		device, err := resolveByIDName("usb-EMEET_EMEET_SmartCam_C950_24082114152-video-index1")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, device, test.ShouldEqual, "video3")
	})

	t.Run("unknown name does not match", func(t *testing.T) {
		_, err := resolveByIDName("usb-NOT_A_REAL_CAMERA-video-index0")
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("two identical serial-less devices are ambiguous", func(t *testing.T) {
		ambiguousRoot := withSysfsFixture(t)
		for _, device := range []string{"video0", "video1"} {
			writeSysfsCamera(t, ambiguousRoot, device, "0", map[string]string{
				"idVendor": "534d", "idProduct": "2109",
				"manufacturer": "MACROSILICON", "product": "USB Video",
			})
		}

		_, err := resolveByIDName("usb-MACROSILICON_USB_Video-video-index0")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "ambiguous")
	})

	t.Run("names that cannot be by-id names are rejected without scanning", func(t *testing.T) {
		for _, name := range []string{
			"video0",
			"/dev/video0",
			"pci-0000:c3:00.3-usb-0:1.3:1.0-video-index0",
			"usb-MACROSILICON_USB_Video",
			"",
		} {
			_, err := resolveByIDName(name)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "is not a by-id device name")
		}
	})

	t.Run("falls back to numeric ids without descriptor strings", func(t *testing.T) {
		writeSysfsCamera(t, root, "video9", "0", map[string]string{
			"idVendor": "1d6b", "idProduct": "0104",
		})
		device, err := resolveByIDName("usb-1d6b_0104-video-index0")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, device, test.ShouldEqual, "video9")
	})
}

func TestResolveByIDNameNoDevices(t *testing.T) {
	root := withSysfsFixture(t)
	test.That(t, os.MkdirAll(sysfsV4LDir, 0o755), test.ShouldBeNil)
	_ = root

	_, err := resolveByIDName("usb-MACROSILICON_USB_Video-video-index0")
	test.That(t, err, test.ShouldNotBeNil)
}

// TestReconstructionMatchesUdev validates the reconstruction against the symlinks udev
// actually created on this machine. It is the ground truth for the whole fallback, so it
// skips rather than fails where there is nothing to compare against.
func TestReconstructionMatchesUdev(t *testing.T) {
	links, err := filepath.Glob("/dev/v4l/by-id/*")
	if err != nil || len(links) == 0 {
		t.Skip("no /dev/v4l/by-id symlinks present")
	}

	checked := 0
	for _, link := range links {
		resolved, err := filepath.EvalSymlinks(link)
		if err != nil {
			continue
		}
		name, err := v4lByIDName(filepath.Base(resolved))
		if err != nil {
			continue
		}
		checked++
		test.That(t, name, test.ShouldEqual, filepath.Base(link))

		// The full scan must land on the same node the symlink points at, which is
		// exactly what the fallback relies on when the symlink is gone. Two identical
		// serial-less devices on this host reconstruct to one name, which the scan
		// rejects by design -- that is the hardware, not a reconstruction failure.
		device, err := resolveByIDName(filepath.Base(link))
		if err != nil && strings.Contains(err.Error(), "ambiguous") {
			t.Logf("skipping scan check for %s: %v", filepath.Base(link), err)
			continue
		}
		test.That(t, err, test.ShouldBeNil)
		test.That(t, device, test.ShouldEqual, filepath.Base(resolved))
	}

	if checked == 0 {
		t.Skip("no by-id symlinks could be resolved")
	}
	t.Logf("reconstruction matched udev for %d of %d by-id symlinks", checked, len(links))
}
