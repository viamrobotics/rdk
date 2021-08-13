package referenceframe

import (
	"math"
	"testing"

	"go.viam.com/test"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/quat"
)

func TestDualQuatTransform(t *testing.T) {
	// Start with point [3, 4, 5] - Rotate by 180 degrees around x-axis and then displace by [4,2,6]
	pt := r3.Vector{3., 4., 5.}                                                      // starting point
	tr := NewPose(r3.Vector{4., 2., 6.}, quat.Number{Real: 0, Imag: 1})              // transformation to do
	trAA := NewPoseFromAxisAngle(r3.Vector{4., 2., 6.}, r3.Vector{1, 0, 0}, math.Pi) // same transformation from axis angle
	// ensure transformation is the same between both defintions
	test.That(t, tr.Transform().Real.Real, test.ShouldAlmostEqual, trAA.Transform().Real.Real)
	test.That(t, tr.Transform().Real.Imag, test.ShouldAlmostEqual, trAA.Transform().Real.Imag)
	test.That(t, tr.Transform().Real.Jmag, test.ShouldAlmostEqual, trAA.Transform().Real.Jmag)
	test.That(t, tr.Transform().Real.Kmag, test.ShouldAlmostEqual, trAA.Transform().Real.Kmag)
	test.That(t, tr.Transform().Dual.Real, test.ShouldAlmostEqual, trAA.Transform().Dual.Real)
	test.That(t, tr.Transform().Dual.Imag, test.ShouldAlmostEqual, trAA.Transform().Dual.Imag)
	test.That(t, tr.Transform().Dual.Jmag, test.ShouldAlmostEqual, trAA.Transform().Dual.Jmag)
	test.That(t, tr.Transform().Dual.Kmag, test.ShouldAlmostEqual, trAA.Transform().Dual.Kmag)

	expectedPoint := r3.Vector{7., -2., 1.}
	transformedPoint := TransformPoint(tr.Transform(), pt)
	test.That(t, transformedPoint.X, test.ShouldAlmostEqual, expectedPoint.X)
	test.That(t, transformedPoint.Y, test.ShouldAlmostEqual, expectedPoint.Y)
	test.That(t, transformedPoint.Z, test.ShouldAlmostEqual, expectedPoint.Z)
}

