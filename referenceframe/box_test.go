package referenceframe

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	spatial "go.viam.com/core/spatialmath"
)

var deg45 float64 = math.Pi / 4.

func TestBoxVsBox(t *testing.T) {
	ov := &spatial.OrientationVector{OX: 1, OY: 1, OZ: 1}
	ov.Normalize()

	cases := []struct {
		A        *box
		B        *box
		Expected bool
	}{
		{
			// test inscribed box
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{2, 2, 2}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test face to face contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{2, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test face to face near contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{2.01, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			false,
		},
		{
			// test coincident edge contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{2, 4, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 3, 1}),
			true,
		},
		{
			// test nearly coincident edges (no contact)
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{2, 4.01, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 3, 1}),
			false,
		},
		{
			// test vertex to vertex contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{2, 2, 2}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test vertex to vertex near contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{2.01, 2, 2}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			false,
		},
		{
			// test edge along face contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, &spatial.EulerAngles{Roll: deg45, Pitch: 0, Yaw: 0}), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 1 + math.Sqrt(2), 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test edge along face near contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, &spatial.EulerAngles{Roll: deg45, Pitch: 0, Yaw: 0}), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 1.01 + math.Sqrt(2), 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			false,
		},
		{
			// test edge to edge contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, &spatial.EulerAngles{Roll: 0, Pitch: 0, Yaw: deg45}), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{2 * math.Sqrt(2), 0, 0}, &spatial.EulerAngles{Roll: 0, Pitch: deg45, Yaw: 0}), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test edge to edge near contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{-.01, 0, 0}, &spatial.EulerAngles{Roll: 0, Pitch: 0, Yaw: deg45}), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{2 * math.Sqrt(2), 0, 0}, &spatial.EulerAngles{Roll: 0, Pitch: deg45, Yaw: 0}), r3.Vector{1, 1, 1}),
			false,
		},
		{
			// test vertex to face contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0.5, -.5, 0}, &spatial.EulerAngles{Roll: deg45, Pitch: deg45, Yaw: 0}), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0.97 + math.Sqrt(3)}, &spatial.EulerAngles{Roll: 0, Pitch: 0, Yaw: 0}), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test vertex to face contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, -0.01}, &spatial.EulerAngles{Roll: deg45, Pitch: deg45, Yaw: 0}), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0.97 + math.Sqrt(3)}, &spatial.EulerAngles{Roll: 0, Pitch: 0, Yaw: 0}), r3.Vector{1, 1, 1}),
			false,
		},
	}

	for _, c := range cases {
		fn := test.ShouldBeTrue
		if !c.Expected {
			fn = test.ShouldBeFalse
		}
		test.That(t, boxVsBox(c.A, c.B), fn)
	}
}

func makeBox(pose spatial.Pose, halfsize r3.Vector) *box {
	b, _ := NewBox(halfsize).NewVolume(pose)
	return b.(*box)
}
