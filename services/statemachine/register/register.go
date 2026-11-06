// Package register registers the state machine service and its implementations.
package register

import (
	// register state machine.
	_ "go.viam.com/rdk/services/statemachine"
	_ "go.viam.com/rdk/services/statemachine/builtin"
)
