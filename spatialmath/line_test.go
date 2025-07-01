package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func makeTestLine(o Orientation, pt r3.Vector, segments []r3.Vector, label string) Geometry {
	pose := NewPose(pt, o)
	testLine, _ := NewLine(pose, segments, label)
	return testLine
}

func TestNewLine(t *testing.T) {
	offset := NewPose(r3.Vector{X: 1, Y: 0, Z: 0}, &EulerAngles{0, 0, math.Pi})
	testSegments := []r3.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 1, Z: 1},
		{X: 2, Y: 0, Z: 0},
	}

	// Test line created from NewLine method
	geometry, err := NewLine(offset, testSegments, "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, geometry, test.ShouldResemble, &line{
		pose:     offset,
		segments: testSegments,
		label:    "",
	})

	// Test line created from GeometryCreator with offset
	geometry = geometry.Transform(PoseInverse(offset))
	test.That(t, PoseAlmostCoincident(geometry.Pose(), NewZeroPose()), test.ShouldBeTrue)
}

func TestNewLineValidation(t *testing.T) {
	// Test valid line creation
	validSegments := []r3.Vector{{0, 0, 0}, {1, 1, 1}}
	line, err := NewLine(NewZeroPose(), validSegments, "test")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, line, test.ShouldNotBeNil)

	// Test nil pose
	_, err = NewLine(nil, validSegments, "test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "Invalid dimension(s)")

	// Test empty segments slice
	_, err = NewLine(NewZeroPose(), []r3.Vector{}, "test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "Invalid dimension(s)")

	// Test nil segments slice
	_, err = NewLine(NewZeroPose(), nil, "test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "Invalid dimension(s)")

	// Test single segment (not enough for a line)
	_, err = NewLine(NewZeroPose(), []r3.Vector{{0, 0, 0}}, "test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "Invalid dimension(s)")
}

func TestLineAlmostEqual(t *testing.T) {
	testSegments := []r3.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 1, Z: 1},
	}
	original := makeTestLine(NewZeroOrientation(), r3.Vector{}, testSegments, "")
	good := makeTestLine(NewZeroOrientation(), r3.Vector{X: 1e-16, Y: 1e-16, Z: 1e-16}, testSegments, "")
	bad := makeTestLine(NewZeroOrientation(), r3.Vector{X: 1e-2, Y: 1e-2, Z: 1e-2}, testSegments, "")
	test.That(t, original.(*line).almostEqual(good), test.ShouldBeTrue)
	test.That(t, original.(*line).almostEqual(bad), test.ShouldBeFalse)
}

func TestLineToPoints(t *testing.T) {
	testSegments := []r3.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 1, Z: 1},
		{X: 2, Y: 0, Z: 0},
	}
	testLine := makeTestLine(NewZeroOrientation(), r3.Vector{}, testSegments, "")

	output := testLine.ToPoints(0.1)
	test.That(t, len(output), test.ShouldEqual, len(testSegments))

	for i, v := range output {
		test.That(t, R3VectorAlmostEqual(v, testSegments[i], 1e-8), test.ShouldBeTrue)
	}
}

func TestLineToProtobuf(t *testing.T) {
	testSegments := []r3.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 1, Z: 1},
		{X: 2, Y: 0, Z: 0},
	}
	testLine := makeTestLine(NewZeroOrientation(), r3.Vector{}, testSegments, "")

	proto := testLine.ToProtobuf()
	test.That(t, proto.Label, test.ShouldEqual, "")
	test.That(t, proto.GetLine(), test.ShouldNotBeNil)
	test.That(t, len(proto.GetLine().Segments), test.ShouldEqual, len(testSegments)*3)

	// Check that the segments are correctly serialized
	for i, s := range testSegments {
		test.That(t, proto.GetLine().Segments[i*3], test.ShouldAlmostEqual, float32(s.X), 1e-6)
		test.That(t, proto.GetLine().Segments[i*3+1], test.ShouldAlmostEqual, float32(s.Y), 1e-6)
		test.That(t, proto.GetLine().Segments[i*3+2], test.ShouldAlmostEqual, float32(s.Z), 1e-6)
	}
}

func TestLineTransform(t *testing.T) {
	// Create original line
	originalSegments := []r3.Vector{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}
	lineGeom, err := NewLine(NewZeroPose(), originalSegments, "test_line")
	test.That(t, err, test.ShouldBeNil)

	// Create a transform that moves everything by (2, 3, 4) and rotates 90 degrees around Z
	transform := NewPoseFromOrientation(&EulerAngles{0, 0, math.Pi / 2})
	transform = Compose(NewPoseFromPoint(r3.Vector{2, 3, 4}), transform)

	// Transform the geometry
	transformedLine := lineGeom.Transform(transform)

	// Check that the pose was updated correctly
	test.That(t, transformedLine.Pose().Point().X, test.ShouldAlmostEqual, 2, 1e-6)
	test.That(t, transformedLine.Pose().Point().Y, test.ShouldAlmostEqual, 3, 1e-6)
	test.That(t, transformedLine.Pose().Point().Z, test.ShouldAlmostEqual, 4, 1e-6)

	// Check that the internal segments were also transformed
	// After 90-degree Z rotation and translation (2,3,4):
	// (0,0,0) -> (2,3,4)
	// (1,0,0) -> (2,4,4)
	// (0,1,0) -> (1,3,4)

	expectedSegments := []r3.Vector{{2, 3, 4}, {2, 4, 4}, {1, 3, 4}}
	transformedSegmentsList := transformedLine.(*line).segments
	test.That(t, len(transformedSegmentsList), test.ShouldEqual, len(expectedSegments))
	for i, expected := range expectedSegments {
		test.That(t, transformedSegmentsList[i].X, test.ShouldAlmostEqual, expected.X, 1e-6)
		test.That(t, transformedSegmentsList[i].Y, test.ShouldAlmostEqual, expected.Y, 1e-6)
		test.That(t, transformedSegmentsList[i].Z, test.ShouldAlmostEqual, expected.Z, 1e-6)
	}
}
