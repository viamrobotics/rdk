package spatialmath

import (
	"math"

	"github.com/golang/geo/r3"
)

// VoxelSDF is a voxelized distance field over a set of static geometries. It
// answers conservative clearance queries - "the nearest obstacle surface is at
// least D away from this point" - in constant time, replacing per-query
// narrow-phase geometry work for the static side of collision checking.
//
// Build cost is paid once per static scene (obstacle surfaces are rasterized
// into an occupancy grid, then an exact Euclidean distance transform is run);
// queries are array lookups. All distances are in the same units as the input
// geometries (mm for RDK).
type VoxelSDF struct {
	origin     r3.Vector // center of voxel (0,0,0)
	res        float64
	nx, ny, nz int

	// dist holds, per voxel, the distance in mm from the voxel center to the
	// nearest occupied voxel center.
	dist []float32

	// occMargin is the rasterization slack: every obstacle surface point is
	// within occMargin of some occupied voxel center. Queries subtract it.
	occMargin float64
}

// maxSDFVoxels bounds the grid size; the resolution is coarsened to fit.
const maxSDFVoxels = 6_000_000

// NewVoxelSDF builds a distance field over the given geometries at the
// requested resolution (which may be coarsened to respect the voxel budget).
// Returns nil when there is nothing to voxelize.
func NewVoxelSDF(geoms []Geometry, resolution float64) *VoxelSDF {
	if len(geoms) == 0 {
		return nil
	}
	// Every geometry must be a type we know how to rasterize; a scene with any
	// unknown type (e.g. octrees) gets no field, leaving the exact checker in
	// sole charge. Silently skipping a geometry would make the field report
	// clear space where an obstacle stands.
	for _, g := range geoms {
		switch g.(type) {
		case *Mesh, *Triangle, *box, *sphere, *capsule, *point, *Cylinder:
		default:
			return nil
		}
	}
	minPt := r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	maxPt := r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
	for _, g := range geoms {
		gMin, gMax := computeGeometryAABB(g)
		minPt, maxPt = expandAABB(minPt, maxPt, gMin)
		minPt, maxPt = expandAABB(minPt, maxPt, gMax)
	}
	size := maxPt.Sub(minPt)
	if size.X <= 0 || size.Y <= 0 || size.Z <= 0 {
		size = r3.Vector{X: math.Max(size.X, 1), Y: math.Max(size.Y, 1), Z: math.Max(size.Z, 1)}
	}

	res := resolution
	for {
		nx := int(size.X/res) + 3
		ny := int(size.Y/res) + 3
		nz := int(size.Z/res) + 3
		if nx*ny*nz <= maxSDFVoxels {
			break
		}
		res *= 1.5
	}

	s := &VoxelSDF{
		origin: minPt.Sub(r3.Vector{X: res, Y: res, Z: res}),
		res:    res,
		nx:     int(size.X/res) + 3,
		ny:     int(size.Y/res) + 3,
		nz:     int(size.Z/res) + 3,
	}
	// A surface point inside a voxel is at most half the voxel diagonal from
	// that voxel's center; rasterization marks any voxel whose center is
	// within that distance of a surface, so the bound holds for every surface
	// point.
	s.occMargin = res * math.Sqrt(3) / 2

	occ := make([]bool, s.nx*s.ny*s.nz)
	for _, g := range geoms {
		s.rasterize(g, occ)
	}

	s.dist = edt3(occ, s.nx, s.ny, s.nz, res)
	return s
}

// Resolution returns the (possibly coarsened) voxel edge length.
func (s *VoxelSDF) Resolution() float64 { return s.res }

func (s *VoxelSDF) idx(x, y, z int) int { return (z*s.ny+y)*s.nx + x }

func (s *VoxelSDF) voxelCenter(x, y, z int) r3.Vector {
	return r3.Vector{
		X: s.origin.X + float64(x)*s.res,
		Y: s.origin.Y + float64(y)*s.res,
		Z: s.origin.Z + float64(z)*s.res,
	}
}

