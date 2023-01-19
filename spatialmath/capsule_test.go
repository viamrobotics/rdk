package spatialmath

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func makeTestCapsule(o Orientation, pt r3.Vector, radius, length float64) Geometry {
	c, _ := NewCapsule(NewPose(pt, o), radius, length, "")
	return c
}

func TestCapsuleConstruction(t *testing.T) {
	c := makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 0.1}, 1, 6.75).(*capsule)
	test.That(t, c.segA.ApproxEqual(r3.Vector{0, 0, -2.275}), test.ShouldBeTrue)
	test.That(t, c.segB.ApproxEqual(r3.Vector{0, 0, 2.475}), test.ShouldBeTrue)
}
