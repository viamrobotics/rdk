// Package register registers all relevant motors
package register

import (
	// register all powersensors.
	_ "go.viam.com/rdk/components/powersensor/fake"
	_ "go.viam.com/rdk/components/powersensor/ina"
	_ "go.viam.com/rdk/components/powersensor/renogy"
)
