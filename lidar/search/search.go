// +build !linux,!darwin

// Package search provides the ability to search for LiDARs on a system.
package search

import "go.viam.com/robotcore/api"

// Devices returns nothing here for unsupported platforms.
func Devices() []api.ComponentConfig {
	return nil
}
