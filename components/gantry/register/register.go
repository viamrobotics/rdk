// Package register registers all relevant gantries
package register

import (
	// for gantries.
	_ "go.viam.com/rdk/components/gantry/fake"
	_ "go.viam.com/rdk/components/gantry/multiaxis"
	_ "go.viam.com/rdk/components/gantry/oneaxis"
)
