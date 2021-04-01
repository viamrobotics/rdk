package search

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/usb"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

func TestDevices(t *testing.T) {
	deviceType := lidar.DeviceType("somelidar")
	lidar.RegisterDeviceType(deviceType, lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription, logger golog.Logger) (lidar.Device, error) {
			return nil, errors.New("not implemented")
		},
		USBInfo: &usb.Identifier{
			Vendor:  0x10c4,
			Product: 0xea60,
		},
	})

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
		Expected []lidar.DeviceDescription
	}{
		{nil, nil},
		{[]string{"/"}, nil},
		{[]string{tempDir2}, []lidar.DeviceDescription{
			{Type: deviceType, Path: "/dev/one"}},
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			usb.SysPaths = tc.Paths
			result := Devices()
			test.That(t, result, test.ShouldResemble, tc.Expected)
		})
	}
}
