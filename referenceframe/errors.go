package referenceframe

import "errors"

// NewParentFrameMissingError returns an error indicating that a frame is missing a parent.
func NewParentFrameMissingError() error {
	return errors.New("parent frame is nil")
}
