// Package register registers all relevant MovementSensors
package register

import (
	// Load all movementsensors.
	_ "go.viam.com/rdk/components/movementsensor/fake"
	_ "go.viam.com/rdk/components/movementsensor/gpsnmea"
	_ "go.viam.com/rdk/components/movementsensor/gpsrtk"
	_ "go.viam.com/rdk/components/movementsensor/imuwit"
)
