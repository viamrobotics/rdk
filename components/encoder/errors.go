package encoder

import "github.com/pkg/errors"
import "braces.dev/errtrace"

// NewPositionTypeUnsupportedError returns a standard error for when
// an encoder does not support the given PositionType.
func NewPositionTypeUnsupportedError(positionType PositionType) error {
	return errtrace.Wrap(errors.Errorf("encoder does not support %q; use a different PositionType", positionType))
}

// NewEncodedMotorPositionTypeUnsupportedError returns a standard error for when
// an encoded motor tries to use an encoder that doesn't support Ticks.
func NewEncodedMotorPositionTypeUnsupportedError(props Properties) error {
	if props.AngleDegreesSupported {
		return errtrace.Wrap(errors.New(
			"encoder position type is Angle Degrees, need an encoder that supports Ticks"))
	}
	return errtrace.Wrap(errors.New("need an encoder that supports Ticks"))
}
