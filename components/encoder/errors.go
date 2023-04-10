package encoder

import "github.com/pkg/errors"

// NewEncoderTypeUnsupportedError returns a standard error for when
// an encoder does not support the given PositionType.
func NewEncoderTypeUnsupportedError(positionType PositionType) error {
	if positionType == 1 {
		return errors.New(
			"Encoder does not support Ticks, use a different PositionType")
	}
	if positionType == 2 {
		return errors.New(
			"Encoder does not support Angles, use a different PositionType")
	}
	return errors.New("Cannot identify PositionType")
}

// NewEncodedMotorTypeUnsupportedError returns a standard error for when
// an encoded motor tries to use an encoder that doesn't support Ticks.
func NewEncodedMotorTypeUnsupportedError(props map[Feature]bool) error {
	if props[AngleDegreesSupported] {
		return errors.New(
			"encoder type is Angle Degrees, need an encoder that supports Ticks for an encoded motor")
	}
	return errors.New("need an encoder that supports Ticks for an encoded motor")
}
