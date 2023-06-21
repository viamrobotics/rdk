package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func makeTestCapsule(o Orientation, pt r3.Vector, radius, length float64) Geometry {
	c, _ := NewCapsule(NewPose(pt, o), radius, length, "")
	return c
}

func TestCapsuleConstruction(t *testing.T) {
	c := makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 0.1}, 1, 6.75).(*capsule)
	test.That(t, c.segA.ApproxEqual(r3.Vector{0, 0, -2.275}), test.ShouldBeTrue)
	test.That(t, c.segB.ApproxEqual(r3.Vector{0, 0, 2.475}), test.ShouldBeTrue)
}

func TestBoxCapsuleCollision(t *testing.T) {
	pt := r3.Vector{-178.95551585002903, 15.388321162835881, -10.110465843295357}
	ov := &OrientationVectorDegrees{OX: -0.43716334939336904, OY: -0.3861114135400337, OZ: -0.812284545144919, Theta: -180}
	pose := NewPose(pt, ov)
	c, err := NewCapsule(pose, 65, 550, "")
	test.That(t, err, test.ShouldBeNil)

	box1Pt := r3.Vector{X: -450, Y: 0, Z: -266}
	box1, err := NewBox(NewPoseFromPoint(box1Pt), r3.Vector{X: 900, Y: 2000, Z: 100}, "")
	test.That(t, err, test.ShouldBeNil)

	col, err := c.CollidesWith(box1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeTrue)

	dist, err := c.DistanceFrom(box1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dist, test.ShouldAlmostEqual, -29.69, 1e-3)
}

func TestPointCapsuleCollision(t *testing.T) {
	o := &R4AA{
		Theta: math.Pi / 2,
		RX:    1,
		RY:    0,
		RZ:    0,
	}
	o2 := &R4AA{
		Theta: math.Pi / 4,
		RX:    0,
		RY:    0,
		RZ:    1,
	}
	pt := r3.Vector{X: 2, Y: 2, Z: 0}

	c, err := NewCapsule(NewZeroPose(), 1, 10, "")
	test.That(t, err, test.ShouldBeNil)
	transformedCapsule := c.Transform(NewPoseFromOrientation(o))
	// transform heading
	transformedCapsule = transformedCapsule.Transform(NewPoseFromOrientation(o2))
	// transform center to non-origin point
	transformedCapsule = transformedCapsule.Transform(NewPoseFromPoint(pt))

	col, err := transformedCapsule.CollidesWith(NewPoint(r3.Vector{5, -1, 0}, ""))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeTrue)

	col, err = transformedCapsule.CollidesWith(NewPoint(r3.Vector{0, 0, 0}, ""))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeFalse)
}
