package serial

// checkProductDeviceIDs returns the DeviceType that corresponds to the given
// vendor and product IDs.
// Note(erd): This probably should be based off registration or some user
// supplied check function.
func checkProductDeviceIDs(vendorID, productID int) DeviceType {
	if vendorID == 0x2341 && productID == 0x0043 {
		return DeviceTypeArduino
	}
	if vendorID == 0x2a19 && productID == 0x0805 {
		return DeviceTypeNumatoGPIO
	}
	return DeviceTypeUnknown
}
