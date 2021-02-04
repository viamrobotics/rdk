// +build !linux,!darwin

package serial

func SearchDevices(filter SearchFilter) ([]DeviceDescription, error) {
	return nil
}
