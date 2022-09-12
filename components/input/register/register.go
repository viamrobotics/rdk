// Package register registers all relevant inputs
package register

import (
	// for inputs.
	_ "go.viam.com/rdk/components/input/fake"
	_ "go.viam.com/rdk/components/input/gamepad"
	_ "go.viam.com/rdk/components/input/gpio"
	_ "go.viam.com/rdk/components/input/mux"
	_ "go.viam.com/rdk/components/input/webgamepad"
)
