// Package kinmath defines mathematical operations useful in kinematics.
package kinmath

import (
	"math"

	pb "go.viam.com/core/proto/api/v1"

	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

const radToDeg = 180 / math.Pi
const degToRad = math.Pi / 180

// If two angles differ by less than this amount, we consider them the same
const angleEpsilon = 1e-6

// QuatTrans defines functions to perform rigid QuatTransformations in 3D.
type QuatTrans struct {
	Quat dualquat.Number
}

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

// NewQuatTrans returns a pointer to a new QuatTrans object whose Quaternion is an identity Quaternion.
func NewQuatTrans() *QuatTrans {
	return &QuatTrans{dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Number{},
	}}
}

// NewQuatTransFromRotation returns a pointer to a new QuatTrans object whose rotation quaternion is set from a provided
// orientation vector.
func NewQuatTransFromRotation(ov *OrientationVec) *QuatTrans {
	// Handle the zero case
	if ov.OX == 0 && ov.OY == 0 && ov.OZ == 0 {
		ov.OX = 1
	}
	NormalizeOV(ov)
	return &QuatTrans{dualquat.Number{
		Real: OVToQuat(ov),
		Dual: quat.Number{},
	}}
}

// NewQuatTransFromArmPos returns a pointer to a new QuatTrans object whose rotation quaternion is set from a provided
// arm position.
func NewQuatTransFromArmPos(pos *pb.ArmPosition) *QuatTrans {
	q := NewQuatTransFromRotation(&OrientationVec{pos.Theta * degToRad, pos.OX, pos.OY, pos.OZ})
	q.SetTranslation(pos.X, pos.Y, pos.Z)
	return q
}

// Clone returns a QuatTrans object identical to this one.
func (m *QuatTrans) Clone() *QuatTrans {
	t := &QuatTrans{}
	// No need for deep copies here, dualquats are primitives all the way down
	t.Quat = m.Quat
	return t
}

// Rotation returns the rotation quaternion.
func (m *QuatTrans) Rotation() quat.Number {
	return m.Quat.Real
}

// Translation multiplies the dual quaternion by its own conjugate to give a dq where the real is the identity quat,
// and the dual is representative of 0.5 * real world millimeters.
func (m *QuatTrans) Translation() dualquat.Number {
	return dualquat.Mul(m.Quat, dualquat.Conj(m.Quat))
}

// SetTranslation correctly sets the translation quaternion against the rotation.
func (m *QuatTrans) SetTranslation(x, y, z float64) {
	m.Quat.Dual = quat.Number{0, x / 2, y / 2, z / 2}
	m.Rotate()
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
	m.Quat.Dual = quat.Mul(m.Quat.Dual, m.Quat.Real)
}

