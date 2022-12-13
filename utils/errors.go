package utils

import (
	"reflect"

	"github.com/pkg/errors"

	"go.viam.com/rdk/resource"
)

// NewResourceNotFoundError is used when a resource is not found.
func NewResourceNotFoundError(name resource.Name) error {
	return errors.Errorf("resource %q not found", name)
}

// NewResourceNotAvailableError is used when a resource is not available because of some error.
func NewResourceNotAvailableError(name resource.Name, err error) error {
	return errors.Wrapf(err, "resource %q not available", name)
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
func DependencyTypeError(name string, expected, actual interface{}) error {
	return errors.Errorf("dependency %q should be an implementation of %s but it was a %T", name, typeStr(expected), actual)
}

// NewUnexpectedTypeError is used when there is a type mismatch.
func NewUnexpectedTypeError(expected, actual interface{}) error {
	return errors.Errorf("expected %s but got %T", typeStr(expected), actual)
}

func typeStr(of interface{}) string {
	if of == nil {
		// full nilness
		return "<unknown (nil interface)>"
	}
	vT := reflect.TypeOf(of)
	if vT.Kind() != reflect.Ptr {
		return vT.String()
	}
	if vT.Elem().Kind() == reflect.Interface {
		// RULE: we never actually expect a *T where T is an interface
		return vT.Elem().String()
	}
	return vT.String()
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
// Future: This should also tell you that expected is not even an interface.
func NewUnimplementedInterfaceError(expected, actual interface{}) error {
	return errors.Errorf("expected implementation of %s but got %T", typeStr(expected), actual)
}

// InvalidIntegerValue is used when an integer value is out of bound or not valid.
func InvalidIntegerValue(val int) error {
	return errors.Errorf("can not validate integer value %d in config", val)
}
