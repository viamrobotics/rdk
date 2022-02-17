package spatialmath

import "github.com/pkg/errors"

func newBadGeometryDimensionsError(g Geometry) error {
	return errors.Errorf("Dimension(s) for Geometry type %T can not be less than or equal to zero", g)
}

func newCollisionTypeUnsupportedError(g1, g2 Geometry) error {
	return errors.Errorf("Collisions between %T and %T are not supported", g1, g2)
}

func newGeometryTypeUnsupportedError(geometryType string) error {
	return errors.Errorf("Unsupported Geometry type: %s", geometryType)
}

func newOrientationTypeUnsupportedError(orientationType string) error {
	return errors.Errorf("Orientation type %s unsupported in json configuration", orientationType)
}

func newRotationMatrixInputError(m []float64) error {
	return errors.Errorf("input slice has %d elements, need exactly 9", len(m))
}
