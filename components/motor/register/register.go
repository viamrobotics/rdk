// Package register registers all relevant motors
package register

import (
	// for motors.
	_ "go.viam.com/rdk/components/motor/dimensionengineering"
	_ "go.viam.com/rdk/components/motor/dmc4000"
	_ "go.viam.com/rdk/components/motor/fake"
	_ "go.viam.com/rdk/components/motor/gpio"
	_ "go.viam.com/rdk/components/motor/gpiostepper"
	_ "go.viam.com/rdk/components/motor/i2cmotors"
	_ "go.viam.com/rdk/components/motor/roboclaw"
	_ "go.viam.com/rdk/components/motor/tmcstepper"
)
