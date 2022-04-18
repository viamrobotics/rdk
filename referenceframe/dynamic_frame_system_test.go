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

	// Displace (2,2,10) from the joint
	point := r3.Vector{2., 2., 10.}
	positions := StartPositions(fs)

	expectP1 := r3.Vector{2., 2., 10.}
	expectP2 := r3.Vector{2., -10., 2.}
	expectP3 := r3.Vector{2., 10., -2.}

	transformPoint1, err := fs.TransformPoint(positions, point, "joint", World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint1.X, test.ShouldAlmostEqual, expectP1.X)
	test.That(t, transformPoint1.Y, test.ShouldAlmostEqual, expectP1.Y)
	test.That(t, transformPoint1.Z, test.ShouldAlmostEqual, expectP1.Z)

	// Rotate 90 degrees one way
	positions["joint"] = []Input{{Value: math.Pi / 2, Units: Radians}}
	transformPoint2, err := fs.TransformPoint(positions, point, "joint", World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint2.X, test.ShouldAlmostEqual, expectP2.X)
	test.That(t, transformPoint2.Y, test.ShouldAlmostEqual, expectP2.Y)
	test.That(t, transformPoint2.Z, test.ShouldAlmostEqual, expectP2.Z)

	// Rotate 90 degrees the other way
	positions["joint"] = []Input{{Value: -math.Pi / 2, Units: Radians}}
	transformPoint3, err := fs.TransformPoint(positions, point, "joint", World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint3.X, test.ShouldAlmostEqual, expectP3.X)
	test.That(t, transformPoint3.Y, test.ShouldAlmostEqual, expectP3.Y)
	test.That(t, transformPoint3.Z, test.ShouldAlmostEqual, expectP3.Z)
}

func TestSimpleTranslationalFrame(t *testing.T) {
	fs := NewEmptySimpleFrameSystem("test")

	// 1D gantry that slides in X
	gantry, err := NewTranslationalFrame("gantry", r3.Vector{1, 0, 0}, Limit{Min: math.Inf(-1), Max: math.Inf(1)})
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantry, fs.World())

	positions := StartPositions(fs)

	startPoint := r3.Vector{0., 0., 0.}
	endPoint := r3.Vector{45., 0., 0.}

	// Confirm we start at origin
	transformPoint1, err := fs.TransformPoint(positions, startPoint, "gantry", World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint1.X, test.ShouldAlmostEqual, 0)
	test.That(t, transformPoint1.Y, test.ShouldAlmostEqual, 0)
	test.That(t, transformPoint1.Z, test.ShouldAlmostEqual, 0)

	// Slide gantry by 45
	positions["gantry"] = []Input{{Value: 45., Units: Millimeters}}
	transformPoint2, err := fs.TransformPoint(positions, startPoint, "gantry", World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint2.X, test.ShouldAlmostEqual, endPoint.X)
	test.That(t, transformPoint2.Y, test.ShouldAlmostEqual, endPoint.Y)
	test.That(t, transformPoint2.Z, test.ShouldAlmostEqual, endPoint.Z)
}
