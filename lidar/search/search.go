// +build !linux,!darwin

// Package search provides the ability to search for LiDAR devices on a system.
package search

import "go.viam.com/robotcore/api"

func Devices() []api.ComponentConfig {
	return nil
}
