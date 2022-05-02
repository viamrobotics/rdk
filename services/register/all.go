// Package register registers all services
package register

import (

	// register services.
	_ "go.viam.com/rdk/services/baseremotecontrol"
	_ "go.viam.com/rdk/services/datamanager"
	_ "go.viam.com/rdk/services/framesystem"
	_ "go.viam.com/rdk/services/metadata"
	_ "go.viam.com/rdk/services/motion"
	_ "go.viam.com/rdk/services/navigation"
	_ "go.viam.com/rdk/services/sensors"
	_ "go.viam.com/rdk/services/vision"
	_ "go.viam.com/rdk/services/web"
)
