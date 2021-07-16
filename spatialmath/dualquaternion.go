// Package spatialmath defines spatial mathematical operations
package spatialmath

import (
	"math"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/utils"

	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

const radToDeg = 180 / math.Pi
const degToRad = math.Pi / 180

// If two angles differ by less than this amount, we consider them the same for the purpose of doing
// math around the poles of orientation.
const angleEpsilon = 0.01 // radians

// DualQuaternion defines functions to perform rigid DualQuaternionformations in 3D.
type DualQuaternion struct {
	Quat dualquat.Number
}

// NewDualQuaternion returns a pointer to a new DualQuaternion object whose Quaternion is an identity Quaternion.
// Since the real part of a qual quaternion should be a unit quaternion, not all zeroes, this should be used
// instead of &DualQuaternion{}.
func NewDualQuaternion() *DualQuaternion {
	return &DualQuaternion{dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Number{},
	}}
}

// NewDualQuaternionFromRotation returns a pointer to a new DualQuaternion object whose rotation quaternion is set from a provided
// orientation vector.
func NewDualQuaternionFromRotation(ov *OrientationVec) *DualQuaternion {
	// Handle the zero case
	if ov.OX == 0 && ov.OY == 0 && ov.OZ == 0 {
		ov.OZ = 1
	}
	ov.Normalize()
	return &DualQuaternion{dualquat.Number{
		Real: ov.ToQuat(),
		Dual: quat.Number{},
	}}
}

// NewDualQuaternionFromDH returns a pointer to a new DualQuaternion object created from a DH parameter
func NewDualQuaternionFromDH(a, d, alpha float64) *DualQuaternion {
	m := mgl64.Ident4()

	m.Set(1, 1, math.Cos(alpha))
	m.Set(1, 2, -1*math.Sin(alpha))

	m.Set(2, 0, 0)
	m.Set(2, 1, math.Sin(alpha))
	m.Set(2, 2, math.Cos(alpha))

	qRot := mgl64.Mat4ToQuat(m)
	q := NewDualQuaternion()
	q.Quat.Real = quat.Number{qRot.W, qRot.X(), qRot.Y(), qRot.Z()}
	q.SetTranslation(a, 0, d)
	return q
}

// NewDualQuaternionFromArmPos returns a pointer to a new DualQuaternion object whose rotation quaternion is set from a provided
// arm position.
func NewDualQuaternionFromArmPos(pos *pb.ArmPosition) *DualQuaternion {
	q := NewDualQuaternionFromRotation(&OrientationVec{pos.Theta * degToRad, pos.OX, pos.OY, pos.OZ})
	q.SetTranslation(pos.X, pos.Y, pos.Z)
	return q
}

// ToArmPos converts a DualQuaternion to an arm position
func (q *DualQuaternion) ToArmPos() *pb.ArmPosition {
	final := &pb.ArmPosition{}
	cartQuat := dualquat.Mul(q.Quat, dualquat.Conj(q.Quat))
	final.X = cartQuat.Dual.Imag
	final.Y = cartQuat.Dual.Jmag
	final.Z = cartQuat.Dual.Kmag
	poseOV := QuatToOV(q.Quat.Real)
	final.Theta = utils.RadToDeg(poseOV.Theta)
	final.OX = poseOV.OX
	final.OY = poseOV.OY
	final.OZ = poseOV.OZ
	return final
}

// Clone returns a DualQuaternion object identical to this one.
func (q *DualQuaternion) Clone() *DualQuaternion {
	t := &DualQuaternion{}
	// No need for deep copies here, dualquats are primitives all the way down
	t.Quat = q.Quat
	return t
}

// Rotation returns the rotation quaternion.
func (q *DualQuaternion) Rotation() quat.Number {
	return q.Quat.Real
}

// Translation multiplies the dual quaternion by its own conjugate to give a dq where the real is the identity quat,
// and the dual is representative of 0.5 * real world millimeters.
func (q *DualQuaternion) Translation() dualquat.Number {
	return dualquat.Mul(q.Quat, dualquat.Conj(q.Quat))
}

// SetTranslation correctly sets the translation quaternion against the rotation.
func (q *DualQuaternion) SetTranslation(x, y, z float64) {
	q.Quat.Dual = quat.Number{0, x / 2, y / 2, z / 2}
	q.Rotate()
}

// SetX sets the x translation.
func (q *DualQuaternion) SetX(x float64) {
	q.Quat.Dual.Imag = x
}

