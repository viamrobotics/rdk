// +build !linux,!darwin

package search

import (
	"go.viam.com/robotcore/lidar"
)

func Devices() ([]lidar.DeviceDescription, error) {
	return nil, nil
}
