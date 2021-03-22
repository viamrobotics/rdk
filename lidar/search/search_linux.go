// +build linux

package search

import (
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/usb"
)

func Devices() ([]lidar.DeviceDescription, error) {
	usbDevices := usb.SearchDevices(
		usb.SearchFilter{},
		func(vendorID, productID int) bool {
			return lidar.CheckProductDeviceIDs(vendorID, productID) != lidar.DeviceTypeUnknown
		})
	lidarDeviceDecss := make([]lidar.DeviceDescription, 0, len(usbDevices))
	for _, dev := range usbDevices {
		devType := lidar.CheckProductDeviceIDs(dev.ID.Vendor, dev.ID.Product)
		lidarDeviceDecss = append(lidarDeviceDecss, lidar.DeviceDescription{
			Type: devType,
			Path: dev.Path,
		})
	}
	return lidarDeviceDecss, nil
}
