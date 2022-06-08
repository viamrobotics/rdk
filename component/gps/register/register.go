// Package register registers all relevant GPSs
package register

import (
	// for GPSs.
	_ "go.viam.com/rdk/component/gps/fake"
	_ "go.viam.com/rdk/component/gps/merge"
	_ "go.viam.com/rdk/component/gps/nmea"
)
