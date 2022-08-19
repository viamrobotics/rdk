package spatialmath

import (
	"math"
	"testing"

	"go.viam.com/test"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/utils"
)

// represent a 45 degree rotation around the x axis in all the representations.
var (
	th = math.Pi / 4.
	// in quaternion representation.
	q45x = quat.Number{math.Cos(th / 2.), math.Sin(th / 2.), 0, 0}
	// in axis-angle representation.
	aa45x = &R4AA{th, 1., 0., 0.}
	// in euler angle representation.
	ea45x = &EulerAngles{Roll: th, Pitch: 0, Yaw: 0}
	// in orientation vector representation.
	ov45x  = &OrientationVector{2. * th, 0., -math.Sqrt(2) / 2., math.Sqrt(2) / 2.}
	ovd45x = &OrientationVectorDegrees{2 * utils.RadToDeg(th), 0., -math.Sqrt(2) / 2, math.Sqrt(2) / 2}
	// in rotation matrix representation.
	rm45x = &RotationMatrix{[9]float64{1, 0, 0, 0, math.Cos(th), math.Sin(th), 0, -math.Sin(th), math.Cos(th)}}
)

func TestZeroOrientation(t *testing.T) {
	zero := NewZeroOrientation()
	test.That(t, zero.OrientationVectorRadians(), test.ShouldResemble, NewOrientationVector())
	test.That(t, zero.OrientationVectorDegrees(), test.ShouldResemble, NewOrientationVectorDegrees())
	test.That(t, zero.AxisAngles(), test.ShouldResemble, NewR4AA())
	test.That(t, zero.Quaternion(), test.ShouldResemble, quat.Number{1, 0, 0, 0})
	test.That(t, zero.EulerAngles(), test.ShouldResemble, NewEulerAngles())
	test.That(t, zero.RotationMatrix(), test.ShouldResemble, &RotationMatrix{[9]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}})
}

func TestQuaternions(t *testing.T) {
	qq45x := Quaternion(q45x)
	testCompatibility(t, &qq45x)
}

func TestEulerAngles(t *testing.T) {
	testCompatibility(t, ea45x)
}

func TestAxisAngles(t *testing.T) {
	testCompatibility(t, aa45x)
}

func TestOrientationVector(t *testing.T) {
	testCompatibility(t, ov45x)
}

func TestOrientationVectorDegrees(t *testing.T) {
	testCompatibility(t, ovd45x)
}

func TestRotationMatrix(t *testing.T) {
	testCompatibility(t, rm45x)
}

func TestSlerp(t *testing.T) {
	q1 := q45x
	q2 := quat.Conj(q45x)
	s1 := slerp(q1, q2, 0.25)
	s2 := slerp(q1, q2, 0.5)

	expect1 := quat.Number{0.9808, 0.1951, 0, 0}
	expect2 := quat.Number{1, 0, 0, 0}

	test.That(t, s1.Real, test.ShouldAlmostEqual, expect1.Real, 0.001)
	test.That(t, s1.Imag, test.ShouldAlmostEqual, expect1.Imag, 0.001)
	test.That(t, s1.Jmag, test.ShouldAlmostEqual, expect1.Jmag, 0.001)
	test.That(t, s1.Kmag, test.ShouldAlmostEqual, expect1.Kmag, 0.001)
	test.That(t, s2.Real, test.ShouldAlmostEqual, expect2.Real)
	test.That(t, s2.Imag, test.ShouldAlmostEqual, expect2.Imag)
	test.That(t, s2.Jmag, test.ShouldAlmostEqual, expect2.Jmag)
	test.That(t, s2.Kmag, test.ShouldAlmostEqual, expect2.Kmag)
}

