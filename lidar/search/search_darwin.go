// +build darwin

package search

import (
	"go.viam.com/robotcore/config"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/usb"
)

// Devices uses macOS io device APIs to find all applicable lidar devices.
func Devices() []config.Component {
	usbDevices := usb.Search(
		usb.NewSearchFilter("IOUserSerial", "usbserial-"),
		func(vendorID, productID int) bool {
			return lidar.CheckProductDeviceIDs(vendorID, productID) != lidar.TypeUnknown
		})
	var lidarComponents []config.Component
	for _, dev := range usbDevices {
		devType := lidar.CheckProductDeviceIDs(dev.ID.Vendor, dev.ID.Product)
		lidarComponents = append(lidarComponents, config.Component{
			Type:  config.ComponentTypeLidar,
			Host:  dev.Path,
			Model: string(devType),
		})
	}
	return lidarComponents
}
