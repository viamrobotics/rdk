package spatialmath

import (
	"math"

	"gonum.org/v1/gonum/num/quat"
)

// See here for a thorough explanation: https://en.wikipedia.org/wiki/Axis%E2%80%93angle_representation
// Basic explanation: Imagine a 3d cartesian grid centered at 0,0,0, and a sphere of radius 1 centered at
// that same point. An orientation can be expressed by first specifying an axis, i.e. a line from the origin
// to a point on that sphere, represented by (rx, ry, rz), and a rotation around that axis, theta.
// These four numbers can be used as-is (R4), or they can be converted to R3, where theta is multiplied by each of
// the unit sphere components to give a vector whose length is theta and whose direction is the original axis.

// R3AA represents an R3 axis angle
type R3AA struct {
	RX float64
	RY float64
	RZ float64
}

// NewR3AA creates an empty R3AA struct
func NewR3AA() *R3AA {
	return &R3AA{RX: 0, RY: 0, RZ: 0}
}

// R4AA represents an R4 axis angle
type R4AA struct {
	Theta float64 `json:"th"`
	RX    float64 `json:"x"`
	RY    float64 `json:"y"`
	RZ    float64 `json:"z"`
}

// NewR4AA creates an empty R4AA struct
func NewR4AA() *R4AA {
	return &R4AA{Theta: 0, RX: 0, RY: 0, RZ: 1}
}

// AxisAngles returns the orientation in axis angle representation
func (r4 *R4AA) AxisAngles() *R4AA {
	return r4
}

// Quaternion returns orientation in quaternion representation
func (r4 *R4AA) Quaternion() quat.Number {
	return r4.ToQuat()
}

// OrientationVectorRadians returns orientation as an orientation vector (in radians)
func (r4 *R4AA) OrientationVectorRadians() *OrientationVector {
	return QuatToOV(r4.Quaternion())
}

// OrientationVectorDegrees returns orientation as an orientation vector (in degrees)
func (r4 *R4AA) OrientationVectorDegrees() *OrientationVectorDegrees {
	return QuatToOVD(r4.Quaternion())
}

// EulerAngles returns orientation in Euler angle representation
func (r4 *R4AA) EulerAngles() *EulerAngles {
	return QuatToEulerAngles(r4.Quaternion())
}

// RotationMatrix returns the orientation in rotation matrix representation
func (r4 *R4AA) RotationMatrix() *RotationMatrix {
	return QuatToRotationMatrix(r4.Quaternion())
}

// ToR3 converts an R4 angle axis to R3
func (r4 *R4AA) ToR3() R3AA {
	return R3AA{r4.RX * r4.Theta, r4.RY * r4.Theta, r4.RZ * r4.Theta}
}

// ToQuat converts an R4 axis angle to a unit quaternion
// See: https://www.euclideanspace.com/maths/geometry/rotations/conversions/angleToQuaternion/index.htm1
func (r4 *R4AA) ToQuat() quat.Number {
	sinA := math.Sin(r4.Theta / 2)
	// Ensure that point xyz is on the unit sphere
	r4.Normalize()

	// Get the unit-sphere components
	ax := r4.RX * sinA
	ay := r4.RY * sinA
	az := r4.RZ * sinA
	w := math.Cos(r4.Theta / 2)
	return quat.Number{w, ax, ay, az}
}

// Normalize scales the x, y, and z components of a R4 axis angle to be on the unit sphere
func (r4 *R4AA) Normalize() {
	norm := math.Sqrt(r4.RX*r4.RX + r4.RY*r4.RY + r4.RZ*r4.RZ)
	if norm == 0.0 { // prevent division by 0
		panic("axis angle vector has length of 0")
	}
	r4.RX /= norm
	r4.RY /= norm
	r4.RZ /= norm
}

// ToR4 converts an R3 angle axis to R4
func (r3 *R3AA) ToR4() R4AA {
	theta := math.Sqrt(r3.RX*r3.RX + r3.RY*r3.RY + r3.RZ*r3.RZ)
	return R4AA{theta, r3.RX / theta, r3.RY / theta, r3.RZ / theta}
}
