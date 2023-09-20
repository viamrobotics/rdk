//go:build !no_cgo

package register

import (
	// blank import registration pattern.
	_ "go.viam.com/rdk/components/arm/register"
	_ "go.viam.com/rdk/components/audioinput/register"
	_ "go.viam.com/rdk/components/base/register"
	_ "go.viam.com/rdk/components/camera/register"
	_ "go.viam.com/rdk/components/gripper/register"
)
