// Package spatialmath defines spatial mathematical operations
package spatialmath

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/core/utils"
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
	Theta float64 `json:"th"`
	OX    float64 `json:"x"`
	OY    float64 `json:"y"`
	OZ    float64 `json:"z"`
}

// OrientationVecDegrees is the orientation vector between two objects, but expressed in degrees rather than radians.
// Because ArmPosition is in degrees, this is necessary.
type OrientationVecDegrees struct {
	Theta float64 `json:"th"`
	OX    float64 `json:"x"`
	OY    float64 `json:"y"`
	OZ    float64 `json:"z"`
}

// NewOrientationVector Creates a zero-initialized OrientationVec
func NewOrientationVector() *OrientationVec {
	return &OrientationVec{Theta: 0, OX: 0, OY: 0, OZ: 1}
}

// Degrees converts the OrientationVec to an OrientationVecDegrees
func (ov *OrientationVec) Degrees() *OrientationVecDegrees {
	return &OrientationVecDegrees{Theta: utils.RadToDeg(ov.Theta), OX: ov.OX, OY: ov.OY, OZ: ov.OZ}
}

// ToQuat converts an orientation vector to a quaternion.
func (ov *OrientationVec) ToQuat() quat.Number {
	// make sure OrientationVec is normalized first
	ov.Normalize()

	// acos(rz) ranges from 0 (north pole) to pi (south pole)
	lat := math.Acos(ov.OZ)

	// If we're pointing at the Z axis then lon is 0, theta is the OV theta
	// Euler angles are gimbal locked here but OV allows us to have smooth(er) movement
	// Since euler angles are used to represent a single orientation, but not to move between different ones, this is OK
	lon := 0.0
	theta := ov.Theta

	if 1-math.Abs(ov.OZ) > angleEpsilon {
		// If we are not at a pole, we need the longitude
		// atan x/y removes some sign information so we use atan2 to do it properly
		lon = math.Atan2(ov.OY, ov.OX)
	}

	var q quat.Number
	// Since the "default" orientation is pointed at the Z axis, we use ZYZ rotation order to properly represent the OV
	q1 := mgl64.AnglesToQuat(lon, lat, theta, mgl64.ZYZ)
	q.Real = q1.W
	q.Imag = q1.X()
	q.Jmag = q1.Y()
	q.Kmag = q1.Z()

	return q
}

// Normalize scales the x, y, and z components of an Orientation Vector to be on the unit sphere
func (ov *OrientationVec) Normalize() {
	norm := math.Sqrt(ov.OX*ov.OX + ov.OY*ov.OY + ov.OZ*ov.OZ)
	if norm == 0.0 { // avoid division by zero
		panic("orientation vec has length of 0")
	}
	ov.OX /= norm
	ov.OY /= norm
	ov.OZ /= norm
}

// EulerAngles returns orientation in Euler angle representation
func (ov *OrientationVec) EulerAngles() *EulerAngles {
	return QuatToEulerAngles(ov.ToQuat())
}

// Quaternion returns orientation in quaternion representation
func (ov *OrientationVec) Quaternion() quat.Number {
	return ov.ToQuat()
}

// OrientationVectorRadians returns orientation as an orientation vector (in radians)
func (ov *OrientationVec) OrientationVectorRadians() *OrientationVec {
	return ov
}

// OrientationVectorDegrees returns orientation as an orientation vector (in degrees)
func (ov *OrientationVec) OrientationVectorDegrees() *OrientationVecDegrees {
	return ov.Degrees()
}

// AxisAngles returns the orientation in axis angle representation
func (ov *OrientationVec) AxisAngles() *R4AA {
	return QuatToR4AA(ov.ToQuat())
}

// NewOrientationVectorDegrees Creates a zero-initialized OrientationVecDegrees
func NewOrientationVectorDegrees() *OrientationVecDegrees {
	return &OrientationVecDegrees{Theta: 0, OX: 0, OY: 0, OZ: 1}
}

// Radians converts a OrientationVecDegrees to an OrientationVec
func (ovd *OrientationVecDegrees) Radians() *OrientationVec {
	return &OrientationVec{Theta: utils.DegToRad(ovd.Theta), OX: ovd.OX, OY: ovd.OY, OZ: ovd.OZ}
}

// ToQuat converts an orientation vector in degrees to a quaternion.
func (ovd *OrientationVecDegrees) ToQuat() quat.Number {
	return ovd.Radians().ToQuat()
}

// EulerAngles returns orientation in Euler angle representation
func (ovd *OrientationVecDegrees) EulerAngles() *EulerAngles {
	return QuatToEulerAngles(ovd.ToQuat())
}

// Quaternion returns orientation in quaternion representation
func (ovd *OrientationVecDegrees) Quaternion() quat.Number {
	return ovd.ToQuat()
}

// OrientationVectorRadians returns orientation as an orientation vector (in radians)
func (ovd *OrientationVecDegrees) OrientationVectorRadians() *OrientationVec {
	return ovd.Radians()
}

// OrientationVectorDegrees returns orientation as an orientation vector (in degrees)
func (ovd *OrientationVecDegrees) OrientationVectorDegrees() *OrientationVecDegrees {
	return ovd
}

// AxisAngles returns the orientation in axis angle representation
func (ovd *OrientationVecDegrees) AxisAngles() *R4AA {
	return QuatToR4AA(ovd.ToQuat())
}
