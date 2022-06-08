// Package register registers all relevant grippers and also subtype specific functions
package register

import (
	// for grippers.
	_ "go.viam.com/rdk/component/gripper/fake"
	_ "go.viam.com/rdk/component/gripper/robotiq"
	_ "go.viam.com/rdk/component/gripper/softrobotics"
	_ "go.viam.com/rdk/component/gripper/vgripper/v1"
	_ "go.viam.com/rdk/component/gripper/vx300s"
	_ "go.viam.com/rdk/component/gripper/wx250s"
	_ "go.viam.com/rdk/component/gripper/yahboom"
)
