// Package register registers all services
package register

import (
	// register services.
	_ "go.viam.com/rdk/services/baseremotecontrol"
	_ "go.viam.com/rdk/services/datamanager"
	_ "go.viam.com/rdk/services/motion"
	_ "go.viam.com/rdk/services/navigation"
	_ "go.viam.com/rdk/services/sensors"
	_ "go.viam.com/rdk/services/shell"
	_ "go.viam.com/rdk/services/slam"
	_ "go.viam.com/rdk/services/vision"
)
