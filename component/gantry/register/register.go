// Package register registers all relevant gantries
package register

import (
	// for gantries.
	_ "go.viam.com/rdk/component/gantry/fake"
	_ "go.viam.com/rdk/component/gantry/multiaxis"
	_ "go.viam.com/rdk/component/gantry/oneaxis"
)
