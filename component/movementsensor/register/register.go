// Package register registers all relevant MovementSensors
package register

import (
	// for GPSs.
	_ "go.viam.com/rdk/component/movementsensor/fake"
	_ "go.viam.com/rdk/component/movementsensor/nmea"
	_ "go.viam.com/rdk/component/movementsensor/rtk"
)
