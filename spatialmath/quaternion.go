package spatialmath

import (
	"encoding/json"
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/utils"
)

// Quaternion is an orientation in quaternion representation.
type Quaternion quat.Number

// Quaternion returns orientation in quaternion representation.
func (q *Quaternion) Quaternion() quat.Number {
	return quat.Number(*q)
}

// AxisAngles returns the orientation in axis angle representation.
func (q *Quaternion) AxisAngles() *R4AA {
	return QuatToR4AA(q.Quaternion())
}

// OrientationVectorRadians returns orientation as an orientation vector (in radians).
func (q *Quaternion) OrientationVectorRadians() *OrientationVector {
	return QuatToOV(q.Quaternion())
}

// OrientationVectorDegrees returns orientation as an orientation vector (in degrees).
func (q *Quaternion) OrientationVectorDegrees() *OrientationVectorDegrees {
	return QuatToOVD(q.Quaternion())
}

// EulerAngles returns orientation in Euler angle representation.
func (q *Quaternion) EulerAngles() *EulerAngles {
	return QuatToEulerAngles(q.Quaternion())
}

// RotationMatrix returns the orientation in rotation matrix representation.
func (q *Quaternion) RotationMatrix() *RotationMatrix {
	return QuatToRotationMatrix(q.Quaternion())
}

// QuatToEulerAngles converts a quaternion to the euler angle representation. Algorithm from Wikipedia.
// https://en.wikipedia.org/wiki/Conversion_between_quaternions_and_Euler_angles#Quaternion_to_Euler_angles_conversion
func QuatToEulerAngles(q quat.Number) *EulerAngles {
	angles := EulerAngles{}

	// yaw (z-axis rotation)
	sinyCosp := 2.0 * (q.Real*q.Kmag + q.Imag*q.Jmag)
	// if math.Abs(sinyCosp) < 1e-8 {
	// sinyCosp = 0
	// }

	cosyCosp := 1.0 - 2.0*(q.Jmag*q.Jmag+q.Kmag*q.Kmag)
	// if math.Abs(cosyCosp) < 1e-8 {
	// cosyCosp = 0
	// }

	angles.Yaw = math.Atan2(sinyCosp, cosyCosp)

	// for a pitch that is π / 2, we experience gimbal lock
	// and must calculate roll based on the real rotation and yaw
	sinp := 2.0 * (q.Real*q.Jmag - q.Kmag*q.Imag)
	if math.Abs(sinp) >= 1.0 {
		// pitch (y-axis rotation)
		angles.Pitch = math.Copysign(math.Pi/2., sinp) // use 90 degrees if out of range
		// roll (x-axis rotation)
		angles.Roll = 2.0*math.Atan2(q.Imag, q.Real) + math.Copysign(angles.Yaw, sinp)
	} else {
		// pitch (y-axis rotation)
		angles.Pitch = math.Asin(sinp)
		// roll (x-axis rotation)
		sinrCosp := 2.0 * (q.Real*q.Imag + q.Jmag*q.Kmag)
		cosrCosp := 1.0 - 2.0*(q.Imag*q.Imag+q.Jmag*q.Jmag)
		angles.Roll = math.Atan2(sinrCosp, cosrCosp)
	}
	return &angles
}

// QuatToOVD converts a quaternion to an orientation vector in degrees.
func QuatToOVD(q quat.Number) *OrientationVectorDegrees {
	ov := QuatToOV(q)
	return ov.Degrees()
}

