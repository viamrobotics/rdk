package referenceframe

import (
	"math"
	"testing"

	spatial "go.viam.com/core/spatialmath"

	"go.viam.com/test"

	"github.com/golang/geo/r3"
)

// The below verifies that a dynamic frame system functions as a static frame system
// A complicated frame transform with multiple branching frames and rotations
func TestComplicatedFrameTransformDynamic(t *testing.T) {
	// build the system
	dfs := NewEmptyDynamicFrameSystem("test")
	fs := FrameSystem(dfs)

	// frame 1 rotate by 45 degrees around z axis and translate
	frame1 := NewStaticFrame("frame1", fs.World(), NewPoseFromAxisAngle(r3.Vector{-1., 2., 0.}, r3.Vector{0., 0., 1.}, math.Pi/4))
	
	err := fs.SetFrame(frame1)
	test.That(t, err, test.ShouldBeNil)
	// frame 2 rotate by 45 degree (relative to frame 1) around z axis and translate
	frame2 := NewStaticFrame("frame2", fs.GetFrame("frame1"), NewPoseFromAxisAngle(r3.Vector{2. * math.Sqrt(2), 0., 0.}, r3.Vector{0., 0., 1.}, math.Pi/4))
	err = fs.SetFrame(frame2)
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
	frame3 := NewStaticFrame("frame3", fs.World(), NewPoseFromAxisAngle(r3.Vector{}, r3.Vector{0., 1., 0.}, math.Pi/2))
	err = fs.SetFrame(frame3)
	test.That(t, err, test.ShouldBeNil)

	// frame4 - pure translation
	frame4 := NewStaticFrame("frame4", fs.GetFrame("frame3"), NewPoseFromPoint(r3.Vector{-2., 7., 1.}))
	err = fs.SetFrame(frame4)
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

func TestSimpleRevoluteFrame(t *testing.T) {
	dfs := NewEmptyDynamicFrameSystem("test")
	fs := FrameSystem(dfs)
	
	// Revolute joint around X axis
	joint := NewRevoluteFrame("joint", fs.World(), spatial.R4AA{RX:1,RY:0,RZ:0})
	joint.SetLimits(-math.Pi*2, math.Pi*2)
	fs.SetFrame(joint)
	
	// Displace (2,2,10) from the joint
	frame := NewStaticFrame("frame", joint, NewPoseFromPoint(r3.Vector{2., 2., 10.}))
	fs.SetFrame(frame)
	
	origin := r3.Vector{0., 0., 0.}
	expectP1 := r3.Vector{2., 2., 10.}
	expectP2 := r3.Vector{2., -10., 2.}
	expectP3 := r3.Vector{2., 10., -2.}
	
	transformPoint1, err := fs.TransformPoint(origin, fs.GetFrame("frame"), fs.World())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint1.X, test.ShouldAlmostEqual, expectP1.X)
	test.That(t, transformPoint1.Y, test.ShouldAlmostEqual, expectP1.Y)
	test.That(t, transformPoint1.Z, test.ShouldAlmostEqual, expectP1.Z)
	
	// Rotate 90 degrees one way
	dfs.SetPosition("joint", []Input{Input{math.Pi/2}})
	transformPoint2, err := fs.TransformPoint(origin, fs.GetFrame("frame"), fs.World())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint2.X, test.ShouldAlmostEqual, expectP2.X)
	test.That(t, transformPoint2.Y, test.ShouldAlmostEqual, expectP2.Y)
	test.That(t, transformPoint2.Z, test.ShouldAlmostEqual, expectP2.Z)
	
	// Rotate 90 degrees the other way
	dfs.SetPosition("joint", []Input{Input{-math.Pi/2}})
	transformPoint3, err := fs.TransformPoint(origin, fs.GetFrame("frame"), fs.World())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint3.X, test.ShouldAlmostEqual, expectP3.X)
	test.That(t, transformPoint3.Y, test.ShouldAlmostEqual, expectP3.Y)
	test.That(t, transformPoint3.Z, test.ShouldAlmostEqual, expectP3.Z)
}

func TestSimplePrismaticFrame(t *testing.T) {
	dfs := NewEmptyDynamicFrameSystem("test")
	fs := FrameSystem(dfs)
	
	// 1D gantry that slides in X
	gantry := NewPrismaticFrame("gantry", fs.World(), []bool{true, false, false})
	gantry.SetLimits([]float64{-999}, []float64{999})
	fs.SetFrame(gantry)
	
	origin := r3.Vector{0., 0., 0.}
	endPoint := r3.Vector{45., 0., 0.}
	
	// Confirm we start at origin
	transformPoint1, err := fs.TransformPoint(origin, fs.GetFrame("gantry"), fs.World())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint1.X, test.ShouldAlmostEqual, origin.X)
	test.That(t, transformPoint1.Y, test.ShouldAlmostEqual, origin.Y)
	test.That(t, transformPoint1.Z, test.ShouldAlmostEqual, origin.Z)
	
	// Slide gantry by 45
	dfs.SetPosition("gantry", []Input{Input{45.}})
	transformPoint2, err := fs.TransformPoint(origin, fs.GetFrame("gantry"), fs.World())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint2.X, test.ShouldAlmostEqual, endPoint.X)
	test.That(t, transformPoint2.Y, test.ShouldAlmostEqual, endPoint.Y)
	test.That(t, transformPoint2.Z, test.ShouldAlmostEqual, endPoint.Z)
}
