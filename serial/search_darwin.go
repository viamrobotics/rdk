// +build darwin

package serial

import (
	"go.viam.com/robotcore/usb"
)

var SearchDevices = func(filter SearchFilter) ([]DeviceDescription, error) {
	usbDevices, err := usb.SearchDevices(
		usb.NewSearchFilter("AppleUSBACMData", "usbmodem"),
		func(vendorID, productID int) bool {
			return checkProductDeviceIDs(vendorID, productID) != DeviceTypeUnknown
		})
	if err != nil {
		return nil, err
	}
	serialDeviceDecss := make([]DeviceDescription, 0, len(usbDevices))
	for _, dev := range usbDevices {
		devType := checkProductDeviceIDs(dev.ID.Vendor, dev.ID.Product)
		if filter.Type != "" && filter.Type != devType {
			continue
		}
		serialDeviceDecss = append(serialDeviceDecss, DeviceDescription{
			Type: devType,
			Path: dev.Path,
		})
	}
	return serialDeviceDecss, nil
}
