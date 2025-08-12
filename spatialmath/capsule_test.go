package spatialmath

import (
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

	col, err := c.CollidesWith(box1, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeTrue)

	dist, err := c.DistanceFrom(box1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dist, test.ShouldAlmostEqual, -29.69, 1e-3)
}

func TestCapsuleIntersectWithPlane(t *testing.T) {
	c := makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0.1, 0.1}, 1, 16.75).(*capsule)
	points, err := CapsuleIntersectionWithPlane(c, r3.Vector{0, 1, 0}, r3.Vector{1, 0, 0}, 32)
	test.That(t, err, test.ShouldBeNil)

	expectedPoints := []r3.Vector{
		{1.00000, 0.1, -8.27500},
		{1.00000, 0.1, -5.88214},
		{1.00000, 0.1, -3.48928},
		{1.00000, 0.1, -1.09642},
		{1.00000, 0.1, 1.29642},
		{1.00000, 0.1, 3.68928},
		{1.00000, 0.1, 6.08214},
		{1.00000, 0.1, 8.47499},
		{1.00000, 0.1, 8.47499},
		{0.93969, 0.1, 8.13297},
		{0.76604, 0.1, 7.83221},
		{0.50000, 0.1, 7.60897},
		{0.17364, 0.1, 7.49019},
		{-0.17364, 0.1, 7.49019},
		{-0.49999, 0.1, 7.60897},
		{-0.76604, 0.1, 7.83221},
		{-0.93969, 0.1, 8.13297},
		{-1.00000, 0.1, 8.47499},
		{-1.00000, 0.1, 6.08214},
		{-1.00000, 0.1, 3.68928},
		{-1.00000, 0.1, 1.29642},
		{-1.00000, 0.1, -1.09642},
		{-1.00000, 0.1, -3.48928},
		{-1.00000, 0.1, -5.88214},
		{-1.00000, 0.1, -8.27500},
		{-1.00000, 0.1, -8.27500},
		{-0.93969, 0.1, -8.61702},
		{-0.76604, 0.1, -8.91778},
		{-0.50000, 0.1, -9.14102},
		{-0.17364, 0.1, -9.25980},
		{0.17364, 0.1, -9.25980},
		{0.49999, 0.1, -9.14102},
		{0.76604, 0.1, -8.91778},
		{0.93969, 0.1, -8.61702},
	}

	test.That(t, len(points), test.ShouldEqual, len(expectedPoints))

	for i, pt := range points {
		test.That(t, pt.X, test.ShouldAlmostEqual, expectedPoints[i].X, 0.0001)
		test.That(t, pt.Y, test.ShouldAlmostEqual, expectedPoints[i].Y, 0.0001)
		test.That(t, pt.Z, test.ShouldAlmostEqual, expectedPoints[i].Z, 0.0001)
	}
}
