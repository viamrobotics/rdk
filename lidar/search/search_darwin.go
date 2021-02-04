// +build darwin

package search

import (
	"github.com/viamrobotics/robotcore/lidar"
	"github.com/viamrobotics/robotcore/usb"
)

func Devices() ([]lidar.DeviceDescription, error) {
	usbDevices, err := usb.SearchDevices(
		usb.NewSearchFilter("IOUserSerial", "usbserial-"),
		func(vendorID, productID int) bool {
			return checkProductDeviceIDs(vendorID, productID) != lidar.DeviceTypeUnknown
		})
	if err != nil {
		return nil, err
	}
	lidarDeviceDecss := make([]lidar.DeviceDescription, 0, len(usbDevices))
	for _, dev := range usbDevices {
		devType := checkProductDeviceIDs(dev.VendorID, dev.ProductID)
		lidarDeviceDecss = append(lidarDeviceDecss, lidar.DeviceDescription{
			Type: devType,
			Path: dev.Path,
		})
	}
	return lidarDeviceDecss, nil
}
