package movementsensor

import (
	"fmt"

	"errors"
)

// AddressReadError returns a standard error for when we cannot read from an I2C bus.
func AddressReadError(err error, address byte, bus string) error {
	msg := fmt.Errorf("can't read from I2C address %d on bus %s", address, bus)
	return errors.Join(err, msg)
}

// UnexpectedDeviceError returns a standard error for we cannot find the expected device
// at the given address.
func UnexpectedDeviceError(address, response byte, deviceName string) error {
	return fmt.Errorf("unexpected non-%s device at address %d: response '%d'",
		deviceName, address, response)
}