// SetY sets the y translation.
func (q *DualQuaternion) SetY(y float64) {
	q.Quat.Dual.Jmag = y
}

// SetZ sets the z translation.
func (q *DualQuaternion) SetZ(z float64) {
	q.Quat.Dual.Kmag = z
}

// Rotate multiplies the dual part of the quaternion by the real part give the correct rotation.
func (q *DualQuaternion) Rotate() {
	q.Quat.Dual = quat.Mul(q.Quat.Dual, q.Quat.Real)
}

// ToDelta returns the difference between two DualQuaternion'.
// We use quaternion/angle axis for this because distances are well-defined.
func (q *DualQuaternion) ToDelta(other *DualQuaternion) []float64 {
	ret := make([]float64, 7)

	quatBetween := quat.Mul(other.Quat.Real, quat.Conj(q.Quat.Real))

	otherTrans := dualquat.Mul(other.Quat, dualquat.Conj(other.Quat))
	mTrans := dualquat.Mul(q.Quat, dualquat.Conj(q.Quat))
	aa := QuatToR4AA(quatBetween)
	ret[0] = otherTrans.Dual.Imag - mTrans.Dual.Imag
	ret[1] = otherTrans.Dual.Jmag - mTrans.Dual.Jmag
	ret[2] = otherTrans.Dual.Kmag - mTrans.Dual.Kmag
	ret[3] = aa.Theta
	ret[4] = aa.RX
	ret[5] = aa.RY
	ret[6] = aa.RZ
	return ret
}

// ToDeltaR3 returns the difference between two DualQuaternion' using R3 angle axis.
// We use quaternion/angle axis for this because distances are well-defined.
func (q *DualQuaternion) ToDeltaR3(other *DualQuaternion) []float64 {
	ret := make([]float64, 6)

	quatBetween := quat.Mul(other.Quat.Real, quat.Conj(q.Quat.Real))

	otherTrans := dualquat.Mul(other.Quat, dualquat.Conj(other.Quat))
	mTrans := dualquat.Mul(q.Quat, dualquat.Conj(q.Quat))
	aa := QuatToR3AA(quatBetween)
	ret[0] = otherTrans.Dual.Imag - mTrans.Dual.Imag
	ret[1] = otherTrans.Dual.Jmag - mTrans.Dual.Jmag
	ret[2] = otherTrans.Dual.Kmag - mTrans.Dual.Kmag
	ret[3] = aa.RX
	ret[4] = aa.RY
	ret[5] = aa.RZ
	return ret
}

// Transformation multiplies the dual quat contained in this DualQuaternion by another dual quat.
func (q *DualQuaternion) Transformation(by dualquat.Number) dualquat.Number {
	// Ensure we are multiplying by a unit dual quaternion
	if vecLen := quat.Abs(by.Real); vecLen != 1 {
		by.Real = quat.Scale(1/vecLen, by.Real)
	}

	return dualquat.Mul(q.Quat, by)
}

// QuatToR4AA converts a quat to an R4 axis angle in the same way the C++ Eigen library does.
// https://eigen.tuxfamily.org/dox/AngleAxis_8h_source.html
func QuatToR4AA(q quat.Number) R4AA {
	denom := Norm(q)

	angle := 2 * math.Atan2(denom, math.Abs(q.Real))
	if q.Real < 0 {
		angle *= -1
	}

	if denom < 1e-6 {
		return R4AA{angle, 1, 0, 0}
	}
	return R4AA{angle, q.Imag / denom, q.Jmag / denom, q.Kmag / denom}
}

// QuatToR3AA converts a quat to an R3 axis angle in the same way the C++ Eigen library does.
// https://eigen.tuxfamily.org/dox/AngleAxis_8h_source.html
func QuatToR3AA(q quat.Number) R3AA {
	denom := Norm(q)

	angle := 2 * math.Atan2(denom, math.Abs(q.Real))
	if q.Real < 0 {
		angle *= -1
	}

	if denom < 1e-6 {
		return R3AA{1, 0, 0}
	}
	return R3AA{angle * q.Imag / denom, angle * q.Jmag / denom, angle * q.Kmag / denom}
}

