// Package kinmath defines mathematical operations useful in kinematics.
package kinmath

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

// QuatTrans defines functions to perform rigid QuatTransformations in 3D.
type QuatTrans struct {
	Quat dualquat.Number
}

// NewQuatTrans returns a pointer to a new QuatTrans object whose Quaternion is an identity Quaternion.
func NewQuatTrans() *QuatTrans {
	return &QuatTrans{dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Number{},
	}}
}

// NewQuatTransFromRotation returns a pointer to a new QuatTrans object whose Quaternion has been xyz rotated by the specified number of degrees
// Prefer not to use this as it's not terribly well defined- gimbal lock will cause weirdness the further Y gets from 0
func NewQuatTransFromRotation(x, y, z float64) *QuatTrans {
	mQuat := mgl64.AnglesToQuat(x, y, z, mgl64.ZYX)
	return &QuatTrans{dualquat.Number{
		Real: quat.Number{mQuat.W, mQuat.X(), mQuat.Y(), mQuat.Z()},
		Dual: quat.Number{},
	}}
}

// Clone TODO
func (m *QuatTrans) Clone() *QuatTrans {
	t := &QuatTrans{}
	// No need for deep copies here, dualquats are primitives all the way down
	t.Quat = m.Quat
	return t
}

// Quaternion TODO
func (m *QuatTrans) Quaternion() dualquat.Number {
	return m.Quat
}

// Rotation returns the rotation quaternion.
func (m *QuatTrans) Rotation() quat.Number {
	return m.Quat.Real
}

// Translation returns the translation quaternion.
func (m *QuatTrans) Translation() quat.Number {
	return m.Quat.Dual
}

// SetX sets the x translation.
func (m *QuatTrans) SetX(x float64) {
	m.Quat.Dual.Imag = x
}

// SetY sets the y translation.
func (m *QuatTrans) SetY(y float64) {
	m.Quat.Dual.Jmag = y
}

// SetZ sets the z translation.
func (m *QuatTrans) SetZ(z float64) {
	m.Quat.Dual.Kmag = z
}

// Rotate multiplies the dual part of the quaternion by the real part give the correct rotation.
func (m *QuatTrans) Rotate() {
	m.Quat.Dual = quat.Mul(m.Quat.Real, m.Quat.Dual)
}

// ToDelta returns the difference between two QuatTrans'.
// We use quaternion/angle axis for this because distances are well-defined.
func (m *QuatTrans) ToDelta(other *QuatTrans) []float64 {
	ret := make([]float64, 6)

	// q and -q are the same rotation, so flip rotation quaternions to the positive hemisphere
	m.Quat.Real.Real = math.Abs(m.Quat.Real.Real)
	other.Quat.Real.Real = math.Abs(other.Quat.Real.Real)

	quatBetween := quat.Mul(other.Quat.Real, quat.Conj(m.Quat.Real))

	otherTrans := dualquat.Scale(2, other.Quat)
	mTrans := dualquat.Mul(m.Quat, dualquat.Conj(m.Quat))

	ret[0] = otherTrans.Dual.Imag - mTrans.Dual.Imag
	ret[1] = otherTrans.Dual.Jmag - mTrans.Dual.Jmag
	ret[2] = otherTrans.Dual.Kmag - mTrans.Dual.Kmag

	axisAngle := QuatToAxisAngle(quatBetween)
	ret[3] = axisAngle[1] * axisAngle[0]
	ret[4] = axisAngle[2] * axisAngle[0]
	ret[5] = axisAngle[3] * axisAngle[0]

	return ret
}

// QuatToAxisAngle converts a quat to an axis angle in the same way the C++ Eigen library does
// https://eigen.tuxfamily.org/dox/AngleAxis_8h_source.html
func QuatToAxisAngle(q quat.Number) []float64 {
	denom := Norm(q)

	angle := 2 * math.Atan2(denom, math.Abs(q.Real))
	if q.Real < 0 {
		angle *= -1
	}

	axisAngle := []float64{angle}

	if denom < 1e-6 {
		axisAngle = append(axisAngle, 1, 0, 0)
	} else {
		axisAngle = append(axisAngle, q.Imag/denom)
		axisAngle = append(axisAngle, q.Jmag/denom)
		axisAngle = append(axisAngle, q.Kmag/denom)
	}
	return axisAngle
}

// Transformation TODO
func (m *QuatTrans) Transformation(by dualquat.Number) dualquat.Number {
	if len := quat.Abs(by.Real); len != 1 {
		by.Real = quat.Scale(1/len, by.Real)
	}

	return dualquat.Mul(m.Quat, by)
}

// Norm TODO
func Norm(q quat.Number) float64 {
	return math.Sqrt(q.Real*q.Real + q.Imag*q.Imag + q.Jmag*q.Jmag + q.Kmag*q.Kmag)
}
