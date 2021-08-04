package referenceframe

import (
	"go.viam.com/test"
	"testing"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/dualquat"
)

/*
Create a test that successfully transforms the pose of *object from *frame1 into *frame2. The Orientation of *frame1 and *frame2
are the same, so the transformation is only made up of two translations.

|              |
|*frame1       |*object
|              |
|
|*frame3
|
|
|              *frame2
|________________
world
*/

// transform the point that is in the world frame is at (5, 7, 0) from frame1 to frame2.
// frame1 has its origin at (0, 7, 0) in the world frame. and frame2 has its origin at (5, 1, 0).
// frame3 is an intermediate frame at (0, 4, 0) in the world frame.
// All 4 frames have the same orientation.
func TestSimpleFrameTranslation(t *testing.T) {
	// build the system
	sfs := NewEmptyStaticFrameSystem("test")
	fs := FrameSystem(sfs)
	frame3 := r3.Vector{0., 4., 0.} // location of frame3 with respect to world frame
	err := fs.SetFrameFromPoint("frame3", fs.World(), frame3)
	test.That(t, err, test.ShouldBeNil)
	frame1 := r3.Vector{0., 3., 0.} // location of frame1 with respect to frame3
	err := fs.SetFrameFromPoint("frame1", fs.GetFrame("frame3"), frame1)
	test.That(t, err, test.ShouldBeNil)
	frame2 := r3.Vector{5., 1., 0.} // location of frame2 with respect to world frame
	err = fs.SetFrameFromPoint("frame2", fs.World(), frame2)
	test.That(t, err, test.ShouldBeNil)

	// do the transformation
	pointFrame1 := r3.Vector{5., 0., 0.} // the point from PoV of frame 1
	sourcePose := NewPoseFromPoint(pointFrame1)
	pointFrame2 := r3.Vector{0., 6., 0.} // the point from PoV of frame 2
	expectedPose := NewPoseFromPoint(pointFrame2)
	transformPose, err := fs.TransformPose(sourcePose, fs.GetFrame("frame1"), fs.GetFrame("frame2"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPose, test.ShouldResemble, expectedPose)
}
