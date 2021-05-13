package lidar

import (
	"sync"

	"go.viam.com/core/usb"
)

var registrations = map[Type]TypeRegistration{}
var registrationsMu sync.Mutex

// TypeRegistration associates a type of device to metadata about it.
type TypeRegistration struct {
	USBInfo *usb.Identifier
}

// RegisterType registers a device type and associates it with metadata.
func RegisterType(deviceType Type, reg TypeRegistration) {
	registrationsMu.Lock()
	registrations[deviceType] = reg
	registrationsMu.Unlock()
}

// CheckProductDeviceIDs takes USB identification details and tries to determine
// its Type from previously registered types.
func CheckProductDeviceIDs(vendorID, productID int) Type {
	registrationsMu.Lock()
	defer registrationsMu.Unlock()

	for t, reg := range registrations {
		if reg.USBInfo != nil &&
			reg.USBInfo.Vendor == vendorID && reg.USBInfo.Product == productID {
			return t
		}
	}
	return TypeUnknown
}
