package utils

import (
	"strings"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"
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

// NewConfigValidationError returns a config validation error
// occurring at a given path.
func NewConfigValidationError(path string, err error) error {
	return errors.Wrapf(err, "error validating %q", path)
}

// NewConfigValidationFieldRequiredError returns a config validation
// error for a field missing at a given path.
func NewConfigValidationFieldRequiredError(path, field string) error {
	return NewConfigValidationError(path, errors.Errorf("%q is required", field))
}

// UncheckedError is used in places where we really do not care about an error but we
// want to at least report it. Never use this for closing writers.
func UncheckedError(err error) {
	uncheckedError(err)
}

func uncheckedError(err error) {
	if err == nil {
		return
	}
	golog.Global().Desugar().WithOptions(zap.AddCallerSkip(2)).Sugar().Debugw("unchecked error", "error", err)
}

// UncheckedErrorFunc is used in places where we really do not care about an error but we
// want to at least report it. Never use this for closing writers.
func UncheckedErrorFunc(f func() error) {
	uncheckedError(f())
}
