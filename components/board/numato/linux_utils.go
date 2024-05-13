//go:build linux

package numato

import "go.viam.com/utils/usb"

func getSerialDevices() []usb.Description {
	return usb.Search(usb.SearchFilter{}, func(vendorID, productID int) bool {
		return true
	})
}
