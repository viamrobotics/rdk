package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

var deg45 = math.Pi / 4.

func TestNewBoxFromOffset(t *testing.T) {
	pt := r3.Vector{X: 1, Y: 0, Z: 0}
	offset := NewPoseFromOrientation(pt, &EulerAngles{0, 0, math.Pi})
	vol := NewBoxFromOffset(r3.Vector{}, offset).NewVolume(Invert(offset))
	test.That(t, AlmostCoincident(vol.Pose(), NewZeroPose()), test.ShouldBeTrue)
	quatCompare(t, vol.Pose().Orientation().Quaternion(), NewZeroOrientation().Quaternion())
}

func TestBoxVsBox(t *testing.T) {
	cases := []struct {
		testname string
		a        *box
		b        *box
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
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{1, 1, 1}),
			makeBox(NewZeroOrientation(), r3.Vector{2, 0, 0}, r3.Vector{1, 1, 1}),
			true,
		},
		{
			"test face to face near contact",
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{1, 1, 1}),
			makeBox(NewZeroOrientation(), r3.Vector{2.01, 0, 0}, r3.Vector{1, 1, 1}),
			false,
		},
		{
			"test coincident edge contact",
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{1, 1, 1}),
			makeBox(NewZeroOrientation(), r3.Vector{2, 4, 0}, r3.Vector{1, 3, 1}),
			true,
		},
		{
			"test nearly coincident edges (no contact)",
			makeBox((NewZeroOrientation()), r3.Vector{0, 0, 0}, r3.Vector{1, 1, 1}),
			makeBox(NewZeroOrientation(), r3.Vector{2, 4.01, 0}, r3.Vector{1, 3, 1}),
			false,
		},
		{
			"test vertex to vertex contact",
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{1, 1, 1}),
			makeBox(NewZeroOrientation(), r3.Vector{2, 2, 2}, r3.Vector{1, 1, 1}),
			true,
		},
		{
			"test vertex to vertex near contact",
			makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{1, 1, 1}),
			makeBox(NewZeroOrientation(), r3.Vector{2.01, 2, 2}, r3.Vector{1, 1, 1}),
			false,
		},
		{
			"test edge along face contact",
			makeBox(&EulerAngles{deg45, 0, 0}, r3.Vector{0, 0, 0}, r3.Vector{1, 1, 1}),
			makeBox(NewZeroOrientation(), r3.Vector{0, 1 + math.Sqrt(2), 0}, r3.Vector{1, 1, 1}),
			true,
		},
		{
			"test edge along face near contact",
			makeBox(&EulerAngles{deg45, 0, 0}, r3.Vector{0, 0, 0}, r3.Vector{1, 1, 1}),
			makeBox(NewZeroOrientation(), r3.Vector{0, 1.01 + math.Sqrt(2), 0}, r3.Vector{1, 1, 1}),
			false,
		},
		{
			"test edge to edge contact",
			makeBox(&EulerAngles{0, 0, deg45}, r3.Vector{0, 0, 0}, r3.Vector{1, 1, 1}),
			makeBox(&EulerAngles{0, deg45, 0}, r3.Vector{2 * math.Sqrt(2), 0, 0}, r3.Vector{1, 1, 1}),
			true,
		},
		{
			"test edge to edge near contact",
			makeBox(&EulerAngles{0, 0, deg45}, r3.Vector{-.01, 0, 0}, r3.Vector{1, 1, 1}),
			makeBox(&EulerAngles{0, deg45, 0}, r3.Vector{2 * math.Sqrt(2), 0, 0}, r3.Vector{1, 1, 1}),
			false,
		},
		{
			"test vertex to face contact",
			makeBox(&EulerAngles{deg45, deg45, 0}, r3.Vector{0.5, -.5, 0}, r3.Vector{1, 1, 1}),
			makeBox(&EulerAngles{0, 0, 0}, r3.Vector{0, 0, 0.97 + math.Sqrt(3)}, r3.Vector{1, 1, 1}),
			true,
		},
		{
			"test vertex to face contact",
			makeBox(&EulerAngles{deg45, deg45, 0}, r3.Vector{0, 0, -0.01}, r3.Vector{1, 1, 1}),
			makeBox(&EulerAngles{0, 0, 0}, r3.Vector{0, 0, 0.97 + math.Sqrt(3)}, r3.Vector{1, 1, 1}),
			false,
		},
	}

	for _, c := range cases {
		t.Run(c.testname, func(t *testing.T) {
			fn := test.ShouldBeTrue
			if !c.expected {
				fn = test.ShouldBeFalse
			}
			test.That(t, boxVsBoxCollision(c.a, c.b), fn)
		})
	}
}

func makeBox(o Orientation, point, halfsize r3.Vector) *box {
	return NewBoxFromOffset(halfsize, NewPoseFromOrientation(point, o)).NewVolume(NewZeroPose()).(*box)
}
