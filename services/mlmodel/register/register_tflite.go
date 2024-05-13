//go:build !no_tflite && (!no_cgo || android)

// Package register registers all relevant ML model services
package register

import (
	// register tflitecpu.
	_ "go.viam.com/rdk/services/mlmodel/tflitecpu"
)
