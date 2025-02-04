package utils

import (
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

// NewRemoteResourceClashError is used when you are more than one resource with the same name exist.
func NewRemoteResourceClashError(name string) error {
	return errors.Errorf("more than one remote resources with name %q exists", name)
}

// NewUnexpectedTypeError is used when there is a type mismatch.
func NewUnexpectedTypeError[ExpectedT any](actual interface{}) error {
	return errors.Errorf("expected %s but got %T", TypeStr[ExpectedT](), actual)
}

// TypeStr returns the a human readable type string of the given value.
func TypeStr[T any]() string {
	zero := new(T)
	vT := reflect.TypeOf(zero).Elem()
	return vT.String()
}

// NewWeakDependenciesUpdateTimeoutError is used when a resource times out during weak dependencies update.
func NewWeakDependenciesUpdateTimeoutError(name string) error {
	timeout := GetResourceConfigurationTimeout(logging.Global())
	id := fmt.Sprintf("resource %s", name)
	timeoutMsg := "weak dependencies update"
	return timeoutErrorHelper(id, timeout, timeoutMsg)
}

// NewBuildTimeoutError is used when a resource times out during construction or reconfiguration.
func NewBuildTimeoutError(name string) error {
	timeout := GetResourceConfigurationTimeout(logging.Global())
	id := fmt.Sprintf("resource %s", name)
	timeoutMsg := "reconfigure"
	return timeoutErrorHelper(id, timeout, timeoutMsg)
}

// NewModuleStartUpTimeoutError is used when a module times out during startup.
func NewModuleStartUpTimeoutError(name string) error {
	timeout := GetModuleStartupTimeout(logging.Global())
	id := fmt.Sprintf("module %s", name)
	timeoutMsg := "startup"
	return timeoutErrorHelper(id, timeout, timeoutMsg)
}

func timeoutErrorHelper(id string, timeout time.Duration, timeoutMsg string) error {
	return fmt.Errorf("%s timed out after %v during %v", id, timeout, timeoutMsg)
}

// NewConfigValidationError returns a config validation error
// occurring at a given path.
// copied from goutils.
func NewConfigValidationError(path string, err error) error {
	return errors.Wrapf(err, "error validating %q", path)
}

// NewConfigValidationFieldRequiredError returns a config validation
// error for a field missing at a given path.
// copied from goutils.
func NewConfigValidationFieldRequiredError(path, field string) error {
	return NewConfigValidationError(path, errors.Errorf("%q is required", field))
}
