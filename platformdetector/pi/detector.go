//go:build linux && arm64

// Package detector ensures code for Raspberry Pi platforms can not be used
// on other platforms.
package pi

import (
	_ "go.viam.com/core/board/pi"
)
