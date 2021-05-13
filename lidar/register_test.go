package lidar_test

import (
	"testing"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/usb"

	"go.viam.com/test"
)

func TestRegistration(t *testing.T) {
	devType1 := lidar.Type("2")

	devType := lidar.CheckProductDeviceIDs(1, 2)
	test.That(t, devType, test.ShouldEqual, lidar.TypeUnknown)
	lidar.RegisterType(devType1, lidar.TypeRegistration{
		USBInfo: &usb.Identifier{
			Vendor:  1,
			Product: 2,
		},
	})
	devType2 := lidar.Type("3")
	lidar.RegisterType(devType2, lidar.TypeRegistration{
		USBInfo: &usb.Identifier{
			Vendor:  3,
			Product: 4,
		},
	})

	devType = lidar.CheckProductDeviceIDs(0, 1)
	test.That(t, devType, test.ShouldEqual, lidar.TypeUnknown)
	devType = lidar.CheckProductDeviceIDs(1, 2)
	test.That(t, devType, test.ShouldEqual, devType1)
	devType = lidar.CheckProductDeviceIDs(3, 4)
	test.That(t, devType, test.ShouldEqual, devType2)
}
