// Package register registers all relevant bases
package register

import (
	// register bases.
	_ "go.viam.com/rdk/components/base/agilex"
	_ "go.viam.com/rdk/components/base/fake"
	_ "go.viam.com/rdk/components/base/sensorbase"
	_ "go.viam.com/rdk/components/base/sensorcontrolled"
	_ "go.viam.com/rdk/components/base/wheeled"
)
