package framesystem

import (
	"github.com/pkg/errors"

	"braces.dev/errtrace"
	"go.viam.com/rdk/resource"
)

// DuplicateResourceNameError returns an error if multiple components are attempted to be registered in the frame system which
// share a name.
func DuplicateResourceNameError(name string) error {
	return errtrace.Wrap(errors.Errorf("got multiple resources with name: %v", name))
}

// DependencyNotFoundError returns an error if the given dependency name could not be found when building the framesystem.
func DependencyNotFoundError(name string) error {
	return errtrace.Wrap(errors.Errorf("frame system could not find dependency with name: %v", name))
}

// NotInputEnabledError is returned when the given component is not InputEnabled but should be.
func NotInputEnabledError(component resource.Resource) error {
	return errtrace.Wrap(errors.Errorf("%v(%T) is not InputEnabled", component.Name(), component))
}
