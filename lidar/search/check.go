package search

import (
	"github.com/viamrobotics/robotcore/lidar"
	"github.com/viamrobotics/robotcore/lidar/rplidar"
)

func checkProductDeviceIDs(vendorID, productID int) lidar.DeviceType {
	if vendorID == 0x10c4 && productID == 0xea60 {
		return rplidar.DeviceType
	}
	return lidar.DeviceTypeUnknown
}
