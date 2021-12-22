package spatialmath

import (
	"gonum.org/v1/gonum/num/quat"
)

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

// NewZeroOrientation returns an orientatation which signifies no rotation
func NewZeroOrientation() Orientation {
	return &quaternion{1, 0, 0, 0}
}

// OrientationBetween returns the orientation representing the difference between the two given Orientations
func OrientationBetween(o1, o2 Orientation) Orientation {
	q := quaternion(quat.Mul(o2.Quaternion(), quat.Conj(o1.Quaternion())))
	return &q
}
