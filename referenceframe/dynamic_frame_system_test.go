package referenceframe

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
)

func TestSimpleRotationalFrame(t *testing.T) {
	fs := NewEmptySimpleFrameSystem("test")

	// Revolute joint around X axis
	joint, err := NewRotationalFrame("joint", spatial.R4AA{RX: 1, RY: 0, RZ: 0}, Limit{Min: -math.Pi * 2, Max: math.Pi * 2})
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(joint, fs.World())

	// define the point coordinates
	pose := NewPoseInFrame("joint", spatial.NewPoseFromPoint(r3.Vector{2., 2., 10.}))
	expected1 := NewPoseInFrame(World, spatial.NewPoseFromPoint(r3.Vector{2., 2., 10.}))
	expected2 := NewPoseInFrame(World, spatial.NewPoseFromPoint(r3.Vector{2., -10, 2}))
	expected3 := NewPoseInFrame(World, spatial.NewPoseFromPoint(r3.Vector{2., 10, -2}))

	positions := StartPositions(fs) // zero position
	testTransformPoint(t, fs, positions, pose, expected1)
	positions["joint"] = []Input{{math.Pi / 2}} // Rotate 90 degrees one way
	testTransformPoint(t, fs, positions, pose, expected2)
	positions["joint"] = []Input{{-math.Pi / 2}} // Rotate 90 degrees the other way
	testTransformPoint(t, fs, positions, pose, expected3)
}

func TestSimpleTranslationalFrame(t *testing.T) {
	fs := NewEmptySimpleFrameSystem("test")

	// 1D gantry that slides in X
	gantry, err := NewTranslationalFrame("gantry", r3.Vector{1, 0, 0}, Limit{Min: math.Inf(-1), Max: math.Inf(1)})
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantry, fs.World())

	// define the point coordinates
	poseStart := NewPoseInFrame("gantry", spatial.NewPoseFromPoint(r3.Vector{}))     // the point from PoV of gantry
	poseEnd1 := NewPoseInFrame(World, spatial.NewPoseFromPoint(r3.Vector{0, 0, 0}))  // gantry starts at origin of world
	poseEnd2 := NewPoseInFrame(World, spatial.NewPoseFromPoint(r3.Vector{45, 0, 0})) // after gantry moves 45

	// test transformations
	positions := StartPositions(fs)
	testTransformPoint(t, fs, positions, poseStart, poseEnd1)
	positions["gantry"] = []Input{{45.}}
	testTransformPoint(t, fs, positions, poseStart, poseEnd2)
}
