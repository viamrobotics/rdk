package kinmath

import (
	"math"
	//~ "fmt"

	"github.com/go-gl/mathgl/mgl64"
)

// Defines the rotational Matrix to perform rigid transforMations in 3d
type Transform struct {
	Mat mgl64.Mat4
}

// Return a pointer to a new Transform object whose Matrix is an identity Matrix
func NewTransform() *Transform {
	return &Transform{mgl64.Ident4()}
}

// Return a pointer to a new Transform object whose Matrix has been xyz rotated by the specified number of degrees
func NewTransformFromRotation(x, y, z float64) *Transform {
	return &Transform{mgl64.HomogRotate3DZ(z * math.Pi / 180).Mul4(
		mgl64.HomogRotate3DY(y * math.Pi / 180).Mul4(
			mgl64.HomogRotate3DX(x * math.Pi / 180))),
	}
}

func (m *Transform) Clone() *Transform {
	t := &Transform{}
	t.Mat = mgl64.Mat4FromCols(m.Mat.Cols())
	return t
}

func (m *Transform) Matrix() mgl64.Mat4 {
	return m.Mat
}

func (m *Transform) At(r, c int) float64 {
	return m.Mat.At(r, c)
}

// Linear and Rotation are the same thing
// Both return the top left 3x3 Matrix
func (m *Transform) Linear() mgl64.Mat3 {
	return m.Mat.Mat3()
}
func (m *Transform) Rotation() mgl64.Mat3 {
	return m.Linear()
}

// Get the XYZ translation parameters
func (m *Transform) Translation() mgl64.Vec3 {
	return m.Mat.Col(3).Vec3()
}

// Set a whole new rotation Matrix
func (m *Transform) SetMatrix(Mat mgl64.Mat4) {
	m.Mat = Mat
}

// Set X translation
func (m *Transform) SetX(x float64) {
	m.Mat.Set(0, 3, x)
}

// Set Y translation
func (m *Transform) SetY(y float64) {
	m.Mat.Set(1, 3, y)
}

// Set Z translation
func (m *Transform) SetZ(z float64) {
	m.Mat.Set(2, 3, z)
}

// Set X rotation. Takes degrees.
func (m *Transform) RotX(x float64) {
	m.Mat = m.Mat.Mul4(mgl64.HomogRotate3DX(x * math.Pi / 180))
}

// Set Y rotation. Takes degrees.
func (m *Transform) RotY(y float64) {
	m.Mat = m.Mat.Mul4(mgl64.HomogRotate3DY(y * math.Pi / 180))
}

// Set Z rotation. Takes degrees.
func (m *Transform) RotZ(z float64) {
	m.Mat = m.Mat.Mul4(mgl64.HomogRotate3DZ(z * math.Pi / 180))
}

// ToDelta returns the difference between two transforms
func (m *Transform) ToDelta(other *Transform) []float64 {
	ret := make([]float64, 6)
	ret[0] = other.At(0, 3) - m.At(0, 3)
	ret[1] = other.At(1, 3) - m.At(1, 3)
	ret[2] = other.At(2, 3) - m.At(2, 3)

	quat := mgl64.Mat4ToQuat(other.Linear().Mul3(m.Linear().Transpose()).Mat4())
	axisAngle := QuatToAxisAngle(quat)
	ret[3] = axisAngle[1] * axisAngle[0]
	ret[4] = axisAngle[2] * axisAngle[0]
	ret[5] = axisAngle[3] * axisAngle[0]
	return ret
}

// This converts a quat to an axis angle in the same way the C++ Eigen library does
// https://eigen.tuxfamily.org/dox/AngleAxis_8h_source.html
func QuatToAxisAngle(quat mgl64.Quat) []float64 {
	denom := quat.Norm()

	angle := 2 * math.Atan2(denom, math.Abs(quat.W))
	if quat.W < 0 {
		angle *= -1
	}

	axisAngle := []float64{angle}

	if denom < 1e-6 {
		axisAngle = append(axisAngle, 1, 0, 0)
	} else {
		x, y, z := quat.V.Mul(1 / denom).Elem()
		axisAngle = append(axisAngle, x, y, z)
	}
	return axisAngle
}
