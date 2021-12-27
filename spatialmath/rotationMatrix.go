package spatialmath

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/quat"
)

// RotationMatrix is a 3x3 matrix in row major order.
// m[3*r + c] is the element in the r'th row and c'th column.
type RotationMatrix struct {
	mat [9]float64
}

// NewRotationMatrix creates the rotation matrix from a slice of floats.
func NewRotationMatrix(m []float64) (*RotationMatrix, error) {
	if len(m) != 9 {
		return nil, fmt.Errorf("input slice has %d elements, need exactly 9", len(m))
	}
	mat := [9]float64{m[0], m[1], m[2], m[3], m[4], m[5], m[6], m[7], m[8]}
	return &RotationMatrix{mat}, nil
}

// OrientationVectorRadians returns orientation as an orientation vector (in radians).
func (rm *RotationMatrix) OrientationVectorRadians() *OrientationVector {
	return QuatToOV(rm.Quaternion())
}

// OrientationVectorDegrees returns orientation as an orientation vector (in degrees).
func (rm *RotationMatrix) OrientationVectorDegrees() *OrientationVectorDegrees {
	return QuatToOVD(rm.Quaternion())
}

// AxisAngles returns the orientation in axis angle representation.
func (rm *RotationMatrix) AxisAngles() *R4AA {
	return QuatToR4AA(rm.Quaternion())
}

// Quaternion returns orientation in quaternion representation.
// reference: http://www.euclideanspace.com/maths/geometry/rotations/conversions/matrixToQuaternion/index.htm
func (rm *RotationMatrix) Quaternion() quat.Number {
	var q quat.Number
	m := rm.mat

	// converting to quaternion form involves taking the square root of the trace and depending on the way the rows/cols
	// are ordered the trace has the potential to be nonpositive. The below case structure will work on all of the valid
	// rotation matrices
	tr := m[0] + m[4] + m[8]
	switch {
	case tr > 0:
		s := 0.5 / math.Sqrt(tr+1.0)
		q = quat.Number{0.25 / s, (m[5] - m[7]) * s, (m[6] - m[2]) * s, (m[1] - m[3]) * s}
	case (m[0] > m[4]) && (m[0] > m[8]):
		s := 2.0 * math.Sqrt(1.0+m[0]-m[4]-m[8])
		q = quat.Number{(m[5] - m[7]) / s, 0.25 * s, (m[3] + m[1]) / s, (m[6] + m[2]) / s}
	case m[4] > m[8]:
		s := 2.0 * math.Sqrt(1.0+m[4]-m[0]-m[8])
		q = quat.Number{(m[6] - m[2]) / s, (m[3] + m[1]) / s, 0.25 * s, (m[7] + m[5]) / s}
	default:
		s := 2.0 * math.Sqrt(1.0+m[8]-m[0]-m[4])
		q = quat.Number{(m[1] - m[3]) / s, (m[6] + m[2]) / s, (m[7] + m[5]) / s, 0.25 * s}
	}
	return Normalize(q)
}

// EulerAngles returns orientation in Euler angle representation.
func (rm *RotationMatrix) EulerAngles() *EulerAngles {
	return QuatToEulerAngles(rm.Quaternion())
}

// RotationMatrix returns the orientation in rotation matrix representation.
func (rm *RotationMatrix) RotationMatrix() *RotationMatrix {
	return rm
}

// Row returns the a 3 element vector corresponding to the specified row.
func (rm *RotationMatrix) Row(row int) r3.Vector {
	return r3.Vector{rm.mat[3*row], rm.mat[3*row+1], rm.mat[3*row+2]}
}

// Col returns the a 3 element vector corresponding to the specified col.
func (rm *RotationMatrix) Col(col int) r3.Vector {
	return r3.Vector{rm.mat[col], rm.mat[3+col], rm.mat[6+col]}
}

// At returns the float corresponding to the element at the specified location.
func (rm *RotationMatrix) At(row, col int) float64 {
	return rm.mat[3*row+col]
}
