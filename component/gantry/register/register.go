// Package register registers all relevant gantries
package register

import (

	// for gantries.
	_ "go.viam.com/rdk/component/gantry/fake"
	_ "go.viam.com/rdk/component/gantry/multiAxis"
	_ "go.viam.com/rdk/component/gantry/oneAxis"
	_ "go.viam.com/rdk/component/gantry/simple"
)
