package usb

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/edaniels/test"
)

func TestSearchDevices(t *testing.T) {
	tempDir1, err := ioutil.TempDir("", "")
	test.That(t, err, test.ShouldBeNil)
	defer os.RemoveAll(tempDir1) // clean up
	tempDir2, err := ioutil.TempDir("", "")
	test.That(t, err, test.ShouldBeNil)
	defer os.RemoveAll(tempDir2) // clean up

	prevSysPaths := sysPaths
	defer func() {
		sysPaths = prevSysPaths
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

	test.That(t, os.WriteFile(filepath.Join(dev1Root, "uevent"), []byte("PRODUCT=10c4/ea60"), 0666), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev3Root, "uevent"), []byte("PRODUCT=10c5/ea61"), 0666), test.ShouldBeNil)

	test.That(t, os.Mkdir(filepath.Join(dev1, "tty"), 0700), test.ShouldBeNil)
	test.That(t, os.Mkdir(filepath.Join(dev3, "tty"), 0700), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev1, "tty", "one"), []byte("a"), 0666), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev3, "tty", "two"), []byte("b"), 0666), test.ShouldBeNil)

	test.That(t, os.Symlink(dev1, path.Join(tempDir2, filepath.Base(dev1))), test.ShouldBeNil)
	test.That(t, os.Symlink(dev3, path.Join(tempDir2, filepath.Base(dev2))), test.ShouldBeNil)

	for _, tc := range []struct {
		IncludeDevice func(vendorID, productID int) bool
		Paths         []string
		Expected      []DeviceDescription
	}{
		{nil, nil, nil},
		{nil, []string{"/"}, nil},
		{nil, []string{tempDir2}, nil},
		{func(vendorID, productID int) bool {
			return true
		}, []string{tempDir2}, []DeviceDescription{
			{ID: Identifier{Vendor: 4292, Product: 60000}, Path: "/dev/one"},
			{ID: Identifier{Vendor: 4293, Product: 60001}, Path: "/dev/two"},
		}},
		{func(vendorID, productID int) bool {
			return vendorID == 4292 && productID == 60000
		}, []string{tempDir2}, []DeviceDescription{
			{ID: Identifier{Vendor: 4292, Product: 60000}, Path: "/dev/one"},
		}},
		{func(vendorID, productID int) bool {
			return false
		}, []string{tempDir2}, nil},
	} {
		sysPaths = tc.Paths
		result := SearchDevices(SearchFilter{}, tc.IncludeDevice)
		test.That(t, result, test.ShouldHaveLength, len(tc.Expected))
		expectedM := map[DeviceDescription]struct{}{}
		for _, e := range tc.Expected {
			expectedM[e] = struct{}{}
		}
		for _, desc := range result {
			delete(expectedM, desc)
		}
		test.That(t, expectedM, test.ShouldBeEmpty)
	}
}
