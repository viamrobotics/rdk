package jetsoncamera

const (
	JetsonOrinAGX = "Jetson Orin AGX"
)

type OSInformation struct {
	Name   string
	Arch   string
	Kernel string
	Device string
}

type CameraDefinition struct {
	Module string
	I2C    []string
}

var cameraInfoMappings = map[string]CameraDefinition{
	"AR0234": {
		Module: "ar0234.ko",
		I2C:    []string{"i2c-30", "i2c-31", "i2c-32", "i2c-33", "i2c-34", "i2c-35", "i2c-36", "i2c-37"},
	},
}
