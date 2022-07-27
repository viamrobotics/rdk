package referenceframe

import (
	"errors"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
)

func frameNames(frames []Frame) []string {
	names := make([]string, len(frames))
	for i, f := range frames {
		names[i] = f.Name()
	}
	return names
}

func TestSimpleFrameSystemFunctions(t *testing.T) {
	// build the system
	fs := NewEmptySimpleFrameSystem("test")
	frame3Pt := r3.Vector{0., 4., 0.} // location of frame3 with respect to world frame
	f3, err := FrameFromPoint("frame3", frame3Pt)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(f3, fs.World())
	test.That(t, err, test.ShouldBeNil)
	frame1Pt := r3.Vector{0., 3., 0.} // location of frame1 with respect to frame3
	f1, err := FrameFromPoint("frame1", frame1Pt)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(f1, fs.GetFrame("frame3"))
	test.That(t, err, test.ShouldBeNil)
	frame2Pt := r3.Vector{5., 1., 0.} // location of frame2 with respect to world frame
	f2, err := FrameFromPoint("frame2", frame2Pt)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(f2, fs.World())
	test.That(t, err, test.ShouldBeNil)

	frames := fs.FrameNames()
	test.That(t, len(frames), test.ShouldEqual, 3)

	f1Parents, err := fs.TracebackFrame(f1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(f1Parents), test.ShouldEqual, 3)
	test.That(t, frameNames(f1Parents), test.ShouldResemble, []string{"frame1", "frame3", "world"})

	parent, err := fs.Parent(fs.GetFrame("frame1"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, parent, test.ShouldResemble, fs.GetFrame("frame3"))

	parent, err = fs.Parent(fs.GetFrame("frame3"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, parent, test.ShouldResemble, fs.World())
	// Pruning frame3 should also remove frame1
	fs.RemoveFrame(fs.GetFrame("frame3"))
	frames = fs.FrameNames()
	test.That(t, len(frames), test.ShouldEqual, 1)

	e2 := errors.New("parent frame with name \"foo\" not in frame system")
	e3 := errors.New("frame with name \"frame2\" already in frame system")
	e4 := errors.New("frame with name \"frame1\" not in frame system")
	if sfs, ok := fs.(*simpleFrameSystem); ok {
		err = sfs.AddFrame(f2, nil)
		test.That(t, err, test.ShouldBeError, NewParentFrameMissingError())

		fFoo := NewZeroStaticFrame("foo")
		err = sfs.checkName("bar", fFoo)
		test.That(t, err, test.ShouldBeError, e2)

		err = sfs.checkName("frame2", fs.World())
		test.That(t, err, test.ShouldBeError, e3)
	}

	_, err = fs.TracebackFrame(f1)
	test.That(t, err, test.ShouldBeError, e4)
}

// A simple Frame translation from the world frame to a frame right above it at (0, 3, 0)
// And then back to the world frame
// transforming a point at (1, 3, 0).
func TestSimpleFrameTranslation(t *testing.T) {
	// build the system
	fs := NewEmptySimpleFrameSystem("test")
	frame, err := FrameFromPoint("frame", r3.Vector{0., 3., 0.}) // location of frame with respect to world frame
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame, fs.World())
	test.That(t, err, test.ShouldBeNil)

	// define the point coordinates and transform between them both ways
	poseWorld := NewPoseInFrame(World, spatial.NewPoseFromPoint(r3.Vector{1, 3, 0}))   // the point from PoV of world
	poseFrame := NewPoseInFrame("frame", spatial.NewPoseFromPoint(r3.Vector{1, 0, 0})) // the point from PoV of frame
	testTransformPoint(t, fs, map[string][]Input{}, poseWorld, poseFrame)
	testTransformPoint(t, fs, map[string][]Input{}, poseFrame, poseWorld)
}

// A simple Frame translation from the world frame to a frame right above it at (0, 3, 0) rotated 180 around Z
// And then back to the world frame
// transforming a point at (1, 3, 0).
func TestSimpleFrameTranslationWithRotation(t *testing.T) {
	// build the system
	fs := NewEmptySimpleFrameSystem("test")
	framePose := spatial.NewPoseFromOrientation(r3.Vector{0., 3., 0.}, &spatial.R4AA{math.Pi, 0., 0., 1.})
	f1, err := NewStaticFrame("frame", framePose)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(f1, fs.World())
	test.That(t, err, test.ShouldBeNil)

	// define the point coordinates and transform between them both ways
	poseWorld := NewPoseInFrame(World, spatial.NewPoseFromPoint(r3.Vector{1, 3, 0}))
	poseFrame := NewPoseInFrame("frame", spatial.NewPoseFromOrientation(r3.Vector{-1., 0, 0}, &spatial.R4AA{math.Pi, 0., 0., 1.}))
	testTransformPoint(t, fs, map[string][]Input{}, poseWorld, poseFrame)
	testTransformPoint(t, fs, map[string][]Input{}, poseFrame, poseWorld)
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
frame1 has its origin at (0, 7, 0) in the world referenceframe. and frame2 has its origin at (5, 1, 0).
frame3 is an intermediate frame at (0, 4, 0) in the world referenceframe.
All 4 frames have the same orientation.
*/
func TestFrameTranslation(t *testing.T) {
	// build the system
	fs := NewEmptySimpleFrameSystem("test")
	frame3Pt := r3.Vector{0., 4., 0.} // location of frame3 with respect to world frame
	f3, err := FrameFromPoint("frame3", frame3Pt)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(f3, fs.World())
	test.That(t, err, test.ShouldBeNil)
	frame1Pt := r3.Vector{0., 3., 0.} // location of frame1 with respect to frame3
	f1, err := FrameFromPoint("frame1", frame1Pt)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(f1, fs.GetFrame("frame3"))
	test.That(t, err, test.ShouldBeNil)
	frame2Pt := r3.Vector{5., 1., 0.} // location of frame2 with respect to world frame
	f2, err := FrameFromPoint("frame2", frame2Pt)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(f2, fs.World())
	test.That(t, err, test.ShouldBeNil)

	// do the transformation
	poseStart := NewPoseInFrame("frame1", spatial.NewPoseFromPoint(r3.Vector{5, 0, 0})) // the point from PoV of frame 1
	poseEnd := NewPoseInFrame("frame2", spatial.NewPoseFromPoint(r3.Vector{0, 6, 0}))   // the point from PoV of frame 2
	testTransformPoint(t, fs, map[string][]Input{}, poseStart, poseEnd)
	testTransformPoint(t, fs, map[string][]Input{}, poseEnd, poseStart)
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
// frame1 has its origin at (0, 7, 0) in the world referenceframe. and frame2 has its origin
// at (5, 1, 0), and orientation 90 degrees around z.
// frame3 is an intermediate frame at (0, 4, 0) in the world referenceframe.
func TestFrameTransform(t *testing.T) {
	// build the system
	fs := NewEmptySimpleFrameSystem("test")
	// location of frame3 with respect to world frame
	frame3Pt := r3.Vector{0., 4., 0.} // location of frame3 with respect to world frame
	f3, err := FrameFromPoint("frame3", frame3Pt)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(f3, fs.World())
	test.That(t, err, test.ShouldBeNil)
	frame1Pt := r3.Vector{0., 3., 0.} // location of frame1 with respect to frame3
	f1, err := FrameFromPoint("frame1", frame1Pt)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(f1, fs.GetFrame("frame3"))
	test.That(t, err, test.ShouldBeNil)
	frame2Pose := spatial.NewPoseFromOrientation(r3.Vector{5., 1., 0.}, &spatial.R4AA{math.Pi / 2, 0., 0., 1.})
	f2, err := NewStaticFrame("frame2", frame2Pose)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(f2, fs.World())
	test.That(t, err, test.ShouldBeNil)

	// do the transformation
	poseStart := NewPoseInFrame("frame1", spatial.NewPoseFromPoint(r3.Vector{5, 0, 0}))
	poseEnd := NewPoseInFrame("frame2", spatial.NewPoseFromOrientation(r3.Vector{6, 0, 0.}, &spatial.R4AA{math.Pi / 2, 0., 0., 1.}))
	testTransformPoint(t, fs, map[string][]Input{}, poseStart, poseEnd)
}

/*
This test uses the same setup as the above test, but this time is concede with representing a geometry in a difference reference frame

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

// transform the object geometry that is in the world frame is at (5, 7, 0) from frame1 to frame2.
// frame1 has its origin at (0, 7, 0) in the world referenceframe. and frame2 has its origin
// at (5, 1, 0), and orientation 90 degrees around z.
// frame3 is an intermediate frame at (0, 4, 0) in the world referenceframe.
func TestGeomtriesTransform(t *testing.T) {
	// build the system
	fs := NewEmptySimpleFrameSystem("test")
	// location of frame3 with respect to world frame
	frame3Pt := r3.Vector{0., 4., 0.} // location of frame3 with respect to world frame
	f3, err := FrameFromPoint("frame3", frame3Pt)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(f3, fs.World())
	test.That(t, err, test.ShouldBeNil)
	frame1Pt := r3.Vector{0., 3., 0.} // location of frame1 with respect to frame3
	f1, err := FrameFromPoint("frame1", frame1Pt)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(f1, fs.GetFrame("frame3"))
	test.That(t, err, test.ShouldBeNil)
	frame2Pose := spatial.NewPoseFromOrientation(r3.Vector{5., 1., 0.}, &spatial.R4AA{math.Pi / 2, 0., 0., 1.})
	f2, err := NewStaticFrame("frame2", frame2Pose)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(f2, fs.World())
	test.That(t, err, test.ShouldBeNil)
	objectFromFrame1 := spatial.NewPoseFromPoint(r3.Vector{5, 0, 0})
	gc, err := spatial.NewBoxCreator(r3.Vector{2, 2, 2}, objectFromFrame1)
	test.That(t, err, test.ShouldBeNil)
	// it shouldn't matter where the transformation of the frame associated with the object is if we are just looking at its geometry
	object, err := NewStaticFrameWithGeometry("object", spatial.NewPoseFromPoint(r3.Vector{1000, 1000, 1000}), gc)

	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(object, f1)
	test.That(t, err, test.ShouldBeNil)

	objectFromFrame2 := spatial.NewPoseFromPoint(r3.Vector{6., 0., 0.}) // the point from PoV of frame 2
	geometries, err := object.Geometries([]Input{})
	test.That(t, err, test.ShouldBeNil)
	tf, err := fs.Transform(map[string][]Input{}, geometries, "frame2")
	test.That(t, err, test.ShouldBeNil)
	framedGeometries, _ := tf.(*GeometriesInFrame)
	test.That(t, framedGeometries.FrameName(), test.ShouldResemble, "frame2")
	test.That(t, spatial.PoseAlmostCoincident(framedGeometries.Geometries()["object"].Pose(), objectFromFrame2), test.ShouldBeTrue)

	gf := NewGeometriesInFrame(World, geometries.Geometries())
	tf, err = fs.Transform(map[string][]Input{}, gf, "frame3")
	test.That(t, err, test.ShouldBeNil)
	framedGeometries, _ = tf.(*GeometriesInFrame)
	test.That(t, framedGeometries.FrameName(), test.ShouldResemble, "frame3")
	objectFromFrame3 := spatial.NewPoseFromPoint(r3.Vector{5, -4, 0.}) // the point from PoV of frame 2
	test.That(t, spatial.PoseAlmostCoincident(framedGeometries.Geometries()["object"].Pose(), objectFromFrame3), test.ShouldBeTrue)
}

func TestComplicatedFrameTransform(t *testing.T) {
	// build the system
	fs := NewEmptySimpleFrameSystem("test")

	// frame 1 rotate by 45 degrees around z axis and translate
	frame1, err := NewStaticFrame("frame1", spatial.NewPoseFromOrientation(r3.Vector{-1., 2., 0.}, &spatial.R4AA{math.Pi / 4, 0., 0., 1.}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame1, fs.World())
	test.That(t, err, test.ShouldBeNil)
	// frame 2 rotate by 45 degree (relative to frame 1) around z axis and translate
	frame2, err := NewStaticFrame("frame2",
		spatial.NewPoseFromOrientation(r3.Vector{2. * math.Sqrt(2), 0., 0.}, &spatial.R4AA{math.Pi / 4, 0., 0., 1.}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame2, fs.GetFrame("frame1"))
	test.That(t, err, test.ShouldBeNil)

	// test out a transform from world to frame
	poseStart := NewPoseInFrame(World, spatial.NewPoseFromPoint(r3.Vector{1, 7, 0}))  // the point from PoV of world
	poseEnd := NewPoseInFrame("frame2", spatial.NewPoseFromPoint(r3.Vector{3, 0, 0})) // the point from PoV of frame 2
	testTransformPoint(t, fs, map[string][]Input{}, poseStart, poseEnd)

	// test out transform between frames
	// frame3 - pure rotation around y 90 degrees
	frame3, err := NewStaticFrame("frame3", spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.R4AA{math.Pi / 2, 0., 1., 0.}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame3, fs.World())
	test.That(t, err, test.ShouldBeNil)

	// frame4 - pure translation
	frame4, err := NewStaticFrame("frame4", spatial.NewPoseFromPoint(r3.Vector{-2., 7., 1.}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame4, fs.GetFrame("frame3"))
	test.That(t, err, test.ShouldBeNil)

	poseStart = NewPoseInFrame("frame2", spatial.NewPoseFromPoint(r3.Vector{3, 0, 0})) // the point from PoV of frame 2
	poseEnd = NewPoseInFrame("frame4", spatial.NewPoseFromPoint(r3.Vector{2, 0, 0}))   // the point from PoV of frame 4
	testTransformPoint(t, fs, map[string][]Input{}, poseStart, poseEnd)

	// back to world frame
	poseStart = NewPoseInFrame("frame4", spatial.NewPoseFromPoint(r3.Vector{2, 0, 0})) // the point from PoV of frame 4
	poseEnd = NewPoseInFrame(World, spatial.NewPoseFromPoint(r3.Vector{1, 7, 0}))      // the point from PoV of world
	testTransformPoint(t, fs, map[string][]Input{}, poseStart, poseEnd)
}

func TestSystemSplitAndRejoin(t *testing.T) {
	// build the system
	fs := NewEmptySimpleFrameSystem("test")

	// frame 1 rotate by 45 degrees around z axis and translate
	frame1, err := NewStaticFrame("frame1", spatial.NewPoseFromOrientation(r3.Vector{-1., 2., 0.}, &spatial.R4AA{math.Pi / 4, 0., 0., 1.}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame1, fs.World())
	test.That(t, err, test.ShouldBeNil)
	// frame 2 rotate by 45 degree (relative to frame 1) around z axis and translate
	frame2, err := NewStaticFrame("frame2",
		spatial.NewPoseFromOrientation(r3.Vector{2. * math.Sqrt(2), 0., 0.}, &spatial.R4AA{math.Pi / 4, 0., 0., 1.}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame2, fs.GetFrame("frame1"))
	test.That(t, err, test.ShouldBeNil)

	// frame3 - pure rotation around y 90 degrees
	frame3, err := NewStaticFrame("frame3", spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.R4AA{math.Pi / 2, 0., 1., 0.}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame3, fs.World())
	test.That(t, err, test.ShouldBeNil)

	// frame4 - pure translation
	frame4, err := NewStaticFrame("frame4", spatial.NewPoseFromPoint(r3.Vector{-2., 7., 1.}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame4, fs.GetFrame("frame3"))
	test.That(t, err, test.ShouldBeNil)

	// complete fs
	t.Logf("frames in fs: %v", fs.FrameNames())

	// This should remove frames 3 and 4 from fs
	fs2, err := fs.DivideFrameSystem(frame3)
	test.That(t, err, test.ShouldBeNil)
	t.Logf("frames in fs after divide: %v", fs.FrameNames())
	t.Logf("frames in fs2 after divide: %v", fs2.FrameNames())

	f4 := fs.GetFrame("frame4")
	test.That(t, f4, test.ShouldBeNil)
	f1 := fs.GetFrame("frame1")
	test.That(t, f1, test.ShouldNotBeNil)

	f4 = fs2.GetFrame("frame4")
	test.That(t, f4, test.ShouldNotBeNil)
	f1 = fs2.GetFrame("frame1")
	test.That(t, f1, test.ShouldBeNil)

	_, err = fs.Transform(map[string][]Input{}, NewPoseInFrame("frame4", spatial.NewPoseFromPoint(r3.Vector{2, 0, 0})), "frame2")
	test.That(t, err, test.ShouldNotBeNil)

	// Put frame3 back where it was
	err = fs.AddFrame(frame3, fs.World())
	test.That(t, err, test.ShouldBeNil)
	err = fs.MergeFrameSystem(fs2, frame3)
	test.That(t, err, test.ShouldBeNil)

	// Comfirm that fs2 is empty now
	t.Logf("frames in fs after merge: %v", fs.FrameNames())
	t.Logf("frames in fs2 after merge: %v", fs2.FrameNames())

	// Confirm new combined frame system now works as it did before
	poseStart := NewPoseInFrame("frame2", spatial.NewPoseFromPoint(r3.Vector{3, 0, 0})) // the point from PoV of frame 2
	poseEnd := NewPoseInFrame("frame4", spatial.NewPoseFromPoint(r3.Vector{2, 0, 0}))   // the point from PoV of frame 4
	testTransformPoint(t, fs, map[string][]Input{}, poseStart, poseEnd)
}

func testTransformPoint(t *testing.T, fs FrameSystem, positions map[string][]Input, start, end *PoseInFrame) {
	t.Helper()
	tf, err := fs.Transform(positions, start, end.FrameName())
	test.That(t, err, test.ShouldBeNil)
	pf, ok := tf.(*PoseInFrame)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, pf.FrameName(), test.ShouldResemble, end.FrameName())
	test.That(t, spatial.PoseAlmostCoincident(pf.Pose(), end.Pose()), test.ShouldBeTrue)
}
