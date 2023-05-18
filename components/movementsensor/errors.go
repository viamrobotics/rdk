package movementsensor

import (
	"fmt"

	"github.com/pkg/errors"
)

// AddressReadError returns a standard error for when we can not from an I2C bus.
func AddressReadError(err error, address byte, bus, board string) error {
	msg := fmt.Sprintf("can't read from I2C address %d on bus %s of board %s",
		address, bus, board)
	return errors.Wrap(err, msg)
}

// UnexpectedDeviceError returns a standard error we can not find the expected device
// at the given address.
func UnexpectedDeviceError(address, defaultAddress byte, deviceName string) error {
	return errors.Errorf("unexpected non-%s device at address %d: response '%d'",
		deviceName, address, defaultAddress)
}
