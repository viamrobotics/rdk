package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestNewBoxFromOffset(t *testing.T) {
	pt := r3.Vector{X: 1, Y: 0, Z: 0}
	offset := NewPoseFromOrientation(pt, &EulerAngles{0, 0, math.Pi})
	vc, err := NewBox(r3.Vector{1, 1, 1}, offset)
	test.That(t, err, test.ShouldBeNil)
	vol := vc.NewVolume(Invert(offset))
	test.That(t, PoseAlmostCoincident(vol.Pose(), NewZeroPose()), test.ShouldBeTrue)
	quatCompare(t, vol.Pose().Orientation().Quaternion(), NewZeroOrientation().Quaternion())
}

func TestBoxAlmostEqual(t *testing.T) {
	original := makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{1, 1, 1})
	good := makeBox(NewZeroOrientation(), r3.Vector{1e-16, 1e-16, 1e-16}, r3.Vector{1 + 1e-16, 1 + 1e-16, 1 + 1e-16})
	bad := makeBox(NewZeroOrientation(), r3.Vector{1e-2, 1e-2, 1e-2}, r3.Vector{1 + 1e-2, 1 + 1e-2, 1 + 1e-2})
	test.That(t, original.AlmostEqual(good), test.ShouldBeTrue)
	test.That(t, original.AlmostEqual(bad), test.ShouldBeFalse)
}

func TestBoxVertices(t *testing.T) {
	offset := r3.Vector{2, 2, 2}
	box := makeBox(NewZeroOrientation(), offset, r3.Vector{2, 2, 2})
	vertices := box.Vertices()
	test.That(t, R3VectorAlmostEqual(vertices[0], r3.Vector{1, 1, 1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[1], r3.Vector{1, 1, -1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[2], r3.Vector{1, -1, 1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[3], r3.Vector{1, -1, -1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[4], r3.Vector{-1, 1, 1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[5], r3.Vector{-1, 1, -1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[6], r3.Vector{-1, -1, 1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[7], r3.Vector{-1, -1, -1}.Add(offset), 1e-8), test.ShouldBeTrue)
}

func TestBoxVsBoxCollision(t *testing.T) {
	deg45 := math.Pi / 4.
	cases := []struct {
		testname string
		a        Volume
		b        Volume
		expected bool
	}{
		{
			"test inscribed box",
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{1, 1, 1}),
			true,
		},
		{
			"test face to face contact",
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
			makeBox(NewZeroOrientation(), r3.Vector{2, 0, 0}, r3.Vector{2, 2, 2}),
			true,
		},
		{
			"test face to face near contact",
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
			makeBox(NewZeroOrientation(), r3.Vector{2.01, 0, 0}, r3.Vector{2, 2, 2}),
			false,
		},
		{
			"test coincident edge contact",
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
			makeBox(NewZeroOrientation(), r3.Vector{2, 4, 0}, r3.Vector{2, 6, 2}),
			true,
		},
		{
			"test nearly coincident edges (no contact)",
			makeBox((NewZeroOrientation()), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
			makeBox(NewZeroOrientation(), r3.Vector{2, 4.01, 0}, r3.Vector{2, 6, 2}),
			false,
		},
		{
			"test vertex to vertex contact",
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
			makeBox(NewZeroOrientation(), r3.Vector{2, 2, 2}, r3.Vector{2, 2, 2}),
			true,
		},
		{
			"test vertex to vertex near contact",
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
			makeBox(NewZeroOrientation(), r3.Vector{2.01, 2, 2}, r3.Vector{2, 2, 2}),
			false,
		},
		{
			"test edge along face contact",
			makeBox(&EulerAngles{deg45, 0, 0}, r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
			makeBox(NewZeroOrientation(), r3.Vector{0, 1 + math.Sqrt(2), 0}, r3.Vector{2, 2, 2}),
			true,
		},
		{
			"test edge along face near contact",
			makeBox(&EulerAngles{deg45, 0, 0}, r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
			makeBox(NewZeroOrientation(), r3.Vector{0, 1.01 + math.Sqrt(2), 0}, r3.Vector{2, 2, 2}),
			false,
		},
		{
			"test edge to edge contact",
			makeBox(&EulerAngles{0, 0, deg45}, r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
			makeBox(&EulerAngles{0, deg45, 0}, r3.Vector{2 * math.Sqrt(2), 0, 0}, r3.Vector{2, 2, 2}),
			true,
		},
		{
			"test edge to edge near contact",
			makeBox(&EulerAngles{0, 0, deg45}, r3.Vector{-.01, 0, 0}, r3.Vector{2, 2, 2}),
			makeBox(&EulerAngles{0, deg45, 0}, r3.Vector{2 * math.Sqrt(2), 0, 0}, r3.Vector{2, 2, 2}),
			false,
		},
		{
			"test vertex to face contact",
			makeBox(&EulerAngles{deg45, deg45, 0}, r3.Vector{0.5, -.5, 0}, r3.Vector{2, 2, 2}),
			makeBox(&EulerAngles{0, 0, 0}, r3.Vector{0, 0, 0.97 + math.Sqrt(3)}, r3.Vector{2, 2, 2}),
			true,
		},
		{
			"test vertex to face contact",
			makeBox(&EulerAngles{deg45, deg45, 0}, r3.Vector{0, 0, -0.01}, r3.Vector{2, 2, 2}),
			makeBox(&EulerAngles{0, 0, 0}, r3.Vector{0, 0, 0.97 + math.Sqrt(3)}, r3.Vector{2, 2, 2}),
			false,
		},
	}

	for _, c := range cases {
		t.Run(c.testname, func(t *testing.T) {
			fn := test.ShouldBeTrue
			if !c.expected {
				fn = test.ShouldBeFalse
			}
			collides, err := c.a.CollidesWith(c.b)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, collides, fn)
		})
	}
}

func TestBoxVsBoxDistance(t *testing.T) {
	cases := []struct {
		testname string
		a        Volume
		b        Volume
		expected float64
	}{
		{
			"test two identical boxes",
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 4, 6}),
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 4, 6}),
			-2,
		},
		{
			"test axis aligned no collision",
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
			makeBox(NewZeroOrientation(), r3.Vector{5, 6, 0}, r3.Vector{2, 2, 2}),
			4, // upper bound on separation distance
		},
		{
			"test axis aligned collision",
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{20, 20, 20}),
			makeBox(NewZeroOrientation(), r3.Vector{20, 20, 20}, r3.Vector{24, 26, 28}),
			-2,
		},
		{
			"test face to face",
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
			makeBox(NewZeroOrientation(), r3.Vector{2, 0, 0}, r3.Vector{2, 2, 2}),
			0,
		},
	}

	for _, c := range cases {
		t.Run(c.testname, func(t *testing.T) {
			distance, err := c.a.DistanceFrom(c.b)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, distance, test.ShouldAlmostEqual, c.expected)
		})
	}
}

func makeBox(o Orientation, point, halfsize r3.Vector) *box {
	vc, _ := NewBox(halfsize, NewPoseFromOrientation(point, o))
	return vc.NewVolume(NewZeroPose()).(*box)
}
