//go:build !notc

package register

import (
	_ "go.viam.com/rdk/services/mlmodel/register"
	_ "go.viam.com/rdk/services/motion/register"
	_ "go.viam.com/rdk/services/navigation/register"
	_ "go.viam.com/rdk/services/vision/register"
)
