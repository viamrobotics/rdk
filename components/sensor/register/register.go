// Package register registers all relevant Sensors
package register

import (
	// for Sensors.
	_ "go.viam.com/rdk/components/sensor/bme280"
	_ "go.viam.com/rdk/components/sensor/ds18b20"
	_ "go.viam.com/rdk/components/sensor/fake"
	_ "go.viam.com/rdk/components/sensor/sht3xd"
	_ "go.viam.com/rdk/components/sensor/ultrasonic"
)
