package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/utils"
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

	col, _, err := c.CollidesWith(box1, defaultCollisionBufferMM)
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

// --- Old naive implementations for benchmarking comparison ---

func naiveSeparatingAxisTest1D(positionDelta, capVec *r3.Vector, plane r3.Vector, halfSizeB [3]float64, rmB *RotationMatrix) float64 {
	sum := math.Abs(positionDelta.Dot(plane))
	for i := 0; i < 3; i++ {
		sum -= math.Abs(rmB.Row(i).Mul(halfSizeB[i]).Dot(plane))
	}
	sum -= math.Abs(capVec.Dot(plane))
	return sum
}

func naiveCapsuleVsBoxCollision(c *capsule, b *box, collisionBufferMM float64) (bool, float64) {
	centerDist := b.centerPt.Sub(c.center)
	dist := centerDist.Norm() - ((c.length / 2) + b.boundingSphereR)
	if dist > collisionBufferMM {
		return false, dist
	}
	rmA := c.rotationMatrix()
	rmB := b.rotationMatrix()
	cutoff := collisionBufferMM + c.radius
	for i := 0; i < 3; i++ {
		dist = naiveSeparatingAxisTest1D(&centerDist, &c.capVec, rmA.Row(i), b.halfSize, rmB)
		if dist > cutoff {
			return false, dist
		}
		dist = naiveSeparatingAxisTest1D(&centerDist, &c.capVec, rmB.Row(i), b.halfSize, rmB)
		if dist > cutoff {
			return false, dist
		}
		for j := 0; j < 3; j++ {
			crossProductPlane := rmA.Row(i).Cross(rmB.Row(j))
			if !utils.Float64AlmostEqual(crossProductPlane.Norm(), 0, floatEpsilon) {
				dist = naiveSeparatingAxisTest1D(&centerDist, &c.capVec, crossProductPlane, b.halfSize, rmB)
				if dist > cutoff {
					return false, dist
				}
			}
		}
	}
	return true, -1
}

func naiveCapsuleBoxSeparatingAxisDistance(c *capsule, b *box) float64 {
	centerDist := b.centerPt.Sub(c.center)
	if boundingSphereDist := centerDist.Norm() - ((c.length / 2) + b.boundingSphereR); boundingSphereDist > defaultCollisionBufferMM {
		return boundingSphereDist
	}
	rmA := c.rotationMatrix()
	rmB := b.rotationMatrix()
	max := math.Inf(-1)
	for i := 0; i < 3; i++ {
		if separation := naiveSeparatingAxisTest1D(&centerDist, &c.capVec, rmA.Row(i), b.halfSize, rmB); separation > max {
			max = separation
		}
		if separation := naiveSeparatingAxisTest1D(&centerDist, &c.capVec, rmB.Row(i), b.halfSize, rmB); separation > max {
			max = separation
		}
		for j := 0; j < 3; j++ {
			crossProductPlane := rmA.Row(i).Cross(rmB.Row(j))
			if !utils.Float64AlmostEqual(crossProductPlane.Norm(), 0, floatEpsilon) {
				if separation := naiveSeparatingAxisTest1D(&centerDist, &c.capVec, crossProductPlane, b.halfSize, rmB); separation > max {
					max = separation
				}
			}
		}
	}
	return max - c.radius
}

func capsuleBoxBenchCases() []struct {
	name string
	c    *capsule
	b    *box
	buf  float64
} {
	deg45 := math.Pi / 4.0
	return []struct {
		name string
		c    *capsule
		b    *box
		buf  float64
	}{
		{
			"far_apart",
			makeTestCapsule(NewZeroOrientation(), r3.Vector{}, 5, 20).(*capsule),
			makeTestBox(NewZeroOrientation(), r3.Vector{100, 100, 0}, r3.Vector{10, 10, 10}).(*box),
			1,
		},
		{
			"moderate_distance",
			makeTestCapsule(NewZeroOrientation(), r3.Vector{}, 5, 20).(*capsule),
			makeTestBox(NewZeroOrientation(), r3.Vector{20, 0, 0}, r3.Vector{10, 10, 10}).(*box),
			1,
		},
		{
			"near_miss_rotated",
			makeTestCapsule(&EulerAngles{deg45, deg45, 0}, r3.Vector{0, 0, 0}, 5, 30).(*capsule),
			makeTestBox(&EulerAngles{0, deg45, 0}, r3.Vector{15, 0, 0}, r3.Vector{10, 10, 10}).(*box),
			0,
		},
		{
			"colliding",
			makeTestCapsule(NewZeroOrientation(), r3.Vector{}, 5, 20).(*capsule),
			makeTestBox(NewZeroOrientation(), r3.Vector{3, 0, 0}, r3.Vector{10, 10, 10}).(*box),
			0,
		},
		{
			"rotated_deep_penetration",
			makeTestCapsule(&EulerAngles{0.3, 0.5, 0.7}, r3.Vector{1, 2, 3}, 10, 40).(*capsule),
			makeTestBox(&EulerAngles{0.7, 0.1, 0.3}, r3.Vector{5, 5, 5}, r3.Vector{20, 20, 20}).(*box),
			1,
		},
		{
			"elongated_capsule",
			makeTestCapsule(&EulerAngles{0, deg45, 0}, r3.Vector{}, 3, 100).(*capsule),
			makeTestBox(&EulerAngles{deg45, 0, 0}, r3.Vector{10, 0, 0}, r3.Vector{6, 6, 6}).(*box),
			0.5,
		},
	}
}

func BenchmarkCapsuleBoxCollision(b *testing.B) {
	for _, tc := range capsuleBoxBenchCases() {
		b.Run("naive/"+tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				naiveCapsuleVsBoxCollision(tc.c, tc.b, tc.buf)
			}
		})
		b.Run("optimized/"+tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				capsuleVsBoxCollision(tc.c, tc.b, tc.buf)
			}
		})
	}
}

func BenchmarkCapsuleBoxDistance(b *testing.B) {
	for _, tc := range capsuleBoxBenchCases() {
		b.Run("naive/"+tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				naiveCapsuleBoxSeparatingAxisDistance(tc.c, tc.b)
			}
		})
		b.Run("optimized/"+tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				capsuleBoxSeparatingAxisDistance(tc.c, tc.b)
			}
		})
	}
}
