package referenceframe

import (
	"math"
	"testing"

	"go.viam.com/test"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

func TestDualQuatTransform(t *testing.T) {
	// Start with point [3, 4, 5] - Rotate by 180 degrees around x-axis and then displace by [4,2,6]
	pt := NewPoseFromPoint(r3.Vector{3., 4., 5.})                       // starting point
	tr := NewPose(r3.Vector{4., 2., 6.}, quat.Number{Real: 0, Imag: 1}) // transformation to do
	tr1 := dualquat.Mul(tr.Translation(), tr.Rotation())                // rotation first, then translation
	test.That(t, tr.Transform(), test.ShouldResemble, tr1)              // Transform() also does rotation, then translation

	transformedPoint := dualquat.Mul(dualquat.Mul(tr.Transform(), pt.PointDualQuat()), dualquat.Conj(tr.Transform()))
	transformedPose, err := NewPoseFromPointDualQuat(transformedPoint)
	test.That(t, err, test.ShouldBeNil)
	expectedPose := NewPoseFromPoint(r3.Vector{7., -2., 1.})

	test.That(t, transformedPose.Point(), test.ShouldResemble, expectedPose.Point())
	test.That(t, transformedPose.PointDualQuat(), test.ShouldResemble, expectedPose.PointDualQuat())
	test.That(t, transformedPose.Rotation(), test.ShouldResemble, expectedPose.Rotation())
	test.That(t, transformedPose.Translation(), test.ShouldResemble, expectedPose.Translation())
	test.That(t, transformedPose.Transform(), test.ShouldResemble, expectedPose.Transform())

	// Start with point [3, 4, 5] - displace by [4, 2, 6] and then rotate by 180 around the x axis.
	tr2 := dualquat.Mul(tr.Rotation(), tr.Translation()) // translation first, then rotation
	transformedPoint2 := dualquat.Mul(dualquat.Mul(tr2, pt.PointDualQuat()), dualquat.Conj(tr2))
	transformedPose2, err := NewPoseFromPointDualQuat(transformedPoint2)
	test.That(t, err, test.ShouldBeNil)
	expectedPose2 := NewPoseFromPoint(r3.Vector{7., -6., -11.})

	test.That(t, transformedPose2.Point(), test.ShouldResemble, expectedPose2.Point())
	test.That(t, transformedPose2.PointDualQuat(), test.ShouldResemble, expectedPose2.PointDualQuat())
	test.That(t, transformedPose2.Rotation(), test.ShouldResemble, expectedPose2.Rotation())
	test.That(t, transformedPose2.Translation(), test.ShouldResemble, expectedPose2.Translation())
	test.That(t, transformedPose2.Transform(), test.ShouldResemble, expectedPose2.Transform())

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
	worldPose := NewPoseFromPoint(pointWorld)
	pointFrame := r3.Vector{1., 0., 0.} // the point from PoV of frame
	framePose := NewPoseFromPoint(pointFrame)

	// transform point from world to frame
	t.Logf("begin: %v", worldPose)
	transformPose1, err := fs.TransformPose(worldPose, fs.GetFrame("world"), fs.GetFrame("frame"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPose1, test.ShouldResemble, framePose)

	// transform point from frame to world
	t.Logf("begin: %v", framePose)
	transformPose2, err := fs.TransformPose(framePose, fs.GetFrame("frame"), fs.GetFrame("world"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPose2, test.ShouldResemble, worldPose)
}

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
	sourcePose := NewPoseFromPoint(pointFrame1)
	pointFrame2 := r3.Vector{0., 6., 0.} // the point from PoV of frame 2
	expectedPose := NewPoseFromPoint(pointFrame2)
	transformPose, err := fs.TransformPose(sourcePose, fs.GetFrame("frame1"), fs.GetFrame("frame2"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPose, test.ShouldResemble, expectedPose)
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
	frame3 := r3.Vector{0., 4., 0.} // location of frame3 with respect to world frame
	err := fs.SetFrameFromPoint("frame3", fs.World(), frame3)
	test.That(t, err, test.ShouldBeNil)
	frame1 := r3.Vector{0., 3., 0.} // location of frame1 with respect to frame3
	err = fs.SetFrameFromPoint("frame1", fs.GetFrame("frame3"), frame1)
	test.That(t, err, test.ShouldBeNil)
	// location of frame2 with respect to world frame
	rot := math.Pi / 2.
	frame2 := NewPose(r3.Vector{5., 1., 0.}, quat.Number{math.Cos(rot / 2), 0, 0, math.Sin(rot / 2)})
	err = fs.SetFrameFromPose("frame2", fs.World(), frame2)
	test.That(t, err, test.ShouldBeNil)

	// do the transformation
	pointFrame1 := r3.Vector{5., 0., 0.} // the point from PoV of frame 1
	sourcePose := NewPoseFromPoint(pointFrame1)
	pointFrame2 := r3.Vector{6., 0., 0.} // the point from PoV of frame 2
	expectedPose := NewPoseFromPoint(pointFrame2)
	transformPose, err := fs.TransformPose(sourcePose, fs.GetFrame("frame1"), fs.GetFrame("frame2"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPose.Point().X, test.ShouldAlmostEqual, expectedPose.Point().X)
	test.That(t, transformPose.Point().Y, test.ShouldAlmostEqual, expectedPose.Point().Y)
	test.That(t, transformPose.Point().Z, test.ShouldAlmostEqual, expectedPose.Point().Z)
	test.That(t, transformPose.Rotation(), test.ShouldResemble, expectedPose.Rotation())
}
