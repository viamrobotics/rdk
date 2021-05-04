package serial

func checkProductDeviceIDs(vendorID, productID int) DeviceType {
	if vendorID == 0x2341 && productID == 0x0043 {
		return DeviceTypeArduino
	}
	if vendorID == 0x2a19 && productID == 0x0805 {
		return DeviceTypeNumatoGPIO
	}
	return DeviceTypeUnknown
}
