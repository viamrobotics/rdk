// Package usb provides utilities for searching for and working with usb based devices.
package usb

// Description describes a specific USB device.
type Description struct {
	ID   Identifier
	Path string
}

// Identifier identifies a specific USB device by the vendor
// who produced it and the product that it is. These should
// be unique across products.
type Identifier struct {
	Vendor  int
	Product int
}
