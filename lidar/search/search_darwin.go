// +build darwin

package search

import (
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/usb"
)

func Devices() []api.ComponentConfig {
	usbDevices := usb.SearchDevices(
		usb.NewSearchFilter("IOUserSerial", "usbserial-"),
		func(vendorID, productID int) bool {
			return lidar.CheckProductDeviceIDs(vendorID, productID) != lidar.DeviceTypeUnknown
		})
	var lidarComponents []api.ComponentConfig
	for _, dev := range usbDevices {
		devType := lidar.CheckProductDeviceIDs(dev.ID.Vendor, dev.ID.Product)
		lidarComponents = append(lidarComponents, api.ComponentConfig{
			Type:  api.ComponentTypeLidar,
			Host:  dev.Path,
			Model: string(devType),
		})
	}
	return lidarComponents
}
