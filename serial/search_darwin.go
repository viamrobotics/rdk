package serial

import (
	"go.viam.com/robotcore/usb"
)

var SearchDevices = func(filter SearchFilter) []DeviceDescription {
	usbDevices := usb.SearchDevices(
		usb.NewSearchFilter("AppleUSBACMData", "usbmodem"),
		func(vendorID, productID int) bool {
			return checkProductDeviceIDs(vendorID, productID) != DeviceTypeUnknown
		})
	var serialDeviceDescs []DeviceDescription
	for _, dev := range usbDevices {
		devType := checkProductDeviceIDs(dev.ID.Vendor, dev.ID.Product)
		if filter.Type != "" && filter.Type != devType {
			continue
		}
		serialDeviceDescs = append(serialDeviceDescs, DeviceDescription{
			Type: devType,
			Path: dev.Path,
		})
	}
	return serialDeviceDescs
}