// A simple Frame translation from the world frame to a frame right above it at (0, 3, 0)
// And then back to the world frame
// transforming a point at (1, 3, 0)
func TestSimpleFrameTranslation(t *testing.T) {
	// build the system
	sfs := NewEmptyStaticFrameSystem("test")
	fs := FrameSystem(sfs)
	frameLocation := r3.Vector{0., 3., 0.} // location of frame with respect to world frame
	err := fs.SetFrameFromPoint("frame", fs.World(), frameLocation)
	test.That(t, err, test.ShouldBeNil)

	// do the transformation
	pointWorld := r3.Vector{1., 3., 0.} // the point from PoV of world
	pointFrame := r3.Vector{1., 0., 0.} // the point from PoV of frame

	// transform point from world to frame
	transformPoint1, err := fs.TransformPoint(pointWorld, fs.GetFrame("world"), fs.GetFrame("frame"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint1, test.ShouldResemble, pointFrame)

	// transform point from frame to world
	transformPoint2, err := fs.TransformPoint(pointFrame, fs.GetFrame("frame"), fs.GetFrame("world"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint2, test.ShouldResemble, pointWorld)
}

// A simple Frame translation from the world frame to a frame right above it at (0, 3, 0) rotated 180 around Z
// And then back to the world frame
// transforming a point at (1, 3, 0)
func TestSimpleFrameTranslationWithRotation(t *testing.T) {
	// build the system
	sfs := NewEmptyStaticFrameSystem("test")
	fs := FrameSystem(sfs)
	frame := NewPoseFromAxisAngle(r3.Vector{0., 3., 0.}, r3.Vector{0., 0., 1.}, math.Pi)
	err := fs.SetFrameFromPose("frame", fs.World(), frame)
	test.That(t, err, test.ShouldBeNil)

	// do the transformation
	pointWorld := r3.Vector{1., 3., 0.}  // the point from PoV of world
	pointFrame := r3.Vector{-1., 0., 0.} // the point from PoV of frame

	// transform point from world to frame
	transformPoint1, err := fs.TransformPoint(pointWorld, fs.GetFrame("world"), fs.GetFrame("frame"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint1.X, test.ShouldAlmostEqual, pointFrame.X)
	test.That(t, transformPoint1.Y, test.ShouldAlmostEqual, pointFrame.Y)
	test.That(t, transformPoint1.Z, test.ShouldAlmostEqual, pointFrame.Z)

	// transform point from frame to world
	transformPoint2, err := fs.TransformPoint(pointFrame, fs.GetFrame("frame"), fs.GetFrame("world"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint2, test.ShouldResemble, pointWorld)
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
	sfs := NewEmptyStaticFrameSystem("test")
	fs := FrameSystem(sfs)
	frame3 := r3.Vector{0., 4., 0.} // location of frame3 with respect to world frame
	err := fs.SetFrameFromPoint("frame3", fs.World(), frame3)
	test.That(t, err, test.ShouldBeNil)
	frame1 := r3.Vector{0., 3., 0.} // location of frame1 with respect to frame3
	err = fs.SetFrameFromPoint("frame1", fs.GetFrame("frame3"), frame1)
	test.That(t, err, test.ShouldBeNil)
	frame2 := r3.Vector{5., 1., 0.} // location of frame2 with respect to world frame
	err = fs.SetFrameFromPoint("frame2", fs.World(), frame2)
	test.That(t, err, test.ShouldBeNil)

	// do the transformation
	pointFrame1 := r3.Vector{5., 0., 0.} // the point from PoV of frame 1
	pointFrame2 := r3.Vector{0., 6., 0.} // the point from PoV of frame 2
	transformPoint, err := fs.TransformPoint(pointFrame1, fs.GetFrame("frame1"), fs.GetFrame("frame2"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint, test.ShouldResemble, pointFrame2)
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
	sfs := NewEmptyStaticFrameSystem("test")
	fs := FrameSystem(sfs)
	// location of frame3 with respect to world frame
	frame3 := r3.Vector{0., 4., 0.}
	err := fs.SetFrameFromPoint("frame3", fs.World(), frame3)
	test.That(t, err, test.ShouldBeNil)
	// location of frame1 with respect to frame3
	frame1 := r3.Vector{0., 3., 0.}
	err = fs.SetFrameFromPoint("frame1", fs.GetFrame("frame3"), frame1)
	test.That(t, err, test.ShouldBeNil)
	// location of frame2 with respect to world frame
	frame2 := NewPoseFromAxisAngle(r3.Vector{5., 1., 0.}, r3.Vector{0., 0., 1.}, math.Pi/2)
	err = fs.SetFrameFromPose("frame2", fs.World(), frame2)
	test.That(t, err, test.ShouldBeNil)

	// do the transformation
	pointFrame1 := r3.Vector{5., 0., 0.} // the point from PoV of frame 1
	pointFrame2 := r3.Vector{6., 0., 0.} // the point from PoV of frame 2
	transformPoint, err := fs.TransformPoint(pointFrame1, fs.GetFrame("frame1"), fs.GetFrame("frame2"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, pointFrame2.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, pointFrame2.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, pointFrame2.Z)
}

// A more complicated frame transform with multiple branching frames and rotations
func TestComplicatedFrameTransform(t *testing.T) {
	// build the system
	sfs := NewEmptyStaticFrameSystem("test")
	fs := FrameSystem(sfs)

	// frame 1 rotate by 45 degrees around z axis and translate
	frame1 := NewPoseFromAxisAngle(r3.Vector{-1., 2., 0.}, r3.Vector{0., 0., 1.}, math.Pi/4)
	err := fs.SetFrameFromPose("frame1", fs.World(), frame1)
	test.That(t, err, test.ShouldBeNil)
	// frame 2 rotate by 45 degree (relative to frame 1) around z axis and translate
	frame2 := NewPoseFromAxisAngle(r3.Vector{2. * math.Sqrt(2), 0., 0.}, r3.Vector{0., 0., 1.}, math.Pi/4)
	err = fs.SetFrameFromPose("frame2", fs.GetFrame("frame1"), frame2)
	test.That(t, err, test.ShouldBeNil)

	// test out a transform from world to frame
	pointStart := r3.Vector{1., 7., 0.} // the point from PoV of world
	pointEnd := r3.Vector{3., 0., 0.}   // the point from PoV of frame 2
	transformPoint, err := fs.TransformPoint(pointStart, fs.World(), fs.GetFrame("frame2"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, pointEnd.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, pointEnd.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, pointEnd.Z)

	// test out transform between frames
	// frame3 - pure rotation around y 90 degrees
	frame3 := NewPoseFromAxisAngle(r3.Vector{}, r3.Vector{0., 1., 0.}, math.Pi/2)
	err = fs.SetFrameFromPose("frame3", fs.World(), frame3)
	test.That(t, err, test.ShouldBeNil)

	// frame4 - pure translation
	frame4 := r3.Vector{-2., 7., 1.}
	err = fs.SetFrameFromPoint("frame4", fs.GetFrame("frame3"), frame4)
	test.That(t, err, test.ShouldBeNil)

	pointStart = r3.Vector{3., 0., 0.} // the point from PoV of frame 2
	pointEnd = r3.Vector{2., 0., 0.}   // the point from PoV of frame 4
	transformPoint, err = fs.TransformPoint(pointStart, fs.GetFrame("frame2"), fs.GetFrame("frame4"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, pointEnd.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, pointEnd.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, pointEnd.Z)

	// back to world frame
	pointStart = r3.Vector{2., 0., 0.} // the point from PoV of frame 4
	pointEnd = r3.Vector{1., 7., 0.}   // the point from PoV of world
	transformPoint, err = fs.TransformPoint(pointStart, fs.GetFrame("frame4"), fs.World())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, pointEnd.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, pointEnd.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, pointEnd.Z)
}
