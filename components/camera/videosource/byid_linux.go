//go:build linux

package videosource

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// sysfsV4LDir is a variable so tests can point it at a fixture tree.
var sysfsV4LDir = "/sys/class/video4linux"

// resolveByIDName maps a /dev/v4l/by-id name to the device node backing it, returning a
// node name such as "video0".
//
// udev names those symlinks after ID_SERIAL, which it derives from the parent device's USB
// descriptors, and only creates them when ID_SERIAL is set. When a hub re-enumerates
// quickly, those sysfs attributes can still be unpopulated as the video4linux add event is
// processed. ID_SERIAL is then unset and no symlink is created, even though /dev/videoN
// works fine. Reading the descriptors directly recovers the mapping without waiting for
// udev.
//
// This is a fallback only. A camera whose symlink exists resolves through the normal path,
// so a mismatch here leaves behavior unchanged.
func resolveByIDName(target string) (string, error) {
	// Only USB devices ever get a by-id name, so anything else -- a bare /dev/videoN, or a
	// by-path name -- cannot match and is not worth scanning sysfs for.
	if !strings.HasPrefix(target, "usb-") || !strings.Contains(target, "-video-index") {
		return "", fmt.Errorf("%q is not a by-id device name", target)
	}

	entries, err := filepath.Glob(filepath.Join(sysfsV4LDir, "video*"))
	if err != nil {
		return "", fmt.Errorf("failed to list %s: %w", sysfsV4LDir, err)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no video4linux devices present in %s", sysfsV4LDir)
	}

	var matches, tried []string
	for _, entry := range entries {
		device := filepath.Base(entry)
		name, err := v4lByIDName(device)
		if err != nil {
			continue
		}
		if name == target {
			matches = append(matches, device)
			continue
		}
		tried = append(tried, fmt.Sprintf("%s=%s", device, name))
	}

	// Devices that report no serial reconstruct to just vendor and model, so two identical
	// units collide. udev picks one owner for the symlink; sysfs cannot say which, and
	// guessing would silently stream the wrong camera.
	if len(matches) > 1 {
		return "", fmt.Errorf("by-id name %q is ambiguous: %s all match, and without the udev symlink there is no way to tell them apart",
			target, strings.Join(matches, ", "))
	}
	if len(matches) == 1 {
		return matches[0], nil
	}

	if len(tried) == 0 {
		return "", fmt.Errorf("no video4linux device has by-id name %q (no usb descriptors could be read for any of the %d devices present)",
			target, len(entries))
	}

	return "", fmt.Errorf("no video4linux device has by-id name %q (found: %s)", target, strings.Join(tried, ", "))
}

// v4lByIDName reconstructs the /dev/v4l/by-id name udev would give a video device.
//
// usb_id consults the hwdb when a device reports no manufacturer or product string. That
// lookup is not reproduced here; the numeric ids are used instead, matching usb_id's last
// resort. A device relying on the hwdb will not match, which is safe because callers treat
// a miss as "not found".
func v4lByIDName(device string) (string, error) {
	classDir := filepath.Join(sysfsV4LDir, device)

	index := readSysAttr(classDir, "index")
	if index == "" {
		return "", fmt.Errorf("no index attribute for %s", device)
	}

	usbDir, err := usbDeviceDir(classDir)
	if err != nil {
		return "", err
	}

	vendor := udevEncode(readSysAttr(usbDir, "manufacturer"))
	if vendor == "" {
		vendor = udevEncode(readSysAttr(usbDir, "idVendor"))
	}
	model := udevEncode(readSysAttr(usbDir, "product"))
	if model == "" {
		model = udevEncode(readSysAttr(usbDir, "idProduct"))
	}
	if vendor == "" || model == "" {
		return "", fmt.Errorf("incomplete usb descriptors for %s", device)
	}

	idSerial := vendor + "_" + model
	// Capture cards frequently report no serial, leaving just vendor and model.
	if serial := udevEncode(readSysAttr(usbDir, "serial")); serial != "" {
		idSerial += "_" + serial
	}

	return fmt.Sprintf("usb-%s-video-index%s", idSerial, index), nil
}

// usbDeviceDir walks up from a video4linux class directory to the USB device holding the
// descriptor attributes. The class device's "device" link points at the USB interface,
// whose parent carries idVendor and friends.
func usbDeviceDir(classDir string) (string, error) {
	dir, err := filepath.EvalSymlinks(filepath.Join(classDir, "device"))
	if err != nil {
		return "", fmt.Errorf("failed to resolve device link for %s: %w", classDir, err)
	}

	for dir != "/" && dir != "." {
		if _, err := os.Stat(filepath.Join(dir, "idVendor")); err == nil {
			return dir, nil
		}
		dir = filepath.Dir(dir)
	}

	return "", fmt.Errorf("no parent usb device for %s", classDir)
}

func readSysAttr(dir, name string) string {
	contents, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // paths are built from a fixed sysfs root
	if err != nil {
		return ""
	}
	return strings.Trim(string(contents), udevWhitespace)
}

// udevWhitespace is udev's WHITESPACE set. It is narrower than unicode.IsSpace: udev treats
// \v and \f as ordinary unsafe characters that become an underscore in place, and passes
// non-ASCII spaces such as U+00A0 through as valid UTF-8.
const udevWhitespace = " \t\n\r"

// udevEncode mirrors udev_replace_whitespace followed by udev_replace_chars, which is how
// usb_id turns a raw USB descriptor string into an ID_SERIAL component. Leading and
// trailing whitespace is dropped and internal runs collapse to a single underscore; any
// remaining character outside udev's device-node safe set becomes an underscore. Valid
// multi-byte UTF-8 passes through, as udev_replace_chars does.
func udevEncode(s string) string {
	var out strings.Builder
	pendingSeparator := false
	wroteAny := false

	for _, r := range s {
		if strings.ContainsRune(udevWhitespace, r) {
			// Only a run between real content separates, so leading whitespace is
			// dropped and trailing whitespace never flushes.
			pendingSeparator = wroteAny
			continue
		}
		if pendingSeparator {
			out.WriteRune('_')
			pendingSeparator = false
		}
		wroteAny = true

		switch {
		case isDevnodeSafe(r):
			out.WriteRune(r)
		case r == utf8.RuneError:
			out.WriteRune('_')
		case r > unicode.MaxASCII:
			out.WriteRune(r)
		default:
			out.WriteRune('_')
		}
	}

	return out.String()
}

func isDevnodeSafe(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r >= '0' && r <= '9':
		return true
	default:
		return strings.ContainsRune("#+-.:=@_", r)
	}
}
