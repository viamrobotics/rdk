//go:build !linux

package genericlinux

import (
	"github.com/pkg/errors"
	"go.viam.com/rdk/components/board"
)

func gpioInitialize(gpioMappings map[int]GPIOBoardMapping) {
	// Don't even log anything here: if someone is running in a non-Linux environment, things
	// should work fine as long as they don't try using ioctl, and the log would be an unnecessary
	// warning.
}

func gpioGetPin(pinName string) (board.GPIOPin, error) {
	return nil, errors.Errorf("ioctl GPIO pins are not supported in a non-Linux environment")
}
