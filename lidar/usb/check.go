package usb

import (
	"github.com/echolabsinc/robotcore/lidar"
	"github.com/echolabsinc/robotcore/lidar/rplidar"
)

func checkProductDeviceIDs(vendorID, productID int) lidar.DeviceType {
	if vendorID == 0x10c4 && productID == 0xea60 {
		return rplidar.DeviceType
	}
	return lidar.DeviceTypeUnknown
}
