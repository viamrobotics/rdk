//go:build !no_cgo

package register

import (
	// blank import registration pattern.
	_ "go.viam.com/rdk/components/arm/register"
	_ "go.viam.com/rdk/components/audioinput/register"
)
