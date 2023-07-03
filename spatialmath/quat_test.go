package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
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
	r3aa := startAA.ToR3()
	end1 := R3ToR4(r3aa)
	test.That(t, math.Abs(end1.Theta-startAA.Theta), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RX-startAA.RX), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RY-startAA.RZ), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RZ-startAA.RZ), test.ShouldBeLessThan, 0.001)
}

func TestEulerAnglesConversion(t *testing.T) {
	// TODO RSDK-2010 (rh) handle edge cases properly while
	// maintaining quadrant in euler conversions
	// if runtime.GOARCH == "arm64" {
	// 	t.Skip()
	// }

	// roll pitch and yaw are not near edge cases
	expectedEA := EulerAngles{math.Pi / 4.0, math.Pi / 4.0, 3.0 * math.Pi / 4.0}
	q := quat.Number{Real: 0.4619397662556435, Imag: -0.19134171618254486, Jmag: 0.4619397662556434, Kmag: 0.7325378163287418}
	endEa := QuatToEulerAngles(q)
	test.That(t, expectedEA.Roll, test.ShouldAlmostEqual, endEa.Roll)
	test.That(t, expectedEA.Pitch, test.ShouldAlmostEqual, endEa.Pitch)
	test.That(t, expectedEA.Yaw, test.ShouldAlmostEqual, endEa.Yaw)
	q2 := endEa.Quaternion()
	quatCompare(t, q, q2)

	// another vanilla angles test
	expectedEA = EulerAngles{-math.Pi / 4.0, -math.Pi / 4.0, math.Pi/4}
	q = quat.Number{Real: 1, Imag: 0, Jmag: 0, Kmag: 0}
	endEa = QuatToEulerAngles(q)
	test.That(t, expectedEA.Roll, test.ShouldAlmostEqual, endEa.Roll)
	test.That(t, expectedEA.Pitch, test.ShouldAlmostEqual, endEa.Pitch)
	test.That(t, expectedEA.Yaw, test.ShouldAlmostEqual, endEa.Yaw)
	q2 = endEa.Quaternion()
	quatCompare(t, q, q2)

	// gimbal lock edge case1: pitch is π / 2
	expectedEA = EulerAngles{-3 * math.Pi / 4.0, math.Pi / 2.0, 0}
	q = quat.Number{Real: 0.2705980500730985, Imag: -0.6532814824381882, Jmag: 0.27059805007309856, Kmag: 0.6532814824381883}
	endEa = QuatToEulerAngles(q)
	test.That(t, expectedEA.Roll, test.ShouldAlmostEqual, endEa.Roll)
	test.That(t, expectedEA.Pitch, test.ShouldAlmostEqual, endEa.Pitch)
	test.That(t, expectedEA.Yaw, test.ShouldAlmostEqual, endEa.Yaw)
	q2 = endEa.Quaternion()
	quatCompare(t, q, q2)

	// gimbal lock edge case: pitch is π / 2
	expectedEA = EulerAngles{math.Pi / 4.0, math.Pi / 2.0, math.Pi}
	q = quat.Number{Real: 0.2705980500730985, Imag: -0.6532814824381882, Jmag: 0.27059805007309856, Kmag: 0.6532814824381883}
	endEa = QuatToEulerAngles(q)
	test.That(t, expectedEA.Roll, test.ShouldAlmostEqual, endEa.Roll)
	test.That(t, expectedEA.Pitch, test.ShouldAlmostEqual, endEa.Pitch)
	test.That(t, expectedEA.Yaw, test.ShouldAlmostEqual, endEa.Yaw)
	q2 = endEa.Quaternion()
	quatCompare(t, q, q2)
}

