// Package register registers all relevant inputs and also subtype specific functions
package register

import (
	// for all inputs
	_ "go.viam.com/core/component/input/fake"
	_ "go.viam.com/core/component/input/gamepad"
	_ "go.viam.com/core/component/input/mux"
	_ "go.viam.com/core/component/input/webgamepad"
)
