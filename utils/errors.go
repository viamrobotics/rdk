package utils

import "github.com/pkg/errors"

// NewUnexpectedTypeError is used when there is a type mismatch.
func NewUnexpectedTypeError(expected interface{}, actual interface{}) error {
	return errors.Errorf("expected %T but got %T", expected, actual)
}
