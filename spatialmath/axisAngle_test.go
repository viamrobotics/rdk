package spatialmath

import (
	"testing"

	"go.viam.com/test"
)

func TestAAConversion(t *testing.T) {
	r3 := R3AA{1.5, 1.5, 1.5}
	r4 := r3.ToR4()
	test.That(t, r4.Theta, test.ShouldAlmostEqual, 2.598076211353316)
	test.That(t, r4.RX, test.ShouldAlmostEqual, 0.5773502691896257)
	test.That(t, r4.RY, test.ShouldAlmostEqual, 0.5773502691896257)
	test.That(t, r4.RZ, test.ShouldAlmostEqual, 0.5773502691896257)
	r3_2 := r4.ToR3()
	test.That(t, r3_2.RX, test.ShouldAlmostEqual, 1.5)
	test.That(t, r3_2.RY, test.ShouldAlmostEqual, 1.5)
	test.That(t, r3_2.RZ, test.ShouldAlmostEqual, 1.5)
}
