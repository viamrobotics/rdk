package utils

import (
	"github.com/pkg/errors"

	"go.viam.com/rdk/resource"
)

// NewResourceNotFoundError is used when a resource is not found.
func NewResourceNotFoundError(name resource.Name) error {
	return errors.Errorf("resource %q not found", name)
}

// DependencyNotFoundError is used when a resource is not found in a dependencies.
func DependencyNotFoundError(name string) error {
	return errors.Errorf("%q missing from dependencies", name)
}

// DependencyTypeError is used when a resource is not found in a dependencies.
func DependencyTypeError(name string, expected string, actual interface{}) error {
	return errors.Errorf("dependency %q should an implementation of %s but it was a %T", name, expected, actual)
}

// NewUnexpectedTypeError is used when there is a type mismatch.
func NewUnexpectedTypeError(expected interface{}, actual interface{}) error {
	return errors.Errorf("expected %T but got %T", expected, actual)
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(expected string, actual interface{}) error {
	return errors.Errorf("expected implementation of %s but got %T", expected, actual)
}
