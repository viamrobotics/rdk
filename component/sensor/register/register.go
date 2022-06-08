// Package register registers all relevant Sensors
package register

import (
	// for Sensors.
	_ "go.viam.com/rdk/component/sensor/ds18b20"
	_ "go.viam.com/rdk/component/sensor/fake"
	_ "go.viam.com/rdk/component/sensor/ultrasonic"
)
