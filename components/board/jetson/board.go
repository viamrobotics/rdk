// Package jetson implements a jetson-based board.
package jetson

import (
	"bytes"
	"os"
	"runtime"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"periph.io/x/host/v3"

	"go.viam.com/rdk/components/board/genericlinux"
)

const modelName = "jetson"

func init() {
	if _, err := host.Init(); err != nil {
		golog.Global().Debugw("error initializing host", "error", err)
	}

	gpioMappings, err := genericlinux.GetGPIOBoardMappings(modelName, boardInfoMappings)
	var noBoardErr genericlinux.NoBoardFoundError
	if errors.As(err, &noBoardErr) {
		golog.Global().Debugw("error getting jetson GPIO board mapping", "error", err)
	}

	genericlinux.RegisterBoard(modelName, gpioMappings, false)
}

// isJetsonOrinAGX returns true if the device is a Jetson Orin AGX Developer Kit.
func IsJetsonOrinAGX() bool {
	const devicePath = "/sys/firmware/devicetree/base/model"
	if runtime.GOOS == "linux" && runtime.GOARCH == "arm64" {
		file, err := os.ReadFile(devicePath)
		if err != nil {
			return false
		}
		if bytes.Contains(file, []byte("AGX Orin")) {
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}

// IsECamCUOAGXConnected returns true if the e-CAM20_CUOAGX duaghter-baoard is connected.
func IsCAM20CUOAGXConnected() bool {
	const i2cPath = "/dev/i2c-30"
	if _, err := os.Stat(i2cPath); os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}

// IsAR0234DriverInstalled returns true if the ar0234.ko driver is installed.
func IsAR0234DriverInstalled() bool {
	const driverPath = "/lib/modules/5.10.104/extra/ar0234.ko"
	if _, err := os.Stat(driverPath); os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}

// PrintECamCUOAGXError returns a string with the error message for the e-CAM20_CUOAGX.
func PrintCAM20CUOAGXError() string {
	if IsAR0234DriverInstalled() {
		if IsCAM20CUOAGXConnected() {
			return "e-CAM20_CUOAGX daughter board is connected and the AR0234 driver is installed but the video capture interface requested is not avialable. Please ensure camera is connected, driver is working correctly, and the video interface is available"
		} else {
			return "e-CAM20_CUOAGX daughter board is not connected or not powerd on. Please check daughterboard conenction to the Orin AGX over the J509 connector."
		}
	} else {
		return "The E-Con Systems AR0234 driver is not installed. Please follow instructions for driver installation and verify with 'dmesg | grep ar0234'."
	}
}
