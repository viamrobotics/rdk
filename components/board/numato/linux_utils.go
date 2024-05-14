//go:build linux

package numato

import "go.viam.com/utils/usb"

// getSerialDevices returns all devices connected by USB on a linux machine.
func getSerialDevices() []usb.Description {
	return usb.Search(usb.SearchFilter{}, func(vendorID, productID int) bool {
		return true
	})
}
