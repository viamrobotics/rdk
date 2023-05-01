package framesystem

import (
	"fmt"

	"go.viam.com/rdk/resource"
)

func DuplicateResourceShortNameError(name string) error {
	return fmt.Errorf("got multiple resources with name: %v", name)
}

func DependencyNotFoundError(name string) error {
	return fmt.Errorf("frame system could not find dependency with name: %v", name)
}

func NotInputEnabledError(component resource.Resource) error {
	return fmt.Errorf("%v(%T) is not InputEnabled", component.Name(), component)
}
