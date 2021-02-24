// +build linux

package search

import (
	"github.com/viamrobotics/robotcore/lidar"
	"github.com/viamrobotics/robotcore/usb"
)

func Devices() ([]lidar.DeviceDescription, error) {
	usbDevices, err := usb.SearchDevices(
		usb.SearchFilter{},
		func(vendorID, productID int) bool {
			return lidar.CheckProductDeviceIDs(vendorID, productID) != lidar.DeviceTypeUnknown
		})
	if err != nil {
		return nil, err
	}
	lidarDeviceDecss := make([]lidar.DeviceDescription, 0, len(usbDevices))
	for _, dev := range usbDevices {
		lidar.CheckProductDeviceIDs(dev.ID.Vendor, dev.ID.Product)
		lidarDeviceDecss = append(lidarDeviceDecss, lidar.DeviceDescription{
			Type: devType,
			Path: dev.Path,
		})
	}
	return lidarDeviceDecss, nil
}
