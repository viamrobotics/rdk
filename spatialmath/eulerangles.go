package spatialmath

import (
	"math"

	"gonum.org/v1/gonum/num/quat"
)

// EulerAngles are three angles (in radians) used to represent the rotation of an object in 3D Euclidean space
// The Tait–Bryan angle formalism is used, with rotations around three distinct axes in the z-y′-x″ sequence.
type EulerAngles struct {
	Roll  float64 `json:"roll"`  // phi, X
	Pitch float64 `json:"pitch"` // theta, Y
	Yaw   float64 `json:"yaw"`   // psi, Z
}

// NewEulerAngles creates an empty EulerAngles struct.
func NewEulerAngles() *EulerAngles {
	return &EulerAngles{Roll: 0, Pitch: 0, Yaw: 0}
}

// EulerAngles returns orientation in Euler angle representation.
func (ea *EulerAngles) EulerAngles() *EulerAngles {
	return ea
}

// Quaternion returns orientation in quaternion representation.
func (ea *EulerAngles) Quaternion() quat.Number {
	cy := math.Cos(ea.Yaw * 0.5)
	sy := math.Sin(ea.Yaw * 0.5)
	cp := math.Cos(ea.Pitch * 0.5)
	sp := math.Sin(ea.Pitch * 0.5)
	cr := math.Cos(ea.Roll * 0.5)
	sr := math.Sin(ea.Roll * 0.5)

	q := quat.Number{}
	q.Real = cr*cp*cy + sr*sp*sy
	q.Imag = sr*cp*cy - cr*sp*sy
	q.Jmag = cr*sp*cy + sr*cp*sy
	q.Kmag = cr*cp*sy - sr*sp*cy

	return q
}

// OrientationVectorRadians returns orientation as an orientation vector (in radians).
func (ea *EulerAngles) OrientationVectorRadians() *OrientationVector {
	return QuatToOV(ea.Quaternion())
}

// OrientationVectorDegrees returns orientation as an orientation vector (in degrees).
func (ea *EulerAngles) OrientationVectorDegrees() *OrientationVectorDegrees {
	return QuatToOVD(ea.Quaternion())
}

// AxisAngles returns the orientation in axis angle representation.
func (ea *EulerAngles) AxisAngles() *R4AA {
	return QuatToR4AA(ea.Quaternion())
}

// RotationMatrix returns the orientation in rotation matrix representation.
func (ea *EulerAngles) RotationMatrix() *RotationMatrix {
	return QuatToRotationMatrix(ea.Quaternion())
}

// Hash returns a hash value for these Euler angles.
func (ea *EulerAngles) Hash() int {
	hash := 0
	hash += (5 * (int(ea.Roll*1000) + 1000)) * 2
	hash += (6 * (int(ea.Pitch*1000) + 2000)) * 3
	hash += (7 * (int(ea.Yaw*1000) + 3000)) * 4
	return hash
}
