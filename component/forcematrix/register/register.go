// Package register registers all relevant ForceMatrix sensors
package register

import (

	// for ForceMatrix sensors.
	_ "go.viam.com/rdk/component/forcematrix/fake"
	_ "go.viam.com/rdk/component/forcematrix/vforcematrixtraditional"
	_ "go.viam.com/rdk/component/forcematrix/vforcematrixwithmux"
)
