package motor

import "github.com/pkg/errors"

// NewResetZeroPositionUnsupportedError returns a standard error for when a motor
// is required to support reseting the zero position.
func NewResetZeroPositionUnsupportedError(motorName string) error {
	return errors.Errorf("motor with name %s does not support ResetZeroPosition", motorName)
}

// NewPropertyUnsupportedError returns an error representing the need
// for a motor to support a particular property.
func NewPropertyUnsupportedError(prop Properties, motorName string) error {
	return errors.Errorf("motor named %s has wrong support for property %#v", motorName, prop.PositionReporting)
}

// NewZeroRPMError returns an error representing a request to move a motor at
// zero speed (i.e., moving the motor without moving the motor).
func NewZeroRPMError() error {
	return errors.New("Cannot move motor at 0 RPM")
}

// NewGoToUnsupportedError returns error when a motor is required to support GoTo feature.
func NewGoToUnsupportedError(motorName string) error {
	return errors.Errorf("motor with name %s does not support GoTo", motorName)
}
