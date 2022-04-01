// Package register registers all relevant inputs
package register

import (

	// for inputs.
	_ "go.viam.com/rdk/component/input/fake"
	_ "go.viam.com/rdk/component/input/gamepad"
	_ "go.viam.com/rdk/component/input/gpio"
	_ "go.viam.com/rdk/component/input/mux"
	_ "go.viam.com/rdk/component/input/webgamepad"
)
