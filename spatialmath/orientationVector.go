// Package spatialmath defines spatial mathematical operations
package spatialmath

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/num/quat"
)

// OrientationVec containing ox, oy, oz, theta represents an orientation vector
// Structured similarly to an angle axis, an orientation vector works differently. Rather than representing an orientation
// with an arbitrary axis and a rotation around it from an origin, an orientation vector represents orientation
// such that the ox/oy/oz components represent the point on the cartesian unit sphere at which your end effector is pointing
// from the origin, and that unit vector forms an axis around which theta rotates. This means that incrementing/decrementing
// theta will perform an in-line rotation of the end effector.
// Theta is defined as rotation between two planes: the plane defined by the origin, the point (0,0,1), and the rx,ry,rz
// point, and the plane defined by the origin, the rx,ry,rz point, and the new local Z axis. So if theta is kept at
// zero as the north/south pole is circled, the Roll will correct itself to remain in-line.
type OrientationVec struct {
	Theta float64
	OX    float64
	OY    float64
	OZ    float64
}

// ToQuat converts an orientation vector to a quaternion.
func (ov *OrientationVec) ToQuat() quat.Number {

	// acos(rz) ranges from 0 (north pole) to pi (south pole)
	lat := -math.Pi/2 + math.Acos(ov.OZ)

	// If we're pointing at the Z axis then lon can be 0
	lon := 0.0
	theta := ov.Theta

	// The contents of ov.OX are not in radians but we use angleEpsilon anyway to check how close we are to the pole
	if 1+ov.OX < angleEpsilon {
		lon = math.Pi
	} else if 1-math.Abs(ov.OZ) > angleEpsilon {
		// atan x/y removes some sign information so we use atan2 to do it properly
		lon = math.Atan2(ov.OY, ov.OX)
	} else {
		// We are pointed at a pole (Z = 1 or -1) so theta is reversed
		theta *= -1
	}

	var q quat.Number
	q1 := mgl64.AnglesToQuat(lon, lat, theta, mgl64.ZYX)
	q.Real = q1.W
	q.Imag = q1.X()
	q.Jmag = q1.Y()
	q.Kmag = q1.Z()
	return q
}

// Normalize scales the x, y, and z components of an Orientation Vector to be on the unit sphere
func (ov *OrientationVec) Normalize() {
	norm := math.Sqrt(ov.OX*ov.OX + ov.OY*ov.OY + ov.OZ*ov.OZ)
	ov.OX /= norm
	ov.OY /= norm
	ov.OZ /= norm
}
