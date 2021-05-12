// +build !linux,!darwin

package serial

// SearchDevices returns nothing here for unsupported platforms.
func SearchDevices(filter SearchFilter) []DeviceDescription {
	return nil
}
