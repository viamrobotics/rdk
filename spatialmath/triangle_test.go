package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestBasicTriangleFunctions(t *testing.T) {
	expectedPts := []r3.Vector{{0, 0, 0}, {0, 3, 0}, {3, 0, 0}}
	tri := NewTriangle(expectedPts[0], expectedPts[1], expectedPts[2])

	expectedNormal := r3.Vector{0, 0, 1}
	expectedArea := 4.5
	expectedCentroid := r3.Vector{1, 1, 0}

	t.Run("constructor", func(t *testing.T) {
		test.That(t, tri.Points(), test.ShouldResemble, expectedPts)
		// the cross product of the normal with what is expected should result in nothing
		test.That(t, tri.Normal().Cross(expectedNormal), test.ShouldResemble, r3.Vector{})
	})

	t.Run("area", func(t *testing.T) {
		test.That(t, tri.Area(), test.ShouldEqual, expectedArea)
	})

	t.Run("centroid", func(t *testing.T) {
		test.That(t, tri.Centroid(), test.ShouldResemble, expectedCentroid)
	})

	t.Run("transform", func(t *testing.T) {
		tf := NewPose(r3.Vector{1, 1, 1}, &OrientationVector{OZ: 1, Theta: math.Pi})
		transformed := tri.Transform(tf)

		// Triangle.Transform must return *Triangle (not just Geometry) - this is relied upon
		// throughout the codebase for type assertions like tri.Transform(pose).(*Triangle)
		tri2, ok := transformed.(*Triangle)
		test.That(t, ok, test.ShouldBeTrue)

		for i := range tri2.Points() {
			expected := NewPoint(expectedPts[i], "").Transform(tf).Pose().Point()
			test.That(t, R3VectorAlmostEqual(tri2.Points()[i], expected, 1e-12), test.ShouldBeTrue)
		}
	})

	t.Run("ClosestTriangleInsidePoint", func(t *testing.T) {
		t.Run("closest inside triangle point to interior point (triangle on coordinate plane)", func(t *testing.T) {
			closestPoint, isInside := ClosestTriangleInsidePoint(tri, r3.Vector{1, 1, 1})
			test.That(t, closestPoint, test.ShouldResemble, r3.Vector{1, 1, 0})
			test.That(t, isInside, test.ShouldBeTrue)
		})

		t.Run("ClosestTriangleInsidePoint: interior point (triangle off coordinate plane)", func(t *testing.T) {
			rotatedPts := []r3.Vector{{0, 0, 0}, {50, 0, 0}, {0, 30, 40}}
			rotatedTri := NewTriangle(rotatedPts[0], rotatedPts[1], rotatedPts[2])
			closestPoint, isInside := ClosestTriangleInsidePoint(rotatedTri, r3.Vector{1, 3 + 4, 4 - 3})
			test.That(t, closestPoint, test.ShouldResemble, r3.Vector{1, 3, 4})
			test.That(t, isInside, test.ShouldBeTrue)
		})

		t.Run("ClosestTriangleInsidePoint: point above edge", func(t *testing.T) {
			closestPoint, isInside := ClosestTriangleInsidePoint(tri, r3.Vector{2, 0, 1})
			test.That(t, closestPoint, test.ShouldResemble, r3.Vector{2, 0, 0})
			test.That(t, isInside, test.ShouldBeTrue)
		})

		t.Run("ClosestTriangleInsidePoint: point above vertex", func(t *testing.T) {
			closestPoint, isInside := ClosestTriangleInsidePoint(tri, r3.Vector{0, 3, 1})
			test.That(t, closestPoint, test.ShouldResemble, r3.Vector{0, 3, 0})
			test.That(t, isInside, test.ShouldBeTrue)
		})

		t.Run("ClosestTriangleInsidePoint: outside vertex (obtuse case)", func(t *testing.T) {
			_, isInside := ClosestTriangleInsidePoint(tri, r3.Vector{1, -1, 1})
			test.That(t, isInside, test.ShouldBeFalse)
		})

		t.Run("ClosestTriangleInsidePoint: outside vertex (straight case)", func(t *testing.T) {
			_, isInside := ClosestTriangleInsidePoint(tri, r3.Vector{0, 4, 0})
			test.That(t, isInside, test.ShouldBeFalse)
		})
	})

	t.Run("closestPointTrianglePoint", func(t *testing.T) {
		t.Run("closestPointTrianglePoint: interior point", func(t *testing.T) {
			closestPoint := ClosestPointTrianglePoint(tri, r3.Vector{1, 1, 1})
			test.That(t, closestPoint, test.ShouldResemble, r3.Vector{1, 1, 0})
		})

		t.Run("ClosestTriangleInsidePoint: closest point is edge", func(t *testing.T) {
			closestPoint := ClosestPointTrianglePoint(tri, r3.Vector{3, 2, 1})
			test.That(t, closestPoint, test.ShouldResemble, r3.Vector{2, 1, 0})
		})

		t.Run("ClosestTriangleInsidePoint: closest point is vertex", func(t *testing.T) {
			closestPoint := ClosestPointTrianglePoint(tri, r3.Vector{-1, -1, 1})
			test.That(t, closestPoint, test.ShouldResemble, r3.Vector{0, 0, 0})
		})
	})
}
