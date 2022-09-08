// Package register registers all services
package register

import (
	// register services.
	_ "go.viam.com/rdk/services/armremotecontrol"
	_ "go.viam.com/rdk/services/baseremotecontrol"
	_ "go.viam.com/rdk/services/datamanager"
	_ "go.viam.com/rdk/services/motion/register"
	_ "go.viam.com/rdk/services/navigation/register"
	_ "go.viam.com/rdk/services/sensors/register"
	_ "go.viam.com/rdk/services/shell/register"
	_ "go.viam.com/rdk/services/slam/register"
	_ "go.viam.com/rdk/services/vision/register"
)
