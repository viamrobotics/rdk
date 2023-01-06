package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestNewPoint(t *testing.T) {
	offset := NewPose(r3.Vector{X: 1, Y: 0, Z: 0}, &EulerAngles{0, 0, math.Pi})

	// test point created from NewBox method
	geometry := NewPoint(offset.Point(), "")
	test.That(t, geometry, test.ShouldResemble, &point{offset.Point(), ""})

	// test point created from GeometryCreator with offset
	geometry = NewPoint(offset.Point(), "").Transform(PoseInverse(offset))
	test.That(t, PoseAlmostCoincident(geometry.Pose(), NewZeroPose()), test.ShouldBeTrue)
}

func TestPointAlmostEqual(t *testing.T) {
	original := NewPoint(r3.Vector{}, "")
	good := NewPoint(r3.Vector{1e-18, 1e-18, 1e-18}, "")
	bad := NewPoint(r3.Vector{1e-2, 1e-2, 1e-2}, "")
	test.That(t, original.AlmostEqual(good), test.ShouldBeTrue)
	test.That(t, original.AlmostEqual(bad), test.ShouldBeFalse)
}
