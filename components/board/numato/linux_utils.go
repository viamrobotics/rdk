//go:build linux

package numato

import "go.viam.com/utils/usb"

// getSerialDevices returns all devices connected by USB on a linux machine.
// This is used to get the productID of the numato board being used.
func getSerialDevices() []usb.Description {
	return usb.Search(usb.SearchFilter{}, func(vendorID, productID int) bool {
		return true
	})
}
