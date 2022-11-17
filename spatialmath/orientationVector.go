package spatialmath

import (
	"errors"
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/utils"
)

// OrientationVector containing ox, oy, oz, theta represents an orientation vector
// Structured similarly to an angle axis, an orientation vector works differently. Rather than representing an orientation
// with an arbitrary axis and a rotation around it from an origin, an orientation vector represents orientation
// such that the ox/oy/oz components represent the point on the cartesian unit sphere at which your end effector is pointing
// from the origin, and that unit vector forms an axis around which theta rotates. This means that incrementing/decrementing
// theta will perform an in-line rotation of the end effector.
// Theta is defined as rotation between two planes: the plane defined by the origin, the point (0,0,1), and the rx,ry,rz
// point, and the plane defined by the origin, the rx,ry,rz point, and the new local Z axis. So if theta is kept at
// zero as the north/south pole is circled, the Roll will correct itself to remain in-line.
type OrientationVector struct {
	Theta float64 `json:"th"`
	OX    float64 `json:"x"`
	OY    float64 `json:"y"`
	OZ    float64 `json:"z"`
}

// OrientationVectorDegrees is the orientation vector between two objects, but expressed in degrees rather than radians.
// Because protobuf Pose is in degrees, this is necessary.
type OrientationVectorDegrees struct {
	Theta float64 `json:"th"`
	OX    float64 `json:"x"`
	OY    float64 `json:"y"`
	OZ    float64 `json:"z"`
}

// NewOrientationVector Creates a zero-initialized OrientationVector.
func NewOrientationVector() *OrientationVector {
	return &OrientationVector{Theta: 0, OX: 0, OY: 0, OZ: 1}
}

// IsValid returns an error if configuration is invalid.
func (ovd *OrientationVectorDegrees) IsValid() error {
	if ovd.computeNormal() == 0.0 { // avoid division by zero
		return errors.New("OrientationVectorDegrees has a normal of 0, probably X, Y, and Z are all 0")
	}
	return nil
}

// IsValid returns an error if configuration is invalid.
func (ov *OrientationVector) IsValid() error {
	if ov.computeNormal() == 0.0 { // avoid division by zero
		return errors.New("OrientationVector has a normal of 0, probably X, Y, and Z are all 0")
	}
	return nil
}

// Degrees converts the OrientationVector to an OrientationVectorDegrees.
func (ov *OrientationVector) Degrees() *OrientationVectorDegrees {
	return &OrientationVectorDegrees{Theta: utils.RadToDeg(ov.Theta), OX: ov.OX, OY: ov.OY, OZ: ov.OZ}
}

// ToQuat converts an orientation vector to a quaternion.
func (ov *OrientationVector) ToQuat() quat.Number {
	// make sure OrientationVector is normalized first
	ov.Normalize()

	// acos(rz) ranges from 0 (north pole) to pi (south pole)
	lat := math.Acos(ov.OZ)

	// If we're pointing at the Z axis then lon is 0, theta is the OV theta
	// Euler angles are gimbal locked here but OV allows us to have smooth(er) movement
	// Since euler angles are used to represent a single orientation, but not to move between different ones, this is OK
	lon := 0.0
	theta := ov.Theta

	if 1-math.Abs(ov.OZ) > defaultAngleEpsilon {
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

func (ovd *OrientationVectorDegrees) computeNormal() float64 {
	return math.Sqrt(ovd.OX*ovd.OX + ovd.OY*ovd.OY + ovd.OZ*ovd.OZ)
}

func (ov *OrientationVector) computeNormal() float64 {
	return math.Sqrt(ov.OX*ov.OX + ov.OY*ov.OY + ov.OZ*ov.OZ)
}

// Normalize scales the x, y, and z components of an Orientation Vector to be on the unit sphere.
func (ovd *OrientationVectorDegrees) Normalize() {
	norm := ovd.computeNormal()
	if norm == 0.0 { // avoid division by zero
		panic("orientation vec has length of 0")
	}
	ovd.OX /= norm
	ovd.OY /= norm
	ovd.OZ /= norm
}

// Normalize scales the x, y, and z components of an Orientation Vector to be on the unit sphere.
func (ov *OrientationVector) Normalize() {
	norm := ov.computeNormal()
	if norm == 0.0 { // avoid division by zero
		panic("orientation vec has length of 0")
	}
	ov.OX /= norm
	ov.OY /= norm
	ov.OZ /= norm
}

// EulerAngles returns orientation in Euler angle representation.
func (ov *OrientationVector) EulerAngles() *EulerAngles {
	return QuatToEulerAngles(ov.ToQuat())
}

// Quaternion returns orientation in quaternion representation.
func (ov *OrientationVector) Quaternion() quat.Number {
	return ov.ToQuat()
}

// OrientationVectorRadians returns orientation as an orientation vector (in radians).
func (ov *OrientationVector) OrientationVectorRadians() *OrientationVector {
	return ov
}

// OrientationVectorDegrees returns orientation as an orientation vector (in degrees).
func (ov *OrientationVector) OrientationVectorDegrees() *OrientationVectorDegrees {
	return ov.Degrees()
}

// AxisAngles returns the orientation in axis angle representation.
func (ov *OrientationVector) AxisAngles() *R4AA {
	return QuatToR4AA(ov.ToQuat())
}

// RotationMatrix returns the orientation in rotation matrix representation.
func (ov *OrientationVector) RotationMatrix() *RotationMatrix {
	return QuatToRotationMatrix(ov.Quaternion())
}

// NewOrientationVectorDegrees Creates a zero-initialized OrientationVectorDegrees.
func NewOrientationVectorDegrees() *OrientationVectorDegrees {
	return &OrientationVectorDegrees{Theta: 0, OX: 0, OY: 0, OZ: 1}
}

// Radians converts a OrientationVectorDegrees to an OrientationVector.
func (ovd *OrientationVectorDegrees) Radians() *OrientationVector {
	return &OrientationVector{Theta: utils.DegToRad(ovd.Theta), OX: ovd.OX, OY: ovd.OY, OZ: ovd.OZ}
}

// ToQuat converts an orientation vector in degrees to a quaternion.
func (ovd *OrientationVectorDegrees) ToQuat() quat.Number {
	return ovd.Radians().ToQuat()
}

// EulerAngles returns orientation in Euler angle representation.
func (ovd *OrientationVectorDegrees) EulerAngles() *EulerAngles {
	return QuatToEulerAngles(ovd.ToQuat())
}

// Quaternion returns orientation in quaternion representation.
func (ovd *OrientationVectorDegrees) Quaternion() quat.Number {
	return ovd.ToQuat()
}

// OrientationVectorRadians returns orientation as an orientation vector (in radians).
func (ovd *OrientationVectorDegrees) OrientationVectorRadians() *OrientationVector {
	return ovd.Radians()
}

// OrientationVectorDegrees returns orientation as an orientation vector (in degrees).
func (ovd *OrientationVectorDegrees) OrientationVectorDegrees() *OrientationVectorDegrees {
	return ovd
}

// AxisAngles returns the orientation in axis angle representation.
func (ovd *OrientationVectorDegrees) AxisAngles() *R4AA {
	return QuatToR4AA(ovd.ToQuat())
}

// RotationMatrix returns the orientation in rotation matrix representation.
func (ovd *OrientationVectorDegrees) RotationMatrix() *RotationMatrix {
	return QuatToRotationMatrix(ovd.Quaternion())
}
