package spatialmath

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
)

const radToDeg = 180 / math.Pi

// If two angles differ by less than this amount, we consider them the same for the purpose of doing
// math around the poles of orientation.
// This needs to be very small in order to account for the small steps taken by IK. Otherwise singularities happen.
const angleEpsilon = 0.0001

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
	// Ensure we are multiplying by a unit dual quaternion
	if vecLen := 1 / quat.Abs(by.Real); vecLen != 1 {
		by.Real.Real *= vecLen
		by.Real.Imag *= vecLen
		by.Real.Jmag *= vecLen
		by.Real.Kmag *= vecLen
	}

	return dualquat.Mul(q.Number, by)
}

// MatToEuler Converts a 4x4 matrix to Euler angles.
// Euler angles are terrible, don't use them.
func MatToEuler(mat mgl64.Mat4) []float64 {
	sy := math.Sqrt(mat.At(0, 0)*mat.At(0, 0) + mat.At(1, 0)*mat.At(1, 0))
	var angles []float64
	if sy < 1e-6 { // singular
		angles = append(angles, math.Atan2(-mat.At(1, 2), mat.At(1, 1)))
		angles = append(angles, math.Atan2(-mat.At(2, 0), sy))
		angles = append(angles, 0)
	} else {
		angles = append(angles, math.Atan2(mat.At(2, 1), mat.At(2, 2)))
		angles = append(angles, math.Atan2(-mat.At(2, 0), sy))
		angles = append(angles, math.Atan2(mat.At(1, 0), mat.At(0, 0)))
	}
	for i := range angles {
		angles[i] *= radToDeg
	}
	return angles
}

// OffsetBy takes two offsets and computes the final position.
func OffsetBy(a, b *commonpb.Pose) *commonpb.Pose {
	q1 := newDualQuaternionFromProtobuf(a)
	q2 := newDualQuaternionFromProtobuf(b)
	q3 := &dualQuaternion{q1.Transformation(q2.Number)}

	return q3.ToProtobuf()
}
