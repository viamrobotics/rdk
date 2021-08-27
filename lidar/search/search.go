//go:build !linux && !darwin
// +build !linux,!darwin

// Package search provides the ability to search for LiDARs on a system.
package search

import "go.viam.com/core/config"

// Lidars returns nothing here for unsupported platforms.
func Lidars() []config.Component {
	return nil
}
