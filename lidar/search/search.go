// +build !linux,!darwin

package search

import (
	"go.viam.com/robotcore/lidar"
)

func Devices() []lidar.DeviceDescription {
	return nil
}
