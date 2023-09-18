//go:build !notc

package register

import (
	// blank import registration pattern
	_ "go.viam.com/rdk/services/motion/register"
	_ "go.viam.com/rdk/services/navigation/register"
	_ "go.viam.com/rdk/services/vision/register"
)
