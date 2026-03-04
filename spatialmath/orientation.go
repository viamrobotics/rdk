package spatialmath

import (
	"gonum.org/v1/gonum/num/quat"
)

// If two angles differ by less than this amount, we consider them the same for the purpose of doing
// math around the poles of orientation.
// This needs to be very small in order to account for the small steps taken by IK. Otherwise singularities happen.
const defaultAngleEpsilon = 1e-4

// Orientation is an interface used to express the different parameterizations of the orientation
// of a rigid object or a frame of reference in 3D Euclidean space.
type Orientation interface {
	OrientationVectorRadians() *OrientationVector
	OrientationVectorDegrees() *OrientationVectorDegrees
	AxisAngles() *R4AA
	Quaternion() quat.Number
	EulerAngles() *EulerAngles
	RotationMatrix() *RotationMatrix
}

// NewZeroOrientation returns an orientation which signifies no rotation.
func NewZeroOrientation() Orientation {
	return &Quaternion{1, 0, 0, 0}
}

// OrientationAlmostEqual will return a bool describing whether 2 poses have approximately the same orientation.
func OrientationAlmostEqual(o1, o2 Orientation) bool {
	return OrientationAlmostEqualEps(o1, o2, defaultAngleEpsilon)
}

// OrientationAlmostEqualEps will return a bool describing whether 2 poses have approximately the same orientation.
func OrientationAlmostEqualEps(o1, o2 Orientation, epsilon float64) bool {
	if o1 == nil {
		return o2 == nil
	} else if o2 == nil {
		return false
	}

	return QuatToR3AA(OrientationBetween(o1, o2).Quaternion()).Norm2() < epsilon
}

// OrientationBetween returns the orientation representing the difference between the two given orientations.
func OrientationBetween(o1, o2 Orientation) Orientation {
	q := Quaternion(quat.Mul(o2.Quaternion(), quat.Conj(o1.Quaternion())))
	return &q
}

// OrientationInverse returns the orientation representing the inverse of the given orientation.
func OrientationInverse(o Orientation) Orientation {
	q := Quaternion(quat.Inv(o.Quaternion()))
	return &q
}

// IsDefaultOrientation return true if there is no orientation or if it's the default
// only works for OrientationVectorDegrees right now
func IsDefaultOrientation(o Orientation) bool {
	if o == nil {
		return true
	}

	switch oo := o.(type) {
	case *OrientationVectorDegrees:
		return oo.OX == 0 && oo.OY == 0 && oo.OZ == 1 && oo.Theta == 0
	case *OrientationVectorRadians:
		return oo.OX == 0 && oo.OY == 0 && oo.OZ == 1 && oo.Theta == 0

	}

	return false
}
