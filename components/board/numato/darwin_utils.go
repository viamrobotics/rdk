//go:build darwin

package numato

import "go.viam.com/utils/usb"

// getSerialDevices returns all devices connected by USB on a mac.
func getSerialDevices() []usb.Description {
	return usb.Search(usb.NewSearchFilter("AppleUSBACMData", "usbmodem"), func(vendorID, productID int) bool {
		return true
	})
}
