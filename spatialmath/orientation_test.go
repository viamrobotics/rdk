package spatialmath

import (
	"math"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/utils"

	"gonum.org/v1/gonum/num/quat"
)

// represent a 45 degree rotation around the x axis in all the representations
var (
	th     = math.Pi / 4.
	q45x   = quat.Number{math.Cos(th / 2.), math.Sin(th / 2.), 0, 0}                // in quaternion representation
	aa45x  = &R4AA{th, 1., 0., 0.}                                                  // in axis-angle representation
	ea45x  = &EulerAngles{Roll: th, Pitch: 0, Yaw: 0}                               // in euler angle representation
	ov45x  = &OrientationVector{2. * th, 0., -math.Sqrt(2) / 2., math.Sqrt(2) / 2.} // in orientation vector representation
	ovd45x = &OrientationVectorDegrees{2 * utils.RadToDeg(th), 0., -math.Sqrt(2) / 2, math.Sqrt(2) / 2}
)

func TestZeroOrientation(t *testing.T) {
	zero := NewZeroOrientation()
	test.That(t, zero.OrientationVectorRadians(), test.ShouldResemble, NewOrientationVector())
	test.That(t, zero.OrientationVectorDegrees(), test.ShouldResemble, NewOrientationVectorDegrees())
	test.That(t, zero.AxisAngles(), test.ShouldResemble, NewR4AA())
	test.That(t, zero.Quaternion(), test.ShouldResemble, quat.Number{1, 0, 0, 0})
	test.That(t, zero.EulerAngles(), test.ShouldResemble, NewEulerAngles())
}

func TestQuaternions(t *testing.T) {
	qq45x := quaternion(q45x)
	test.That(t, qq45x.OrientationVectorRadians().Theta, test.ShouldAlmostEqual, ov45x.Theta)
	test.That(t, qq45x.OrientationVectorRadians().OX, test.ShouldAlmostEqual, ov45x.OX)
	test.That(t, qq45x.OrientationVectorRadians().OY, test.ShouldAlmostEqual, ov45x.OY)
	test.That(t, qq45x.OrientationVectorRadians().OZ, test.ShouldAlmostEqual, ov45x.OZ)
	test.That(t, qq45x.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, ovd45x.Theta)
	test.That(t, qq45x.OrientationVectorDegrees().OX, test.ShouldAlmostEqual, ovd45x.OX)
	test.That(t, qq45x.OrientationVectorDegrees().OY, test.ShouldAlmostEqual, ovd45x.OY)
	test.That(t, qq45x.OrientationVectorDegrees().OZ, test.ShouldAlmostEqual, ovd45x.OZ)
	test.That(t, qq45x.Quaternion().Real, test.ShouldAlmostEqual, q45x.Real)
	test.That(t, qq45x.Quaternion().Imag, test.ShouldAlmostEqual, q45x.Imag)
	test.That(t, qq45x.Quaternion().Jmag, test.ShouldAlmostEqual, q45x.Jmag)
	test.That(t, qq45x.Quaternion().Kmag, test.ShouldAlmostEqual, q45x.Kmag)
	test.That(t, qq45x.AxisAngles().Theta, test.ShouldAlmostEqual, aa45x.Theta)
	test.That(t, qq45x.AxisAngles().RX, test.ShouldAlmostEqual, aa45x.RX)
	test.That(t, qq45x.AxisAngles().RY, test.ShouldAlmostEqual, aa45x.RY)
	test.That(t, qq45x.AxisAngles().RZ, test.ShouldAlmostEqual, aa45x.RZ)
	test.That(t, qq45x.EulerAngles().Roll, test.ShouldAlmostEqual, ea45x.Roll)
	test.That(t, qq45x.EulerAngles().Pitch, test.ShouldAlmostEqual, ea45x.Pitch)
	test.That(t, qq45x.EulerAngles().Yaw, test.ShouldAlmostEqual, ea45x.Yaw)
}

