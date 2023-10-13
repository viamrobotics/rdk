//go:build !no_media && !no_cgo

package register

import (
	// blank import registration pattern.
	_ "go.viam.com/rdk/components/audioinput/register"
	_ "go.viam.com/rdk/components/camera/register"
)