// QuatToEuler converts a rotation unit quaternion to euler angles.
// See the following wikipedia page for the formulas used here:
// https://en.wikipedia.org/wiki/Conversion_between_quaternions_and_Euler_angles#Quaternion_to_Euler_angles_conversion
// Euler angles are terrible, don't use them.
func QuatToEuler(q quat.Number) []float64 {
	w := q.Real
	x := q.Imag
	y := q.Jmag
	z := q.Kmag

	var angles []float64

	angles = append(angles, math.Atan2(2*(w*x+y*z), 1-2*(x*x+y*y)))
	angles = append(angles, math.Asin(2*(w*y-x*z)))
	angles = append(angles, math.Atan2(2*(w*z+y*x), 1-2*(y*y+z*z)))

	for i := range angles {

		angles[i] *= radToDeg
	}
	return angles
}

// QuatToOV converts a quaternion to an orientation vector
func QuatToOV(q quat.Number) *OrientationVec {
	xAxis := quat.Number{0, -1, 0, 0}
	zAxis := quat.Number{0, 0, 0, 1}
	ov := &OrientationVec{}
	// Get the transform of our +X and +Z points
	newX := quat.Mul(quat.Mul(q, xAxis), quat.Conj(q))
	newZ := quat.Mul(quat.Mul(q, zAxis), quat.Conj(q))
	ov.OX = newZ.Imag
	ov.OY = newZ.Jmag
	ov.OZ = newZ.Kmag

	// The contents of ov.newX.Kmag are not in radians but we can use angleEpsilon anyway to check how close we are to
	// the pole because it's a convenient small number
	if 1-math.Abs(newZ.Kmag) < angleEpsilon {
		// Special case for when we point directly along the Z axis
		// Get the vector normal to the local-x, global-z, origin plane
		ov.Theta = -math.Atan2(newX.Jmag, -newX.Imag)
		if newZ.Kmag < 0 {
			ov.Theta = -math.Atan2(newX.Jmag, newX.Imag)
		}

	} else {
		v1 := mgl64.Vec3{newZ.Imag, newZ.Jmag, newZ.Kmag}
		v2 := mgl64.Vec3{newX.Imag, newX.Jmag, newX.Kmag}

		// Get the vector normal to the local-x, local-z, origin plane
		norm1 := v1.Cross(v2)

		// Get the vector normal to the global-z, local-z, origin plane
		norm2 := v1.Cross(mgl64.Vec3{zAxis.Imag, zAxis.Jmag, zAxis.Kmag})

		// For theta, we find the angle between the planes defined by local-x, global-z, origin and local-x, local-z, origin
		cosTheta := norm1.Dot(norm2) / (norm1.Len() * norm2.Len())
		// Account for floating point error
		if cosTheta > 1 {
			cosTheta = 1
		}
		if cosTheta < -1 {
			cosTheta = -1
		}

		theta := math.Acos(cosTheta)
		if theta > angleEpsilon {
			// Acos will always produce a positive number, we need to determine directionality of the angle
			// We rotate newZ by -theta around the newX axis and see if we wind up coplanar with local-x, global-z, origin
			// If so theta is negative, otherwise positive
			// An R4AA is a convenient way to rotate a point by an amount around an arbitrary axis
			aa := R4AA{-theta, ov.OX, ov.OY, ov.OZ}
			q2 := aa.ToQuat()
			testZ := quat.Mul(quat.Mul(q2, zAxis), quat.Conj(q2))
			norm3 := v1.Cross(mgl64.Vec3{testZ.Imag, testZ.Jmag, testZ.Kmag})
			cosTest := norm1.Dot(norm3) / (norm1.Len() * norm3.Len())
			if 1-cosTest < angleEpsilon*angleEpsilon {
				ov.Theta = -theta
			} else {
				ov.Theta = theta
			}
		} else {
			ov.Theta = 0
		}
	}

	return ov
}

// MatToEuler Converts a 4x4 matrix to Euler angles.
// Euler angles are terrible, don't use them.
func MatToEuler(mat mgl64.Mat4) []float64 {
	sy := math.Sqrt(mat.At(0, 0)*mat.At(0, 0) + mat.At(1, 0)*mat.At(1, 0))
	singular := sy < 1e-6
	var angles []float64
	if singular {
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

// Norm returns the norm of the quaternion, i.e. the sqrt of the squares of the imaginary parts.
func Norm(q quat.Number) float64 {
	return math.Sqrt(q.Imag*q.Imag + q.Jmag*q.Jmag + q.Kmag*q.Kmag)
}

// Flip will multiply a quaternion by -1, returning a quaternion representing the same orientation but in the opposing octant.
func Flip(q quat.Number) quat.Number {
	return quat.Number{-q.Real, -q.Imag, -q.Jmag, -q.Kmag}
}
