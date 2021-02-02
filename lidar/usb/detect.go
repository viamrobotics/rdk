// +build !linux,!darwin

package usb

import (
	"github.com/viamrobotics/robotcore/lidar"
)

func DetectDevices() []lidar.DeviceDescription {
	return nil
}
