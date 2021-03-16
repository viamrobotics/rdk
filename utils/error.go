package utils

import (
	"errors"
	"strings"

	"go.uber.org/multierr"
)

// FilterOutError filters out an error based on the given target. For
// example, if err was context.Canceled and so was the target, this
// would return nil. Furthermore, if err was a multierr containing
// a context.Canceled, it would also be filtered out from a new
// multierr.
func FilterOutError(err, target error) error {
	if err == nil {
		return nil
	}
	if target == nil {
		return err
	}
	errs := multierr.Errors(err)
	if len(errs) == 1 {
		// multierr flattens errors so we can assume this
		// is not a multierr
		if errors.Is(err, target) || strings.Contains(err.Error(), target.Error()) {
			return nil
		}
		return err
	}
	newErrs := make([]error, 0, len(errs))
	for _, e := range errs {
		if FilterOutError(e, target) == nil {
			continue
		}
		newErrs = append(newErrs, e)
	}
	return multierr.Combine(newErrs...)
}
