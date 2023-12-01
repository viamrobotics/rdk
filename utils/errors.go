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

// NewBuildTimeoutError is used when a resource times out during construction or reconfiguration.
func NewBuildTimeoutError(name string) error {
	timeout := GetResourceConfigurationTimeout(logging.Global())
	id := fmt.Sprintf("module %s", name)
	timeoutMsg := "reconfigure"
	return timeoutErrorHelper(id, timeout, timeoutMsg, DefaultResourceConfigurationTimeout, ResourceConfigurationTimeoutEnvVar)
}

// NewModuleStartUpTimeoutError is used when a module times out during startup.
func NewModuleStartUpTimeoutError(name string) error {
	timeout := GetModuleStartupTimeout(logging.Global())
	id := fmt.Sprintf("module %s", name)
	timeoutMsg := "startup"
	return timeoutErrorHelper(id, timeout, timeoutMsg, DefaultModuleStartupTimeout, ModuleStartupTimeoutEnvVar)
}

func timeoutErrorHelper(id string, timeout time.Duration, timeoutMsg string, defaultTimeout time.Duration, timeoutEnvVar string) error {
	extraMsg := ""
	if timeout == defaultTimeout {
		extraMsg = fmt.Sprintf(" Update %s env variable to override", timeoutEnvVar)
	}
	return fmt.Errorf(
		"%s timed out after %v during %v.%v",
		id, timeout, timeoutMsg, extraMsg,
	)
}
