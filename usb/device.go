package usb

type DeviceDescription struct {
	ID   Identifier
	Path string
}

type Identifier struct {
	Vendor  int
	Product int
}
