package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func makeTestPoints(o Orientation, pt r3.Vector, points []r3.Vector, label string) Geometry {
	pose := NewPose(pt, o)
	pointsGeom, _ := NewPoints(pose, points, label)
	return pointsGeom
}

func TestNewPoints(t *testing.T) {
	offset := NewPose(r3.Vector{X: 1, Y: 0, Z: 0}, &EulerAngles{0, 0, math.Pi})
	testPoints := []r3.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 1, Z: 1},
		{X: -1, Y: -1, Z: -1},
	}

	// Test points created from NewPoints method
	geometry, err := NewPoints(offset, testPoints, "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, geometry, test.ShouldResemble, &points{
		pose:   offset,
		points: testPoints,
		label:  "",
	})

	// Test points created from GeometryCreator with offset
	geometry = geometry.Transform(PoseInverse(offset))
	test.That(t, PoseAlmostCoincident(geometry.Pose(), NewZeroPose()), test.ShouldBeTrue)
}

func TestNewPointsValidation(t *testing.T) {
	// Test valid points creation
	validPoints := []r3.Vector{{0, 0, 0}, {1, 1, 1}}
	points, err := NewPoints(NewZeroPose(), validPoints, "test")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, points, test.ShouldNotBeNil)

	// Test nil pose
	_, err = NewPoints(nil, validPoints, "test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "Invalid dimension(s)")

	// Test nil points slice
	_, err = NewPoints(NewZeroPose(), nil, "test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "Invalid dimension(s)")
}

func TestPointsAlmostEqual(t *testing.T) {
	testPoints := []r3.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 1, Z: 1},
	}
	original := makeTestPoints(NewZeroOrientation(), r3.Vector{}, testPoints, "")
	good := makeTestPoints(NewZeroOrientation(), r3.Vector{X: 1e-16, Y: 1e-16, Z: 1e-16}, testPoints, "")
	bad := makeTestPoints(NewZeroOrientation(), r3.Vector{X: 1e-2, Y: 1e-2, Z: 1e-2}, testPoints, "")
	test.That(t, original.(*points).almostEqual(good), test.ShouldBeTrue)
	test.That(t, original.(*points).almostEqual(bad), test.ShouldBeFalse)
}

func TestPointsToPoints(t *testing.T) {
	testPoints := []r3.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 1, Z: 1},
		{X: -1, Y: -1, Z: -1},
	}
	pts := makeTestPoints(NewZeroOrientation(), r3.Vector{}, testPoints, "")

	output := pts.ToPoints(0.1)
	test.That(t, len(output), test.ShouldEqual, len(testPoints))

	for i, v := range output {
		test.That(t, R3VectorAlmostEqual(v, testPoints[i], 1e-8), test.ShouldBeTrue)
	}
}

func TestPointsToProtobuf(t *testing.T) {
	testPoints := []r3.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 1, Z: 1},
		{X: -1, Y: -1, Z: -1},
	}
	pts := makeTestPoints(NewZeroOrientation(), r3.Vector{}, testPoints, "")

	proto := pts.ToProtobuf()
	test.That(t, proto.Label, test.ShouldEqual, "")
	test.That(t, proto.GetPoints(), test.ShouldNotBeNil)
	test.That(t, len(proto.GetPoints().Array), test.ShouldEqual, len(testPoints)*3)

	// Check that the points are correctly serialized
	for i, p := range testPoints {
		test.That(t, proto.GetPoints().Array[i*3], test.ShouldAlmostEqual, float32(p.X), 1e-6)
		test.That(t, proto.GetPoints().Array[i*3+1], test.ShouldAlmostEqual, float32(p.Y), 1e-6)
		test.That(t, proto.GetPoints().Array[i*3+2], test.ShouldAlmostEqual, float32(p.Z), 1e-6)
	}
}

func TestPointsTransform(t *testing.T) {
	// Create original points
	originalPoints := []r3.Vector{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}
	pointsGeom, err := NewPoints(NewZeroPose(), originalPoints, "test_points")
	test.That(t, err, test.ShouldBeNil)

	// Create a transform that moves everything by (2, 3, 4) and rotates 90 degrees around Z
	transform := NewPoseFromOrientation(&EulerAngles{0, 0, math.Pi / 2})
	transform = Compose(NewPoseFromPoint(r3.Vector{2, 3, 4}), transform)

	// Transform the geometry
	transformedPoints := pointsGeom.Transform(transform)

	// Check that the pose was updated correctly
	test.That(t, transformedPoints.Pose().Point().X, test.ShouldAlmostEqual, 2, 1e-6)
	test.That(t, transformedPoints.Pose().Point().Y, test.ShouldAlmostEqual, 3, 1e-6)
	test.That(t, transformedPoints.Pose().Point().Z, test.ShouldAlmostEqual, 4, 1e-6)

	// Check that the internal points were also transformed
	// After 90-degree Z rotation and translation (2,3,4):
	// (0,0,0) -> (2,3,4)
	// (1,0,0) -> (2,4,4)
	// (0,1,0) -> (1,3,4)

	expectedPoints := []r3.Vector{{2, 3, 4}, {2, 4, 4}, {1, 3, 4}}
	transformedPointsList := transformedPoints.(*points).points
	test.That(t, len(transformedPointsList), test.ShouldEqual, len(expectedPoints))
	for i, expected := range expectedPoints {
		test.That(t, transformedPointsList[i].X, test.ShouldAlmostEqual, expected.X, 1e-6)
		test.That(t, transformedPointsList[i].Y, test.ShouldAlmostEqual, expected.Y, 1e-6)
		test.That(t, transformedPointsList[i].Z, test.ShouldAlmostEqual, expected.Z, 1e-6)
	}
}
