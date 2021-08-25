package referenceframe

import (
	"math"
	"testing"

	spatial "go.viam.com/core/spatialmath"

	"go.viam.com/test"

	"github.com/golang/geo/r3"
)

func TestSimpleRevoluteFrame(t *testing.T) {
	dfs := NewEmptySimpleFrameSystem("test")
	fs := FrameSystem(dfs)

	// Revolute joint around X axis
	joint := NewRevoluteFrame("joint", spatial.R4AA{RX: 1, RY: 0, RZ: 0})
	joint.SetLimits(-math.Pi*2, math.Pi*2)
	fs.AddFrame(joint, fs.World())

	// Displace (2,2,10) from the joint
	point := r3.Vector{2., 2., 10.}
	positions := dfs.StartPositions()

	expectP1 := r3.Vector{2., 2., 10.}
	expectP2 := r3.Vector{2., -10., 2.}
	expectP3 := r3.Vector{2., 10., -2.}

	transformPoint1, err := fs.TransformPoint(positions, point, fs.GetFrame("joint"), fs.World())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint1.X, test.ShouldAlmostEqual, expectP1.X)
	test.That(t, transformPoint1.Y, test.ShouldAlmostEqual, expectP1.Y)
	test.That(t, transformPoint1.Z, test.ShouldAlmostEqual, expectP1.Z)

	// Rotate 90 degrees one way
	positions["joint"] = []Input{Input{math.Pi / 2}}
	transformPoint2, err := fs.TransformPoint(positions, point, fs.GetFrame("joint"), fs.World())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint2.X, test.ShouldAlmostEqual, expectP2.X)
	test.That(t, transformPoint2.Y, test.ShouldAlmostEqual, expectP2.Y)
	test.That(t, transformPoint2.Z, test.ShouldAlmostEqual, expectP2.Z)

	// Rotate 90 degrees the other way
	positions["joint"] = []Input{Input{-math.Pi / 2}}
	transformPoint3, err := fs.TransformPoint(positions, point, fs.GetFrame("joint"), fs.World())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint3.X, test.ShouldAlmostEqual, expectP3.X)
	test.That(t, transformPoint3.Y, test.ShouldAlmostEqual, expectP3.Y)
	test.That(t, transformPoint3.Z, test.ShouldAlmostEqual, expectP3.Z)
}

func TestSimplePrismaticFrame(t *testing.T) {
	dfs := NewEmptySimpleFrameSystem("test")
	fs := FrameSystem(dfs)

	// 1D gantry that slides in X
	gantry := NewPrismaticFrame("gantry", []bool{true, false, false})
	gantry.SetLimits([]float64{-999}, []float64{999})
	fs.AddFrame(gantry, fs.World())

	positions := dfs.StartPositions()

	startPoint := r3.Vector{0., 0., 0.}
	endPoint := r3.Vector{45., 0., 0.}

	// Confirm we start at origin
	transformPoint1, err := fs.TransformPoint(positions, startPoint, fs.GetFrame("gantry"), fs.World())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint1.X, test.ShouldAlmostEqual, 0)
	test.That(t, transformPoint1.Y, test.ShouldAlmostEqual, 0)
	test.That(t, transformPoint1.Z, test.ShouldAlmostEqual, 0)

	// Slide gantry by 45
	positions["gantry"] = []Input{Input{45.}}
	transformPoint2, err := fs.TransformPoint(positions, startPoint, fs.GetFrame("gantry"), fs.World())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint2.X, test.ShouldAlmostEqual, endPoint.X)
	test.That(t, transformPoint2.Y, test.ShouldAlmostEqual, endPoint.Y)
	test.That(t, transformPoint2.Z, test.ShouldAlmostEqual, endPoint.Z)
}
