// +build !linux,!darwin

package serial

func SearchDevices(filter SearchFilter) ([]DeviceDescription, error) {
	println("here!")
	return nil
}
