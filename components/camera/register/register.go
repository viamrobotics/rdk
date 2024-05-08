//go:build !no_cgo || android

// Package register registers all relevant cameras and also API specific functions
package register

import (
	// for cameras.
	_ "go.viam.com/rdk/components/camera/fake"
	_ "go.viam.com/rdk/components/camera/transformpipeline"
)
