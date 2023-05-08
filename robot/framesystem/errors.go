package framesystem

import (
	"github.com/pkg/errors"
	"go.viam.com/rdk/resource"
)

func DuplicateResourceShortNameError(name string) error {
	return errors.Errorf("got multiple resources with name: %v", name)
}

func DependencyNotFoundError(name string) error {
	return errors.Errorf("frame system could not find dependency with name: %v", name)
}

func NotInputEnabledError(component resource.Resource) error {
	return errors.Errorf("%v(%T) is not InputEnabled", component.Name(), component)
}
