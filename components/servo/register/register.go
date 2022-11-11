// Package register registers all relevant servos
package register

import (
	// for servos.
	_ "go.viam.com/rdk/components/servo/fake"
	_ "go.viam.com/rdk/components/servo/generic"
)