// QuatToOV converts a quaternion to an orientation vector.
func QuatToOV(q quat.Number) *OrientationVector {
	xAxis := quat.Number{0, -1, 0, 0}
	zAxis := quat.Number{0, 0, 0, 1}
	ov := &OrientationVector{}
	// Get the transform of our +X and +Z points
	newX := quat.Mul(quat.Mul(q, xAxis), quat.Conj(q))
	newZ := quat.Mul(quat.Mul(q, zAxis), quat.Conj(q))
	ov.OX = newZ.Imag
	ov.OY = newZ.Jmag
	ov.OZ = newZ.Kmag

	// The contents of ov.newX.Kmag are not in radians but we can use angleEpsilon anyway to check how close we are to
	// the pole because it's a convenient small number
	if 1-math.Abs(newZ.Kmag) > defaultAngleEpsilon {
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
		if theta > defaultAngleEpsilon {
			// Acos will always produce a positive number, we need to determine directionality of the angle
			// We rotate newZ by -theta around the newX axis and see if we wind up coplanar with local-x, global-z, origin
			// If so theta is negative, otherwise positive
			// An R4AA is a convenient way to rotate a point by an amount around an arbitrary axis
			aa := R4AA{-theta, ov.OX, ov.OY, ov.OZ}
			q2 := aa.ToQuat()
			testZ := quat.Mul(quat.Mul(q2, zAxis), quat.Conj(q2))
			norm3 := v1.Cross(mgl64.Vec3{testZ.Imag, testZ.Jmag, testZ.Kmag})
			cosTest := norm1.Dot(norm3) / (norm1.Len() * norm3.Len())
			if 1-cosTest < defaultAngleEpsilon*defaultAngleEpsilon {
				ov.Theta = -theta
			} else {
				ov.Theta = theta
			}
		} else {
			ov.Theta = 0
		}
	} else {
		// Special case for when we point directly along the Z axis
		// Get the vector normal to the local-x, global-z, origin plane
		ov.Theta = -math.Atan2(newX.Jmag, -newX.Imag)
		if newZ.Kmag < 0 {
			ov.Theta = -math.Atan2(newX.Jmag, newX.Imag)
		}
	}
	// the IEEE 754 Standard for Floating-Points allows both negative and positive zero representations.
	// If one of the above conditions casts ov.Theta to -0, transform it to +0 for consistency.
	// String comparisons of floating point numbers may fail otherwise.
	if ov.Theta == -0. {
		ov.Theta = 0.
	}

	return ov
}

// QuatToR4AA converts a quat to an R4 axis angle in the same way the C++ Eigen library does.
// https://eigen.tuxfamily.org/dox/AngleAxis_8h_source.html
func QuatToR4AA(q quat.Number) *R4AA {
	denom := Norm(q)

	angle := 2 * math.Atan2(denom, math.Abs(q.Real))
	if q.Real < 0 {
		angle *= -1
	}

	if denom < 1e-6 {
		return &R4AA{Theta: angle, RX: 0, RY: 0, RZ: 1}
	}
	return &R4AA{angle, q.Imag / denom, q.Jmag / denom, q.Kmag / denom}
}

// QuatToR3AA converts a quat to an R3 axis angle in the same way the C++ Eigen library does.
// https://eigen.tuxfamily.org/dox/AngleAxis_8h_source.html
func QuatToR3AA(q quat.Number) r3.Vector {
	denom := Norm(q)

	angle := 2 * math.Atan2(denom, math.Abs(q.Real))
	if q.Real < 0 {
		angle *= -1
	}

	if denom < 1e-6 {
		return r3.Vector{0, 0, 0}
	}
	return r3.Vector{angle * q.Imag / denom, angle * q.Jmag / denom, angle * q.Kmag / denom}
}

// QuatToRotationMatrix converts a quat to a Rotation Matrix
// reference: https://github.com/go-gl/mathgl/blob/592312d8590acb0686c14740dcf60e2f32d9c618/mgl64/quat.go#L168
func QuatToRotationMatrix(q quat.Number) *RotationMatrix {
	w, x, y, z := q.Real, q.Imag, q.Jmag, q.Kmag
	mat := [9]float64{
		1 - 2*y*y - 2*z*z, 2*x*y + 2*w*z, 2*x*z - 2*w*y,
		2*x*y - 2*w*z, 1 - 2*x*x - 2*z*z, 2*y*z + 2*w*x,
		2*x*z + 2*w*y, 2*y*z - 2*w*x, 1 - 2*x*x - 2*y*y,
	}
	return &RotationMatrix{mat}
}

