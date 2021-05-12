// +build !linux,!darwin

package usb

type SearchFilter struct{}

// SearchDevices returns nothing here for unsupported platforms.
func SearchDevices(filter SearchFilter, includeDevice func(vendorID, productID int) bool) ([]DeviceDescription, error) {
	return nil, nil
}
