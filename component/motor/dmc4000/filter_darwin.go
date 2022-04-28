package dmc4000

import("go.viam.com/utils/usb")

func init() {
	usbFilter = usb.NewSearchFilter("IOUserSerial","usbserial-")
}
