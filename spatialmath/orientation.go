package spatialmath

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/core/utils"
)

// Orientation is an interface used to express the different parameterizations of a rotation in 3D Euclidean space.
type Orientation interface {
	OrientationVector() *OrientationVec
	OrientationVectorDegrees() *OrientationVecDegrees
	AxisAngles() *R4AA
	Quaternion() quat.Number
	EulerAngles() *EulerAngles
}

// NewZeroOrientation returns an orientatation which signifies no rotation
func NewZeroOrientation() Orientation {
	return &EulerAngles{0, 0, 0}
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
func QuatToOVD(q quat.Number) *OrientationVecDegrees {
	ov := QuatToOV(q)
	return ov.Degrees()
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
		//~ fmt.Println(ov)
	}

	return ov
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
