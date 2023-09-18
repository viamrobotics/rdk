//go:build linux && (arm64 || arm) && !no_pigpio

package pi

import (
	// for easily importing implementation.
	_ "go.viam.com/rdk/components/board/pi/impl"
)
