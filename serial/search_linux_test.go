package serial

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"go.viam.com/robotcore/usb"

	"github.com/edaniels/test"
)

func TestSearchDevices(t *testing.T) {
	tempDir1, err := ioutil.TempDir("", "")
	test.That(t, err, test.ShouldBeNil)
	defer os.RemoveAll(tempDir1) // clean up
	tempDir2, err := ioutil.TempDir("", "")
	test.That(t, err, test.ShouldBeNil)
	defer os.RemoveAll(tempDir2) // clean up

	prevSysPaths := usb.SysPaths
	defer func() {
		usb.SysPaths = prevSysPaths
	}()

	dev1Root, err := ioutil.TempDir(tempDir1, "")
	test.That(t, err, test.ShouldBeNil)
	dev2Root, err := ioutil.TempDir(tempDir1, "")
	test.That(t, err, test.ShouldBeNil)
	dev3Root, err := ioutil.TempDir(tempDir1, "")
	test.That(t, err, test.ShouldBeNil)
	dev1, err := ioutil.TempDir(dev1Root, "")
	test.That(t, err, test.ShouldBeNil)
	dev2, err := ioutil.TempDir(dev2Root, "")
	test.That(t, err, test.ShouldBeNil)
	dev3, err := ioutil.TempDir(dev3Root, "")
	test.That(t, err, test.ShouldBeNil)

	test.That(t, os.WriteFile(filepath.Join(dev1Root, "uevent"), []byte("PRODUCT=2341/0043"), 0666), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev3Root, "uevent"), []byte("PRODUCT=10c5/ea61"), 0666), test.ShouldBeNil)

	test.That(t, os.Mkdir(filepath.Join(dev1, "tty"), 0700), test.ShouldBeNil)
	test.That(t, os.Mkdir(filepath.Join(dev3, "tty"), 0700), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev1, "tty", "one"), []byte("a"), 0666), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev3, "tty", "two"), []byte("b"), 0666), test.ShouldBeNil)

	test.That(t, os.Symlink(dev1, path.Join(tempDir2, filepath.Base(dev1))), test.ShouldBeNil)
	test.That(t, os.Symlink(dev3, path.Join(tempDir2, filepath.Base(dev2))), test.ShouldBeNil)

	for i, tc := range []struct {
		Filter   SearchFilter
		Paths    []string
		Expected []DeviceDescription
	}{
		{SearchFilter{}, nil, nil},
		{SearchFilter{}, []string{"/"}, nil},
		{SearchFilter{}, []string{tempDir2}, []DeviceDescription{
			{Type: DeviceTypeArduino, Path: "/dev/one"}},
		},
		{SearchFilter{Type: DeviceTypeArduino}, []string{tempDir2}, []DeviceDescription{
			{Type: DeviceTypeArduino, Path: "/dev/one"}},
		},
		{SearchFilter{Type: DeviceTypeJetson}, []string{tempDir2}, nil},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			usb.SysPaths = tc.Paths
			result := SearchDevices(tc.Filter)
			test.That(t, result, test.ShouldHaveLength, len(tc.Expected))
			expectedM := map[DeviceDescription]struct{}{}
			for _, e := range tc.Expected {
				expectedM[e] = struct{}{}
			}
			for _, desc := range result {
				delete(expectedM, desc)
			}
			test.That(t, expectedM, test.ShouldBeEmpty)
		})
	}
}
