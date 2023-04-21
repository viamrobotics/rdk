// Package register registers all relevant MovementSensors
package register

import (
	// Load all encoders.
	_ "go.viam.com/rdk/components/encoder/ams"
	_ "go.viam.com/rdk/components/encoder/incremental"
	_ "go.viam.com/rdk/components/encoder/single"
)
