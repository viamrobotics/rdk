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
		tri2 := tri.Transform(tf)
		for i := range tri2.Points() {
			test.That(t, tri2.Points()[i], test.ShouldResemble, NewPoint(expectedPts[i], "").Transform(tf).Pose().Point())
		}
	})

	t.Run("closest triangle inside point", func(t *testing.T) {
		// interior
		closestPoint, isInside := closestTriangleInsidePoint(tri, r3.Vector{1, 1, 1})
		test.That(t, closestPoint, test.ShouldResemble, r3.Vector{1, 1, 0})
		test.That(t, isInside, test.ShouldResemble, true)

		// directly above edge
		closestPoint, isInside = closestTriangleInsidePoint(tri, r3.Vector{2, 0, 1})
		test.That(t, closestPoint, test.ShouldResemble, r3.Vector{2, 0, 0})
		test.That(t, isInside, test.ShouldResemble, true)

		// directly above vertex
		closestPoint, isInside = closestTriangleInsidePoint(tri, r3.Vector{0, 3, 1})
		test.That(t, closestPoint, test.ShouldResemble, r3.Vector{0, 3, 0})
		test.That(t, isInside, test.ShouldResemble, true)

		// outside (obtuse)
		closestPoint, isInside = closestTriangleInsidePoint(tri, r3.Vector{1, -1, 1})
		test.That(t, isInside, test.ShouldResemble, false)

		// outside (straight)
		closestPoint, isInside = closestTriangleInsidePoint(tri, r3.Vector{0, 4, 0})
		test.That(t, isInside, test.ShouldResemble, false)

		// interior, an isoceles right triangle rotated off the xy-plane; numbers large to keep integers (floats hit rounding error)
		rotatedPts := []r3.Vector{{0, 0, 0}, {50, 0, 0}, {0, 30, 40}}
		rotatedTri := NewTriangle(rotatedPts[0], rotatedPts[1], rotatedPts[2])
		closestPoint, isInside = closestTriangleInsidePoint(rotatedTri, r3.Vector{1, 3 + 4, 4 - 3})
		test.That(t, closestPoint, test.ShouldResembleProto, r3.Vector{1, 3, 4})
		test.That(t, isInside, test.ShouldResemble, true)
	})
}
