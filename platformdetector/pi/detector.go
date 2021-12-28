//go:build linux && arm64

// Package pi ensures code for Raspberry Pi platforms can not be used
// on other platforms.
package pi

// TODO(maximpertsov): add to board component?

import (
	_ "go.viam.com/rdk/component/board/pi"
)
