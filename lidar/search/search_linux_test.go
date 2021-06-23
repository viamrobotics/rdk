package search

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"go.viam.com/utils/testutils"

	"go.viam.com/core/config"
	"go.viam.com/core/lidar"
	"go.viam.com/core/usb"

	"go.viam.com/test"
)

func TestDevices(t *testing.T) {
	deviceType := lidar.Type("somelidar")
	lidar.RegisterType(deviceType, lidar.TypeRegistration{
		USBInfo: &usb.Identifier{
			Vendor:  0x10c4,
			Product: 0xea60,
		},
	})

	tempDir1 := testutils.TempDirT(t, "", "")
	defer os.RemoveAll(tempDir1) // clean up
	tempDir2 := testutils.TempDirT(t, "", "")
	defer os.RemoveAll(tempDir2) // clean up

	prevSysPaths := usb.SysPaths
	defer func() {
		usb.SysPaths = prevSysPaths
	}()

	dev1Root := testutils.TempDirT(t, tempDir1, "")
	dev2Root := testutils.TempDirT(t, tempDir1, "")
	dev3Root := testutils.TempDirT(t, tempDir1, "")
	dev1 := testutils.TempDirT(t, dev1Root, "")
	dev2 := testutils.TempDirT(t, dev2Root, "")
	dev3 := testutils.TempDirT(t, dev3Root, "")

	test.That(t, os.WriteFile(filepath.Join(dev1Root, "uevent"), []byte("PRODUCT=10c4/ea60"), 0666), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev3Root, "uevent"), []byte("PRODUCT=10c5/ea61"), 0666), test.ShouldBeNil)

	test.That(t, os.Mkdir(filepath.Join(dev1, "tty"), 0700), test.ShouldBeNil)
	test.That(t, os.Mkdir(filepath.Join(dev3, "tty"), 0700), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev1, "tty", "one"), []byte("a"), 0666), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev3, "tty", "two"), []byte("b"), 0666), test.ShouldBeNil)

	test.That(t, os.Symlink(dev1, path.Join(tempDir2, filepath.Base(dev1))), test.ShouldBeNil)
	test.That(t, os.Symlink(dev3, path.Join(tempDir2, filepath.Base(dev2))), test.ShouldBeNil)

	for i, tc := range []struct {
		Paths    []string
		Expected []config.Component
	}{
		{nil, nil},
		{[]string{"/"}, nil},
		{[]string{tempDir2}, []config.Component{
			{
				Type:  config.ComponentTypeLidar,
				Host:  "/dev/one",
				Model: string(deviceType),
			},
		}},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			usb.SysPaths = tc.Paths
			result := Devices()
			test.That(t, result, test.ShouldResemble, tc.Expected)
		})
	}
}
