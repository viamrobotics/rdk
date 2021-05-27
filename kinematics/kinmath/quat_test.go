package kinmath

import (
	"math"
	"testing"

	"go.viam.com/test"

	"gonum.org/v1/gonum/num/quat"
)

func TestAngleAxisConversion(t *testing.T) {
	// Test that we can convert back and forth losslessly between angle axis and quaternions

	startAA := R4AA{2.5980762, 0.577350, 0.577350, 0.577350}
	quat := startAA.ToQuat()
	end1 := QuatToR4AA(quat)
	test.That(t, math.Abs(end1.Theta-startAA.Theta), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RX-startAA.RX), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RY-startAA.RZ), test.ShouldBeLessThan, 0.001)
	test.That(t, math.Abs(end1.RZ-startAA.RZ), test.ShouldBeLessThan, 0.001)
}

func TestFlip(t *testing.T) {
	// Test that we can convert back and forth losslessly between angle axis and quaternions

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
