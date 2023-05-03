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

// NewMissingParentError returns an error for when a part has named a parent whose part is missing from the collection of Parts that are
// becoming a FrameSystem object
func MissingParentError(partName, parentName string) error {
	return errors.Errorf("part with name %s references non-existent parent %s", partName, parentName)
}

var NoWorldConnectionError = errors.New("there are no robot parts that connect to a 'world' node. Root node must be named 'world'")