func TestOrientationTransform(t *testing.T) {
	aa := &R4AA{Theta: math.Pi / 2., RX: 0., RY: 1., RZ: 0.}
	ovd := &OrientationVectorDegrees{Theta: 0.0, OX: 1., OY: 0.0, OZ: 0.0}
	ovdResult := aa.OrientationVectorDegrees()
	aaResult := ovd.AxisAngles()
	t.Logf("result as orientation vector: Theta: %.2f, X: %.2f, Y: %.2f, Z: %.2f", ovdResult.Theta, ovdResult.OX, ovdResult.OY, ovdResult.OZ)
	test.That(t, ovdResult.Theta, test.ShouldAlmostEqual, ovd.Theta)
	test.That(t, ovdResult.OX, test.ShouldAlmostEqual, ovd.OX)
	test.That(t, ovdResult.OY, test.ShouldAlmostEqual, ovd.OY)
	test.That(t, ovdResult.OZ, test.ShouldAlmostEqual, ovd.OZ)
	t.Logf("result as axis angle: Theta: %.2f, X: %.2f, Y: %.2f, Z: %.2f", aaResult.Theta, aaResult.RX, aaResult.RY, aaResult.RZ)
	test.That(t, aaResult.Theta, test.ShouldAlmostEqual, aa.Theta)
	test.That(t, aaResult.RX, test.ShouldAlmostEqual, aa.RX)
	test.That(t, aaResult.RY, test.ShouldAlmostEqual, aa.RY)
	test.That(t, aaResult.RZ, test.ShouldAlmostEqual, aa.RZ)
}

func TestOrientationAlmostEqual(t *testing.T) {
	test.That(t, OrientationAlmostEqual(aa45x, ea45x), test.ShouldBeTrue)
	test.That(t, OrientationAlmostEqual(aa45x, NewZeroOrientation()), test.ShouldBeFalse)
}

func TestOrientationBetween(t *testing.T) {
	aa := &R4AA{Theta: math.Pi / 2., RX: 0., RY: 1., RZ: 0.}
	btw := OrientationBetween(aa, ov45x).OrientationVectorDegrees()
	result := &OrientationVectorDegrees{Theta: 135.0, OX: -1., OY: 0.0, OZ: 0.0}
	test.That(t, result.Theta, test.ShouldAlmostEqual, btw.Theta)
	test.That(t, result.OX, test.ShouldAlmostEqual, btw.OX)
	test.That(t, result.OY, test.ShouldAlmostEqual, btw.OY)
	test.That(t, result.OZ, test.ShouldAlmostEqual, btw.OZ)
}

func TestOrientationInverse(t *testing.T) {
	test.That(t, OrientationAlmostEqual(OrientationInverse(aa45x), &R4AA{-th, 1., 0., 0.}), test.ShouldBeTrue)
}

func testCompatibility(t *testing.T, o Orientation) {
	t.Helper()

	// Orientation Vectors
	test.That(t, o.OrientationVectorRadians().Theta, test.ShouldAlmostEqual, ov45x.Theta)
	test.That(t, o.OrientationVectorRadians().OX, test.ShouldAlmostEqual, ov45x.OX)
	test.That(t, o.OrientationVectorRadians().OY, test.ShouldAlmostEqual, ov45x.OY)
	test.That(t, o.OrientationVectorRadians().OZ, test.ShouldAlmostEqual, ov45x.OZ)
	test.That(t, o.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, ovd45x.Theta)
	test.That(t, o.OrientationVectorDegrees().OX, test.ShouldAlmostEqual, ovd45x.OX)
	test.That(t, o.OrientationVectorDegrees().OY, test.ShouldAlmostEqual, ovd45x.OY)
	test.That(t, o.OrientationVectorDegrees().OZ, test.ShouldAlmostEqual, ovd45x.OZ)

	// Quaternion
	test.That(t, o.Quaternion().Real, test.ShouldAlmostEqual, q45x.Real)
	test.That(t, o.Quaternion().Imag, test.ShouldAlmostEqual, q45x.Imag)
	test.That(t, o.Quaternion().Jmag, test.ShouldAlmostEqual, q45x.Jmag)
	test.That(t, o.Quaternion().Kmag, test.ShouldAlmostEqual, q45x.Kmag)

	// Axis angles
	test.That(t, o.AxisAngles().Theta, test.ShouldAlmostEqual, aa45x.Theta)
	test.That(t, o.AxisAngles().RX, test.ShouldAlmostEqual, aa45x.RX)
	test.That(t, o.AxisAngles().RY, test.ShouldAlmostEqual, aa45x.RY)
	test.That(t, o.AxisAngles().RZ, test.ShouldAlmostEqual, aa45x.RZ)

	// Euler angles
	test.That(t, o.EulerAngles().Roll, test.ShouldAlmostEqual, ea45x.Roll)
	test.That(t, o.EulerAngles().Pitch, test.ShouldAlmostEqual, ea45x.Pitch)
	test.That(t, o.EulerAngles().Yaw, test.ShouldAlmostEqual, ea45x.Yaw)

	// Rotation matrices
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			test.That(t, o.RotationMatrix().At(i, j), test.ShouldAlmostEqual, rm45x.At(i, j))
		}
	}
}
