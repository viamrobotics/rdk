package movementsensor

import (
	"fmt"

	"braces.dev/errtrace"
	"github.com/pkg/errors"
)

// AddressReadError returns a standard error for when we cannot read from an I2C bus.
func AddressReadError(err error, address byte, bus string) error {
	msg := fmt.Sprintf("can't read from I2C address %d on bus %s", address, bus)
	return errtrace.Wrap(errors.Wrap(err, msg))
}

// UnexpectedDeviceError returns a standard error for we cannot find the expected device
// at the given address.
func UnexpectedDeviceError(address, response byte, deviceName string) error {
	return errtrace.Wrap(errors.Errorf("unexpected non-%s device at address %d: response '%d'",
		deviceName, address, response))
}
