package motor

import "github.com/pkg/errors"

// NewGoTillStopUnsupportedError returns a standard error for when a motor
// is required to support the GoTillStop feature.
func NewGoTillStopUnsupportedError(motorName string) error {
	return errors.Errorf("motor with name %s must support GoTillStop", motorName)
}
