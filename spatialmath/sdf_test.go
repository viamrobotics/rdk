package spatialmath

import (
	"math/rand"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

// TestVoxelSDFConservative: PointClearance must never exceed the true distance
// to the nearest obstacle surface, across geometry types and random queries.
func TestVoxelSDFConservative(t *testing.T) {
	b, err := NewBox(NewPoseFromPoint(r3.Vector{X: 200, Y: 0, Z: 0}), r3.Vector{X: 100, Y: 100, Z: 100}, "b")
	test.That(t, err, test.ShouldBeNil)
	s, err := NewSphere(NewPoseFromPoint(r3.Vector{X: -200, Y: 100, Z: 50}), 40, "s")
	test.That(t, err, test.ShouldBeNil)
	c, err := NewCapsule(NewPoseFromPoint(r3.Vector{X: 0, Y: -200, Z: 0}), 30, 160, "c")
	test.That(t, err, test.ShouldBeNil)
	tri := NewTriangle(r3.Vector{X: 0, Y: 200, Z: -50}, r3.Vector{X: 50, Y: 250, Z: 60}, r3.Vector{X: -60, Y: 220, Z: 40})
	mesh := NewMesh(NewZeroPose(), []*Triangle{tri}, "m")
	// Tilted cylinder: regression for the whitelisted-but-unbuildable case
	// (computeGeometryAABB used to panic on cylinders).
	cyl, err := NewCylinder(NewPose(r3.Vector{X: 150, Y: 150, Z: -50}, &OrientationVectorDegrees{OX: 1, OZ: 1, Theta: 20}), 35, 120, "cyl")
	test.That(t, err, test.ShouldBeNil)
	geoms := []Geometry{b, s, c, mesh, cyl}

	sdf := NewVoxelSDF(geoms, 10)
	test.That(t, sdf, test.ShouldNotBeNil)

	trueDist := func(p r3.Vector) float64 {
		probe := NewPoint(p, "probe")
		best := 1e18
		for _, g := range geoms {
			d, err := probe.DistanceFrom(g)
			test.That(t, err, test.ShouldBeNil)
			best = min(best, d)
		}
		return best
	}

	rnd := rand.New(rand.NewSource(3))
	for i := 0; i < 2000; i++ {
		p := r3.Vector{
			X: (rnd.Float64() - 0.5) * 1200,
			Y: (rnd.Float64() - 0.5) * 1200,
			Z: (rnd.Float64() - 0.5) * 600,
		}
		lb := sdf.PointClearance(p)
		if lb <= 0 {
			continue // no claim made
		}
		td := trueDist(p)
		test.That(t, lb, test.ShouldBeLessThanOrEqualTo, td+1e-9)
	}
}

// TestVoxelSDFRejectsUnknownTypes: scenes containing unsupported geometry
// types must produce no field rather than a field with holes.
func TestVoxelSDFRejectsUnknownTypes(t *testing.T) {
	test.That(t, NewVoxelSDF(nil, 10), test.ShouldBeNil)
}

// TestSphereCoverContains: every sampled surface/interior point of a geometry
// must lie inside at least one cover sphere.
func TestSphereCoverContains(t *testing.T) {
	b, err := NewBox(NewPose(r3.Vector{X: 20, Y: -30, Z: 40}, &OrientationVectorDegrees{OZ: 1, Theta: 30}),
		r3.Vector{X: 120, Y: 60, Z: 30}, "b")
	test.That(t, err, test.ShouldBeNil)
	c, err := NewCapsule(NewPoseFromPoint(r3.Vector{X: 5, Y: 5, Z: 5}), 25, 200, "c")
	test.That(t, err, test.ShouldBeNil)
	tri := NewTriangle(r3.Vector{X: 0, Y: 0, Z: 0}, r3.Vector{X: 90, Y: 10, Z: 40}, r3.Vector{X: -20, Y: 80, Z: -30})
	mesh := NewMesh(NewPoseFromPoint(r3.Vector{X: 10, Y: 20, Z: 30}), []*Triangle{tri}, "m")

	for _, g := range []Geometry{b, c, mesh} {
		cover := SphereCover(g)
		test.That(t, len(cover), test.ShouldBeGreaterThan, 0)
		for _, p := range g.ToPoints(5) {
			worst := 1e18
			for _, sb := range cover {
				if d := p.Sub(sb.Center).Norm() - sb.R; d < worst {
					worst = d
				}
			}
			if worst > 1e-6 {
				t.Fatalf("geom %s (%T): point %v outside cover by %.3f", g.Label(), g, p, worst)
			}
		}
	}
}
