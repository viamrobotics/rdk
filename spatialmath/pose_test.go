package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

func TestBasicPoseConstruction(t *testing.T) {
	p := NewZeroPose()
	// Should return an identity dual quat
	test.That(t, p.Orientation().OrientationVectorRadians(), test.ShouldResemble, &OrientationVector{0, 0, 0, 1})

	// point at +Y, rotate 90 degrees
	ov := &OrientationVector{math.Pi / 2, 0, 1, 0}
	ov.Normalize()

	p = NewPose(r3.Vector{1, 2, 3}, ov)
	ovCompare(t, p.Orientation().OrientationVectorRadians(), ov)
	ptCompare(t, p.Point(), r3.Vector{1, 2, 3})

	aa := QuatToR4AA(ov.ToQuat())
	p = NewPose(r3.Vector{1, 2, 3}, &R4AA{aa.Theta, aa.RX, aa.RY, aa.RZ})
	ptCompare(t, p.Point(), r3.Vector{1, 2, 3})
	ovCompare(t, p.Orientation().OrientationVectorRadians(), ov)

	p = NewPoseFromPoint(r3.Vector{1, 2, 3})
	ptCompare(t, p.Point(), r3.Vector{1, 2, 3})
	test.That(t, p.Orientation().OrientationVectorRadians(), test.ShouldResemble, &OrientationVector{0, 0, 0, 1})

	p1 := NewPose(r3.Vector{1, 2, 3}, ov)
	p2 := NewPoseFromPoint(r3.Vector{1, 2, 3})
	pComp := Compose(p1, p2)
	ptCompare(t, pComp.Point(), r3.Vector{0, 5, 5})

	p2 = NewPose(r3.Vector{2, 3, 4}, ov)
	delta := PoseDelta(p1, p2)
	ptCompare(t, delta.Point(), r3.Vector{1.0, 1.0, 1.0})
	ovCompare(t, delta.Orientation().OrientationVectorRadians(), NewOrientationVector())

	p2 = NewPoseFromPoint(r3.Vector{2, 3, 4})

	pb := PoseBetween(p1, p2)
	test.That(t, PoseAlmostEqual(Compose(p1, pb), p2), test.ShouldBeTrue)
	pbi := PoseBetweenInverse(p1, p2)
	test.That(t, PoseAlmostEqual(Compose(pbi, p1), p2), test.ShouldBeTrue)
	pbi2 := Compose(p2, PoseInverse(p1))
	test.That(t, PoseAlmostEqual(pbi, pbi2), test.ShouldBeTrue)

	p = NewPoseFromOrientation(&R4AA{0, 4, 5, 6})
	test.That(t, p.Orientation().OrientationVectorRadians(), test.ShouldResemble, &OrientationVector{0, 0, 0, 1})
}

func ptCompare(t *testing.T, p1, p2 r3.Vector) {
	t.Helper()
	test.That(t, p1.X, test.ShouldAlmostEqual, p2.X)
	test.That(t, p1.Y, test.ShouldAlmostEqual, p2.Y)
	test.That(t, p1.Z, test.ShouldAlmostEqual, p2.Z)
}

func TestDualQuatTransform(t *testing.T) {
	// Start with point [3, 4, 5] - Rotate by 180 degrees around x-axis and then displace by [4,2,6]
	pt := NewPoseFromPoint(r3.Vector{3., 4., 5.}) // starting point
	tr := &dualQuaternion{dualquat.Number{Real: quat.Number{Real: 0, Imag: 1}}}
	tr.SetTranslation(r3.Vector{4., 2., 6.})

	trAA := NewPose(r3.Vector{4., 2., 6.}, &R4AA{math.Pi, 1, 0, 0}) // same transformation from axis angle
	// ensure transformation is the same between both definitions
	test.That(t, tr.Real.Real, test.ShouldAlmostEqual, newDualQuaternionFromPose(trAA).Real.Real)
	test.That(t, tr.Real.Imag, test.ShouldAlmostEqual, newDualQuaternionFromPose(trAA).Real.Imag)
	test.That(t, tr.Real.Jmag, test.ShouldAlmostEqual, newDualQuaternionFromPose(trAA).Real.Jmag)
	test.That(t, tr.Real.Kmag, test.ShouldAlmostEqual, newDualQuaternionFromPose(trAA).Real.Kmag)
	test.That(t, tr.Dual.Real, test.ShouldAlmostEqual, newDualQuaternionFromPose(trAA).Dual.Real)
	test.That(t, tr.Dual.Imag, test.ShouldAlmostEqual, newDualQuaternionFromPose(trAA).Dual.Imag)
	test.That(t, tr.Dual.Jmag, test.ShouldAlmostEqual, newDualQuaternionFromPose(trAA).Dual.Jmag)
	test.That(t, tr.Dual.Kmag, test.ShouldAlmostEqual, newDualQuaternionFromPose(trAA).Dual.Kmag)

	expectedPose := NewPoseFromPoint(r3.Vector{7., -2., 1.})
	expectedPoint := expectedPose.Point()
	transformedPoint := Compose(tr, pt).Point()
	test.That(t, transformedPoint.X, test.ShouldAlmostEqual, expectedPoint.X)
	test.That(t, transformedPoint.Y, test.ShouldAlmostEqual, expectedPoint.Y)
	test.That(t, transformedPoint.Z, test.ShouldAlmostEqual, expectedPoint.Z)
}

