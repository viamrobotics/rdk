// Package registerapis is a convenience package to register the APIs for all
// built-in components.
package registerapis

import (
	// Register components.
	_ "go.viam.com/rdk/components/arm"
	_ "go.viam.com/rdk/components/audioin"
	_ "go.viam.com/rdk/components/audioout"
	_ "go.viam.com/rdk/components/base"
	_ "go.viam.com/rdk/components/board"
	_ "go.viam.com/rdk/components/button"
	_ "go.viam.com/rdk/components/encoder"
	_ "go.viam.com/rdk/components/gantry"
	_ "go.viam.com/rdk/components/generic"
	_ "go.viam.com/rdk/components/gripper"
	_ "go.viam.com/rdk/components/input"
	_ "go.viam.com/rdk/components/motor"
	_ "go.viam.com/rdk/components/movementsensor"
	_ "go.viam.com/rdk/components/posetracker"
	_ "go.viam.com/rdk/components/powersensor"
	_ "go.viam.com/rdk/components/sensor"
	_ "go.viam.com/rdk/components/servo"
	_ "go.viam.com/rdk/components/switch"
)
