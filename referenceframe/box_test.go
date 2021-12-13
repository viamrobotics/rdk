package referenceframe

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	spatial "go.viam.com/core/spatialmath"
)

func TestBoxVsBox(t *testing.T) {
	cases := []struct {
		A        *box
		B        *box
		Expected bool
	}{
		{
			// test no collision
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{2.1, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			false,
		},
		{
			// test face to face contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{2, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test inscribed box
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{2, 2, 2}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test edge to edge contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{2, 4, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 3, 1}),
			true,
		},
		// {
		// 	// test edge intersecting face
		// 	NewBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
		// 	NewBox(spatial.NewPoseFromOrientation(r3.Vector{2, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
		// 	true,
		// },
		// {
		// 	// test edge along face contact
		// },
		{
			// test vertex to vertex contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{2, 2, 2}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			true,
		},
		// {
		// 	// test edge to vertex contact
		// },
		{
			// test vertex to face contact
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, &spatial.EulerAngles{math.Pi / 4., math.Pi / 4., math.Pi / 4.}), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{1. + math.Sqrt(3)/2, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			true,
		},
		{
			// test vertex to face near collision
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, &spatial.EulerAngles{math.Pi / 4., math.Pi / 4., math.Pi / 4.}), r3.Vector{1, 1, 1}),
			makeBox(spatial.NewPoseFromOrientation(r3.Vector{1.1 + math.Sqrt(3)/2, 0, 0}, spatial.NewZeroOrientation()), r3.Vector{1, 1, 1}),
			false,
		},
	}

	for _, c := range cases {
		fn := test.ShouldBeTrue
		if !c.Expected {
			fn = test.ShouldBeFalse
		}
		test.That(t, BoxVsBox(c.A, c.B), fn)
	}
}

func makeBox(pose spatial.Pose, halfsize r3.Vector) *box {
	b, _ := NewBox(halfsize).NewVolume(pose)
	return b
}
