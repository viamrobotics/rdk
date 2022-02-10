package motor

import "github.com/pkg/errors"

// NewGoTillStopUnsupportedError returns a standard error for when a motor
// is required to support the GoTillStop feature.
func NewGoTillStopUnsupportedError(motorName string) error {
	return errors.Errorf("motor with name %s does not support GoTillStop", motorName)
}

// NewResetZeroPositionUnsupportedError returns a standard error for when a motor
// is required to support reseting the zero position.
func NewResetZeroPositionUnsupportedError(motorName string) error {
	return errors.Errorf("motor with name %s does not support ResetZeroPosition", motorName)
}
