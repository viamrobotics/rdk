package spatialmath

import (
	"fmt"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestNewSphereFromOffset(t *testing.T) {
	pt := r3.Vector{X: 1, Y: 0, Z: 0}
	offset := NewPoseFromOrientation(pt, &EulerAngles{0, 0, math.Pi})
	vc, err := NewSphere(1, offset)
	test.That(t, err, test.ShouldBeNil)
	vol := vc.NewVolume(Invert(offset))
	test.That(t, PoseAlmostCoincident(vol.Pose(), NewZeroPose()), test.ShouldBeTrue)
	quatCompare(t, vol.Pose().Orientation().Quaternion(), NewZeroOrientation().Quaternion())
}

func TestSphereAlmostEqual(t *testing.T) {
	original := makeSphere(r3.Vector{}, 1)
	good := makeSphere(r3.Vector{1e-16, 1e-16, 1e-16}, 1+1e-16)
	bad := makeSphere(r3.Vector{1e-2, 1e-2, 1e-2}, 1+1e-2)
	test.That(t, original.AlmostEqual(good), test.ShouldBeTrue)
	test.That(t, original.AlmostEqual(bad), test.ShouldBeFalse)
}

func TestSphereVertices(t *testing.T) {
	test.That(t, R3VectorAlmostEqual(makeSphere(r3.Vector{}, 1).Vertices()[0], r3.Vector{}, 1e-8), test.ShouldBeTrue)
}

func TestSphereVsSphere(t *testing.T) {
	cases := []struct {
		testname string
		a        Volume
		b        Volume
		expected float64
	}{
		{
			"test inscribed spheres",
			makeSphere(r3.Vector{}, 1),
			makeSphere(r3.Vector{}, 2),
			-3,
		},
		{
			"test tangent spheres",
			makeSphere(r3.Vector{}, 1),
			makeSphere(r3.Vector{0, 0, 2}, 1),
			0,
		},
		{
			"separated spheres",
			makeSphere(r3.Vector{}, 1),
			makeSphere(r3.Vector{0, 0, 2 + 1e-3}, 1),
			1e-3,
		},
	}

	for _, c := range cases {
		t.Run(c.testname+" collision", func(t *testing.T) {
			fn := test.ShouldBeFalse
			if c.expected <= 0.0 {
				fn = test.ShouldBeTrue
			}
			collides, err := c.a.CollidesWith(c.b)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, collides, fn)
		})
		t.Run(c.testname+" distance", func(t *testing.T) {
			distance, err := c.a.DistanceFrom(c.b)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, distance, test.ShouldAlmostEqual, c.expected)
		})
	}
}

func TestSphereVsBox(t *testing.T) {
	cases := []struct {
		testname string
		volumes  [2]Volume
		expected float64
	}{
		{
			"separated face closest",
			[2]Volume{
				makeSphere(r3.Vector{0, 0, 2 + 1e-3}, 1),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			1e-3,
		},
		{
			"separated edge closest",
			[2]Volume{
				makeSphere(r3.Vector{0, 2, 2}, 1),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			math.Sqrt(2) - 1,
		},
		{
			"separated vertex closest",
			[2]Volume{
				makeSphere(r3.Vector{2, 2, 2}, 1),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			math.Sqrt(3) - 1,
		},
		{
			"collision face tangent",
			[2]Volume{
				makeSphere(r3.Vector{0, 0, 2}, 1),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			0,
		},
		{
			"collision edge tangent",
			[2]Volume{
				makeSphere(r3.Vector{0, 2, 2}, math.Sqrt(2)),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			0,
		},
		{
			"collision vertex tangent",
			[2]Volume{
				makeSphere(r3.Vector{2, 2, 2}, math.Sqrt(3)),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			0,
		},
		{
			"collision face closest",
			[2]Volume{
				makeSphere(r3.Vector{-.2, 0.1, .75}, 1),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			-1.25,
		},
		{
			"inscribed",
			[2]Volume{
				makeSphere(r3.Vector{2, 2, 2}, 1),
				makeBox(NewZeroOrientation(), r3.Vector{2, 2, 2}, r3.Vector{2, 2, 2}),
			},
			-2,
		},
	}

	for _, c := range cases {
		for i := 0; i < 2; i++ {
			t.Run(fmt.Sprintf("%s %T %T collision", c.testname, c.volumes[i], c.volumes[(i+1)%2]), func(t *testing.T) {
				fn := test.ShouldBeFalse
				if c.expected <= 0.0 {
					fn = test.ShouldBeTrue
				}
				collides, err := c.volumes[i].CollidesWith(c.volumes[(i+1)%2])
				test.That(t, err, test.ShouldBeNil)
				test.That(t, collides, fn)
			})
			t.Run(fmt.Sprintf("%s %T %T distance", c.testname, c.volumes[i], c.volumes[(i+1)%2]), func(t *testing.T) {
				distance, err := c.volumes[i].DistanceFrom(c.volumes[(i+1)%2])
				test.That(t, err, test.ShouldBeNil)
				test.That(t, distance, test.ShouldAlmostEqual, c.expected)
			})
		}
	}
}

func makeSphere(point r3.Vector, radius float64) *sphere {
	vc, _ := NewSphere(radius, NewPoseFromPoint(point))
	return vc.NewVolume(NewZeroPose()).(*sphere)
}