// Normalize a quaternion, returning its, versor (unit quaternion).
func Normalize(q quat.Number) quat.Number {
	length := math.Sqrt(q.Real*q.Real + q.Imag*q.Imag + q.Jmag*q.Jmag + q.Kmag*q.Kmag)
	if math.Abs(length-1.0) < 1e-10 {
		return q
	}
	if length == 0 {
		return NewZeroOrientation().Quaternion()
	}
	if length == math.Inf(1) {
		length = float64(math.MaxFloat64)
	}
	return quat.Number{q.Real / length, q.Imag / length, q.Jmag / length, q.Kmag / length}
}

// Norm returns the norm of the quaternion, i.e. the sqrt of the sum of the squares of the imaginary parts.
func Norm(q quat.Number) float64 {
	return math.Sqrt(q.Imag*q.Imag + q.Jmag*q.Jmag + q.Kmag*q.Kmag)
}

// Flip will multiply a quaternion by -1, returning a quaternion representing the same orientation but in the opposing octant.
func Flip(q quat.Number) quat.Number {
	return quat.Number{-q.Real, -q.Imag, -q.Jmag, -q.Kmag}
}

// QuaternionAlmostEqual is an equality test for all the float components of a quaternion. Quaternions have double coverage, q == -q, and
// this function will *not* account for this. Use OrientationAlmostEqual unless you're certain this is what you want.
func QuaternionAlmostEqual(a, b quat.Number, tol float64) bool {
	return utils.Float64AlmostEqual(a.Imag, b.Imag, tol) &&
		utils.Float64AlmostEqual(a.Jmag, b.Jmag, tol) &&
		utils.Float64AlmostEqual(a.Kmag, b.Kmag, tol) &&
		utils.Float64AlmostEqual(a.Real, b.Real, tol)
}

// Used for interpolating orientations.
// Intro to lerp vs slerp: https://threadreaderapp.com/thread/1176137498323501058.html
func slerp(qN1, qN2 quat.Number, by float64) quat.Number {
	q1 := mgl64.Quat{qN1.Real, mgl64.Vec3{qN1.Imag, qN1.Jmag, qN1.Kmag}}
	q2 := mgl64.Quat{qN2.Real, mgl64.Vec3{qN2.Imag, qN2.Jmag, qN2.Kmag}}

	// check we don't have a double cover issue
	// interpolating to the opposite hemisphere will produce unexpected results
	if oppositeHemisphere(qN1, qN2) {
		q2 = q2.Scale(-1)
	}

	// Use mgl64's quats because they have nlerp and slerp built in
	q1, q2 = q1.Normalize(), q2.Normalize()
	var q mgl64.Quat
	// Use nlerp for 0.5 since it's faster and equal to slerp
	if by == 0.5 {
		q = mgl64.QuatNlerp(q1, q2, by)
	} else {
		q = mgl64.QuatSlerp(q1, q2, by)
	}
	return quat.Number{q.W, q.X(), q.Y(), q.Z()}
}

// oppositeHemisphere returns true if all sings are opposite between two quats.
func oppositeHemisphere(q1, q2 quat.Number) bool {
	if math.Signbit(q1.Real) == math.Signbit(q2.Real) {
		return false
	}
	if math.Signbit(q1.Imag) == math.Signbit(q2.Imag) {
		return false
	}
	if math.Signbit(q1.Jmag) == math.Signbit(q2.Jmag) {
		return false
	}
	if math.Signbit(q1.Kmag) == math.Signbit(q2.Kmag) {
		return false
	}
	return true
}

// MarshalJSON marshals to W, X, Y, Z json.
func (q Quaternion) MarshalJSON() ([]byte, error) {
	return json.Marshal(quaternionJSONFromQuaternion(&q))
}

type quaternionJSON struct {
	W, X, Y, Z float64
}

func (oj *quaternionJSON) toQuaternion() *Quaternion {
	x := quat.Number{
		Real: oj.W,
		Imag: oj.X,
		Jmag: oj.Y,
		Kmag: oj.Z,
	}
	x = Normalize(x)
	return &Quaternion{
		Real: x.Real,
		Imag: x.Imag,
		Jmag: x.Jmag,
		Kmag: x.Kmag,
	}
}

func quaternionJSONFromQuaternion(q *Quaternion) quaternionJSON {
	return quaternionJSON{
		W: q.Real,
		X: q.Imag,
		Y: q.Jmag,
		Z: q.Kmag,
	}
}
