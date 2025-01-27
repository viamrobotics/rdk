// Package register registers all services
package register

import (
	// register services.
	_ "go.viam.com/rdk/services/baseremotecontrol/register"
	_ "go.viam.com/rdk/services/datamanager/register"
	_ "go.viam.com/rdk/services/discovery/register"
	_ "go.viam.com/rdk/services/generic/register"
	_ "go.viam.com/rdk/services/shell/register"
	_ "go.viam.com/rdk/services/slam/register"
)
