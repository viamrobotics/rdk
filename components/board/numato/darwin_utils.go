//go:build darwin

package numato

import "go.viam.com/utils/usb"

// getSerialDevices returns all device descriptions connected by USB on mac. This is used to get the
// productID of the numato board being used.
func getSerialDevices() []usb.Description {
	return usb.Search(usb.NewSearchFilter("AppleUSBACMData", "usbmodem"), func(vendorID, productID int) bool {
		return true
	})
}
