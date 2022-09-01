package motionplan

import "github.com/pkg/errors"

func NewIncorrectInputLengthError(actual, expected int) error {
	return errors.Errorf("incorrect number of inputs to transform, expected %d but got %d", expected, actual)
}
