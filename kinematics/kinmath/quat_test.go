package kinmath

import (
	"math"
	"testing"

	"go.viam.com/test"

	"gonum.org/v1/gonum/num/quat"
)

func TestAngleAxisConversion1(t *testing.T) {
	// Test that we can convert back and forth losslessly between angle axis and quaternions

	startAA := R4AA{2.5980762, 0.577350, 0.577350, 0.577350}
	quat := startAA.ToQuat()
	end1 := QuatToR4AA(quat)
	test.That(t, math.Abs(end1.Theta-startAA.Theta), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RX-startAA.RX), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RY-startAA.RZ), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RZ-startAA.RZ), test.ShouldBeLessThan, 0.001)
}

func TestAngleAxisConversion2(t *testing.T) {
	// Test that we can convert back and forth losslessly between r4 and r3 angle axis

	startAA := R4AA{2.5980762, 0.577350, 0.577350, 0.577350}
	r3 := startAA.ToR3()
	end1 := r3.ToR4()
	test.That(t, math.Abs(end1.Theta-startAA.Theta), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RX-startAA.RX), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RY-startAA.RZ), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RZ-startAA.RZ), test.ShouldBeLessThan, 0.001)
}

func TestFlip(t *testing.T) {
	// Test that flipping quaternions to the opposite octant results in the same rotation

	startAA := R4AA{2.5980762, 0.577350, -0.577350, -0.577350}
	quat1 := startAA.ToQuat()
	quat2 := startAA.ToQuat()
	qb1 := quat.Mul(quat1, quat.Conj(quat2))
	qb2 := quat.Mul(quat1, quat.Conj(Flip(quat2)))

	end1 := QuatToR4AA(qb1)
	end2 := QuatToR4AA(qb2)
	test.That(t, math.Abs(end1.Theta-end2.Theta), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RX-end2.RX), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RY-end2.RZ), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RZ-end2.RZ), test.ShouldBeLessThan, 0.001)
}

func TestOVConversion(t *testing.T) {
	// Ensure a robust, lossless quaternion/ov/quaternion/ov transformation
	q1 := quat.Number{0.5, 0.5, 0.5, 0.5}
	ov1 := QuatToOV(q1)
	q2 := ov1.ToQuat()
	ov2 := QuatToOV(q2)
	test.That(t, math.Abs(ov1.Theta-ov2.Theta), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(ov1.RX-ov2.RX), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(ov1.RY-ov2.RY), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(ov1.RZ-ov2.RZ), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(q1.Real-q2.Real), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(q1.Imag-q2.Imag), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(q1.Jmag-q2.Jmag), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(q1.Kmag-q2.Kmag), test.ShouldBeLessThan, 0.001)
}

func TestR4Normalize(t *testing.T) {
	// Test that Normalize() will produce a unit vector
	ov1 := R4AA{0, 999, 0, 0}
	ov1.Normalize()
	test.That(t, ov1.Theta, test.ShouldEqual, 0)
	test.That(t, ov1.RX, test.ShouldEqual, 1)
	test.That(t, ov1.RY, test.ShouldEqual, 0)
	test.That(t, ov1.RZ, test.ShouldEqual, 0)
}

func TestOVNormalize(t *testing.T) {
	// Test that Normalize() will produce a unit vector
	ov1 := OrientVec{0, 999, 0, 0}
	ov1.Normalize()
	test.That(t, ov1.Theta, test.ShouldEqual, 0)
	test.That(t, ov1.RX, test.ShouldEqual, 1)
	test.That(t, ov1.RY, test.ShouldEqual, 0)
	test.That(t, ov1.RZ, test.ShouldEqual, 0)
	ov1 = OrientVec{0, 0.5, 0, 0}
	ov1.Normalize()
	test.That(t, ov1.Theta, test.ShouldEqual, 0)
	test.That(t, ov1.RX, test.ShouldEqual, 1)
	test.That(t, ov1.RY, test.ShouldEqual, 0)
	test.That(t, ov1.RZ, test.ShouldEqual, 0)
}
