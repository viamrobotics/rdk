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
	return &dualQuaternion{dualquat.Number{
		Real: o.Quaternion(),
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
		return &dualQuaternion{q.Number}
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

	//nolint: gocritic
	if q.Real.Real == 1 {
		// Since we're working with unit quaternions, if either Real is 1, then that quat is an identity quat
		newReal = by.Real
	} else if by.Real.Real == 1 {
		newReal = q.Real
	} else {
		newReal = quat.Mul(q.Real, by.Real)
	}
	// Multiplication is faster than division. Thus if this is hit, it is faster to divide once and multiply four times.
	// However, if this is not hit, it may be a wash. The branch predictor won't save us as the following lines rely on newReal.
	if vecLen := 1 / quat.Abs(newReal); vecLen-1 > 1e-10 || vecLen-1 < -1e-10 {
		newReal.Real *= vecLen
		newReal.Imag *= vecLen
		newReal.Jmag *= vecLen
		newReal.Kmag *= vecLen
	}

	if q.Dual.Real == 0 && q.Dual.Imag == 0 && q.Dual.Jmag == 0 && q.Dual.Kmag == 0 {
		return dualquat.Number{
			Real: newReal,
			Dual: quat.Mul(q.Real, by.Dual),
		}
	} else if by.Dual.Real == 0 && by.Dual.Imag == 0 && by.Dual.Jmag == 0 && by.Dual.Kmag == 0 {
		return dualquat.Number{
			Real: newReal,
			Dual: quat.Mul(q.Dual, by.Real),
		}
	}

	return dualquat.Number{
		Real: newReal,
		// Equivalent to but faster than quat.Add(quat.Mul(q.Real, by.Dual), quat.Mul(q.Dual, by.Real))
		Dual: quat.Number{
			Real: q.Real.Real*by.Dual.Real - q.Real.Imag*by.Dual.Imag - q.Real.Jmag*by.Dual.Jmag - q.Real.Kmag*by.Dual.Kmag +
				q.Dual.Real*by.Real.Real - q.Dual.Imag*by.Real.Imag - q.Dual.Jmag*by.Real.Jmag - q.Dual.Kmag*by.Real.Kmag,
			Imag: q.Real.Real*by.Dual.Imag + q.Real.Imag*by.Dual.Real + q.Real.Jmag*by.Dual.Kmag - q.Real.Kmag*by.Dual.Jmag +
				q.Dual.Real*by.Real.Imag + q.Dual.Imag*by.Real.Real + q.Dual.Jmag*by.Real.Kmag - q.Dual.Kmag*by.Real.Jmag,
			Jmag: q.Real.Real*by.Dual.Jmag - q.Real.Imag*by.Dual.Kmag + q.Real.Jmag*by.Dual.Real + q.Real.Kmag*by.Dual.Imag +
				q.Dual.Real*by.Real.Jmag - q.Dual.Imag*by.Real.Kmag + q.Dual.Jmag*by.Real.Real + q.Dual.Kmag*by.Real.Imag,
			Kmag: q.Real.Real*by.Dual.Kmag + q.Real.Imag*by.Dual.Jmag - q.Real.Jmag*by.Dual.Imag + q.Real.Kmag*by.Dual.Real +
				q.Dual.Real*by.Real.Kmag + q.Dual.Imag*by.Real.Jmag - q.Dual.Jmag*by.Real.Imag + q.Dual.Kmag*by.Real.Real,
		},
	}
}

// OffsetBy takes two offsets and computes the final position.
func OffsetBy(a, b *commonpb.Pose) *commonpb.Pose {
	q1 := newDualQuaternionFromProtobuf(a)
	q2 := newDualQuaternionFromProtobuf(b)
	q3 := &dualQuaternion{q1.Transformation(q2.Number)}

	return q3.ToProtobuf()
}
