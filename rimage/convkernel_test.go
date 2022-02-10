package rimage

import (
	"testing"

	"go.viam.com/test"
)

func TestNewKernel(t *testing.T) {
	k, err := NewKernel(5, 5)
	// test creation
	test.That(t, err, test.ShouldBeNil)
	test.That(t, k.Width, test.ShouldEqual, 5)
	test.That(t, k.Height, test.ShouldEqual, 5)
	// test content
	test.That(t, k.Content[0], test.ShouldResemble, []float64{0, 0, 0, 0, 0})
	test.That(t, k.At(4, 4), test.ShouldEqual, 0)
	test.That(t, k.At(3, 2), test.ShouldEqual, 0)
	// test set
	k.Set(3, 2, 1)
	test.That(t, k.At(3, 2), test.ShouldEqual, 1)
	// test AbsSum
	k.Set(1, 2, -1)
	test.That(t, k.AbSum(), test.ShouldEqual, 2)
	// test Normalize
	normalized := k.Normalize()
	test.That(t, normalized.At(3, 2), test.ShouldEqual, 0.5)

	k2, err := NewKernel(-1, 5)
	// test error
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, k2, test.ShouldBeNil)

	// test in normalize sum = 0
	k3, err := NewKernel(5, 5)
	test.That(t, err, test.ShouldBeNil)
	normalized3 := k3.Normalize()
	test.That(t, normalized3, test.ShouldNotBeNil)
}
