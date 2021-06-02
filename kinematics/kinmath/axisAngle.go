package kinmath

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
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

// R4AA represents an R4 axis angle
type R4AA struct {
	Theta float64
	RX    float64
	RY    float64
	RZ    float64
}

// OrientVec represents an orientation vector
// Structured similarly to an R4AA, an orientation vector works differently. Rather than representing an orientation
// with an arbitrary axis and a rotation around it from an origin, an orientation vector represents orientation
// such that the rx/ry/rz components represent the point on the cartesian unit sphere at which your end effector is pointing
// from the origin, and that unit vector forms an axis around which theta rotates. This means that incrementing/decrementing
// theta will perform an in-line rotation of the end effector.
// Theta is defined as rotation between two planes: the plane defined by the origin, the point (0,0,1), and the rx,ry,rz
// point, and the plane defined by the origin, the rx,ry,rz point, and the new local Z axis. So if theta is kept at
// zero as the north/south pole is circled, the Roll will correct itself to remain in-line.
type OrientVec struct {
	Theta float64
	RX    float64
	RY    float64
	RZ    float64
}

// ToR3 converts an R4 angle axis to R3
func (r4 *R4AA) ToR3() R3AA {
	return R3AA{r4.RX * r4.Theta, r4.RY * r4.Theta, r4.RZ * r4.Theta}
}

// ToQuat converts an R4 axis angle to a unit quaternion
// See: https://www.euclideanspace.com/maths/geometry/rotations/conversions/angleToQuaternion/index.htm1
func (r4 *R4AA) ToQuat() quat.Number {
	if r4.Theta < 1e-6 {
		// If angle is zero, we return the identity quaternion
		return quat.Number{1, 0, 0, 0}
	}
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
	r4.RX /= norm
	r4.RY /= norm
	r4.RZ /= norm
}

// ToR4 converts an R3 angle axis to R4
func (r3 *R3AA) ToR4() R4AA {
	theta := math.Sqrt(r3.RX*r3.RX + r3.RY*r3.RY + r3.RZ*r3.RZ)
	return R4AA{theta, r3.RX / theta, r3.RY / theta, r3.RZ / theta}
}

// ToQuat converts an orientation vector to a quaternion.
func (ov *OrientVec) ToQuat() quat.Number {

	q := quat.Number{}
	// acos(rz) ranges from 0 (north pole) to pi (south pole)
	lat := -math.Pi/2 + math.Acos(ov.RZ)

	// If we're pointing at the Z axis then lon can be 0
	lon := 0.0
	if ov.RX == -1 {
		lon = math.Pi
	} else if ov.RY != 0 || ov.RX != 0 {
		// atan x/y removes some sign information so we need to do some special stuff to wind up in the right quadrant
		lon = math.Atan2(ov.RY, ov.RX)
	}

	q1 := mgl64.AnglesToQuat(lon, lat, ov.Theta, mgl64.ZYX)
	q.Real = q1.W
	q.Imag = q1.X()
	q.Jmag = q1.Y()
	q.Kmag = q1.Z()
	return q
}

// Normalize scales the x, y, and z components of an Orientation Vector to be on the unit sphere
func (ov *OrientVec) Normalize() {
	norm := math.Sqrt(ov.RX*ov.RX + ov.RY*ov.RY + ov.RZ*ov.RZ)
	ov.RX /= norm
	ov.RY /= norm
	ov.RZ /= norm
}
