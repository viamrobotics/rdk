package utils

import (
	"fmt"
)

func CombineErrors(errs ...error) error {
	realErrors := []error{}
	for _, e := range errs {
		if e != nil {
			realErrors = append(realErrors, e)
		}
	}

	if len(realErrors) == 0 {
		return nil
	}

	s := "multiple errors: "
	for _, e := range realErrors {
		s += e.Error() + ", "
	}
	return fmt.Errorf(s)
}
