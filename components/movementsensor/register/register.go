// Package register registers all relevant MovementSensors
package register

import (
	// Load all movementsensors.
	_ "go.viam.com/rdk/components/movementsensor/fake"
	_ "go.viam.com/rdk/components/movementsensor/imuwit"
	_ "go.viam.com/rdk/components/movementsensor/nmea"
	_ "go.viam.com/rdk/components/movementsensor/rtk"
)
