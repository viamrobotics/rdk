package spatialmath

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

// dualQuaternion defines functions to perform rigid dualQuaternionformations in 3D.
// If you find yourself importing gonum.org/v1/gonum/num/dualquat in some other package, you should probably be
// using these instead.
type dualQuaternion struct {
	dualquat.Number
}

// newDualQuaternion returns a pointer to a new dualQuaternion object whose Quaternion is an identity Quaternion.
// Since the real part of a qual quaternion should be a unit quaternion, not all zeroes, this should be used
// instead of &dualQuaternion{}.
func newDualQuaternion() *dualQuaternion {
	return &dualQuaternion{dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Number{},
	}}
}

// newDualQuaternionFromRotation returns a pointer to a new dualQuaternion object whose rotation
// quaternion is set from a provided orientation.
func newDualQuaternionFromRotation(o Orientation) *dualQuaternion {
	ov := o.OrientationVectorRadians()
	// Handle the zero case
	if ov.OX == 0 && ov.OY == 0 && ov.OZ == 0 {
		ov.OZ = 1
	}
	ov.Normalize()
	return &dualQuaternion{dualquat.Number{
		Real: ov.ToQuat(),
		Dual: quat.Number{},
	}}
}

// newDualQuaternionFromDH returns a pointer to a new dualQuaternion object created from a DH parameter.
func newDualQuaternionFromDH(a, d, alpha float64) *dualQuaternion {
	m := mgl64.Ident4()

	m.Set(1, 1, math.Cos(alpha))
	m.Set(1, 2, -1*math.Sin(alpha))

	m.Set(2, 0, 0)
	m.Set(2, 1, math.Sin(alpha))
	m.Set(2, 2, math.Cos(alpha))

	qRot := mgl64.Mat4ToQuat(m)
	q := newDualQuaternion()
	q.Real = quat.Number{qRot.W, qRot.X(), qRot.Y(), qRot.Z()}
	q.SetTranslation(r3.Vector{a, 0, d})
	return q
}

// newDualQuaternionFromProtobuf returns a pointer to a new dualQuaternion object whose rotation quaternion is set from a provided
// protobuf pose.
func newDualQuaternionFromProtobuf(pos *commonpb.Pose) *dualQuaternion {
	q := newDualQuaternionFromRotation(&OrientationVectorDegrees{pos.Theta, pos.OX, pos.OY, pos.OZ})
	q.SetTranslation(r3.Vector{pos.X, pos.Y, pos.Z})
	return q
}

// newDualQuaternionFromPose takes any pose, checks if it is already a DQ and returns that if so, otherwise creates a
// new one.
func newDualQuaternionFromPose(p Pose) *dualQuaternion {
	if q, ok := p.(*dualQuaternion); ok {
		return q.Clone()
	}
	q := newDualQuaternionFromRotation(p.Orientation())
	q.SetTranslation(p.Point())
	return q
}

// newDualQuaternionFromPose takes any pose, checks if it is already a DQ and returns that if so, otherwise creates a
// new one.
func dualQuaternionFromPose(p Pose) *dualQuaternion {
	if q, ok := p.(*dualQuaternion); ok {
		return q
	}
	q := newDualQuaternionFromRotation(p.Orientation().OrientationVectorRadians())
	q.SetTranslation(p.Point())
	return q
}

// ToProtobuf converts a dualQuaternion to a protobuf pose.
func (q *dualQuaternion) ToProtobuf() *commonpb.Pose {
	final := &commonpb.Pose{}
	cartQuat := dualquat.Mul(q.Number, dualquat.Conj(q.Number))
	final.X = cartQuat.Dual.Imag
	final.Y = cartQuat.Dual.Jmag
	final.Z = cartQuat.Dual.Kmag
	poseOVD := QuatToOVD(q.Real)
	final.Theta = poseOVD.Theta
	final.OX = poseOVD.OX
	final.OY = poseOVD.OY
	final.OZ = poseOVD.OZ
	return final
}

// Clone returns a dualQuaternion object identical to this one.
func (q *dualQuaternion) Clone() *dualQuaternion {
	// No need for deep copies here, a dualquat.Number is primitives all the way down
	return &dualQuaternion{q.Number}
}

// Point multiplies the dual quaternion by its own conjugate to give a dq where the real is the identity quat,
// and the dual is representative of real world millimeters. We then return the XYZ point on its own.
// We intentionally do not return the resulting dual quaternion, because we do not want to mix dq's representing
// transformations and ones representing pure points.
func (q *dualQuaternion) Point() r3.Vector {
	tQuat := dualquat.Mul(q.Number, dualquat.Conj(q.Number)).Dual
	return r3.Vector{tQuat.Imag, tQuat.Jmag, tQuat.Kmag}
}

// Orientation returns the rotation quaternion as an Orientation.
func (q *dualQuaternion) Orientation() Orientation {
	return (*Quaternion)(&q.Real)
}

// SetTranslation correctly sets the translation quaternion against the rotation.
func (q *dualQuaternion) SetTranslation(pt r3.Vector) {
	q.Dual = quat.Number{0, pt.X / 2, pt.Y / 2, pt.Z / 2}
	q.rotate()
}

// rotate multiplies the dual part of the quaternion by the real part give the correct rotation.
func (q *dualQuaternion) rotate() {
	q.Dual = quat.Mul(q.Dual, q.Real)
}

// Invert returns a dualQuaternion representing the opposite transformation. So if the input q would transform a -> b,
// then Invert(p) will transform b -> a.
func (q *dualQuaternion) Invert() Pose {
	return &dualQuaternion{dualquat.ConjQuat(q.Number)}
}

// SetZ sets the z translation.
func (q *dualQuaternion) SetZ(z float64) {
	q.Dual.Kmag = z
}

// Transformation multiplies the dual quat contained in this dualQuaternion by another dual quat.
func (q *dualQuaternion) Transformation(by dualquat.Number) dualquat.Number {
	var newReal quat.Number
	var newDual quat.Number
	if vecLen := 1 / quat.Abs(by.Real); vecLen != 1 {
		by.Real.Real *= vecLen
		by.Real.Imag *= vecLen
		by.Real.Jmag *= vecLen
		by.Real.Kmag *= vecLen
	}
	//nolint: gocritic
	if q.Real.Real == 1 {
		// Since we're working with unit quaternions, if either Real is 1, then that quat is an identity quat
		newReal = by.Real
	} else if by.Real.Real == 1 {
		newReal = q.Real
	} else {
		// Ensure we are multiplying by a unit dual quaternion
		newReal = quat.Mul(q.Real, by.Real)
	}

	//nolint: gocritic
	if q.Dual.Real == 0 && q.Dual.Imag == 0 && q.Dual.Jmag == 0 && q.Dual.Kmag == 0 {
		newDual = quat.Mul(q.Real, by.Dual)
	} else if by.Dual.Real == 0 && by.Dual.Imag == 0 && by.Dual.Jmag == 0 && by.Dual.Kmag == 0 {
		newDual = quat.Mul(q.Dual, by.Real)
	} else {
		newDual = quat.Add(quat.Mul(q.Real, by.Dual), quat.Mul(q.Dual, by.Real))
	}
	return dualquat.Number{
		Real: newReal,
		Dual: newDual,
	}
}

// OffsetBy takes two offsets and computes the final position.
func OffsetBy(a, b *commonpb.Pose) *commonpb.Pose {
	q1 := newDualQuaternionFromProtobuf(a)
	q2 := newDualQuaternionFromProtobuf(b)
	q3 := &dualQuaternion{q1.Transformation(q2.Number)}

	return q3.ToProtobuf()
}
