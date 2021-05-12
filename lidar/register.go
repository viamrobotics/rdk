package lidar

import (
	"sync"

	"go.viam.com/robotcore/usb"
)

var registrations = map[DeviceType]DeviceTypeRegistration{}
var registrationsMu sync.Mutex

// DeviceTypeRegistration associates a type of device to metadata about it.
type DeviceTypeRegistration struct {
	USBInfo *usb.Identifier
}

// RegisterDeviceType registers a device type and associates it with metadata.
func RegisterDeviceType(deviceType DeviceType, reg DeviceTypeRegistration) {
	registrationsMu.Lock()
	registrations[deviceType] = reg
	registrationsMu.Unlock()
}

// CheckProductDeviceIDs takes USB identification details and tries to determine
// its DeviceType from previously registered types.
func CheckProductDeviceIDs(vendorID, productID int) DeviceType {
	registrationsMu.Lock()
	defer registrationsMu.Unlock()

	for t, reg := range registrations {
		if reg.USBInfo != nil &&
			reg.USBInfo.Vendor == vendorID && reg.USBInfo.Product == productID {
			return t
		}
	}
	return DeviceTypeUnknown
}
