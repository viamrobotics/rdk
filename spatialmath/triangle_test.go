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

}
