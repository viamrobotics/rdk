package spatialmath

import (
	"math"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/utils"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/quat"
)

func TestOrientationConsistency(t *testing.T) {
	// represent a 45 degree rotation around the x axis in all the representations
	th := math.Pi / 4.
	q := quat.Number{math.Cos(th / 2.), math.Sin(th / 2.), 0, 0}              // in quaternion representation
	aa := &R4AA{th, 1., 0., 0.}                                               // in axis-angle representation
	ea := &EulerAngles{Roll: th, Pitch: 0, Yaw: 0}                            // in euler angle representation
	ov := &OrientationVec{2. * th, 0., -math.Sqrt(2) / 2., math.Sqrt(2) / 2.} // in orientation vector representation
	ovd := &OrientationVecDegrees{90., 0., -math.Sqrt(2) / 2, math.Sqrt(2) / 2}

	qo := NewOrientationFromQuaternion(q)
	aao := NewOrientationFromAxisAngles(aa)
	eao := NewOrientationFromEulerAngles(ea)
	ovo := NewOrientationFromOV(ov)
	ovdo := NewOrientationFromOVD(ovd)

	// axis angle
	test.That(t, aao.(*quaternion).Real, test.ShouldAlmostEqual, qo.(*quaternion).Real)
	test.That(t, aao.(*quaternion).Imag, test.ShouldAlmostEqual, qo.(*quaternion).Imag)
	test.That(t, aao.(*quaternion).Jmag, test.ShouldAlmostEqual, qo.(*quaternion).Jmag)
	test.That(t, aao.(*quaternion).Kmag, test.ShouldAlmostEqual, qo.(*quaternion).Kmag)
	// euler angle
	test.That(t, eao.(*quaternion).Real, test.ShouldAlmostEqual, qo.(*quaternion).Real)
	test.That(t, eao.(*quaternion).Imag, test.ShouldAlmostEqual, qo.(*quaternion).Imag)
	test.That(t, eao.(*quaternion).Jmag, test.ShouldAlmostEqual, qo.(*quaternion).Jmag)
	test.That(t, eao.(*quaternion).Kmag, test.ShouldAlmostEqual, qo.(*quaternion).Kmag)
	// orientation vec
	test.That(t, ovo.(*quaternion).Real, test.ShouldAlmostEqual, qo.(*quaternion).Real)
	test.That(t, ovo.(*quaternion).Imag, test.ShouldAlmostEqual, qo.(*quaternion).Imag)
	test.That(t, ovo.(*quaternion).Jmag, test.ShouldAlmostEqual, qo.(*quaternion).Jmag)
	test.That(t, ovo.(*quaternion).Kmag, test.ShouldAlmostEqual, qo.(*quaternion).Kmag)
	// orientation vec degrees
	test.That(t, ovdo.(*quaternion).Real, test.ShouldAlmostEqual, qo.(*quaternion).Real)
	test.That(t, ovdo.(*quaternion).Imag, test.ShouldAlmostEqual, qo.(*quaternion).Imag)
	test.That(t, ovdo.(*quaternion).Jmag, test.ShouldAlmostEqual, qo.(*quaternion).Jmag)
	test.That(t, ovdo.(*quaternion).Kmag, test.ShouldAlmostEqual, qo.(*quaternion).Kmag)

}

func TestZeroOrientation(t *testing.T) {
	zero := NewZeroOrientation()
	test.That(t, zero.OV(), test.ShouldResemble, NewOrientationVec())
	test.That(t, zero.OVD(), test.ShouldResemble, NewOrientationVecDegrees())
	test.That(t, zero.AxisAngles(), test.ShouldResemble, NewR4AA())
	test.That(t, zero.Quaternion(), test.ShouldResemble, quat.Number{1, 0, 0, 0})
	test.That(t, zero.EulerAngles(), test.ShouldResemble, NewEulerAngles())
}

func TestQuaternions(t *testing.T) {
	theta := math.Pi / 6.
	n := r3.Vector{0, math.Sqrt(2) / 2., math.Sqrt(2) / 2.}
	q := quat.Number{math.Cos(theta / 2.), n.X * math.Sin(theta/2.), n.Y * math.Sin(theta/2.), n.Z * math.Sin(theta/2.)}
	o := NewOrientationFromQuaternion(q)
	test.That(t, o.Quaternion(), test.ShouldResemble, q)
}

func TestEulerAngles(t *testing.T) {
	a := &EulerAngles{math.Pi / 2., math.Pi / 2., 0.}
	o := NewOrientationFromEulerAngles(a)
	test.That(t, o.EulerAngles().Roll, test.ShouldAlmostEqual, a.Roll)
	test.That(t, o.EulerAngles().Pitch, test.ShouldAlmostEqual, a.Pitch)
	test.That(t, o.EulerAngles().Yaw, test.ShouldAlmostEqual, a.Yaw)
}

func TestAxisAngles(t *testing.T) {
	a := &R4AA{Theta: math.Pi / 3., RX: 0., RY: 1., RZ: 1.}
	a.Normalize()
	o := NewOrientationFromAxisAngles(a)
	test.That(t, o.AxisAngles().Theta, test.ShouldAlmostEqual, a.Theta)
	test.That(t, o.AxisAngles().RX, test.ShouldAlmostEqual, a.RX)
	test.That(t, o.AxisAngles().RY, test.ShouldAlmostEqual, a.RY)
	test.That(t, o.AxisAngles().RZ, test.ShouldAlmostEqual, a.RZ)
}

func TestOrientationVec(t *testing.T) {
	a := &OrientationVec{Theta: math.Pi / 6., OX: 1., OY: 0., OZ: 0.5}
	a.Normalize()
	o := NewOrientationFromOV(a)
	test.That(t, o.OV().Theta, test.ShouldAlmostEqual, a.Theta)
	test.That(t, o.OV().OX, test.ShouldAlmostEqual, a.OX)
	test.That(t, o.OV().OY, test.ShouldAlmostEqual, a.OY)
	test.That(t, o.OV().OZ, test.ShouldAlmostEqual, a.OZ)

	b := &OrientationVecDegrees{Theta: utils.RadToDeg(a.Theta), OX: a.OX, OY: a.OY, OZ: a.OZ}
	o = NewOrientationFromOVD(b)
	test.That(t, o.OVD().Theta, test.ShouldAlmostEqual, b.Theta)
	test.That(t, o.OVD().OX, test.ShouldAlmostEqual, b.OX)
	test.That(t, o.OVD().OY, test.ShouldAlmostEqual, b.OY)
	test.That(t, o.OVD().OZ, test.ShouldAlmostEqual, b.OZ)
}
