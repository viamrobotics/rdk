//go:build !no_tflite && !no_cgo

// Package register registers all services
package register

import (
	// register services.
	_ "go.viam.com/rdk/services/mlmodel/register"
)
