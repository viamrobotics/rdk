// Package register registers all relevant grippers and also API specific functions
package register

import (
	// for grippers.
	_ "go.viam.com/rdk/components/gripper/fake"
	_ "go.viam.com/rdk/components/gripper/robotiq"
	_ "go.viam.com/rdk/components/gripper/softrobotics"
)
