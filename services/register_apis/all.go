// Package registerapis is a convenience package to register the APIs for all
// built-in services.
package registerapis

import (
	// Register services.
	_ "go.viam.com/rdk/services/baseremotecontrol"
	_ "go.viam.com/rdk/services/datamanager"
	_ "go.viam.com/rdk/services/discovery"
	_ "go.viam.com/rdk/services/generic"
	_ "go.viam.com/rdk/services/mlmodel"
	_ "go.viam.com/rdk/services/navigation"
	_ "go.viam.com/rdk/services/shell"
	_ "go.viam.com/rdk/services/slam"
	_ "go.viam.com/rdk/services/video"
	_ "go.viam.com/rdk/services/worldstatestore"
)
