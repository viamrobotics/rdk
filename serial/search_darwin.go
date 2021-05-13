package serial

import (
	"go.viam.com/core/usb"
)

// Search uses macOS io device APIs to find all applicable serial devices.
// It's a variable in case you need to override it during tests.
var Search = func(filter SearchFilter) []Description {
	usbDevices := usb.Search(
		usb.NewSearchFilter("AppleUSBACMData", "usbmodem"),
		func(vendorID, productID int) bool {
			return checkProductDeviceIDs(vendorID, productID) != TypeUnknown
		})
	var serialDeviceDescs []Description
	for _, dev := range usbDevices {
		devType := checkProductDeviceIDs(dev.ID.Vendor, dev.ID.Product)
		if filter.Type != "" && filter.Type != devType {
			continue
		}
		serialDeviceDescs = append(serialDeviceDescs, Description{
			Type: devType,
			Path: dev.Path,
		})
	}
	return serialDeviceDescs
}
