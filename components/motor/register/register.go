// Package register registers all relevant motors
package register

import (
	// for motors.
	_ "go.viam.com/rdk/components/motor/fake"
	_ "go.viam.com/rdk/components/motor/gpio"
	_ "go.viam.com/rdk/components/motor/gpiostepper"
)
