package utils

import "github.com/pkg/errors"

// NewUnexpectedTypeError is used when there is a type mismatch.
func NewUnexpectedTypeError(expected interface{}, actual interface{}) error {
	return errors.Errorf("expected %T but got %T", expected, actual)
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(expected string, actual interface{}) error {
	return errors.Errorf("expected implementation of %s but got %T", expected, actual)
}
