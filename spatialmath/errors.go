package spatialmath

import "github.com/pkg/errors"

// ErrGeometryTypeUnsupported is an error that is returned when an unsupported GeometryType is specified
// (either implicitly or explicitly) in a GeometryConfig.
var ErrGeometryTypeUnsupported = errors.New("unsupported Geometry type")

func newBadGeometryDimensionsError(g Geometry) error {
	return errors.Errorf("Invalid dimension(s) for Geometry type %T", g)
}

func newCollisionTypeUnsupportedError(g1, g2 Geometry) error {
	return errors.Errorf("Collisions between %T and %T are not supported", g1, g2)
}

func newOrientationTypeUnsupportedError(orientationType string) error {
	return errors.Errorf("Orientation type %s unsupported in json configuration", orientationType)
}

func newRotationMatrixInputError(m []float64) error {
	return errors.Errorf("input slice has %d elements, need exactly 9", len(m))
}
