// +build !linux,!darwin

package search

import (
	"github.com/viamrobotics/robotcore/lidar"
)

func Devices() ([]lidar.DeviceDescription, error) {
	return nil, nil
}
