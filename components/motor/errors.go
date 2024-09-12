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
	return errors.Errorf("motor named %s has wrong support for property %#v", motorName, prop)
}

// NewZeroRPMError returns an error representing a request to move a motor at
// zero speed (i.e., moving the motor without moving the motor).
func NewZeroRPMError() error {
	return errors.New("Cannot move motor at an RPM that is nearly 0")
}

// NewZeroRevsError returns an error representing a request to move a motor for 0 revolutions.
func NewZeroRevsError() error {
	return errors.New("Cannot move motor for 0 revolutions")
}

// NewGoToUnsupportedError returns error when a motor is required to support GoTo feature.
func NewGoToUnsupportedError(motorName string) error {
	return errors.Errorf("motor with name %s does not support GoTo", motorName)
}

// NewControlParametersUnimplementedError returns an error when a control parameters are
// unimplemented in the config being used of a controlledMotor.
func NewControlParametersUnimplementedError() error {
	return errors.New("control parameters must be configured to setup a motor with controls")
}

// NewSetRPMUnsupportedError returns an error when a motor does not support SetRPM.
func NewSetRPMUnsupportedError(motorName string) error {
	return errors.Errorf("motor named %s does not support SetRPM", motorName)
}