func TestMatrixConversion(t *testing.T) {
	// Test that lossless conversion between quaternions and rotation matrices is achieved
	q := quat.Number{0.7071067811865476, 0.7071067811865476, 0, 0}
	quatCompare(t, q, QuatToRotationMatrix(q).Quaternion())
	q = quat.Number{0.7071067811865476, -0.7071067811865476, 0, 0}
	quatCompare(t, q, QuatToRotationMatrix(q).Quaternion())
	q = quat.Number{0.96, 0, -0.28, 0}
	quatCompare(t, q, QuatToRotationMatrix(q).Quaternion())
	q = quat.Number{0.96, 0, 0, -0.28}
	quatCompare(t, q, QuatToRotationMatrix(q).Quaternion())

	// Should be negative theta
	q = quat.Number{0.96, -0.28, 0, 0}
	quatCompare(t, q, QuatToRotationMatrix(q).Quaternion())

	// Test the complementary angle
	q = quat.Number{0.96, 0.28, 0, 0}
	quatCompare(t, q, QuatToRotationMatrix(q).Quaternion())

	// Another odd angle
	q = quat.Number{0.5, -0.5, -0.5, -0.5}
	quatCompare(t, q, Flip(QuatToRotationMatrix(q).Quaternion()))
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
	test.That(t, math.Abs(end1.RY-end2.RY), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RZ-end2.RZ), test.ShouldBeLessThan, 0.001)
}

func TestDHConversion(t *testing.T) {
	// Test conversion of a DH param to a dual quaternion
	dhParam := []float64{-0.425, 0.1333, math.Pi / 2}
	dq1 := newDualQuaternionFromDH(dhParam[0], dhParam[1], dhParam[2])
	dq2 := newDualQuaternionFromPose(NewPose(
		r3.Vector{X: -0.425, Y: 0, Z: 0.1333},
		&OrientationVectorDegrees{OY: -1, Theta: 90},
	))
	quatCompare(t, dq1.Real, dq2.Real)
	quatCompare(t, dq1.Dual, dq2.Dual)
}

func TestQuatDefault(t *testing.T) {
	q1 := newDualQuaternionFromRotation(&OrientationVector{})
	q2 := newDualQuaternionFromRotation(&OrientationVector{OZ: 1})
	quatCompare(t, q1.Real, q2.Real)
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
	ovConvert(t, &OrientationVector{Theta: 2.47208, OX: 1, OY: 0, OZ: 0})
	ovConvert(t, &OrientationVector{Theta: 2.47208, OX: -1, OY: 0, OZ: 0})
	ovConvert(t, &OrientationVector{Theta: 2.47208, OX: 0, OY: 1, OZ: 0})
	ovConvert(t, &OrientationVector{Theta: 2.47208, OX: 0, OY: -1, OZ: 0})

	// Test a small angle that was hitting defaultAngleEpsilon erroneously
	ovConvert(t, &OrientationVector{Theta: 0.02, OX: 0.5048437942940054, OY: 0.5889844266763397, OZ: 0.631054742867507})

	// An OV that initially gave problems in testing
	ovConvert(t, &OrientationVector{Theta: 0, OX: -0.32439089809469324, OY: -0.9441256803955101, OZ: -0.05828588895294498})
	ovConvert(t, &OrientationVector{Theta: -0.5732162806942777, OX: -0.32439089809469324, OY: -0.9441256803955101, OZ: -0.05828588895294498})
}

func TestOVConversionPoles(t *testing.T) {
	// Ensure a robust, lossless quaternion/ov/quaternion/ov transformation near the poles
	// North pole
	ovConvert(t, &OrientationVector{Theta: 2.47208, OX: 0, OY: 0, OZ: 1})
	ovConvert(t, &OrientationVector{Theta: 0, OX: 0, OY: 0, OZ: 1})
	ovConvert(t, &OrientationVector{Theta: -2.47208, OX: 0, OY: 0, OZ: 1})
	ovConvert(t, &OrientationVector{Theta: -0.78, OX: 0, OY: 0, OZ: 1})

	// South pole
	ovConvert(t, &OrientationVector{Theta: 2.47208, OX: 0, OY: 0, OZ: -1})
	ovConvert(t, &OrientationVector{Theta: 0, OX: 0, OY: 0, OZ: -1})
	ovConvert(t, &OrientationVector{Theta: -2.47208, OX: 0, OY: 0, OZ: -1})
	ovConvert(t, &OrientationVector{Theta: -0.78, OX: 0, OY: 0, OZ: -1})
}

