// Package register registers all relevant grippers and also subtype specific functions
package register

import (
	// for grippers.
	_ "go.viam.com/rdk/components/gripper/fake"
	_ "go.viam.com/rdk/components/gripper/robotiq"
	_ "go.viam.com/rdk/components/gripper/softrobotics"
	_ "go.viam.com/rdk/components/gripper/vgripper/v1"
	_ "go.viam.com/rdk/components/gripper/yahboom"
)
