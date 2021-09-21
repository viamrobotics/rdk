package spatialmath

import (
	"math"

	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/core/utils"
)

// Orientation is an interface used to express the different parameterizations of a rotation in 3D Euclidean space.
type Orientation interface {
	OV() *OrientationVec
	OVD() *OrientationVecDegrees
	AxisAngles() *R4AA
	Quaternion() quat.Number
	EulerAngles() *EulerAngles
}

// use quaternions as the private implementation of the Orientation interface
type quaternion quat.Number

// NewZeroOrientation returns an orientatation which signifies no rotation
func NewZeroOrientation() Orientation {
	return &quaternion{1, 0, 0, 0}
}

// NewOrientationFromAxisAngles turns an input axis-angle representation into a general Orientation object
func NewOrientationFromAxisAngles(aa *R4AA) Orientation {
	q := quaternion(aa.ToQuat())
	return &q
}

// NewOrientationFromQuaternion turns an input quaternion into a general Orientation object
func NewOrientationFromQuaternion(q quat.Number) Orientation {
	qq := quaternion(q)
	return &qq
}

// NewOrientationFromOV turns an input orientation vector into a general Orientation object
func NewOrientationFromOV(ov *OrientationVec) Orientation {
	q := quaternion(ov.ToQuat())
	return &q
}

// NewOrientationFromOVD turns an input orientation vector using degrees into a general Orientation object
func NewOrientationFromOVD(ovd *OrientationVecDegrees) Orientation {
	ov := &OrientationVec{Theta: utils.DegToRad(ovd.Theta), OX: ovd.OX, OY: ovd.OY, OZ: ovd.OZ}
	return NewOrientationFromOV(ov)
}

// NewOrientationFromEulerAngles turns an input set of euler angles and outputs a general Orientation object. Algorithm from Wikipedia.
//https://en.wikipedia.org/wiki/Conversion_between_quaternions_and_Euler_angles#Quaternion_to_Euler_angles_conversion
func NewOrientationFromEulerAngles(ea *EulerAngles) Orientation {
	// Abbreviations for the various angular functions
	cy := math.Cos(ea.Yaw * 0.5)
	sy := math.Sin(ea.Yaw * 0.5)
	cp := math.Cos(ea.Pitch * 0.5)
	sp := math.Sin(ea.Pitch * 0.5)
	cr := math.Cos(ea.Roll * 0.5)
	sr := math.Sin(ea.Roll * 0.5)

	q := quaternion{}
	q.Real = cr*cp*cy + sr*sp*sy
	q.Imag = sr*cp*cy - cr*sp*sy
	q.Jmag = cr*sp*cy + sr*cp*sy
	q.Kmag = cr*cp*sy - sr*sp*cy

	return &q
}

// Fulfill the interface methods using the private quaternion type

// OV return the orientation vector representation of the orientation
func (q *quaternion) OV() *OrientationVec {
	return QuatToOV(quat.Number(*q))
}

// OVD return the orientation vector representation (using degrees) of the orientation
func (q *quaternion) OVD() *OrientationVecDegrees {
	ov := QuatToOV(quat.Number(*q))
	return &OrientationVecDegrees{Theta: utils.RadToDeg(ov.Theta), OX: ov.OX, OY: ov.OY, OZ: ov.OZ}
}

// AxisAngle returns the axis angle representation of the orientation
func (q *quaternion) AxisAngles() *R4AA {
	aa := QuatToR4AA(quat.Number(*q))
	return &aa
}

// Quaternion returns the quaternion representation of the orientation
func (q *quaternion) Quaternion() quat.Number {
	return quat.Number(*q)
}

// EulerAngles returns the euler angle representation of the orientation. Algorithm from Wikipedia.
// https://en.wikipedia.org/wiki/Conversion_between_quaternions_and_Euler_angles#Quaternion_to_Euler_angles_conversion
func (q *quaternion) EulerAngles() *EulerAngles {
	angles := EulerAngles{}

	// roll (x-axis rotation)
	sinrCosp := 2 * (q.Real*q.Imag + q.Jmag*q.Kmag)
	cosrCosp := 1 - 2*(q.Imag*q.Imag+q.Jmag*q.Jmag)
	angles.Roll = math.Atan2(sinrCosp, cosrCosp)

	// pitch (y-axis rotation)
	sinp := 2 * (q.Real*q.Jmag - q.Kmag*q.Imag)
	if math.Abs(sinp) >= 1 {
		angles.Pitch = math.Copysign(math.Pi/2., sinp) // use 90 degrees if out of range
	} else {
		angles.Pitch = math.Asin(sinp)
	}

	// yaw (z-axis rotation)
	sinyCosp := 2 * (q.Real*q.Kmag + q.Imag*q.Jmag)
	cosyCosp := 1 - 2*(q.Jmag*q.Jmag+q.Kmag*q.Kmag)
	angles.Yaw = math.Atan2(sinyCosp, cosyCosp)

	return &angles
}
