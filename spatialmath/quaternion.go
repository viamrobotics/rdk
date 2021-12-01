package spatialmath

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/num/quat"
)

type quaternion quat.Number

// Quaternion returns orientation in quaternion representation
func (q *quaternion) Quaternion() quat.Number {
	return quat.Number(*q)
}

// AxisAngles returns the orientation in axis angle representation
func (q *quaternion) AxisAngles() *R4AA {
	return QuatToR4AA(q.Quaternion())
}

// OrientationVectorRadians returns orientation as an orientation vector (in radians)
func (q *quaternion) OrientationVectorRadians() *OrientationVector {
	return QuatToOV(q.Quaternion())
}

// OrientationVectorDegrees returns orientation as an orientation vector (in degrees)
func (q *quaternion) OrientationVectorDegrees() *OrientationVectorDegrees {
	return QuatToOVD(q.Quaternion())
}

// EulerAngles returns orientation in Euler angle representation
func (q *quaternion) EulerAngles() *EulerAngles {
	return QuatToEulerAngles(q.Quaternion())
}

// RotationMatrix returns the orientation in rotation matrix representation
func (q *quaternion) RotationMatrix() *RotationMatrix {
	return QuatToRotationMatrix(q.Quaternion())
}

// QuatToEulerAngles converts a quaternion to the euler angle representation. Algorithm from Wikipedia.
// https://en.wikipedia.org/wiki/Conversion_between_quaternions_and_Euler_angles#Quaternion_to_Euler_angles_conversion
func QuatToEulerAngles(q quat.Number) *EulerAngles {
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

// QuatToOVD converts a quaternion to an orientation vector in degrees
func QuatToOVD(q quat.Number) *OrientationVectorDegrees {
	ov := QuatToOV(q)
	return ov.Degrees()
}

// QuatToOV converts a quaternion to an orientation vector
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
	if 1-math.Abs(newZ.Kmag) > angleEpsilon {
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

// Used for interpolating orientations.
// Intro to lerp vs slerp: https://threadreaderapp.com/thread/1176137498323501058.html
func slerp(qN1, qN2 quat.Number, by float64) quat.Number {

	q1 := mgl64.Quat{qN1.Real, mgl64.Vec3{qN1.Imag, qN1.Jmag, qN1.Kmag}}
	q2 := mgl64.Quat{qN2.Real, mgl64.Vec3{qN2.Imag, qN2.Jmag, qN2.Kmag}}

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