func TestEulerAngles(t *testing.T) {
	test.That(t, ea45x.OrientationVectorRadians().Theta, test.ShouldAlmostEqual, ov45x.Theta)
	test.That(t, ea45x.OrientationVectorRadians().OX, test.ShouldAlmostEqual, ov45x.OX)
	test.That(t, ea45x.OrientationVectorRadians().OY, test.ShouldAlmostEqual, ov45x.OY)
	test.That(t, ea45x.OrientationVectorRadians().OZ, test.ShouldAlmostEqual, ov45x.OZ)
	test.That(t, ea45x.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, ovd45x.Theta)
	test.That(t, ea45x.OrientationVectorDegrees().OX, test.ShouldAlmostEqual, ovd45x.OX)
	test.That(t, ea45x.OrientationVectorDegrees().OY, test.ShouldAlmostEqual, ovd45x.OY)
	test.That(t, ea45x.OrientationVectorDegrees().OZ, test.ShouldAlmostEqual, ovd45x.OZ)
	test.That(t, ea45x.Quaternion().Real, test.ShouldAlmostEqual, q45x.Real)
	test.That(t, ea45x.Quaternion().Imag, test.ShouldAlmostEqual, q45x.Imag)
	test.That(t, ea45x.Quaternion().Jmag, test.ShouldAlmostEqual, q45x.Jmag)
	test.That(t, ea45x.Quaternion().Kmag, test.ShouldAlmostEqual, q45x.Kmag)
	test.That(t, ea45x.AxisAngles().Theta, test.ShouldAlmostEqual, aa45x.Theta)
	test.That(t, ea45x.AxisAngles().RX, test.ShouldAlmostEqual, aa45x.RX)
	test.That(t, ea45x.AxisAngles().RY, test.ShouldAlmostEqual, aa45x.RY)
	test.That(t, ea45x.AxisAngles().RZ, test.ShouldAlmostEqual, aa45x.RZ)
	test.That(t, ea45x.EulerAngles().Roll, test.ShouldAlmostEqual, ea45x.Roll)
	test.That(t, ea45x.EulerAngles().Pitch, test.ShouldAlmostEqual, ea45x.Pitch)
	test.That(t, ea45x.EulerAngles().Yaw, test.ShouldAlmostEqual, ea45x.Yaw)
}

func TestAxisAngles(t *testing.T) {
	test.That(t, aa45x.OrientationVectorRadians().Theta, test.ShouldAlmostEqual, ov45x.Theta)
	test.That(t, aa45x.OrientationVectorRadians().OX, test.ShouldAlmostEqual, ov45x.OX)
	test.That(t, aa45x.OrientationVectorRadians().OY, test.ShouldAlmostEqual, ov45x.OY)
	test.That(t, aa45x.OrientationVectorRadians().OZ, test.ShouldAlmostEqual, ov45x.OZ)
	test.That(t, aa45x.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, ovd45x.Theta)
	test.That(t, aa45x.OrientationVectorDegrees().OX, test.ShouldAlmostEqual, ovd45x.OX)
	test.That(t, aa45x.OrientationVectorDegrees().OY, test.ShouldAlmostEqual, ovd45x.OY)
	test.That(t, aa45x.OrientationVectorDegrees().OZ, test.ShouldAlmostEqual, ovd45x.OZ)
	test.That(t, aa45x.Quaternion().Real, test.ShouldAlmostEqual, q45x.Real)
	test.That(t, aa45x.Quaternion().Imag, test.ShouldAlmostEqual, q45x.Imag)
	test.That(t, aa45x.Quaternion().Jmag, test.ShouldAlmostEqual, q45x.Jmag)
	test.That(t, aa45x.Quaternion().Kmag, test.ShouldAlmostEqual, q45x.Kmag)
	test.That(t, aa45x.AxisAngles().Theta, test.ShouldAlmostEqual, aa45x.Theta)
	test.That(t, aa45x.AxisAngles().RX, test.ShouldAlmostEqual, aa45x.RX)
	test.That(t, aa45x.AxisAngles().RY, test.ShouldAlmostEqual, aa45x.RY)
	test.That(t, aa45x.AxisAngles().RZ, test.ShouldAlmostEqual, aa45x.RZ)
	test.That(t, aa45x.EulerAngles().Roll, test.ShouldAlmostEqual, ea45x.Roll)
	test.That(t, aa45x.EulerAngles().Pitch, test.ShouldAlmostEqual, ea45x.Pitch)
	test.That(t, aa45x.EulerAngles().Yaw, test.ShouldAlmostEqual, ea45x.Yaw)
}

