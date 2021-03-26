package kinmath

import (
	//~ "math"
	//~ "fmt"
	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

// Defines the rotational Matrix to perform rigid QuatTransations in 3d
type QuatTrans struct {
	Quat dualquat.Number
}

// Return a pointer to a new QuatTrans object whose Matrix is an identity Matrix
func NewQuatTrans() *QuatTrans {
	return &QuatTrans{dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Number{},
	}}
}

// Return a pointer to a new QuatTrans object whose Matrix has been xyz rotated by the specified number of degrees
func NewQuatTransFromRotation(x, y, z float64) *QuatTrans {
	mQuat := mgl64.AnglesToQuat(x,y,z, mgl64.ZYX)
	return &QuatTrans{dualquat.Number{
		Real: quat.Number{mQuat.W, mQuat.X(), mQuat.Y(), mQuat.Z()},
		Dual: quat.Number{},
	}}
}

func (m *QuatTrans) Clone() *QuatTrans {
	t := &QuatTrans{}
	// No need for deep copies here
	t.Quat = m.Quat
	return t
}

//~ func (m *QuatTrans) Matrix() mgl64.Mat4 {
	//~ return m.Mat
//~ }

func (m *QuatTrans) Quaternion() dualquat.Number {
	return m.Quat
}

//~ func (m *QuatTrans) at(r, c int) float64 {
	//~ return m.Mat.At(r, c)
//~ }

// Linear and Rotation are the same thing
// Both return the top left 3x3 Matrix
//~ func (m *QuatTrans) Linear() mgl64.Mat3 {
	//~ return m.Mat.Mat3()
//~ }
func (m *QuatTrans) Rotation() quat.Number {
	return m.Quat.Real
}

// Get the XYZ translation parameters
func (m *QuatTrans) Translation() quat.Number {
	return m.Quat.Dual
}

// Set a whole new rotation Matrix
func (m *QuatTrans) SetQuat(Quat dualquat.Number) {
	m.Quat = Quat
}

// Set X translation
func (m *QuatTrans) SetX(x float64) {
	m.Quat.Dual.Imag = x
}

// Set Y translation
func (m *QuatTrans) SetY(y float64) {
	m.Quat.Dual.Jmag = y
}

// Set Z translation
func (m *QuatTrans) SetZ(z float64) {
	m.Quat.Dual.Kmag = z
}

// Set X rotation. Takes degrees.
//~ func (m *QuatTrans) RotX(x float64) {
	//~ m.Mat = m.Mat.Mul4(mgl64.HomogRotate3DX(x * math.Pi / 180))
//~ }

//~ // Set Y rotation. Takes degrees.
//~ func (m *QuatTrans) RotY(y float64) {
	//~ m.Mat = m.Mat.Mul4(mgl64.HomogRotate3DY(y * math.Pi / 180))
//~ }

//~ // Set Z rotation. Takes degrees.
//~ func (m *QuatTrans) RotZ(z float64) {
	//~ m.Mat = m.Mat.Mul4(mgl64.HomogRotate3DZ(z * math.Pi / 180))
//~ }

// ToDelta returns the difference between two QuatTranss
// We use quaternion/angle axis for this because distances are well-defined
func (m *QuatTrans) ToDelta(other *QuatTrans) []float64 {
	ret := make([]float64, 8)
	
	//~ otherTrans := dualquat.Mul(other.Quat, dualquat.Conj(other.Quat))
	//~ mTrans := dualquat.Mul(m.Quat, dualquat.Conj(m.Quat))
	
	//~ fmt.Println("other real", otherTrans.Real)
	//~ fmt.Println("m real", mTrans.Real)
	
	//~ ret[0] = otherTrans.Dual.Imag - mTrans.Dual.Imag
	//~ ret[1] = otherTrans.Dual.Jmag - mTrans.Dual.Jmag
	//~ ret[2] = otherTrans.Dual.Kmag - mTrans.Dual.Kmag

	//~ ret[3] = 100 * (other.Quat.Real.Imag * other.Quat.Real.Real) - (m.Quat.Real.Imag * m.Quat.Real.Real)
	//~ ret[4] = 100 * (other.Quat.Real.Jmag * other.Quat.Real.Real) - (m.Quat.Real.Jmag * m.Quat.Real.Real)
	//~ ret[5] = 100 * (other.Quat.Real.Kmag * other.Quat.Real.Real) - (m.Quat.Real.Kmag * m.Quat.Real.Real)
	
	ret[0] = other.Quat.Real.Real - m.Quat.Real.Real
	ret[1] = other.Quat.Real.Imag - m.Quat.Real.Imag
	ret[2] = other.Quat.Real.Jmag - m.Quat.Real.Jmag
	ret[3] = other.Quat.Real.Kmag - m.Quat.Real.Kmag
	ret[4] = other.Quat.Dual.Real - m.Quat.Dual.Real
	ret[5] = other.Quat.Dual.Imag - m.Quat.Dual.Imag
	ret[6] = other.Quat.Dual.Jmag - m.Quat.Dual.Jmag
	ret[7] = other.Quat.Dual.Kmag - m.Quat.Dual.Kmag
	
	return ret
}

// This converts a quat to an axis angle in the same way the C++ Eigen library does
// https://eigen.tuxfamily.org/dox/AngleAxis_8h_source.html
//~ func QuatToAxisAngle(quat mgl64.Quat) []float64 {
	//~ denom := quat.Norm()

	//~ angle := 2 * math.Atan2(denom, math.Abs(quat.W))
	//~ if quat.W < 0 {
		//~ angle *= -1
	//~ }

	//~ axisAngle := []float64{angle}

	//~ if denom < 1e-6 {
		//~ axisAngle = append(axisAngle, 1, 0, 0)
	//~ } else {
		//~ x, y, z := quat.V.Mul(1 / denom).Elem()
		//~ axisAngle = append(axisAngle, x, y, z)
	//~ }
	//~ return axisAngle
//~ }

func (m *QuatTrans) Transformation(by dualquat.Number) dualquat.Number {
	if len := quat.Abs(by.Real); len != 1 {
		by.Real = quat.Scale(1/len, by.Real)
	}
	
	return dualquat.Mul(m.Quat, by)
}
