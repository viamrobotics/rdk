package referenceframe

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
)

func TestPoseInFrame(t *testing.T) {
	pose := spatial.NewPoseFromOrientationVector(r3.Vector{1, 2, 3}, &spatial.OrientationVector{math.Pi / 2, 0, 0, -1})
	pF := NewPoseInFrame("frame", pose)
	test.That(t, pF.FrameName(), test.ShouldEqual, "frame")
	test.That(t, spatial.PoseAlmostEqual(pF.Pose(), pose), test.ShouldBeTrue)
	convertedPF := ProtobufToPoseInFrame(PoseInFrameToProtobuf(pF))
	test.That(t, pF.FrameName(), test.ShouldEqual, convertedPF.FrameName())
	test.That(t, spatial.PoseAlmostEqual(pF.Pose(), convertedPF.Pose()), test.ShouldBeTrue)
}

func TestGeometryInFrame(t *testing.T) {
	pose := spatial.NewPoseFromOrientationVector(r3.Vector{1, 2, 3}, &spatial.OrientationVector{math.Pi / 2, 0, 0, -1})
	geometry, err := spatial.NewBox(pose, r3.Vector{4, 5, 6})
	test.That(t, err, test.ShouldBeNil)
	gF := NewGeometriesInFrame("frame", []spatial.Geometry{geometry})
	test.That(t, gF.FrameName(), test.ShouldEqual, "frame")
	test.That(t, gF.Geometries()[0].AlmostEqual(geometry), test.ShouldBeTrue)
	convertedGF, err := ProtobufToGeometriesInFrame(GeometriesInFrameToProtobuf(gF))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gF.FrameName(), test.ShouldEqual, convertedGF.FrameName())
	test.That(t, gF.Geometries()[0].AlmostEqual(convertedGF.Geometries()[0]), test.ShouldBeTrue)
}
