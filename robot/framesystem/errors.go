package framesystem

import (
	"github.com/pkg/errors"

	"go.viam.com/rdk/resource"
)

// DuplicateResourceShortNameError returns an error if mutiple components are attempted to be registered in the frame system which
// share a short name.
func DuplicateResourceShortNameError(name string) error {
	return errors.Errorf("got multiple resources with name: %v", name)
}

// DependencyNotFoundError returns an error if the given dependency name could not be found when building the framesystem.
func DependencyNotFoundError(name string) error {
	return errors.Errorf("frame system could not find dependency with name: %v", name)
}

// NotInputEnabledError is returned when the given component is not InputEnabled but should be.
func NotInputEnabledError(component resource.Resource) error {
	return errors.Errorf("%v(%T) is not InputEnabled", component.Name(), component)
}
