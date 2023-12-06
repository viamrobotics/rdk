// Package testutils implements test utilities.
package testutils

import "go.viam.com/rdk/resource"

// FakeConvertedAttributes is a helper for testing if validation works.
type FakeConvertedAttributes struct {
	Thing string
}

// Validate validates that the single fake attribute Thing exists properly
// in the struct, meant to implement the validator interface in component.go.
func (convAttr *FakeConvertedAttributes) Validate(path string) ([]string, error) {
	if convAttr.Thing == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "Thing")
	}
	return nil, nil
}
