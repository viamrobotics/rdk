package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

var deg45 float64 = math.Pi / 4.

func TestNewBoxFromOffset(t *testing.T) {
	pt := r3.Vector{X: 1, Y: 0, Z: 0}
	offset := NewPoseFromOrientation(pt, &EulerAngles{0, 0, math.Pi})
	vol := NewBoxFromOffset(r3.Vector{}, offset).NewVolume(NewPoseFromPoint(pt))
	test.That(t, AlmostCoincident(vol.Pose(), NewZeroPose()), test.ShouldBeTrue)
}

func TestBoxVsBox(t *testing.T) {
	cases := []struct {
		A        *box
		B        *box
		Expected bool
	}{
		{
			// test inscribed box
			makeBox(NewPoseFromOrientation(r3.Vector{0, 0, 0}, NewZeroOrientation()), r3.Vector{2, 2, 2}),
			makeBox(NewPoseFromOrientation(r3.Vector{0, 0, 0}, NewZeroOrientation()), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test face to face contact
			makeBox(NewPoseFromOrientation(r3.Vector{0, 0, 0}, NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(NewPoseFromOrientation(r3.Vector{2, 0, 0}, NewZeroOrientation()), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test face to face near contact
			makeBox(NewPoseFromOrientation(r3.Vector{0, 0, 0}, NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(NewPoseFromOrientation(r3.Vector{2.01, 0, 0}, NewZeroOrientation()), r3.Vector{1, 1, 1}),
			false,
		},
		{
			// test coincident edge contact
			makeBox(NewPoseFromOrientation(r3.Vector{0, 0, 0}, NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(NewPoseFromOrientation(r3.Vector{2, 4, 0}, NewZeroOrientation()), r3.Vector{1, 3, 1}),
			true,
		},
		{
			// test nearly coincident edges (no contact)
			makeBox(NewPoseFromOrientation(r3.Vector{0, 0, 0}, NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(NewPoseFromOrientation(r3.Vector{2, 4.01, 0}, NewZeroOrientation()), r3.Vector{1, 3, 1}),
			false,
		},
		{
			// test vertex to vertex contact
			makeBox(NewPoseFromOrientation(r3.Vector{0, 0, 0}, NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(NewPoseFromOrientation(r3.Vector{2, 2, 2}, NewZeroOrientation()), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test vertex to vertex near contact
			makeBox(NewPoseFromOrientation(r3.Vector{0, 0, 0}, NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(NewPoseFromOrientation(r3.Vector{2.01, 2, 2}, NewZeroOrientation()), r3.Vector{1, 1, 1}),
			false,
		},
		{
			// test edge along face contact
			makeBox(NewPoseFromOrientation(r3.Vector{0, 0, 0}, &EulerAngles{deg45, 0, 0}), r3.Vector{1, 1, 1}),
			makeBox(NewPoseFromOrientation(r3.Vector{0, 1 + math.Sqrt(2), 0}, NewZeroOrientation()), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test edge along face near contact
			makeBox(NewPoseFromOrientation(r3.Vector{0, 0, 0}, &EulerAngles{deg45, 0, 0}), r3.Vector{1, 1, 1}),
			makeBox(NewPoseFromOrientation(r3.Vector{0, 1.01 + math.Sqrt(2), 0}, NewZeroOrientation()), r3.Vector{1, 1, 1}),
			false,
		},
		{
			// test edge to edge contact
			makeBox(NewPoseFromOrientation(r3.Vector{0, 0, 0}, &EulerAngles{0, 0, deg45}), r3.Vector{1, 1, 1}),
			makeBox(NewPoseFromOrientation(r3.Vector{2 * math.Sqrt(2), 0, 0}, &EulerAngles{0, deg45, 0}), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test edge to edge near contact
			makeBox(NewPoseFromOrientation(r3.Vector{-.01, 0, 0}, &EulerAngles{0, 0, deg45}), r3.Vector{1, 1, 1}),
			makeBox(NewPoseFromOrientation(r3.Vector{2 * math.Sqrt(2), 0, 0}, &EulerAngles{0, deg45, 0}), r3.Vector{1, 1, 1}),
			false,
		},
		{
			// test vertex to face contact
			makeBox(NewPoseFromOrientation(r3.Vector{0.5, -.5, 0}, &EulerAngles{deg45, deg45, 0}), r3.Vector{1, 1, 1}),
			makeBox(NewPoseFromOrientation(r3.Vector{0, 0, 0.97 + math.Sqrt(3)}, &EulerAngles{0, 0, 0}), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test vertex to face contact
			makeBox(NewPoseFromOrientation(r3.Vector{0, 0, -0.01}, &EulerAngles{deg45, deg45, 0}), r3.Vector{1, 1, 1}),
			makeBox(NewPoseFromOrientation(r3.Vector{0, 0, 0.97 + math.Sqrt(3)}, &EulerAngles{0, 0, 0}), r3.Vector{1, 1, 1}),
			false,
		},
	}

	for _, c := range cases {
		fn := test.ShouldBeTrue
		if !c.Expected {
			fn = test.ShouldBeFalse
		}
		test.That(t, boxVsBoxCollision(c.A, c.B), fn)
	}
}

func makeBox(pose Pose, halfsize r3.Vector) *box {
	b := NewBoxFromOffset(halfsize, pose).NewVolume(NewZeroPose())
	return b.(*box)
}
