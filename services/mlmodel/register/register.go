//go:build !no_tflite && !no_cgo

// Package register registers all relevant ML model services
package register

import (
	// for ML model service  models.
	_ "go.viam.com/rdk/services/mlmodel/tflitecpu"
)
