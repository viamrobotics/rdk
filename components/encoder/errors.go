package encoder

import "github.com/pkg/errors"

// NewPositionTypeUnsupportedError returns a standard error for when
// an encoder does not support the given PositionType.
func NewPositionTypeUnsupportedError(positionType PositionType) error {
	return errors.Errorf("encoder does not support %q; use a different PositionType", positionType)
}

// NewEncodedMotorPositionTypeUnsupportedError returns a standard error for when
// an encoded motor tries to use an encoder that doesn't support Ticks.
func NewEncodedMotorPositionTypeUnsupportedError(props Properties) error {
	if props.AngleDegreesSupported {
		return errors.New(
			"encoder position type is Angle Degrees, need an encoder that supports Ticks")
	}
	return errors.New("need an encoder that supports Ticks")
}
