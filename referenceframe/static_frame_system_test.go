package referenceframe

import (
	"math"
	"testing"

	"go.viam.com/test"
	spatial "go.viam.com/core/spatialmath"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/quat"
	"gonum.org/v1/gonum/num/dualquat"
)

var blankPos map[string][]Input

func TestDualQuatTransform(t *testing.T) {
	// Start with point [3, 4, 5] - Rotate by 180 degrees around x-axis and then displace by [4,2,6]
	pt := spatial.NewPoseFromPoint(r3.Vector{3., 4., 5.}) // starting point
	tr := &spatial.DualQuaternion{dualquat.Number{Real: quat.Number{Real: 0, Imag: 1}}}
	tr.SetTranslation(4., 2., 6.)
	
	trAA := spatial.NewPoseFromAxisAngle(r3.Vector{4., 2., 6.}, r3.Vector{1, 0, 0}, math.Pi) // same transformation from axis angle
	// ensure transformation is the same between both definitions
	test.That(t, tr.Real.Real, test.ShouldAlmostEqual, spatial.NewDualQuaternionFromPose(trAA).Real.Real)
	test.That(t, tr.Real.Imag, test.ShouldAlmostEqual, spatial.NewDualQuaternionFromPose(trAA).Real.Imag)
	test.That(t, tr.Real.Jmag, test.ShouldAlmostEqual, spatial.NewDualQuaternionFromPose(trAA).Real.Jmag)
	test.That(t, tr.Real.Kmag, test.ShouldAlmostEqual, spatial.NewDualQuaternionFromPose(trAA).Real.Kmag)
	test.That(t, tr.Dual.Real, test.ShouldAlmostEqual, spatial.NewDualQuaternionFromPose(trAA).Dual.Real)
	test.That(t, tr.Dual.Imag, test.ShouldAlmostEqual, spatial.NewDualQuaternionFromPose(trAA).Dual.Imag)
	test.That(t, tr.Dual.Jmag, test.ShouldAlmostEqual, spatial.NewDualQuaternionFromPose(trAA).Dual.Jmag)
	test.That(t, tr.Dual.Kmag, test.ShouldAlmostEqual, spatial.NewDualQuaternionFromPose(trAA).Dual.Kmag)

	expectedPose := spatial.NewPoseFromPoint(r3.Vector{7., -2., 1.})
	expectedPoint := expectedPose.Point()
	transformedPoint := spatial.Compose(tr, pt).Point()
	test.That(t, transformedPoint.X, test.ShouldAlmostEqual, expectedPoint.X)
	test.That(t, transformedPoint.Y, test.ShouldAlmostEqual, expectedPoint.Y)
	test.That(t, transformedPoint.Z, test.ShouldAlmostEqual, expectedPoint.Z)
}

