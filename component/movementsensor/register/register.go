// Package register registers all relevant MovementSensors
package register

import (
	// Load all movementsensors.
	_ "go.viam.com/rdk/component/movementsensor/fake"
	_ "go.viam.com/rdk/component/movementsensor/imuwit"
	_ "go.viam.com/rdk/component/movementsensor/nmea"
	_ "go.viam.com/rdk/component/movementsensor/rtk"
)
