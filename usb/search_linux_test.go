package usb

import (
	"fmt"
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

	prevSysPaths := SysPaths
	defer func() {
		SysPaths = prevSysPaths
	}()

	dev2Root, err := ioutil.TempDir(tempDir1, "")
	test.That(t, err, test.ShouldBeNil)
	dev3Root, err := ioutil.TempDir(tempDir1, "")
	test.That(t, err, test.ShouldBeNil)
	dev4Root, err := ioutil.TempDir(tempDir1, "")
	test.That(t, err, test.ShouldBeNil)
	dev5Root, err := ioutil.TempDir(tempDir1, "")
	test.That(t, err, test.ShouldBeNil)
	dev6Root, err := ioutil.TempDir(tempDir1, "")
	test.That(t, err, test.ShouldBeNil)
	dev7Root, err := ioutil.TempDir(tempDir1, "")
	test.That(t, err, test.ShouldBeNil)
	dev1, err := ioutil.TempDir(tempDir2, "")
	test.That(t, err, test.ShouldBeNil)
	_, err = ioutil.TempDir(dev2Root, "")
	test.That(t, err, test.ShouldBeNil)
	dev3, err := ioutil.TempDir(dev3Root, "")
	test.That(t, err, test.ShouldBeNil)
	dev4, err := ioutil.TempDir(dev4Root, "")
	test.That(t, err, test.ShouldBeNil)
	dev5, err := ioutil.TempDir(dev5Root, "")
	test.That(t, err, test.ShouldBeNil)
	dev6, err := ioutil.TempDir(dev6Root, "")
	test.That(t, err, test.ShouldBeNil)
	dev7, err := ioutil.TempDir(dev7Root, "")
	test.That(t, err, test.ShouldBeNil)

	test.That(t, os.WriteFile(filepath.Join(tempDir2, "uevent"), []byte("PRODUCT=10c4/ea60"), 0666), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev3Root, "uevent"), []byte("PRODUCT=10c5/ea61"), 0666), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev4Root, "uevent"), []byte("PRODUCT=10c5X/ea61"), 0666), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev5Root, "uevent"), []byte("PRODUCT=10c5/ea6X"), 0666), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev6Root, "uevent"), []byte("PRODUCT=10c4/ea60"), 0666), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev7Root, "uevent"), []byte("PRODUCT=10c4/ea60"), 0666), test.ShouldBeNil)

	test.That(t, os.Mkdir(filepath.Join(dev1, "tty"), 0700), test.ShouldBeNil)
	test.That(t, os.Mkdir(filepath.Join(dev3, "tty"), 0700), test.ShouldBeNil)
	test.That(t, os.Mkdir(filepath.Join(dev6, "tty"), 0666), test.ShouldBeNil)
	test.That(t, os.Mkdir(filepath.Join(dev7, "tty"), 0700), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev1, "tty", "one"), []byte("a"), 0666), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev3, "tty", "two"), []byte("b"), 0666), test.ShouldBeNil)

	test.That(t, os.Symlink(filepath.Join("../", filepath.Base(tempDir2), filepath.Base(dev1)), path.Join(tempDir2, filepath.Base(dev1)+"1")), test.ShouldBeNil)
	test.That(t, os.Symlink(dev3, path.Join(tempDir2, filepath.Base(dev3))), test.ShouldBeNil)
	test.That(t, os.Symlink(dev4, path.Join(tempDir2, filepath.Base(dev4))), test.ShouldBeNil)
	test.That(t, os.Symlink(dev5, path.Join(tempDir2, filepath.Base(dev5))), test.ShouldBeNil)
	test.That(t, os.Symlink(dev6, path.Join(tempDir2, filepath.Base(dev6))), test.ShouldBeNil)
	test.That(t, os.Symlink(dev7, path.Join(tempDir2, filepath.Base(dev7))), test.ShouldBeNil)

	for i, tc := range []struct {
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
			return vendorID == 4292 && productID == 60000
		}, []string{"somewhereelse", tempDir2}, []DeviceDescription{
			{ID: Identifier{Vendor: 4292, Product: 60000}, Path: "/dev/one"},
		}},
		{func(vendorID, productID int) bool {
			return false
		}, []string{tempDir2}, nil},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			SysPaths = tc.Paths
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
		})
	}
}
