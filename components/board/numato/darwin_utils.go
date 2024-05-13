//go:build darwin

package numato

import "go.viam.com/utils/usb"

func getSerialDevices() []usb.Description {
	return usb.Search(usb.NewSearchFilter("AppleUSBACMData", "usbmodem"), func(vendorID, productID int) bool {
		return true
	})
}
