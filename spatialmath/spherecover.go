package spatialmath

import (
	"math"

	"github.com/golang/geo/r3"
)

// SphereBound is one sphere of a conservative cover: the covered geometry is
// entirely contained in the union of its cover's spheres.
type SphereBound struct {
	Center r3.Vector
	R      float64
}

// sphereCoverMeshCells is the per-axis cell count for mesh covers. 4^3 cells
// keep covers under ~64 spheres with radii a fraction of the link size.
const sphereCoverMeshCells = 4

// SphereCover returns a conservative sphere cover of the geometry in world
// coordinates, or nil when the geometry type is not supported (callers must
// treat nil as "no bound available"). Covers for primitives are computed on
// the fly from their world pose (cheap arithmetic); mesh covers are computed
// once in the mesh's local frame, cached on its shared state, and transformed
// per call.
func SphereCover(g Geometry) []SphereBound {
	switch geom := g.(type) {
	case *sphere:
		return []SphereBound{{Center: geom.pose.Point(), R: geom.radius}}
	case *point:
		return []SphereBound{{Center: geom.position, R: 0}}
	case *capsule:
		return capsuleSphereCover(geom.segA, geom.segB, geom.radius)
	case *Cylinder:
		center := geom.pose.Point()
		axis := geom.pose.Orientation().Quaternion()
		half := TransformPoint(axis, r3.Vector{}, r3.Vector{Z: geom.height / 2})
		return capsuleSphereCover(center.Sub(half), center.Add(half), geom.radius)
	case *box:
		return boxSphereCover(geom)
	case *Mesh:
		return meshSphereCover(geom)
	default:
		return nil
	}
}

// capsuleSphereCover covers the capsule with spheres spaced one radius apart
// along the axis; spacing s and radius r+s/2 guarantee coverage.
func capsuleSphereCover(segA, segB r3.Vector, r float64) []SphereBound {
	axis := segB.Sub(segA)
	l := axis.Norm()
	if r <= 0 {
		r = 1
	}
	n := int(math.Ceil(l/r)) + 1
	if n > 32 {
		n = 32
	}
	out := make([]SphereBound, 0, n)
	if n == 1 {
		return append(out, SphereBound{Center: segA.Add(axis.Mul(0.5)), R: r + l/2})
	}
	spacing := l / float64(n-1)
	for i := 0; i < n; i++ {
		out = append(out, SphereBound{Center: segA.Add(axis.Mul(float64(i) / float64(n-1))), R: r + spacing/2})
	}
	return out
}

// boxSphereCover subdivides the box into cells small enough that each cell's
// half-diagonal is a modest sphere and covers each cell with one sphere.
func boxSphereCover(b *box) []SphereBound {
	half := r3.Vector{X: b.halfSize[0], Y: b.halfSize[1], Z: b.halfSize[2]}
	// Aim for cells no larger than the smallest box dimension (so flat boxes
	// get thin spheres), capped at 4 cells per axis.
	target := math.Max(20, math.Min(half.X, math.Min(half.Y, half.Z))*2)
	nx := clampInt(int(math.Ceil(half.X*2/target)), 1, 4)
	ny := clampInt(int(math.Ceil(half.Y*2/target)), 1, 4)
	nz := clampInt(int(math.Ceil(half.Z*2/target)), 1, 4)

	cell := r3.Vector{X: half.X / float64(nx), Y: half.Y / float64(ny), Z: half.Z / float64(nz)}
	r := cell.Norm() // cell half-diagonal
	rm := b.center.Orientation().RotationMatrix()
	c := b.center.Point()

	out := make([]SphereBound, 0, nx*ny*nz)
	for ix := 0; ix < nx; ix++ {
		for iy := 0; iy < ny; iy++ {
			for iz := 0; iz < nz; iz++ {
				local := r3.Vector{
					X: -half.X + (2*float64(ix)+1)*cell.X,
					Y: -half.Y + (2*float64(iy)+1)*cell.Y,
					Z: -half.Z + (2*float64(iz)+1)*cell.Z,
				}
				world := r3.Vector{
					X: rm.Row(0).Dot(local),
					Y: rm.Row(1).Dot(local),
					Z: rm.Row(2).Dot(local),
				}
				out = append(out, SphereBound{Center: c.Add(world), R: r})
			}
		}
	}
	return out
}

