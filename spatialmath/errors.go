package spatialmath

import "github.com/pkg/errors"

var errGeometryTypeUnsupported = errors.New("unsupported Geometry type")

func newBadGeometryDimensionsError(g Geometry) error {
	return errors.Errorf("Invalid dimension(s) for Geometry type %T", g)
}

func newBadCapsuleLengthError(l, r float64) error {
	return errors.Errorf("Capsule total length %f cannot be less than 2x radius %f", l, r)
}

var errCollisionTypeUnsupported = errors.New("unsupported collision between geometry types")

func newOrientationTypeUnsupportedError(orientationType string) error {
	return errors.Errorf("Orientation type %s unsupported in json configuration", orientationType)
}

func newRotationMatrixInputError(m []float64) error {
	return errors.Errorf("input slice has %d elements, need exactly 9", len(m))
}
