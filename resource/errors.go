package resource

import (
	"fmt"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

// NewNotFoundError is used when a resource is not found.
func NewNotFoundError(name Name) error {
	return &notFoundError{name}
}

// IsNotFoundError returns if the given error is any kind of not found error.
func IsNotFoundError(err error) bool {
	var errArt *notFoundError
	return errors.As(err, &errArt)
}

type notFoundError struct {
	name Name
}

func (e *notFoundError) Error() string {
	return fmt.Sprintf("resource %q not found", e.name)
}

// NewNotAvailableError is used when a resource is not available because of some error.
func NewNotAvailableError(name Name, err error) error {
	return &notAvailableError{name, err}
}

// IsNotAvailableError returns if the given error is any kind of not available error.
func IsNotAvailableError(err error) bool {
	var errArt *notAvailableError
	return errors.As(err, &errArt)
}

type notAvailableError struct {
	name   Name
	reason error
}

func (e *notAvailableError) Error() string {
	return fmt.Sprintf("resource %q not available; reason=%q", e.name, e.reason)
}

// NewMustRebuildError is returned when a resource cannot be reconfigured in place and
// instead must be rebuilt. Almost all models/drivers should be able to reconfigure in
// place to support the best user experience.
func NewMustRebuildError(name Name) error {
	return &mustRebuildError{name: name}
}

// IsMustRebuildError returns whether or not the given error is a MustRebuildError.
func IsMustRebuildError(err error) bool {
	var errArt *mustRebuildError
	return errors.As(err, &errArt)
}

type mustRebuildError struct {
	name Name
}

func (e *mustRebuildError) Error() string {
	return fmt.Sprintf("cannot reconfigure %q; must rebuild", e.name)
}

// NewBuildTimeoutError is used when a resource times out during construction or reconfiguration.
func NewBuildTimeoutError(name Name) error {
	timeout := utils.GetResourceConfigurationTimeout(logging.Global())
	extraMsg := ""
	if timeout == utils.DefaultResourceConfigurationTimeout {
		extraMsg = fmt.Sprintf(" Update %s env variable to override", utils.ResourceConfigurationTimeoutEnvVar)
	}
	return fmt.Errorf(
		"resource %s timed out after %v during reconfigure.%v",
		name, timeout, extraMsg,
	)
}

// DependencyNotFoundError is used when a resource is not found in a dependencies.
func DependencyNotFoundError(name Name) error {
	// This error represents a logical configuration error. No need to include a stack trace.
	return fmt.Errorf("Resource missing from dependencies. Resource: %v", name)
}

// DependencyTypeError is used when a resource doesn't implement the expected interface.
func DependencyTypeError[T Resource](name Name, actual interface{}) error {
	// This error represents a coding error. Include a stack trace for diagnostics.
	return errors.Errorf("dependency %q should be an implementation of %s but it was a %T", name, utils.TypeStr[T](), actual)
}

// TypeError is used when a resource is an unexpected type.
func TypeError[T Resource](actual Resource) error {
	// This error represents a coding error. Include a stack trace for diagnostics.
	return errors.Errorf("expected implementation of %s but it was a %T", utils.TypeStr[T](), actual)
}
