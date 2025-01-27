package spatialmath

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestClosestPoint(t *testing.T) {
	p0 := r3.Vector{0, 0, 0}
	p1 := r3.Vector{1, 0, 0}
	p2 := r3.Vector{0, 0, 2}
	tri := NewTriangle(p0, p1, p2)

	qp1 := r3.Vector{-1, 0, 0}
	cp1 := ClosestPointTriangleCoplanarPoint(tri, qp1)
	cp2 := ClosestPointTrianglePoint(tri, qp1)
	cp3, inside := ClosestTriangleInsidePoint(tri, qp1)
	test.That(t, inside, test.ShouldBeFalse)
	test.That(t, cp3.ApproxEqual(qp1), test.ShouldBeTrue)
	test.That(t, cp1.ApproxEqual(cp2), test.ShouldBeTrue)
}
