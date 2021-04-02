package lidar

import (
	"sync"

	"go.viam.com/robotcore/usb"
)

var registrations = map[DeviceType]DeviceTypeRegistration{}
var registrationsMu sync.Mutex

type DeviceTypeRegistration struct {
	USBInfo *usb.Identifier
}

func RegisterDeviceType(deviceType DeviceType, reg DeviceTypeRegistration) {
	registrationsMu.Lock()
	registrations[deviceType] = reg
	registrationsMu.Unlock()
}

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
