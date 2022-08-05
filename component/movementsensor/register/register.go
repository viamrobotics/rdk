// Package register registers all relevant MovementSensors
package register

import (
	_ "go.viam.com/rdk/component/movementsensor/nmea"
	_ "go.viam.com/rdk/component/movementsensor/rtk"
)
