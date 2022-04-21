// Package register registers all components
package register

import (

	// register components.
	_ "go.viam.com/rdk/component/arm/register"
	_ "go.viam.com/rdk/component/base/register"
	_ "go.viam.com/rdk/component/board/register"
	_ "go.viam.com/rdk/component/camera/register"
	_ "go.viam.com/rdk/component/gantry/register"
	_ "go.viam.com/rdk/component/gps/register"
	_ "go.viam.com/rdk/component/gripper/register"
	_ "go.viam.com/rdk/component/imu/register"
	_ "go.viam.com/rdk/component/input/register"
	_ "go.viam.com/rdk/component/motor/register"

	// register subtypes without implementations directly.
	_ "go.viam.com/rdk/component/posetracker"
	_ "go.viam.com/rdk/component/sensor/register"
	_ "go.viam.com/rdk/component/servo/register"
)