// rasterize marks the voxels whose centers lie within occMargin of the
// geometry's surface (for meshes) or interior (for solid primitives).
func (s *VoxelSDF) rasterize(g Geometry, occ []bool) {
	switch geom := g.(type) {
	case *Mesh:
		// Meshes are surface soup (not treated as solid anywhere else either).
		q := geom.pose.Orientation().Quaternion()
		t := geom.pose.Point()
		for _, tri := range geom.triangles {
			world := &Triangle{
				p0: TransformPoint(q, t, tri.p0),
				p1: TransformPoint(q, t, tri.p1),
				p2: TransformPoint(q, t, tri.p2),
			}
			tMin, tMax := triAABB(world)
			s.markRegion(occ, tMin, tMax, func(p r3.Vector) bool {
				return ClosestPointTrianglePoint(world, p).Sub(p).Norm() <= s.occMargin
			})
		}
	case *Triangle:
		tMin, tMax := triAABB(geom)
		s.markRegion(occ, tMin, tMax, func(p r3.Vector) bool {
			return ClosestPointTrianglePoint(geom, p).Sub(p).Norm() <= s.occMargin
		})
	case *box:
		gMin, gMax := computeGeometryAABB(geom)
		s.markRegion(occ, gMin, gMax, func(p r3.Vector) bool {
			return pointVsBoxDistance(p, geom) <= s.occMargin
		})
	case *sphere:
		c := geom.pose.Point()
		r := geom.radius
		gMin := r3.Vector{X: c.X - r, Y: c.Y - r, Z: c.Z - r}
		gMax := r3.Vector{X: c.X + r, Y: c.Y + r, Z: c.Z + r}
		s.markRegion(occ, gMin, gMax, func(p r3.Vector) bool {
			return p.Sub(c).Norm()-r <= s.occMargin
		})
	case *capsule:
		gMin, gMax := computeGeometryAABB(geom)
		s.markRegion(occ, gMin, gMax, func(p r3.Vector) bool {
			return DistToLineSegment(geom.segA, geom.segB, p)-geom.radius <= s.occMargin
		})
	case *point:
		s.markRegion(occ, geom.position, geom.position, func(p r3.Vector) bool {
			return p.Sub(geom.position).Norm() <= s.occMargin
		})
	case *Cylinder:
		// Conservative: treat as its bounding capsule.
		gMin, gMax := computeGeometryAABB(geom)
		center := geom.Pose().Point()
		axis := geom.Pose().Orientation().Quaternion()
		half := TransformPoint(axis, r3.Vector{}, r3.Vector{Z: geom.height / 2})
		segA := center.Sub(half)
		segB := center.Add(half)
		s.markRegion(occ, gMin, gMax, func(p r3.Vector) bool {
			return DistToLineSegment(segA, segB, p)-geom.radius <= s.occMargin
		})
	default:
		// Unreachable: NewVoxelSDF rejects scenes containing unknown types.
	}
}

// markRegion runs pred over every voxel center in the AABB (padded by one
// voxel) and marks the ones it accepts.
func (s *VoxelSDF) markRegion(occ []bool, rMin, rMax r3.Vector, pred func(r3.Vector) bool) {
	x0 := clampInt(int(math.Floor((rMin.X-s.origin.X)/s.res))-1, 0, s.nx-1)
	x1 := clampInt(int(math.Ceil((rMax.X-s.origin.X)/s.res))+1, 0, s.nx-1)
	y0 := clampInt(int(math.Floor((rMin.Y-s.origin.Y)/s.res))-1, 0, s.ny-1)
	y1 := clampInt(int(math.Ceil((rMax.Y-s.origin.Y)/s.res))+1, 0, s.ny-1)
	z0 := clampInt(int(math.Floor((rMin.Z-s.origin.Z)/s.res))-1, 0, s.nz-1)
	z1 := clampInt(int(math.Ceil((rMax.Z-s.origin.Z)/s.res))+1, 0, s.nz-1)
	for z := z0; z <= z1; z++ {
		for y := y0; y <= y1; y++ {
			base := (z*s.ny + y) * s.nx
			for x := x0; x <= x1; x++ {
				if !occ[base+x] && pred(s.voxelCenter(x, y, z)) {
					occ[base+x] = true
				}
			}
		}
	}
}

