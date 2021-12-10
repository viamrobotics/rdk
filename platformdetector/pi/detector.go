//go:build pi
// +build pi

// Package detector ensures code for Raspberry Pi platforms can not be used
// on other platforms.
package pi

import (
	_ "go.viam.com/core/component/board/pi"
)
