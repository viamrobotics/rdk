package kinmath

import (
	"testing"

	"go.viam.com/test"

	"gonum.org/v1/gonum/num/quat"
)

func TestAngleAxisConversion(t *testing.T) {
	// Test that we can convert back and forth losslessly between angle axis and quaternions

	startAA := []float64{2.5980762, 0.577350, 0.577350, 0.577350}
	quat := AxisAngleToQuat(startAA[0], startAA[1], startAA[2], startAA[3])
	endAA := QuatToAxisAngle(quat)
	for i, v := range startAA {
		test.That(t, v-endAA[i], test.ShouldBeLessThan, 0.001)
	}
}

func TestFlip(t *testing.T) {
	// Test that we can convert back and forth losslessly between angle axis and quaternions

	startAA := []float64{2.5980762, 0.577350, -0.577350, -0.577350}
	quat1 := AxisAngleToQuat(startAA[0], startAA[1], startAA[2], startAA[3])
	quat2 := AxisAngleToQuat(startAA[0], startAA[1], startAA[2], startAA[3])
	qb1 := quat.Mul(quat1, quat.Conj(quat2))
	qb2 := quat.Mul(quat1, quat.Conj(Flip(quat2)))

	endAA1 := QuatToAxisAngle(qb1)
	endAA2 := QuatToAxisAngle(qb2)
	for i, v := range endAA1 {
		test.That(t, v-endAA2[i], test.ShouldBeLessThan, 0.001)
	}
}
