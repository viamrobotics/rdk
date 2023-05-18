package movementsensor

import (
	"fmt"

	"github.com/pkg/errors"
)

func AddressReadError(err error, address byte, bus, board string) error {
	msg := fmt.Sprintf("can't read from I2C address %d on bus %s of board %s",
		address, bus, board)
	return errors.Wrap(err, msg)
}

func UnexpectedDeviceError(address, defaultAddress byte, deviceName string) error {
	return errors.Errorf("unexpected non-%s device at address %d: response '%d'",
		deviceName, address, defaultAddress)
}
