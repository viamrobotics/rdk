// Package register registers all components
//
package register

import (

	// register arm.
	_ "go.viam.com/rdk/component/arm/register"

	// register base.
	_ "go.viam.com/rdk/component/base/register"

	// register board.
	_ "go.viam.com/rdk/component/board/register"

	// register camera.
	_ "go.viam.com/rdk/component/camera/register"

	// register force matrix.
	_ "go.viam.com/rdk/component/forcematrix/register"

	// register gantry.
	_ "go.viam.com/rdk/component/gantry/register"

	// register gps.
	_ "go.viam.com/rdk/component/gps/register"

	// register gripper.
	_ "go.viam.com/rdk/component/gripper/register"

	// register imu.
	_ "go.viam.com/rdk/component/imu/register"

	// register input.
	_ "go.viam.com/rdk/component/input/register"

	// register motor.
	_ "go.viam.com/rdk/component/motor/register"

	// register sensor.
	_ "go.viam.com/rdk/component/sensor/register"

	// register servo.
	_ "go.viam.com/rdk/component/servo/register"
)
