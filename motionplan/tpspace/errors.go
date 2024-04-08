package tpspace

import (
	"fmt"
)

// NewNonMatchingInputError creates an error describing when.
func NewNonMatchingInputError(val1, val2 float64) error {
	return fmt.Errorf("inputs %f and %f did not match. Index and alpha values must match when interpolating PTG frames", val1, val2)
}
