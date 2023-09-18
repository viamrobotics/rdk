//go:build !no_tflite && !notc

// Package register registers all relevant ML model services
package register

import (
	// for ML model service  models.
	_ "go.viam.com/rdk/services/mlmodel/tflitecpu"
)
