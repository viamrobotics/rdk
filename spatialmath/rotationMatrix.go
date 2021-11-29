package spatialmath

import (
	"math"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/quat"
)

// RotationMatrix is a 3x3 matrix in row major order.
// m[3*r + c] is the element in the r'th row and c'th column.
type RotationMatrix [9]float64

// NewRotationMatrix creates the identity rotation matrix representing zero rotation
func NewRotationMatrix() *RotationMatrix {
	return &RotationMatrix{
		1, 0, 0,
		0, 1, 0,
		0, 0, 1,
	}
}

// OrientationVectorRadians returns orientation as an orientation vector (in radians)
func (rm *RotationMatrix) OrientationVectorRadians() *OrientationVector {
	return QuatToOV(rm.Quaternion())
}

// OrientationVectorDegrees returns orientation as an orientation vector (in degrees)
func (rm *RotationMatrix) OrientationVectorDegrees() *OrientationVectorDegrees {
	return QuatToOVD(rm.Quaternion())
}

// AxisAngles returns the orientation in axis angle representation
func (rm *RotationMatrix) AxisAngles() *R4AA {
	return QuatToR4AA(rm.Quaternion())
}

// Quaternion returns orientation in quaternion representation
// reference: http://www.euclideanspace.com/maths/geometry/rotations/conversions/matrixToQuaternion/index.htm
func (rm *RotationMatrix) Quaternion() quat.Number {
	if tr := rm[0] + rm[4] + rm[8]; tr > 0 {
		s := 0.5 / math.Sqrt(tr+1.0)
		return quat.Number{0.25 / s, (rm[5] - rm[7]) * s, (rm[6] - rm[2]) * s, (rm[1] - rm[3]) * s}
	}

	if (rm[0] > rm[4]) && (rm[0] > rm[8]) {
		s := 2.0 * math.Sqrt(1.0+rm[0]-rm[4]-rm[8])
		return quat.Number{(rm[5] - rm[7]) / s, 0.25 * s, (rm[3] + rm[1]) / s, (rm[6] + rm[2]) / s}
	}

	if rm[4] > rm[8] {
		s := 2.0 * math.Sqrt(1.0+rm[4]-rm[0]-rm[8])
		return quat.Number{(rm[6] - rm[2]) / s, (rm[3] + rm[1]) / s, 0.25 * s, (rm[7] + rm[5]) / s}
	}

	s := 2.0 * math.Sqrt(1.0+rm[8]-rm[0]-rm[4])
	return quat.Number{(rm[1] - rm[3]) / s, (rm[6] + rm[2]) / s, (rm[7] + rm[5]) / s, 0.25 * s}
}

// EulerAngles returns orientation in Euler angle representation
func (rm *RotationMatrix) EulerAngles() *EulerAngles {
	return QuatToEulerAngles(rm.Quaternion())
}

// RotationMatrix returns the orientation in rotation matrix representation
func (rm *RotationMatrix) RotationMatrix() *RotationMatrix {
	return rm
}

// Row returns the a 3 element vector corresponding to the specified row
func (rm *RotationMatrix) Row(row int) r3.Vector {
	return r3.Vector{rm[3*row], rm[3*row+1], rm[3*row+2]}
}
