//go:build !no_cgo

// Package register registers all relevant bases
package register

import (
	// register bases.
	_ "go.viam.com/rdk/components/base/sensorcontrolled"
)
