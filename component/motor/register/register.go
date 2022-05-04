// Package register registers all relevant motors
package register

import (

	// for motors.
	_ "go.viam.com/rdk/component/motor/dmc4000"
	_ "go.viam.com/rdk/component/motor/ezopmp"
	_ "go.viam.com/rdk/component/motor/fake"
	_ "go.viam.com/rdk/component/motor/gpio"
	_ "go.viam.com/rdk/component/motor/gpiostepper"
	_ "go.viam.com/rdk/component/motor/roboclaw"
	_ "go.viam.com/rdk/component/motor/tmcstepper"
)
