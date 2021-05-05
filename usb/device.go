// Package usb provides utilities for searching for and working with usb based devices.
package usb

type DeviceDescription struct {
	ID   Identifier
	Path string
}

type Identifier struct {
	Vendor  int
	Product int
}
