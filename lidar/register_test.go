package lidar_test

import (
	"testing"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/usb"

	"github.com/edaniels/test"
)

func TestRegistration(t *testing.T) {
	devType1 := lidar.DeviceType("2")

	devType := lidar.CheckProductDeviceIDs(1, 2)
	test.That(t, devType, test.ShouldEqual, lidar.DeviceTypeUnknown)
	lidar.RegisterDeviceType(devType1, lidar.DeviceTypeRegistration{
		USBInfo: &usb.Identifier{
			Vendor:  1,
			Product: 2,
		},
	})
	devType2 := lidar.DeviceType("3")
	lidar.RegisterDeviceType(devType2, lidar.DeviceTypeRegistration{
		USBInfo: &usb.Identifier{
			Vendor:  3,
			Product: 4,
		},
	})

	devType = lidar.CheckProductDeviceIDs(0, 1)
	test.That(t, devType, test.ShouldEqual, lidar.DeviceTypeUnknown)
	devType = lidar.CheckProductDeviceIDs(1, 2)
	test.That(t, devType, test.ShouldEqual, devType1)
	devType = lidar.CheckProductDeviceIDs(3, 4)
	test.That(t, devType, test.ShouldEqual, devType2)
}
