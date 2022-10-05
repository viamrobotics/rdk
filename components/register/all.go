// Package register registers all components
package register

import (
	// register components.
	_ "go.viam.com/rdk/components/arm/register"
	_ "go.viam.com/rdk/components/audioinput/register"
	_ "go.viam.com/rdk/components/base/register"
	_ "go.viam.com/rdk/components/board/register"
	_ "go.viam.com/rdk/components/camera/register"
	_ "go.viam.com/rdk/components/gantry/register"
	_ "go.viam.com/rdk/components/generic/register"
	_ "go.viam.com/rdk/components/gripper/register"
	_ "go.viam.com/rdk/components/input/register"
	_ "go.viam.com/rdk/components/motor/register"
	_ "go.viam.com/rdk/components/movementsensor/register"
	// register subtypes without implementations directly.
	_ "go.viam.com/rdk/components/posetracker"
	_ "go.viam.com/rdk/components/sensor/register"
	_ "go.viam.com/rdk/components/servo/register"
)
