package utils

import (
	"github.com/pkg/errors"

	"go.viam.com/rdk/resource"
)

// NewResourceNotFoundError is used when a resource is not found.
func NewResourceNotFoundError(name resource.Name) error {
	return errors.Errorf("resource %q not found", name)
}

// NewRemoteResourceClashError is used when you are more than one resource with the same name exist.
func NewRemoteResourceClashError(name string) error {
	return errors.Errorf("more that one remote resources with name %q exists", name)
}

// DependencyNotFoundError is used when a resource is not found in a dependencies.
func DependencyNotFoundError(name string) error {
	return errors.Errorf("%q missing from dependencies", name)
}

// DependencyTypeError is used when a resource doesn't implement the expected interface.
func DependencyTypeError(name, expected, actual interface{}) error {
	return errors.Errorf("dependency %q should an implementation of %T but it was a %T", name, expected, actual)
}

// NewUnexpectedTypeError is used when there is a type mismatch.
func NewUnexpectedTypeError(expected, actual interface{}) error {
	return errors.Errorf("expected %T but got %T", expected, actual)
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(expected, actual interface{}) error {
	return errors.Errorf("expected implementation of %T but got %T", expected, actual)
}