func TestOrientationVector(t *testing.T) {
	test.That(t, ov45x.OrientationVectorRadians().Theta, test.ShouldAlmostEqual, ov45x.Theta)
	test.That(t, ov45x.OrientationVectorRadians().OX, test.ShouldAlmostEqual, ov45x.OX)
	test.That(t, ov45x.OrientationVectorRadians().OY, test.ShouldAlmostEqual, ov45x.OY)
	test.That(t, ov45x.OrientationVectorRadians().OZ, test.ShouldAlmostEqual, ov45x.OZ)
	test.That(t, ov45x.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, ovd45x.Theta)
	test.That(t, ov45x.OrientationVectorDegrees().OX, test.ShouldAlmostEqual, ovd45x.OX)
	test.That(t, ov45x.OrientationVectorDegrees().OY, test.ShouldAlmostEqual, ovd45x.OY)
	test.That(t, ov45x.OrientationVectorDegrees().OZ, test.ShouldAlmostEqual, ovd45x.OZ)
	test.That(t, ov45x.Quaternion().Real, test.ShouldAlmostEqual, q45x.Real)
	test.That(t, ov45x.Quaternion().Imag, test.ShouldAlmostEqual, q45x.Imag)
	test.That(t, ov45x.Quaternion().Jmag, test.ShouldAlmostEqual, q45x.Jmag)
	test.That(t, ov45x.Quaternion().Kmag, test.ShouldAlmostEqual, q45x.Kmag)
	test.That(t, ov45x.AxisAngles().Theta, test.ShouldAlmostEqual, aa45x.Theta)
	test.That(t, ov45x.AxisAngles().RX, test.ShouldAlmostEqual, aa45x.RX)
	test.That(t, ov45x.AxisAngles().RY, test.ShouldAlmostEqual, aa45x.RY)
	test.That(t, ov45x.AxisAngles().RZ, test.ShouldAlmostEqual, aa45x.RZ)
	test.That(t, ov45x.EulerAngles().Roll, test.ShouldAlmostEqual, ea45x.Roll)
	test.That(t, ov45x.EulerAngles().Pitch, test.ShouldAlmostEqual, ea45x.Pitch)
	test.That(t, ov45x.EulerAngles().Yaw, test.ShouldAlmostEqual, ea45x.Yaw)
}

func TestOrientationVectorDegrees(t *testing.T) {
	test.That(t, ovd45x.OrientationVectorRadians().Theta, test.ShouldAlmostEqual, ov45x.Theta)
	test.That(t, ovd45x.OrientationVectorRadians().OX, test.ShouldAlmostEqual, ov45x.OX)
	test.That(t, ovd45x.OrientationVectorRadians().OY, test.ShouldAlmostEqual, ov45x.OY)
	test.That(t, ovd45x.OrientationVectorRadians().OZ, test.ShouldAlmostEqual, ov45x.OZ)
	test.That(t, ovd45x.OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, ovd45x.Theta)
	test.That(t, ovd45x.OrientationVectorDegrees().OX, test.ShouldAlmostEqual, ovd45x.OX)
	test.That(t, ovd45x.OrientationVectorDegrees().OY, test.ShouldAlmostEqual, ovd45x.OY)
	test.That(t, ovd45x.OrientationVectorDegrees().OZ, test.ShouldAlmostEqual, ovd45x.OZ)
	test.That(t, ovd45x.Quaternion().Real, test.ShouldAlmostEqual, q45x.Real)
	test.That(t, ovd45x.Quaternion().Imag, test.ShouldAlmostEqual, q45x.Imag)
	test.That(t, ovd45x.Quaternion().Jmag, test.ShouldAlmostEqual, q45x.Jmag)
	test.That(t, ovd45x.Quaternion().Kmag, test.ShouldAlmostEqual, q45x.Kmag)
	test.That(t, ovd45x.AxisAngles().Theta, test.ShouldAlmostEqual, aa45x.Theta)
	test.That(t, ovd45x.AxisAngles().RX, test.ShouldAlmostEqual, aa45x.RX)
	test.That(t, ovd45x.AxisAngles().RY, test.ShouldAlmostEqual, aa45x.RY)
	test.That(t, ovd45x.AxisAngles().RZ, test.ShouldAlmostEqual, aa45x.RZ)
	test.That(t, ovd45x.EulerAngles().Roll, test.ShouldAlmostEqual, ea45x.Roll)
	test.That(t, ovd45x.EulerAngles().Pitch, test.ShouldAlmostEqual, ea45x.Pitch)
	test.That(t, ovd45x.EulerAngles().Yaw, test.ShouldAlmostEqual, ea45x.Yaw)
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

func TestOrientationBetween(t *testing.T) {
	aa := &R4AA{Theta: math.Pi / 2., RX: 0., RY: 1., RZ: 0.}
	btw := OrientationBetween(aa, ov45x).OrientationVectorDegrees()
	result := &OrientationVectorDegrees{Theta: 135.0, OX: -1., OY: 0.0, OZ: 0.0}
	test.That(t, result.Theta, test.ShouldAlmostEqual, btw.Theta)
	test.That(t, result.OX, test.ShouldAlmostEqual, btw.OX)
	test.That(t, result.OY, test.ShouldAlmostEqual, btw.OY)
	test.That(t, result.OZ, test.ShouldAlmostEqual, btw.OZ)
}
