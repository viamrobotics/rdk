package spatialmath

import (
	"fmt"
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

var defaultPrecision = 6

// DualQuaternion defines functions to perform rigid dualQuaternionformations in 3D.
// If you find yourself importing gonum.org/v1/gonum/num/dualquat in some other package, you should probably be
// using these instead.
type DualQuaternion struct {
	dualquat.Number
}

// newDualQuaternion returns a pointer to a new dualQuaternion object whose Quaternion is an identity Quaternion.
// Since the real part of a qual quaternion should be a unit quaternion, not all zeroes, this should be used
// instead of &dualQuaternion{}.
func newDualQuaternion() *DualQuaternion {
	return &DualQuaternion{dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Number{},
	}}
}

// newDualQuaternionFromRotation returns a pointer to a new dualQuaternion object whose rotation
// quaternion is set from a provided orientation.
func newDualQuaternionFromRotation(o Orientation) *DualQuaternion {
	return &DualQuaternion{dualquat.Number{
		Real: o.Quaternion(),
		Dual: quat.Number{},
	}}
}

// newDualQuaternionFromDH returns a pointer to a new dualQuaternion object created from a DH parameter.
func newDualQuaternionFromDH(a, d, alpha float64) *DualQuaternion {
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

// newDualQuaternionFromProtobuf returns a pointer to a new dualQuaternion object whose rotation
// quaternion is set from a provided protobuf pose.
func newDualQuaternionFromProtobuf(pos *commonpb.Pose) *DualQuaternion {
	q := newDualQuaternionFromRotation(&OrientationVectorDegrees{pos.Theta, pos.OX, pos.OY, pos.OZ})
	q.SetTranslation(r3.Vector{pos.X, pos.Y, pos.Z})
	return q
}

// newDualQuaternionFromPose takes any pose, checks if it is already a DQ and returns that if so,
// otherwise creates a new one.
func newDualQuaternionFromPose(p Pose) *DualQuaternion {
	if q, ok := p.(*DualQuaternion); ok {
		return &DualQuaternion{q.Number}
	}
	q := newDualQuaternionFromRotation(p.Orientation())
	q.SetTranslation(p.Point())
	return q
}

// DualQuaternionFromPose takes any pose, checks if it is already a DQ and returns that if so,
// otherwise creates a new one.
//
// Dan: What's the difference between this and the above? It's not clear that
// `OrientationVectorRadians` is meaningful.
func DualQuaternionFromPose(p Pose) *DualQuaternion {
	if q, ok := p.(*DualQuaternion); ok {
		return q
	}
	q := newDualQuaternionFromRotation(p.Orientation().OrientationVectorRadians())
	q.SetTranslation(p.Point())
	return q
}

// ToProtobuf converts a dualQuaternion to a protobuf pose.
func (q *DualQuaternion) ToProtobuf() *commonpb.Pose {
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
func (q *DualQuaternion) Point() r3.Vector {
	tQuat := dualquat.Mul(q.Number, dualquat.Conj(q.Number)).Dual
	return r3.Vector{tQuat.Imag, tQuat.Jmag, tQuat.Kmag}
}

// Orientation returns the rotation quaternion as an Orientation.
func (q *DualQuaternion) Orientation() Orientation {
	return (*Quaternion)(&q.Real)
}

// SetTranslation correctly sets the translation quaternion against the rotation.
func (q *DualQuaternion) SetTranslation(pt r3.Vector) {
	q.Dual = quat.Number{0, pt.X / 2, pt.Y / 2, pt.Z / 2}
	q.rotate()
}

// rotate multiplies the dual part of the quaternion by the real part give the correct rotation.
func (q *DualQuaternion) rotate() {
	q.Dual = quat.Mul(q.Dual, q.Real)
}

// Invert returns a dualQuaternion representing the opposite transformation. So if the input q would transform a -> b,
// then Invert(p) will transform b -> a.
func (q *DualQuaternion) Invert() Pose {
	return &DualQuaternion{dualquat.ConjQuat(q.Number)}
}

// SetZ sets the z translation.
func (q *DualQuaternion) SetZ(z float64) {
	q.Dual.Kmag = z
}

// Transformation multiplies the dual quat contained in this dualQuaternion by another dual quat.
func (q *DualQuaternion) Transformation(by dualquat.Number) dualquat.Number {
	var newReal quat.Number

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

// Format implements fmt.Formatter to allow for finer grain control over printing.
// dualquat.Number also has this implemented so in order to get custom printing we need to also implement this as opposed to simply
// implementing fmt.Stringer.
// Some example verbs and how they would display a zero pose are shown below
//
//	Verb    | String
//	%v      | {X:0 Y:0 Z:0 OX:0 OY:0 OZ:1 Theta:0°}
//	%s      | {X:0 Y:0 Z:0 OX:0 OY:0 OZ:1 Theta:0°}
//	%.3v    | {X:0.000 Y:0.000 Z:0.000 OX:0.000 OY:0.000 OZ:1.000 Theta:0.000°}
//	%#v     | r3.Vector{0 0 0} *spatialmath.OrientationVectorDegrees{0 0 0 1}
//	%g      | ((1+0i+0j+0k)+(+0+0i+0j+0k)ϵ)
func (q *DualQuaternion) Format(fs fmt.State, verb rune) {
	prec, ok := fs.Precision()
	if !ok {
		prec = defaultPrecision
	}
	width, _ := fs.Width()
	format := fmt.Sprintf("%%%d.%df", width, prec)
	pt := q.Point()
	o := q.Orientation().OrientationVectorDegrees()
	switch verb {
	case 'v':
		if fs.Flag('#') {
			//nolint:errcheck
			fmt.Fprintf(fs, "%T"+format+" %T"+format, pt, pt, o, *o)
			return
		}
		fallthrough
	case 's':
		format = fmt.Sprintf("{X:%[1]s Y:%[1]s Z:%[1]s OX:%[1]s OY:%[1]s OZ:%[1]s Theta:%[1]s°}", format)
		//nolint:errcheck
		fmt.Fprintf(fs, format, pt.X, pt.Y, pt.Z, o.OX, o.OY, o.OZ, o.Theta)
	default:
		q.Number.Format(fs, verb)
	}
}

// OffsetBy takes two offsets and computes the final position.
func OffsetBy(a, b *commonpb.Pose) *commonpb.Pose {
	q1 := newDualQuaternionFromProtobuf(a)
	q2 := newDualQuaternionFromProtobuf(b)
	q3 := &DualQuaternion{q1.Transformation(q2.Number)}

	return q3.ToProtobuf()
}

// Hash returns a hash value for this dual quaternion.
func (q *DualQuaternion) Hash() int {
	return HashPose(q)
}
