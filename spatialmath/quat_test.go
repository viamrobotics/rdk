package spatialmath

import (
	"math"
	"testing"

	"go.viam.com/test"

	pb "go.viam.com/core/proto/api/v1"

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

func TestDHConversion(t *testing.T) {
	// Test conversion of a DH param to a dual quaternion
	dhParam := []float64{-0.425, 0.1333, math.Pi / 2}
	dq1 := NewDualQuaternionFromDH(dhParam[0], dhParam[1], dhParam[2])
	dq2 := NewDualQuaternionFromArmPos(&pb.ArmPosition{X: -0.425, Y: 0, Z: 0.1333, OY: -1, Theta: 90})
	quatCompare(t, dq1.Quat.Real, dq2.Quat.Real)
	quatCompare(t, dq1.Quat.Dual, dq2.Quat.Dual)
}

func TestQuatDefault(t *testing.T) {
	q1 := NewDualQuaternionFromRotation(&OrientationVec{})
	q2 := NewDualQuaternionFromRotation(&OrientationVec{OZ: 1})
	quatCompare(t, q1.Quat.Real, q2.Quat.Real)
}

func TestQuatConversion(t *testing.T) {
	// Ensure a robust, lossless quaternion/ov/quaternion/ov transformation
	quatConvert(t, quat.Number{0.7071067811865476, 0.7071067811865476, 0, 0})
	quatConvert(t, quat.Number{0.7071067811865476, -0.7071067811865476, 0, 0})
	quatConvert(t, quat.Number{0.96, 0, -0.28, 0})
	quatConvert(t, quat.Number{0.96, 0, 0, -0.28})

	// Should be negative theta
	quatConvert(t, quat.Number{0.96, -0.28, 0, 0})

	// Test the complementary angle
	quatConvert(t, quat.Number{0.96, 0.28, 0, 0})

	// Another odd angle
	quatConvert(t, quat.Number{0.5, -0.5, -0.5, -0.5})

	// Some orientation vectors
	ovConvert(t, &OrientationVec{Theta: 2.47208, OX: 1, OY: 0, OZ: 0})
	ovConvert(t, &OrientationVec{Theta: 2.47208, OX: -1, OY: 0, OZ: 0})
	ovConvert(t, &OrientationVec{Theta: 2.47208, OX: 0, OY: 1, OZ: 0})
	ovConvert(t, &OrientationVec{Theta: 2.47208, OX: 0, OY: -1, OZ: 0})

	// Test a small angle that was hitting angleEpsilon erroneously
	ovConvert(t, &OrientationVec{Theta: 0.02, OX: 0.5048437942940054, OY: 0.5889844266763397, OZ: 0.631054742867507})

	// An OV that initially gave problems in testing
	ovConvert(t, &OrientationVec{Theta: 0, OX: -0.32439089809469324, OY: -0.9441256803955101, OZ: -0.05828588895294498})
	ovConvert(t, &OrientationVec{Theta: -0.5732162806942777, OX: -0.32439089809469324, OY: -0.9441256803955101, OZ: -0.05828588895294498})
}

func TestOVConversionPoles(t *testing.T) {
	// Ensure a robust, lossless quaternion/ov/quaternion/ov transformation near the poles
	// North pole
	ovConvert(t, &OrientationVec{Theta: 2.47208, OX: 0, OY: 0, OZ: 1})
	ovConvert(t, &OrientationVec{Theta: 0, OX: 0, OY: 0, OZ: 1})
	ovConvert(t, &OrientationVec{Theta: -2.47208, OX: 0, OY: 0, OZ: 1})
	ovConvert(t, &OrientationVec{Theta: -0.78, OX: 0, OY: 0, OZ: 1})

	// South pole
	ovConvert(t, &OrientationVec{Theta: 2.47208, OX: 0, OY: 0, OZ: -1})
	ovConvert(t, &OrientationVec{Theta: 0, OX: 0, OY: 0, OZ: -1})
	ovConvert(t, &OrientationVec{Theta: -2.47208, OX: 0, OY: 0, OZ: -1})
	ovConvert(t, &OrientationVec{Theta: -0.78, OX: 0, OY: 0, OZ: -1})

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
	ov1 := &OrientationVec{Theta: 0, OX: 999, OY: 0, OZ: 0}
	ov1.Normalize()
	test.That(t, ov1.Theta, test.ShouldEqual, 0)
	test.That(t, ov1.OX, test.ShouldEqual, 1)
	test.That(t, ov1.OY, test.ShouldEqual, 0)
	test.That(t, ov1.OZ, test.ShouldEqual, 0)
	ov1 = &OrientationVec{Theta: 0, OX: 0.5, OY: 0, OZ: 0}
	ov1.Normalize()
	test.That(t, ov1.Theta, test.ShouldEqual, 0)
	test.That(t, ov1.OX, test.ShouldEqual, 1)
	test.That(t, ov1.OY, test.ShouldEqual, 0)
	test.That(t, ov1.OZ, test.ShouldEqual, 0)
}

func ovConvert(t *testing.T, ov1 *OrientationVec) {
	q1 := ov1.ToQuat()
	ov2 := QuatToOV(q1)
	q2 := ov2.ToQuat()

	ovCompare(t, ov1, ov2)
	quatCompare(t, q1, q2)
}

func quatConvert(t *testing.T, q1 quat.Number) {
	ov1 := QuatToOV(q1)
	q2 := ov1.ToQuat()
	ov2 := QuatToOV(q2)
	ovCompare(t, ov1, ov2)
	quatCompare(t, q1, q2)
}

func ovCompare(t *testing.T, ov1, ov2 *OrientationVec) {
	test.That(t, ov1.Theta, test.ShouldAlmostEqual, ov2.Theta)
	test.That(t, ov1.OX, test.ShouldAlmostEqual, ov2.OX)
	test.That(t, ov1.OY, test.ShouldAlmostEqual, ov2.OY)
	test.That(t, ov1.OZ, test.ShouldAlmostEqual, ov2.OZ)
}

func quatCompare(t *testing.T, q1, q2 quat.Number) {
	test.That(t, q1.Real, test.ShouldAlmostEqual, q2.Real)
	test.That(t, q1.Imag, test.ShouldAlmostEqual, q2.Imag)
	test.That(t, q1.Jmag, test.ShouldAlmostEqual, q2.Jmag)
	test.That(t, q1.Kmag, test.ShouldAlmostEqual, q2.Kmag)
}
