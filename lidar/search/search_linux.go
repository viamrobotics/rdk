// +build linux

package search

import (
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/usb"
)

func Devices() []lidar.DeviceDescription {
	usbDevices := usb.SearchDevices(
		usb.SearchFilter{},
		func(vendorID, productID int) bool {
			return lidar.CheckProductDeviceIDs(vendorID, productID) != lidar.DeviceTypeUnknown
		})
	var lidarDeviceDescs []lidar.DeviceDescription
	for _, dev := range usbDevices {
		devType := lidar.CheckProductDeviceIDs(dev.ID.Vendor, dev.ID.Product)
		lidarDeviceDescs = append(lidarDeviceDescs, lidar.DeviceDescription{
			Type: devType,
			Path: dev.Path,
		})
	}
	return lidarDeviceDescs
}
