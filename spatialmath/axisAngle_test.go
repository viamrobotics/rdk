package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestAAConversion(t *testing.T) {
	r3aa := r3.Vector{1.5, 1.5, 1.5}
	r4 := R3ToR4(r3aa)
	test.That(t, r4.Theta, test.ShouldAlmostEqual, 2.598076211353316)
	test.That(t, r4.RX, test.ShouldAlmostEqual, 0.5773502691896257)
	test.That(t, r4.RY, test.ShouldAlmostEqual, 0.5773502691896257)
	test.That(t, r4.RZ, test.ShouldAlmostEqual, 0.5773502691896257)
	r3_2 := r4.ToR3()
	test.That(t, r3_2.X, test.ShouldAlmostEqual, 1.5)
	test.That(t, r3_2.Y, test.ShouldAlmostEqual, 1.5)
	test.That(t, r3_2.Z, test.ShouldAlmostEqual, 1.5)
}

func TestR3ToR4EdgeCases(t *testing.T) {
	t.Run("zero vector returns identity", func(t *testing.T) {
		r4 := R3ToR4(r3.Vector{})
		test.That(t, r4.Theta, test.ShouldEqual, 0.0)
		test.That(t, math.IsNaN(r4.RX), test.ShouldBeFalse)
		test.That(t, math.IsNaN(r4.RY), test.ShouldBeFalse)
		test.That(t, math.IsNaN(r4.RZ), test.ShouldBeFalse)
	})

	t.Run("unit X rotation is preserved", func(t *testing.T) {
		r4 := R3ToR4(r3.Vector{1, 0, 0})
		test.That(t, r4.Theta, test.ShouldAlmostEqual, 1.0)
		test.That(t, r4.RX, test.ShouldAlmostEqual, 1.0)
		test.That(t, r4.RY, test.ShouldAlmostEqual, 0.0)
		test.That(t, r4.RZ, test.ShouldAlmostEqual, 0.0)
	})
}
