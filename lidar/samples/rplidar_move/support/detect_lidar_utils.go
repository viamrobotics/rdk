package support

type LidarType int

const (
	LidarTypeUnknown = LidarType(iota)
	LidarTypeRPLidar
)

func (lt LidarType) String() string {
	switch lt {
	case LidarTypeRPLidar:
		return "RPLidar"
	default:
		return "unknown"
	}
}

type LidarDeviceDescription struct {
	Type LidarType
	Path string
}

func checkProductLidarDevice(vendorID, productID int) LidarType {
	if vendorID == 0x10c4 && productID == 0xea60 {
		return LidarTypeRPLidar
	}
	return LidarTypeUnknown
}
