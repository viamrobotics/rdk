// +build !linux,!darwin

package usb

import (
	"github.com/echolabsinc/robotcore/lidar"
)

func DetectDevices() []lidar.DeviceDescription {
	return nil
}