// PointClearance returns a conservative lower bound on the distance from p to
// the nearest obstacle surface. Points outside the grid are clamped, with the
// out-of-grid distance credited back.
func (s *VoxelSDF) PointClearance(p r3.Vector) float64 {
	fx := (p.X - s.origin.X) / s.res
	fy := (p.Y - s.origin.Y) / s.res
	fz := (p.Z - s.origin.Z) / s.res

	x := clampInt(int(fx+0.5), 0, s.nx-1)
	y := clampInt(int(fy+0.5), 0, s.ny-1)
	z := clampInt(int(fz+0.5), 0, s.nz-1)

	vc := s.voxelCenter(x, y, z)
	// dist(p, surface) >= dist(vc, nearest occupied center) - |p - vc| - occMargin
	return float64(s.dist[s.idx(x, y, z)]) - p.Sub(vc).Norm() - s.occMargin
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// edt3 computes, for every voxel, the Euclidean distance (in mm) to the
// nearest occupied voxel center, using the exact separable squared-distance
// transform of Felzenszwalb & Huttenlocher applied along each axis.
func edt3(occ []bool, nx, ny, nz int, res float64) []float32 {
	const inf = math.MaxFloat32 / 4
	n := nx * ny * nz
	f := make([]float64, n)
	for i, o := range occ {
		if o {
			f[i] = 0
		} else {
			f[i] = inf
		}
	}

	maxDim := max(nx, max(ny, nz))
	d := make([]float64, maxDim)
	vIdx := make([]int, maxDim)
	zEnv := make([]float64, maxDim+1)
	row := make([]float64, maxDim)

	// X axis.
	for z := 0; z < nz; z++ {
		for y := 0; y < ny; y++ {
			base := (z*ny + y) * nx
			for x := 0; x < nx; x++ {
				row[x] = f[base+x]
			}
			dt1(row[:nx], d, vIdx, zEnv)
			for x := 0; x < nx; x++ {
				f[base+x] = d[x]
			}
		}
	}
	// Y axis.
	for z := 0; z < nz; z++ {
		for x := 0; x < nx; x++ {
			for y := 0; y < ny; y++ {
				row[y] = f[(z*ny+y)*nx+x]
			}
			dt1(row[:ny], d, vIdx, zEnv)
			for y := 0; y < ny; y++ {
				f[(z*ny+y)*nx+x] = d[y]
			}
		}
	}
	// Z axis.
	for y := 0; y < ny; y++ {
		for x := 0; x < nx; x++ {
			for z := 0; z < nz; z++ {
				row[z] = f[(z*ny+y)*nx+x]
			}
			dt1(row[:nz], d, vIdx, zEnv)
			for z := 0; z < nz; z++ {
				f[(z*ny+y)*nx+x] = d[z]
			}
		}
	}

	out := make([]float32, n)
	for i, v := range f {
		out[i] = float32(math.Sqrt(v) * res)
	}
	return out
}

// dt1 is the 1D squared-distance transform (lower envelope of parabolas).
func dt1(f, d []float64, v []int, z []float64) {
	n := len(f)
	k := 0
	v[0] = 0
	z[0] = math.Inf(-1)
	z[1] = math.Inf(1)
	for q := 1; q < n; q++ {
		var s float64
		for {
			s = ((f[q] + float64(q*q)) - (f[v[k]] + float64(v[k]*v[k]))) / float64(2*q-2*v[k])
			if s > z[k] {
				break
			}
			k--
		}
		k++
		v[k] = q
		z[k] = s
		z[k+1] = math.Inf(1)
	}
	k = 0
	for q := 0; q < n; q++ {
		for z[k+1] < float64(q) {
			k++
		}
		dq := float64(q - v[k])
		d[q] = dq*dq + f[v[k]]
	}
}
