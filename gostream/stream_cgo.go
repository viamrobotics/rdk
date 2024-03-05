//go:build !no_cgo

package gostream

import (
	// import microphone.
	_ "github.com/pion/mediadevices/pkg/driver/microphone"
)