// A simple Frame translation from the world frame to a frame right above it at (0, 3, 0)
// And then back to the world frame
// transforming a point at (1, 3, 0)
func TestSimpleFrameTranslation(t *testing.T) {
	// build the system
	sfs := NewEmptySimpleFrameSystem("test")
	fs := FrameSystem(sfs)
	frameLocation := r3.Vector{0., 3., 0.} // location of frame with respect to world frame
	frame := FrameFromPoint("frame", fs.World(), frameLocation)
	err := sfs.SetFrame(frame)
	test.That(t, err, test.ShouldBeNil)

	// do the transformation
	pointWorld := r3.Vector{1., 3., 0.}
	pwFrame := FrameFromPoint("", fs.GetFrame("world"), pointWorld) // the point from PoV of world
	pointFrame := r3.Vector{1., 0., 0.}
	pfFrame := FrameFromPoint("", fs.GetFrame("frame"), pointFrame) // the point from PoV of world

	// transform point from world to frame
	transformPoint1, err := fs.TransformPoint(blankPos, pwFrame, fs.GetFrame("frame"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint1.Point(), test.ShouldResemble, pointFrame)

	// transform point from frame to world
	transformPoint2, err := fs.TransformPoint(blankPos, pfFrame, fs.GetFrame("world"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint2.Point(), test.ShouldResemble, pointWorld)
}

// A simple Frame translation from the world frame to a frame right above it at (0, 3, 0) rotated 180 around Z
// And then back to the world frame
// transforming a point at (1, 3, 0)
func TestSimpleFrameTranslationWithRotation(t *testing.T) {
	// build the system
	sfs := NewEmptySimpleFrameSystem("test")
	fs := FrameSystem(sfs)
	frame := spatial.NewPoseFromAxisAngle(r3.Vector{0., 3., 0.}, r3.Vector{0., 0., 1.}, math.Pi)
	err := sfs.SetFrameFromPose("frame", fs.World(), frame)
	test.That(t, err, test.ShouldBeNil)

	// do the transformation
	pointWorld := r3.Vector{1., 3., 0.}
	pwFrame := FrameFromPoint("", fs.GetFrame("world"), pointWorld) // the point from PoV of world
	pointFrame := r3.Vector{-1., 0., 0.}
	pfFrame := FrameFromPoint("", fs.GetFrame("frame"), pointFrame) // the point from PoV of world

	// transform point from world to frame
	transformPoint1, err := fs.TransformPoint(blankPos, pwFrame, fs.GetFrame("frame"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint1.Point().X, test.ShouldAlmostEqual, pointFrame.X)
	test.That(t, transformPoint1.Point().Y, test.ShouldAlmostEqual, pointFrame.Y)
	test.That(t, transformPoint1.Point().Z, test.ShouldAlmostEqual, pointFrame.Z)

	// transform point from frame to world
	transformPoint2, err := fs.TransformPoint(blankPos, pfFrame, fs.GetFrame("world"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint2.Point(), test.ShouldResemble, pointWorld)
}

/*
Transforms the pose of *object from *frame1 into *frame2. The Orientation of *frame1 and *frame2
are the same, so the final composed transformation is only made up of one translation.

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

transform the point that is in the world frame is at (5, 7, 0) from frame1 to frame2.
frame1 has its origin at (0, 7, 0) in the world frame. and frame2 has its origin at (5, 1, 0).
frame3 is an intermediate frame at (0, 4, 0) in the world frame.
All 4 frames have the same orientation.
*/
func TestFrameTranslation(t *testing.T) {
	// build the system
	sfs := NewEmptySimpleFrameSystem("test")
	fs := FrameSystem(sfs)
	frame3Pt := r3.Vector{0., 4., 0.} // location of frame3 with respect to world frame
	err := sfs.SetFrame(FrameFromPoint("frame3", fs.World(), frame3Pt))
	test.That(t, err, test.ShouldBeNil)
	frame1Pt := r3.Vector{0., 3., 0.} // location of frame1 with respect to frame3
	err = sfs.SetFrame(FrameFromPoint("frame1", fs.GetFrame("frame3"), frame1Pt))
	test.That(t, err, test.ShouldBeNil)
	frame2Pt := r3.Vector{5., 1., 0.} // location of frame2 with respect to world frame
	err = sfs.SetFrame(FrameFromPoint("frame2", fs.World(), frame2Pt))
	test.That(t, err, test.ShouldBeNil)

	// do the transformation
	pointFrame1 := r3.Vector{5., 0., 0.} // the point from PoV of frame 1
	pf1Frame := FrameFromPoint("", fs.GetFrame("frame1"), pointFrame1)
	pointFrame2 := r3.Vector{0., 6., 0.} // the point from PoV of frame 2
	transformPoint, err := fs.TransformPoint(blankPos, pf1Frame, fs.GetFrame("frame2"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint.Point(), test.ShouldResemble, pointFrame2)
}

/*
Very similar to test above, but this time *frame2 is oriented 90 degrees counterclockwise around the z-axis
(+z is pointing out of the page), which means the orientation of the object will also change.

|              |
|*frame1       |*object
|              |
|
|*frame3
|              |
|              |
|          ____|*frame2
|________________
world
*/

// transform the point that is in the world frame is at (5, 7, 0) from frame1 to frame2.
// frame1 has its origin at (0, 7, 0) in the world frame. and frame2 has its origin at (5, 1, 0), and orientation 90 degrees around z.
// frame3 is an intermediate frame at (0, 4, 0) in the world frame.
func TestFrameTransform(t *testing.T) {
	// build the system
	sfs := NewEmptySimpleFrameSystem("test")
	fs := FrameSystem(sfs)
	// location of frame3 with respect to world frame
	frame3Pt := r3.Vector{0., 4., 0.} // location of frame3 with respect to world frame
	err := sfs.SetFrame(FrameFromPoint("frame3", fs.World(), frame3Pt))
	test.That(t, err, test.ShouldBeNil)
	frame1Pt := r3.Vector{0., 3., 0.} // location of frame1 with respect to frame3
	err = sfs.SetFrame(FrameFromPoint("frame1", fs.GetFrame("frame3"), frame1Pt))
	test.That(t, err, test.ShouldBeNil)
	// location of frame2 with respect to world frame
	frame2Pose := spatial.NewPoseFromAxisAngle(r3.Vector{5., 1., 0.}, r3.Vector{0., 0., 1.}, math.Pi/2)
	err = sfs.SetFrame(NewStaticFrame("frame2", fs.World(), frame2Pose))
	test.That(t, err, test.ShouldBeNil)

	// do the transformation
	pointFrame1 := r3.Vector{5., 0., 0.} // the point from PoV of frame 1
	pf1Frame := FrameFromPoint("", fs.GetFrame("frame1"), pointFrame1)
	pointFrame2 := r3.Vector{6., 0., 0.} // the point from PoV of frame 2
	transformPoint, err := fs.TransformPoint(blankPos, pf1Frame, fs.GetFrame("frame2"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint.Point().X, test.ShouldAlmostEqual, pointFrame2.X)
	test.That(t, transformPoint.Point().Y, test.ShouldAlmostEqual, pointFrame2.Y)
	test.That(t, transformPoint.Point().Z, test.ShouldAlmostEqual, pointFrame2.Z)
}

func TestComplicatedFrameTransform(t *testing.T) {
	// build the system
	dfs := NewEmptySimpleFrameSystem("test")
	fs := FrameSystem(dfs)

	// frame 1 rotate by 45 degrees around z axis and translate
	frame1 := NewStaticFrame("frame1", fs.World(), spatial.NewPoseFromAxisAngle(r3.Vector{-1., 2., 0.}, r3.Vector{0., 0., 1.}, math.Pi/4))
	
	err := fs.SetFrame(frame1)
	test.That(t, err, test.ShouldBeNil)
	// frame 2 rotate by 45 degree (relative to frame 1) around z axis and translate
	frame2 := NewStaticFrame("frame2", fs.GetFrame("frame1"), spatial.NewPoseFromAxisAngle(r3.Vector{2. * math.Sqrt(2), 0., 0.}, r3.Vector{0., 0., 1.}, math.Pi/4))
	err = fs.SetFrame(frame2)
	test.That(t, err, test.ShouldBeNil)

	// test out a transform from world to frame
	frameStart := NewStaticFrame("", fs.World(), spatial.NewPoseFromPoint(r3.Vector{1., 7., 0.})) // the point from PoV of world
	pointEnd := r3.Vector{3., 0., 0.}   // the point from PoV of frame 2
	transformPose, err := fs.TransformPoint(blankPos, frameStart, fs.GetFrame("frame2"))
	test.That(t, err, test.ShouldBeNil)
	transformPoint := transformPose.Point()
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, pointEnd.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, pointEnd.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, pointEnd.Z)

	// test out transform between frames
	// frame3 - pure rotation around y 90 degrees
	frame3 := NewStaticFrame("frame3", fs.World(), spatial.NewPoseFromAxisAngle(r3.Vector{}, r3.Vector{0., 1., 0.}, math.Pi/2))
	err = fs.SetFrame(frame3)
	test.That(t, err, test.ShouldBeNil)

	// frame4 - pure translation
	frame4 := NewStaticFrame("frame4", fs.GetFrame("frame3"), spatial.NewPoseFromPoint(r3.Vector{-2., 7., 1.}))
	err = fs.SetFrame(frame4)
	test.That(t, err, test.ShouldBeNil)

	frameStart = NewStaticFrame("", fs.GetFrame("frame2"), spatial.NewPoseFromPoint(r3.Vector{3., 0., 0.}))
	pointEnd = r3.Vector{2., 0., 0.}   // the point from PoV of frame 4
	transformPose, err = fs.TransformPoint(blankPos, frameStart, fs.GetFrame("frame4"))
	test.That(t, err, test.ShouldBeNil)
	transformPoint = transformPose.Point()
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, pointEnd.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, pointEnd.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, pointEnd.Z)

	// back to world frame
	frameStart = NewStaticFrame("", fs.GetFrame("frame4"), spatial.NewPoseFromPoint(r3.Vector{2., 0., 0.})) // the point from PoV of frame 4
	pointEnd = r3.Vector{1., 7., 0.}   // the point from PoV of world
	transformPose, err = fs.TransformPoint(blankPos, frameStart, fs.World())
	test.That(t, err, test.ShouldBeNil)
	transformPoint = transformPose.Point()
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, pointEnd.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, pointEnd.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, pointEnd.Z)
}