// ToDelta returns the difference between two QuatTrans'.
// We use quaternion/angle axis for this because distances are well-defined.
func (m *QuatTrans) ToDelta(other *QuatTrans) []float64 {
	ret := make([]float64, 7)

	quatBetween := quat.Mul(other.Quat.Real, quat.Conj(m.Quat.Real))

	otherTrans := dualquat.Mul(other.Quat, dualquat.Conj(other.Quat))
	mTrans := dualquat.Mul(m.Quat, dualquat.Conj(m.Quat))
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

// ToDeltaR3 returns the difference between two QuatTrans' using R3 angle axis.
// We use quaternion/angle axis for this because distances are well-defined.
func (m *QuatTrans) ToDeltaR3(other *QuatTrans) []float64 {
	ret := make([]float64, 6)

	quatBetween := quat.Mul(other.Quat.Real, quat.Conj(m.Quat.Real))

	otherTrans := dualquat.Mul(other.Quat, dualquat.Conj(other.Quat))
	mTrans := dualquat.Mul(m.Quat, dualquat.Conj(m.Quat))
	aa := QuatToR3AA(quatBetween)
	ret[0] = otherTrans.Dual.Imag - mTrans.Dual.Imag
	ret[1] = otherTrans.Dual.Jmag - mTrans.Dual.Jmag
	ret[2] = otherTrans.Dual.Kmag - mTrans.Dual.Kmag
	ret[3] = aa.RX
	ret[4] = aa.RY
	ret[5] = aa.RZ
	return ret
}

// Transformation multiplies the dual quat contained in this QuatTrans by another dual quat.
func (m *QuatTrans) Transformation(by dualquat.Number) dualquat.Number {
	// Ensure we are multiplying by a unit dual quaternion
	if vecLen := quat.Abs(by.Real); vecLen != 1 {
		by.Real = quat.Scale(1/vecLen, by.Real)
	}

	return dualquat.Mul(m.Quat, by)
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
	xAxis := quat.Number{0, 1, 0, 0}
	zAxis := quat.Number{0, 0, 0, 1}
	ov := &OrientationVec{}
	// Get the xyz point of our axis on the unit sphere
	xyz := quat.Mul(quat.Mul(q, xAxis), quat.Conj(q))
	newZ := quat.Mul(quat.Mul(q, zAxis), quat.Conj(q))
	ov.OX = xyz.Imag
	ov.OY = xyz.Jmag
	ov.OZ = xyz.Kmag

	if math.Abs(xyz.Kmag) == 1 {
		// Special case for when we point directly along the Z axis
		// Get the vector normal to the local-x, global-z, origin plane
		ov.Theta = math.Atan2(-newZ.Jmag, -newZ.Imag)
	} else {
		v1 := mgl64.Vec3{xyz.Imag, xyz.Jmag, xyz.Kmag}
		v2 := mgl64.Vec3{zAxis.Imag, zAxis.Jmag, zAxis.Kmag}
		norm1 := v1.Cross(v2)

		// Get the vector normal to the local-x, local-z, origin plane
		norm2 := v1.Cross(mgl64.Vec3{newZ.Imag, newZ.Jmag, newZ.Kmag})

		// For theta, we find the angle between the plane defined by local-x, global-z, origin and local-x, local-z, origin
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
			// We rotate newZ by -theta around the xyz axis and see if we wind up coplanar with local-x, global-z, origin
			// If so theta is positive, otherwise negative
			// An R4AA is a convenient way to rotate a point by an amount around an arbitrary axis
			aa := R4AA{-theta, xyz.Imag, xyz.Jmag, xyz.Kmag}
			q2 := aa.ToQuat()
			testZ := quat.Mul(quat.Mul(q2, newZ), quat.Conj(q2))
			norm3 := v1.Cross(mgl64.Vec3{testZ.Imag, testZ.Jmag, testZ.Kmag})
			cosTest := norm1.Dot(norm3) / (norm1.Len() * norm3.Len())
			if 1-cosTest < angleEpsilon {
				ov.Theta = theta
			} else {
				ov.Theta = -theta
			}
		} else {
			ov.Theta = 0
		}
	}

	return ov
}

// OVToQuat converts an orientation vector to a quaternion.
func OVToQuat(ov *OrientationVec) quat.Number {
	q := quat.Number{}
	// acos(rz) ranges from 0 (north pole) to pi (south pole)
	lat := -math.Pi/2 + math.Acos(ov.OZ)

	// If we're pointing at the Z axis then lon can be 0
	lon := 0.0
	if ov.OX == -1 {
		lon = math.Pi
	} else if ov.OZ != 1 && ov.OZ != -1 {
		// atan x/y removes some sign information so we use atan2 to do it properly
		lon = math.Atan2(ov.OY, ov.OX)
	}

	q1 := mgl64.AnglesToQuat(lon, lat, ov.Theta, mgl64.ZYX)
	q.Real = q1.W
	q.Imag = q1.X()
	q.Jmag = q1.Y()
	q.Kmag = q1.Z()
	return q
}

// NormalizeOV scales the x, y, and z components of an Orientation Vector to be on the unit sphere
func NormalizeOV(ov *OrientationVec) {
	norm := math.Sqrt(ov.OX*ov.OX + ov.OY*ov.OY + ov.OZ*ov.OZ)
	ov.OX /= norm
	ov.OY /= norm
	ov.OZ /= norm
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
