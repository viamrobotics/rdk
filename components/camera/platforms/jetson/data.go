package jetsoncamera

const (
	// Jetson Devices
	OrinAGX = "Jetson AGX Orin"
	Unknown = "unknown"
	// Daughterboards
	ECAM = "e-CAM20_CUOAGX"
	// Cameras
	AR0234 = "AR0234CS"
)

type OSInformation struct {
	Name   string // e.g. "linux"
	Arch   string // e.g. "arm64"
	Kernel string // e.g. "4.9.140-tegra"
	Device string // e.g. "NVIDIA Jetson AGX Xavier"
}

type CameraInformation struct {
	// map of daughterboard name to I2C bus names
	Daughterboards map[string][]string // e.g. "i2c-30", "i2c-31"
	// map of camera product name to object-file camera driver
	Modules map[string]string // e.g. "ar0234.ko"
}

var cameraInfoMappings = map[string]CameraInformation{
	OrinAGX: {
		Daughterboards: map[string][]string{
			ECAM: []string{"i2c-30", "i2c-31", "i2c-32", "i2c-33", "i2c-34", "i2c-35", "i2c-36", "i2c-37"},
		},
		Modules: map[string]string{
			AR0234: "ar0234.ko",
		},
	},
}
