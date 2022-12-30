package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestNewPoint(t *testing.T) {
	offset := NewPoseFromOrientation(r3.Vector{X: 1, Y: 0, Z: 0}, &EulerAngles{0, 0, math.Pi})

	// test point created from NewBox method
	geometry := NewPoint(offset.Point(), "")
	test.That(t, geometry, test.ShouldResemble, &point{offset.Point(), ""})

	// test point created from GeometryCreator with offset
	geometry = NewPointCreator(offset, "").NewGeometry(PoseInverse(offset))
	test.That(t, PoseAlmostCoincident(geometry.Pose(), NewZeroPose()), test.ShouldBeTrue)
}

func TestPointAlmostEqual(t *testing.T) {
	original := NewPoint(r3.Vector{}, "")
	good := NewPoint(r3.Vector{1e-16, 1e-16, 1e-16}, "")
	bad := NewPoint(r3.Vector{1e-2, 1e-2, 1e-2}, "")
	test.That(t, original.AlmostEqual(good), test.ShouldBeTrue)
	test.That(t, original.AlmostEqual(bad), test.ShouldBeFalse)
}

func TestPointVertices(t *testing.T) {
	offset := r3.Vector{2, 2, 2}
	test.That(t, R3VectorAlmostEqual(NewPoint(offset, "").Vertices()[0], offset, 1e-8), test.ShouldBeTrue)
}