// meshSphereCover buckets the mesh's triangles into a local-frame grid, covers
// each bucket's vertices with one sphere (a triangle is inside any sphere
// containing its vertices), caches the local cover on the shared mesh state,
// and transforms centers into world space per call.
func meshSphereCover(m *Mesh) []SphereBound {
	if m.state == nil || len(m.triangles) == 0 {
		return nil
	}
	m.state.sphereCoverOnce.Do(func() {
		m.state.sphereCover = buildLocalMeshCover(m.triangles)
	})
	local := m.state.sphereCover
	if len(local) == 0 {
		return nil
	}
	q := m.pose.Orientation().Quaternion()
	t := m.pose.Point()
	out := make([]SphereBound, len(local))
	for i, sb := range local {
		out[i] = SphereBound{Center: TransformPoint(q, t, sb.Center), R: sb.R}
	}
	return out
}

func buildLocalMeshCover(tris []*Triangle) []SphereBound {
	minPt := r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	maxPt := r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
	for _, tri := range tris {
		tMin, tMax := triAABB(tri)
		minPt, maxPt = expandAABB(minPt, maxPt, tMin)
		minPt, maxPt = expandAABB(minPt, maxPt, tMax)
	}
	size := maxPt.Sub(minPt)
	n := sphereCoverMeshCells
	cell := r3.Vector{
		X: math.Max(size.X/float64(n), 1e-6),
		Y: math.Max(size.Y/float64(n), 1e-6),
		Z: math.Max(size.Z/float64(n), 1e-6),
	}

	type bucket struct {
		min, max r3.Vector
		any      bool
	}
	buckets := make([]bucket, n*n*n)
	cellOf := func(p r3.Vector) int {
		ix := clampInt(int((p.X-minPt.X)/cell.X), 0, n-1)
		iy := clampInt(int((p.Y-minPt.Y)/cell.Y), 0, n-1)
		iz := clampInt(int((p.Z-minPt.Z)/cell.Z), 0, n-1)
		return (iz*n+iy)*n + ix
	}
	add := func(bi int, p r3.Vector) {
		b := &buckets[bi]
		if !b.any {
			b.min, b.max, b.any = p, p, true
			return
		}
		b.min, b.max = expandAABB(b.min, b.max, p)
	}
	for _, tri := range tris {
		// Assign whole triangles by centroid so each triangle lives in exactly
		// one bucket; the bucket AABB grows to include all three vertices, so
		// coverage still holds.
		bi := cellOf(tri.Centroid())
		add(bi, tri.p0)
		add(bi, tri.p1)
		add(bi, tri.p2)
	}

	out := []SphereBound{}
	for i := range buckets {
		b := &buckets[i]
		if !b.any {
			continue
		}
		c := b.min.Add(b.max).Mul(0.5)
		out = append(out, SphereBound{Center: c, R: b.max.Sub(c).Norm()})
	}
	return out
}

// BoundingSphere returns a single conservative bounding sphere for the
// geometry (world frame). ok is false for geometry types with no cheap bound;
// callers must then skip sphere-based pruning.
func BoundingSphere(g Geometry) (r3.Vector, float64, bool) {
	switch geom := g.(type) {
	case *sphere:
		return geom.pose.Point(), geom.radius, true
	case *point:
		return geom.position, 0, true
	case *capsule:
		return geom.center, geom.length / 2, true
	case *box:
		return geom.centerPt, geom.boundingSphereR, true
	case *Cylinder:
		h := geom.height / 2
		return geom.pose.Point(), math.Sqrt(h*h + geom.radius*geom.radius), true
	case *Mesh:
		if geom.state == nil {
			return r3.Vector{}, 0, false
		}
		return geom.pose.Point(), geom.localBoundingRadius(), true
	default:
		return r3.Vector{}, 0, false
	}
}
