//go:build pi
// +build pi

// Package detector ensures code for Raspberry Pi platforms can not be used
// on other platforms.
package detector

import (
	_ "go.viam.com/core/board/pi"
)
