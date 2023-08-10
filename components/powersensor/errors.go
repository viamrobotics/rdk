package powersensor

import "errors"

var (
	// ErrMethodUnimplementedVoltage returns error if the Voltage method is unimplemented.
	ErrMethodUnimplementedVoltage = errors.New("Voltage Unimplemented")
	// ErrMethodUnimplementedCurrent returns error if the Current method is unimplemented.
	ErrMethodUnimplementedCurrent = errors.New("Current Unimplemented")
	// ErrMethodUnimplementedPower returns error if the Power method is unimplemented.
	ErrMethodUnimplementedPower = errors.New("Power Unimplemented")
)