func TestPoseInterpolation(t *testing.T) {
	p1 := NewPoseFromPoint(r3.Vector{1, 2, 3})
	p2 := NewPoseFromPoint(r3.Vector{3, 6, 9})
	intP := Interpolate(p1, p2, 0.5)
	ptCompare(t, intP.Point(), r3.Vector{2, 4, 6})

	p1 = NewPoseFromPoint(r3.Vector{0, 0, 0})
	p2 = NewPoseFromPoint(r3.Vector{10, 100, 1000})
	intP = Interpolate(p1, p2, 0.33)
	ptCompare(t, intP.Point(), r3.Vector{3.3, 33, 330})

	ov := &OrientationVector{math.Pi / 2, 0, 0, -1}
	ov.Normalize()
	p1 = NewPose(r3.Vector{100, 100, 200}, ov)
	p2 = NewPose(r3.Vector{100, 200, 200}, ov)
	intP = Interpolate(p1, p2, 0.1)
	ptCompare(t, intP.Point(), r3.Vector{100, 110, 200})
}

func TestLidarPose(t *testing.T) {
	ea := NewEulerAngles()
	// 45 degrees above horizon
	// Positive pitch rotation rotates from the default of pointing up at the +Z axis, forwards towards the +X axis.
	ea.Pitch = math.Pi / 4
	// Point to the left (at positive Y axis)
	ea.Yaw = math.Pi / 2

	// lidar sees a point 400mm away
	dist := 400.

	pose1 := NewPoseFromOrientation(ea)
	pose2 := NewPoseFromPoint(r3.Vector{0, 0, dist})
	seenPoint := Compose(pose1, pose2).Point()

	expectPoint := r3.Vector{0, 282.842712474619, 282.842712474619}

	test.That(t, expectPoint.X, test.ShouldAlmostEqual, seenPoint.X)
	test.That(t, expectPoint.Y, test.ShouldAlmostEqual, seenPoint.Y)
	test.That(t, expectPoint.Z, test.ShouldAlmostEqual, seenPoint.Z)
}

func TestPoseAlmostEqual(t *testing.T) {
	p1 := NewPoseFromPoint(r3.Vector{1.0, 2.0, 3.0})
	p2 := NewPoseFromPoint(r3.Vector{1.0000000001, 1.999999999, 3.0000000001})
	p3 := NewPoseFromPoint(r3.Vector{1.0000001, 2.999999, 3.0000001})
	test.That(t, PoseAlmostCoincident(p1, p2), test.ShouldBeTrue)
	test.That(t, PoseAlmostCoincident(p1, p3), test.ShouldBeFalse)
}

var (
	ov  = &OrientationVector{math.Pi / 2, 0, 0, -1}
	p1b = NewPose(r3.Vector{1, 2, 3}, ov)
	p2b = NewPose(r3.Vector{2, 3, 4}, ov)
)

func BenchmarkDeltaPose(b *testing.B) {
	for n := 0; n < b.N; n++ {
		PoseDelta(p1b, p2b)
	}
}

func BenchmarkPoseBetween(b *testing.B) {
	for n := 0; n < b.N; n++ {
		PoseBetween(p1b, p2b)
	}
}
