//go:build !no_cgo

package register

import (
	// blank import registration pattern.
	_ "go.viam.com/rdk/services/motion/register"
	_ "go.viam.com/rdk/services/navigation/register"
)
