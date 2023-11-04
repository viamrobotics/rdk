// Package jetsoncamera contains information about the
// daughterboards and camera modules that are supported on jetson platforms.
package jetsoncamera

const (
	// OrinAGX is the name of the Jetson Orin AGX development kit.
	OrinAGX = "Jetson AGX Orin"
	// Unknown is the default value for unknown OS attributes.
	Unknown = "unknown"
	// ECAM https://www.e-consystems.com/nvidia-cameras/jetson-agx-orin-cameras/full-hd-ar0234-color-global-shutter-camera.asp
	ECAM = "e-CAM20_CUOAGX"
	// AR0234 https://www.e-consystems.com/camera-modules/ar0234-global-shutter-camera-module.asp
	AR0234 = "AR0234CS"
)

// OSInformation contains information about the OS
// that the camera is running on.
type OSInformation struct {
	Name   string // e.g. "linux"
	Arch   string // e.g. "arm64"
	Kernel string // e.g. "4.9.140-tegra"
	Device string // e.g. "NVIDIA Jetson AGX Xavier"
}

// CameraInformation contains information about the
// daughterboards and camera modules that are supported.
type CameraInformation struct {
	// map of daughterboard name to I2C bus names
	Daughterboards map[string][]string // e.g. "i2c-30", "i2c-31"
	// map of camera product name to object-file camera driver
	Modules map[string]string // e.g. "ar0234.ko"
}

var cameraInfoMappings = map[string]CameraInformation{
	OrinAGX: {
		Daughterboards: map[string][]string{
			ECAM: {"i2c-30", "i2c-31", "i2c-32", "i2c-33", "i2c-34", "i2c-35", "i2c-36", "i2c-37"},
		},
		Modules: map[string]string{
			AR0234: "ar0234.ko",
		},
	},
}
