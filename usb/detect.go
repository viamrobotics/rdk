// +build !linux,!darwin

package usb

type SearchFilter struct{}

func SearchDevices(filter SearchFilter, includeDevice func(vendorID, productID int) bool) ([]DeviceDescription, error) {
	return nil, nil
}