func TestQuatNormalize(t *testing.T) {
	tests := []struct {
		rotation quat.Number
		expected quat.Number
	}{
		{quat.Number{0, 0, 0, 0}, quat.Number{1, 0, 0, 0}},
		{quat.Number{0, 1, 0, 0}, quat.Number{0, 1, 0, 0}},
		{quat.Number{0, 0.0000000000001, 0, 0}, quat.Number{0, 1, 0, 0}},
		{quat.Number{0, float64(math.MaxFloat64), 1, 0}, quat.Number{0, 1, 0, 0}},
		{quat.Number{4, 2, 8, 4}, quat.Number{0.4, 0.2, 0.8, 0.4}},
		{quat.Number{0, 3.0, 4.0, 5.0}, quat.Number{0, 3.0 / math.Sqrt(50), 4.0 / math.Sqrt(50), 5.0 / math.Sqrt(50)}},
	}

	for _, c := range tests {
		quatCompare(t, Normalize(c.rotation), c.expected)
	}
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
	ov1 := &OrientationVector{Theta: 0, OX: 999, OY: 0, OZ: 0}
	ov1.Normalize()
	test.That(t, ov1.Theta, test.ShouldEqual, 0)
	test.That(t, ov1.OX, test.ShouldEqual, 1)
	test.That(t, ov1.OY, test.ShouldEqual, 0)
	test.That(t, ov1.OZ, test.ShouldEqual, 0)
	ov1 = &OrientationVector{Theta: 0, OX: 0.5, OY: 0, OZ: 0}
	ov1.Normalize()
	test.That(t, ov1.Theta, test.ShouldEqual, 0)
	test.That(t, ov1.OX, test.ShouldEqual, 1)
	test.That(t, ov1.OY, test.ShouldEqual, 0)
	test.That(t, ov1.OZ, test.ShouldEqual, 0)
}

func ovConvert(t *testing.T, ov1 *OrientationVector) {
	t.Helper()
	q1 := ov1.ToQuat()
	ov2 := QuatToOV(q1)
	q2 := ov2.ToQuat()

	ovCompare(t, ov1, ov2)
	quatCompare(t, q1, q2)
}

func quatConvert(t *testing.T, q1 quat.Number) {
	t.Helper()
	ov1 := QuatToOV(q1)
	q2 := ov1.ToQuat()
	ov2 := QuatToOV(q2)
	ovCompare(t, ov1, ov2)
	quatCompare(t, q1, q2)
}

func ovCompare(t *testing.T, ov1, ov2 *OrientationVector) {
	t.Helper()
	test.That(t, ov1.Theta, test.ShouldAlmostEqual, ov2.Theta)
	test.That(t, ov1.OX, test.ShouldAlmostEqual, ov2.OX)
	test.That(t, ov1.OY, test.ShouldAlmostEqual, ov2.OY)
	test.That(t, ov1.OZ, test.ShouldAlmostEqual, ov2.OZ)
}

func quatCompare(t *testing.T, q1, q2 quat.Number) {
	t.Helper()
	test.That(t, q1.Real, test.ShouldAlmostEqual, q2.Real, 1e-8)
	test.That(t, q1.Imag, test.ShouldAlmostEqual, q2.Imag, 1e-8)
	test.That(t, q1.Jmag, test.ShouldAlmostEqual, q2.Jmag, 1e-8)
	test.That(t, q1.Kmag, test.ShouldAlmostEqual, q2.Kmag, 1e-8)
}
