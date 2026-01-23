package spatialmath

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/geo/r3"
)

// bvhNode represents a node in a Bounding Volume Hierarchy tree.
// Each node has an axis-aligned bounding box (AABB) and either:
// - Two children (internal node), or
// - A list of geometries (leaf node)
// wikipedia.org/wiki/Bounding_volume_hierarchy
type bvhNode struct {
	min, max r3.Vector  // AABB bounds
	left     *bvhNode   // left child (nil for leaf)
	right    *bvhNode   // right child (nil for leaf)
	geoms    []Geometry // geometries (only for leaf nodes)
}

// maxGeomsPerLeaf is the threshold for splitting BVH nodes.
const maxGeomsPerLeaf = 8

var (
	bvhConfigOnce     sync.Once
	bvhDebugEnabled   bool
	bvhDebugEvery     uint64
	bvhDebugHitCount  uint64
	bvhDebugBuildSeen uint64
	bvhUseSAH         bool
	bvhProfileEnabled bool
	bvhProfileEvery   uint64

	bvhProfileNodesVisited   uint64
	bvhProfileAABBRejects    uint64
	bvhProfileLeafChecks     uint64
	bvhProfileGeomChecks     uint64
	bvhProfileCallsGeom      uint64
	bvhProfileCallsBVH       uint64
	bvhProfileTimeGeomNanos  uint64
	bvhProfileTimeBVHNanos   uint64
	bvhProfileTimeBuildNanos uint64
	bvhProfileTimeXformNanos uint64
)

func initBVHConfig() {
	bvhConfigOnce.Do(func() {
		val := strings.TrimSpace(strings.ToLower(os.Getenv("SPATIALMATH_BVH_DEBUG")))
		if val == "1" || val == "true" || val == "yes" {
			bvhDebugEnabled = true
		}
		bvhDebugEvery = 10000
		if raw := strings.TrimSpace(os.Getenv("SPATIALMATH_BVH_DEBUG_EVERY")); raw != "" {
			if parsed, err := strconv.ParseUint(raw, 10, 64); err == nil && parsed > 0 {
				bvhDebugEvery = parsed
			}
		}
		sahVal := strings.TrimSpace(strings.ToLower(os.Getenv("SPATIALMATH_BVH_USE_SAH")))
		if sahVal == "1" || sahVal == "true" || sahVal == "yes" {
			bvhUseSAH = true
		}
		profVal := strings.TrimSpace(strings.ToLower(os.Getenv("SPATIALMATH_BVH_PROFILE")))
		if profVal == "1" || profVal == "true" || profVal == "yes" {
			bvhProfileEnabled = true
		}
		bvhProfileEvery = 100000
		if raw := strings.TrimSpace(os.Getenv("SPATIALMATH_BVH_PROFILE_EVERY")); raw != "" {
			if parsed, err := strconv.ParseUint(raw, 10, 64); err == nil && parsed > 0 {
				bvhProfileEvery = parsed
			}
		}
	})
}

func bvhDebugf(format string, args ...any) {
	initBVHConfig()
	if !bvhDebugEnabled {
		return
	}
	fmt.Printf(format, args...)
}

func bvhDebugHit(label string) {
	initBVHConfig()
	if !bvhDebugEnabled {
		return
	}
	n := atomic.AddUint64(&bvhDebugHitCount, 1)
	if n%bvhDebugEvery == 0 {
		fmt.Printf("bvh debug: %s hits=%d\n", label, n)
	}
}

type bvhProfileToken struct {
	start time.Time
}

func bvhProfileStart() (bvhProfileToken, bool) {
	initBVHConfig()
	if !bvhProfileEnabled {
		return bvhProfileToken{}, false
	}
	return bvhProfileToken{start: time.Now()}, true
}

func bvhProfileAddDuration(counter *uint64, tok bvhProfileToken) {
	if !bvhProfileEnabled {
		return
	}
	atomic.AddUint64(counter, uint64(time.Since(tok.start).Nanoseconds()))
}

func bvhProfileMaybePrint(label string) {
	if !bvhProfileEnabled {
		return
	}
	n := atomic.LoadUint64(&bvhProfileNodesVisited)
	if n == 0 || n%bvhProfileEvery != 0 {
		return
	}
	fmt.Printf(
		"bvh profile [%s]: nodes=%d rejects=%d leaf=%d geom=%d callsGeom=%d callsBVH=%d timeGeomMs=%.2f timeBVHMs=%.2f timeBuildMs=%.2f timeXformMs=%.2f\n",
		label,
		n,
		atomic.LoadUint64(&bvhProfileAABBRejects),
		atomic.LoadUint64(&bvhProfileLeafChecks),
		atomic.LoadUint64(&bvhProfileGeomChecks),
		atomic.LoadUint64(&bvhProfileCallsGeom),
		atomic.LoadUint64(&bvhProfileCallsBVH),
		float64(atomic.LoadUint64(&bvhProfileTimeGeomNanos))/1e6,
		float64(atomic.LoadUint64(&bvhProfileTimeBVHNanos))/1e6,
		float64(atomic.LoadUint64(&bvhProfileTimeBuildNanos))/1e6,
		float64(atomic.LoadUint64(&bvhProfileTimeXformNanos))/1e6,
	)
}

// buildBVH constructs a BVH from a list of geometries.
func buildBVH(geoms []Geometry) *bvhNode {
	if len(geoms) == 0 {
		bvhDebugf("buildBVH: no geometries\n")
		return nil
	}
	bvhDebugf("buildBVH: %d geometries\n", len(geoms))
	return buildBVHNode(geoms)
}

func buildBVHNode(geoms []Geometry) *bvhNode {
	node := &bvhNode{}
	initBVHConfig()

	// Compute AABB (axis aligned bounding box) for all geometries
	node.min, node.max = computeGeomsAABB(geoms)
	if bvhDebugEnabled {
		atomic.AddUint64(&bvhDebugBuildSeen, 1)
		bvhDebugf("buildBVHNode: geoms=%d min=%v max=%v\n", len(geoms), node.min, node.max)
	}

	// If few enough geometries, make this a leaf node
	if len(geoms) <= maxGeomsPerLeaf {
		bvhDebugf("buildBVHNode: leaf geoms=%d\n", len(geoms))
		node.geoms = append([]Geometry(nil), geoms...)
		return node
	}

	// Find the longest axis to split on
	extent := node.max.Sub(node.min)
	axis := 0 // X
	if extent.Y > extent.X && extent.Y > extent.Z {
		axis = 1 // Y
	} else if extent.Z > extent.X && extent.Z > extent.Y {
		axis = 2 // Z
	}

	type geomInfo struct {
		geom     Geometry
		centroid r3.Vector
		min      r3.Vector
		max      r3.Vector
	}
	info := make([]geomInfo, len(geoms))
	for i, g := range geoms {
		gMin, gMax := computeGeometryAABB(g)
		info[i] = geomInfo{
			geom:     g,
			centroid: r3.Vector{X: (gMin.X + gMax.X) * 0.5, Y: (gMin.Y + gMax.Y) * 0.5, Z: (gMin.Z + gMax.Z) * 0.5},
			min:      gMin,
			max:      gMax,
		}
	}

	bestAxis := axis
	bestSplit := len(info) / 2
	if bvhUseSAH {
		bestCost := math.Inf(1)
		for axisIdx := 0; axisIdx < 3; axisIdx++ {
			sort.Slice(info, func(i, j int) bool {
				ci := info[i].centroid
				cj := info[j].centroid
				switch axisIdx {
				case 0:
					return ci.X < cj.X
				case 1:
					return ci.Y < cj.Y
				default:
					return ci.Z < cj.Z
				}
			})

			prefixMin := make([]r3.Vector, len(info))
			prefixMax := make([]r3.Vector, len(info))
			suffixMin := make([]r3.Vector, len(info))
			suffixMax := make([]r3.Vector, len(info))

			for i := range info {
				if i == 0 {
					prefixMin[i] = info[i].min
					prefixMax[i] = info[i].max
				} else {
					prefixMin[i] = r3.Vector{
						X: math.Min(prefixMin[i-1].X, info[i].min.X),
						Y: math.Min(prefixMin[i-1].Y, info[i].min.Y),
						Z: math.Min(prefixMin[i-1].Z, info[i].min.Z),
					}
					prefixMax[i] = r3.Vector{
						X: math.Max(prefixMax[i-1].X, info[i].max.X),
						Y: math.Max(prefixMax[i-1].Y, info[i].max.Y),
						Z: math.Max(prefixMax[i-1].Z, info[i].max.Z),
					}
				}
			}
			for i := len(info) - 1; i >= 0; i-- {
				if i == len(info)-1 {
					suffixMin[i] = info[i].min
					suffixMax[i] = info[i].max
				} else {
					suffixMin[i] = r3.Vector{
						X: math.Min(suffixMin[i+1].X, info[i].min.X),
						Y: math.Min(suffixMin[i+1].Y, info[i].min.Y),
						Z: math.Min(suffixMin[i+1].Z, info[i].min.Z),
					}
					suffixMax[i] = r3.Vector{
						X: math.Max(suffixMax[i+1].X, info[i].max.X),
						Y: math.Max(suffixMax[i+1].Y, info[i].max.Y),
						Z: math.Max(suffixMax[i+1].Z, info[i].max.Z),
					}
				}
			}

			for split := 1; split < len(info); split++ {
				leftMin := prefixMin[split-1]
				leftMax := prefixMax[split-1]
				rightMin := suffixMin[split]
				rightMax := suffixMax[split]
				leftExtent := leftMax.Sub(leftMin)
				rightExtent := rightMax.Sub(rightMin)
				leftArea := 2 * (leftExtent.X*leftExtent.Y + leftExtent.Y*leftExtent.Z + leftExtent.Z*leftExtent.X)
				rightArea := 2 * (rightExtent.X*rightExtent.Y + rightExtent.Y*rightExtent.Z + rightExtent.Z*rightExtent.X)
				cost := leftArea*float64(split) + rightArea*float64(len(info)-split)
				if cost < bestCost {
					bestCost = cost
					bestAxis = axisIdx
					bestSplit = split
				}
			}
		}
		axis = bestAxis
	}

	sort.Slice(info, func(i, j int) bool {
		ci := info[i].centroid
		cj := info[j].centroid
		switch axis {
		case 0:
			return ci.X < cj.X
		case 1:
			return ci.Y < cj.Y
		default:
			return ci.Z < cj.Z
		}
	})
	for i := range info {
		geoms[i] = info[i].geom
	}

	// Split at median
	mid := len(geoms) / 2
	if bvhUseSAH {
		mid = bestSplit
		if mid <= 0 || mid >= len(geoms) {
			mid = len(geoms) / 2
		}
	}
	bvhDebugf("buildBVHNode: split axis=%d mid=%d\n", axis, mid)
	node.left = buildBVHNode(geoms[:mid])
	node.right = buildBVHNode(geoms[mid:])

	return node
}

// computeTrianglesAABB computes the AABB encompassing all given triangles.
func computeTrianglesAABB(triangles []*Triangle) (r3.Vector, r3.Vector) {
	minPt := r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	maxPt := r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}

	for _, tri := range triangles {
		for _, pt := range tri.Points() {
			minPt.X = math.Min(minPt.X, pt.X)
			minPt.Y = math.Min(minPt.Y, pt.Y)
			minPt.Z = math.Min(minPt.Z, pt.Z)
			maxPt.X = math.Max(maxPt.X, pt.X)
			maxPt.Y = math.Max(maxPt.Y, pt.Y)
			maxPt.Z = math.Max(maxPt.Z, pt.Z)
		}
	}
	return minPt, maxPt
}

// computeGeometryAABB returns the axis-aligned bounding box for any Geometry.
// The returned min and max vectors define the AABB in world coordinates.
func computeGeometryAABB(g Geometry) (min, max r3.Vector) {
	switch geom := g.(type) {
	case *Triangle:
		return computeTriangleAABB(geom)
	case *sphere:
		return computeSphereAABB(geom)
	case *box:
		return computeBoxAABB(geom)
	case *capsule:
		return computeCapsuleAABB(geom)
	case *point:
		pt := geom.position
		return pt, pt
	case *Mesh:
		// Use existing BVH bounds if available
		if geom.bvh != nil {
			return geom.bvh.min, geom.bvh.max
		}
		// Fallback: compute from triangles
		return computeMeshAABB(geom)
	default:
		// Fallback: use pose point with zero extent
		pt := g.Pose().Point()
		return pt, pt
	}
}

// computeTriangleAABB computes the AABB for a single triangle.
func computeTriangleAABB(t *Triangle) (r3.Vector, r3.Vector) {
	pts := t.Points()
	minPt := r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	maxPt := r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}

	for _, pt := range pts {
		minPt.X = math.Min(minPt.X, pt.X)
		minPt.Y = math.Min(minPt.Y, pt.Y)
		minPt.Z = math.Min(minPt.Z, pt.Z)
		maxPt.X = math.Max(maxPt.X, pt.X)
		maxPt.Y = math.Max(maxPt.Y, pt.Y)
		maxPt.Z = math.Max(maxPt.Z, pt.Z)
	}
	return minPt, maxPt
}

// computeSphereAABB computes the AABB for a sphere.
func computeSphereAABB(s *sphere) (r3.Vector, r3.Vector) {
	center := s.Pose().Point()
	r := s.radius
	return r3.Vector{X: center.X - r, Y: center.Y - r, Z: center.Z - r},
		r3.Vector{X: center.X + r, Y: center.Y + r, Z: center.Z + r}
}

// computeBoxAABB computes the AABB for a rotated box.
// Since the box may be rotated, we need to transform all 8 corners and find the bounds.
func computeBoxAABB(b *box) (r3.Vector, r3.Vector) {
	// Get half-sizes
	hx, hy, hz := b.halfSize[0], b.halfSize[1], b.halfSize[2]

	// Generate 8 corners in local space
	corners := []r3.Vector{
		{X: -hx, Y: -hy, Z: -hz},
		{X: -hx, Y: -hy, Z: hz},
		{X: -hx, Y: hy, Z: -hz},
		{X: -hx, Y: hy, Z: hz},
		{X: hx, Y: -hy, Z: -hz},
		{X: hx, Y: -hy, Z: hz},
		{X: hx, Y: hy, Z: -hz},
		{X: hx, Y: hy, Z: hz},
	}

	minPt := r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	maxPt := r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}

	// Transform each corner to world space
	for _, corner := range corners {
		worldPt := Compose(b.center, NewPoseFromPoint(corner)).Point()
		minPt.X = math.Min(minPt.X, worldPt.X)
		minPt.Y = math.Min(minPt.Y, worldPt.Y)
		minPt.Z = math.Min(minPt.Z, worldPt.Z)
		maxPt.X = math.Max(maxPt.X, worldPt.X)
		maxPt.Y = math.Max(maxPt.Y, worldPt.Y)
		maxPt.Z = math.Max(maxPt.Z, worldPt.Z)
	}
	return minPt, maxPt
}

// computeCapsuleAABB computes the AABB for a capsule.
// A capsule is defined by two endpoints (segA, segB) and a radius.
func computeCapsuleAABB(c *capsule) (r3.Vector, r3.Vector) {
	r := c.radius
	// The AABB is the bounding box of two spheres at segA and segB
	minPt := r3.Vector{
		X: math.Min(c.segA.X, c.segB.X) - r,
		Y: math.Min(c.segA.Y, c.segB.Y) - r,
		Z: math.Min(c.segA.Z, c.segB.Z) - r,
	}
	maxPt := r3.Vector{
		X: math.Max(c.segA.X, c.segB.X) + r,
		Y: math.Max(c.segA.Y, c.segB.Y) + r,
		Z: math.Max(c.segA.Z, c.segB.Z) + r,
	}
	return minPt, maxPt
}

// computeMeshAABB computes the AABB for a mesh by iterating all triangles.
func computeMeshAABB(m *Mesh) (r3.Vector, r3.Vector) {
	minPt := r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	maxPt := r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}

	for _, tri := range m.triangles {
		// Transform triangle to world space
		worldTri := tri.TransformTriangle(m.pose)
		for _, pt := range worldTri.Points() {
			minPt.X = math.Min(minPt.X, pt.X)
			minPt.Y = math.Min(minPt.Y, pt.Y)
			minPt.Z = math.Min(minPt.Z, pt.Z)
			maxPt.X = math.Max(maxPt.X, pt.X)
			maxPt.Y = math.Max(maxPt.Y, pt.Y)
			maxPt.Z = math.Max(maxPt.Z, pt.Z)
		}
	}
	return minPt, maxPt
}

// computeGeomsAABB computes the AABB encompassing all given geometries.
func computeGeomsAABB(geoms []Geometry) (r3.Vector, r3.Vector) {
	minPt := r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	maxPt := r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}

	for _, g := range geoms {
		gMin, gMax := computeGeometryAABB(g)
		minPt.X = math.Min(minPt.X, gMin.X)
		minPt.Y = math.Min(minPt.Y, gMin.Y)
		minPt.Z = math.Min(minPt.Z, gMin.Z)
		maxPt.X = math.Max(maxPt.X, gMax.X)
		maxPt.Y = math.Max(maxPt.Y, gMax.Y)
		maxPt.Z = math.Max(maxPt.Z, gMax.Z)
	}
	return minPt, maxPt
}

// aabbOverlap checks if two AABBs overlap.
func aabbOverlap(min1, max1, min2, max2 r3.Vector) bool {
	return min1.X <= max2.X && max1.X >= min2.X &&
		min1.Y <= max2.Y && max1.Y >= min2.Y &&
		min1.Z <= max2.Z && max1.Z >= min2.Z
}

// aabbDistance computes the minimum distance between two non-overlapping AABBs.
func aabbDistance(min1, max1, min2, max2 r3.Vector) float64 {
	dx := math.Max(0, math.Max(min1.X-max2.X, min2.X-max1.X))
	dy := math.Max(0, math.Max(min1.Y-max2.Y, min2.Y-max1.Y))
	dz := math.Max(0, math.Max(min1.Z-max2.Z, min2.Z-max1.Z))
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// transformAABB transforms an AABB by a pose, returning a new (potentially larger) AABB.
func transformAABB(minPt, maxPt r3.Vector, pose Pose) (r3.Vector, r3.Vector) {
	// Get the 8 corners of the AABB
	corners := []r3.Vector{
		{X: minPt.X, Y: minPt.Y, Z: minPt.Z},
		{X: minPt.X, Y: minPt.Y, Z: maxPt.Z},
		{X: minPt.X, Y: maxPt.Y, Z: minPt.Z},
		{X: minPt.X, Y: maxPt.Y, Z: maxPt.Z},
		{X: maxPt.X, Y: minPt.Y, Z: minPt.Z},
		{X: maxPt.X, Y: minPt.Y, Z: maxPt.Z},
		{X: maxPt.X, Y: maxPt.Y, Z: minPt.Z},
		{X: maxPt.X, Y: maxPt.Y, Z: maxPt.Z},
	}

	newMin := r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	newMax := r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}

	for _, corner := range corners {
		worldPt := Compose(pose, NewPoseFromPoint(corner)).Point()
		newMin.X = math.Min(newMin.X, worldPt.X)
		newMin.Y = math.Min(newMin.Y, worldPt.Y)
		newMin.Z = math.Min(newMin.Z, worldPt.Z)
		newMax.X = math.Max(newMax.X, worldPt.X)
		newMax.Y = math.Max(newMax.Y, worldPt.Y)
		newMax.Z = math.Max(newMax.Z, worldPt.Z)
	}
	return newMin, newMax
}

// bvhCollidesWithBVH checks if two BVH trees collide.
func bvhCollidesWithBVH(node1, node2 *bvhNode, collisionBufferMM float64) (bool, float64) {
	if tok, ok := bvhProfileStart(); ok {
		atomic.AddUint64(&bvhProfileCallsBVH, 1)
		defer bvhProfileAddDuration(&bvhProfileTimeBVHNanos, tok)
	}
	if node1 == nil || node2 == nil {
		bvhDebugf("bvhCollidesWithBVH: nil node\n")
		return false, math.Inf(1)
	}
	bvhDebugf("bvhCollidesWithBVH: buffer=%.3f node1(leaf=%t) node2(leaf=%t)\n",
		collisionBufferMM, node1.geoms != nil, node2.geoms != nil)

	min1 := node1.min
	max1 := node1.max
	min2 := node2.min
	max2 := node2.max

	// Expand first AABB by collision buffer
	min1.X -= collisionBufferMM
	min1.Y -= collisionBufferMM
	min1.Z -= collisionBufferMM
	max1.X += collisionBufferMM
	max1.Y += collisionBufferMM
	max1.Z += collisionBufferMM

	// Check if AABBs overlap
	if !aabbOverlap(min1, max1, min2, max2) {
		bvhDebugHit("bvhCollidesWithBVH: AABBs do not overlap")
		if bvhProfileEnabled {
			atomic.AddUint64(&bvhProfileAABBRejects, 1)
			bvhProfileMaybePrint("bvh")
		}
		return false, aabbDistance(min1, max1, min2, max2)
	}

	// Both are leaves - do triangle-triangle checks
	if node1.geoms != nil && node2.geoms != nil {
		bvhDebugf("bvhCollidesWithBVH: leaf/leaf\n")
		if bvhProfileEnabled {
			atomic.AddUint64(&bvhProfileLeafChecks, 1)
		}
		return leafCollidesWithLeaf(node1.geoms, node2.geoms, collisionBufferMM)
	}

	// Recurse into children
	// Strategy: descend into the larger node first for better culling
	if node1.geoms != nil {
		bvhDebugf("bvhCollidesWithBVH: leaf/internal\n")
		// node1 is leaf, recurse into node2's children
		leftCollide, leftDist := bvhCollidesWithBVH(node1, node2.left, collisionBufferMM)
		if leftCollide {
			return true, -1
		}
		rightCollide, rightDist := bvhCollidesWithBVH(node1, node2.right, collisionBufferMM)
		if rightCollide {
			return true, -1
		}
		return false, math.Min(leftDist, rightDist)
	}

	if node2.geoms != nil {
		bvhDebugf("bvhCollidesWithBVH: internal/leaf\n")
		// node2 is leaf, recurse into node1's children
		leftCollide, leftDist := bvhCollidesWithBVH(node1.left, node2, collisionBufferMM)
		if leftCollide {
			return true, -1
		}
		rightCollide, rightDist := bvhCollidesWithBVH(node1.right, node2, collisionBufferMM)
		if rightCollide {
			return true, -1
		}
		return false, math.Min(leftDist, rightDist)
	}

	// Both are internal nodes - check all 4 combinations
	bvhDebugf("bvhCollidesWithBVH: internal/internal\n")
	minDist := math.Inf(1)
	pairs := [][2]*bvhNode{
		{node1.left, node2.left},
		{node1.left, node2.right},
		{node1.right, node2.left},
		{node1.right, node2.right},
	}

	for _, pair := range pairs {
		collide, dist := bvhCollidesWithBVH(pair[0], pair[1], collisionBufferMM)
		if collide {
			return true, -1
		}
		if dist < minDist {
			minDist = dist
		}
	}

	return false, minDist
}

// leafCollidesWithLeaf performs collision checks between two leaf nodes using the Geometry interface.
func leafCollidesWithLeaf(geoms1, geoms2 []Geometry, collisionBufferMM float64) (bool, float64) {
	minDist := math.Inf(1)

	for _, g1 := range geoms1 {
		for _, g2 := range geoms2 {
			// Use the Geometry interface's CollidesWith method
			collides, dist, _ := g1.CollidesWith(g2, collisionBufferMM)
			if collides {
				return true, -1
			}
			if dist < minDist {
				minDist = dist
			}
		}
	}

	return false, minDist
}

// bvhDistanceFromBVH computes the minimum distance between two BVH trees.
func bvhDistanceFromBVH(node1, node2 *bvhNode) float64 {
	if tok, ok := bvhProfileStart(); ok {
		atomic.AddUint64(&bvhProfileCallsBVH, 1)
		defer bvhProfileAddDuration(&bvhProfileTimeBVHNanos, tok)
	}
	if node1 == nil || node2 == nil {
		bvhDebugf("bvhDistanceFromBVH: nil node\n")
		return math.Inf(1)
	}
	bvhDebugf("bvhDistanceFromBVH: node1(leaf=%t) node2(leaf=%t)\n",
		node1.geoms != nil, node2.geoms != nil)

	min1 := node1.min
	max1 := node1.max
	min2 := node2.min
	max2 := node2.max

	// Check if AABBs overlap
	if !aabbOverlap(min1, max1, min2, max2) {
		bvhDebugHit("bvhDistanceFromBVH: AABBs do not overlap")
		if bvhProfileEnabled {
			atomic.AddUint64(&bvhProfileAABBRejects, 1)
			bvhProfileMaybePrint("dist")
		}
		// If AABBs don't overlap, the AABB distance is a lower bound
		// For distant meshes, this is good enough
		return aabbDistance(min1, max1, min2, max2)
	}

	// Both are leaves - compute exact distance
	if node1.geoms != nil && node2.geoms != nil {
		bvhDebugf("bvhDistanceFromBVH: leaf/leaf\n")
		return leafDistanceFromLeaf(node1.geoms, node2.geoms)
	}

	// Recurse into children
	if node1.geoms != nil {
		bvhDebugf("bvhDistanceFromBVH: leaf/internal\n")
		leftDist := bvhDistanceFromBVH(node1, node2.left)
		rightDist := bvhDistanceFromBVH(node1, node2.right)
		return math.Min(leftDist, rightDist)
	}

	if node2.geoms != nil {
		bvhDebugf("bvhDistanceFromBVH: internal/leaf\n")
		leftDist := bvhDistanceFromBVH(node1.left, node2)
		rightDist := bvhDistanceFromBVH(node1.right, node2)
		return math.Min(leftDist, rightDist)
	}

	// Both are internal nodes
	bvhDebugf("bvhDistanceFromBVH: internal/internal\n")
	minDist := math.Inf(1)
	pairs := [][2]*bvhNode{
		{node1.left, node2.left},
		{node1.left, node2.right},
		{node1.right, node2.left},
		{node1.right, node2.right},
	}

	for _, pair := range pairs {
		dist := bvhDistanceFromBVH(pair[0], pair[1])
		if dist < minDist {
			minDist = dist
		}
	}

	return minDist
}

// bvhCollidesWithGeometry traverses the BVH checking against a single geometry.
func bvhCollidesWithGeometry(node *bvhNode, other Geometry, otherMin, otherMax r3.Vector, buffer float64) (bool, float64, error) {
	if tok, ok := bvhProfileStart(); ok {
		atomic.AddUint64(&bvhProfileCallsGeom, 1)
		defer bvhProfileAddDuration(&bvhProfileTimeGeomNanos, tok)
	}
	if node == nil {
		bvhDebugf("bvhCollidesWithGeometry: nil node\n")
		return false, math.Inf(1), nil
	}
	bvhDebugf("bvhCollidesWithGeometry: buffer=%.3f leaf=%t\n", buffer, node.geoms != nil)

	type stackEntry struct {
		node *bvhNode
	}
	stack := []stackEntry{{node: node}}
	minDist := math.Inf(1)

	for len(stack) > 0 {
		if bvhProfileEnabled {
			atomic.AddUint64(&bvhProfileNodesVisited, 1)
		}
		entry := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if entry.node == nil {
			continue
		}

		nodeMin := entry.node.min
		nodeMax := entry.node.max

		// Expand node AABB by buffer
		nodeMin.X -= buffer
		nodeMin.Y -= buffer
		nodeMin.Z -= buffer
		nodeMax.X += buffer
		nodeMax.Y += buffer
		nodeMax.Z += buffer

		// Early exit if AABBs don't overlap
		if !aabbOverlap(nodeMin, nodeMax, otherMin, otherMax) {
			bvhDebugHit("bvhCollidesWithGeometry: AABBs do not overlap")
			if bvhProfileEnabled {
				atomic.AddUint64(&bvhProfileAABBRejects, 1)
				bvhProfileMaybePrint("geom")
			}
			dist := aabbDistance(nodeMin, nodeMax, otherMin, otherMax)
			if dist < minDist {
				minDist = dist
			}
			continue
		}

		// Leaf node: check each geometry against other
		if entry.node.geoms != nil {
			bvhDebugf("bvhCollidesWithGeometry: leaf\n")
			if bvhProfileEnabled {
				atomic.AddUint64(&bvhProfileLeafChecks, 1)
			}
			for _, g := range entry.node.geoms {
				if bvhProfileEnabled {
					atomic.AddUint64(&bvhProfileGeomChecks, 1)
				}
				gMin, gMax := computeGeometryAABB(g)
				gMin.X -= buffer
				gMin.Y -= buffer
				gMin.Z -= buffer
				gMax.X += buffer
				gMax.Y += buffer
				gMax.Z += buffer
				if !aabbOverlap(gMin, gMax, otherMin, otherMax) {
					continue
				}
				collides, dist, err := g.CollidesWith(other, buffer)
				if err != nil {
					return false, 0, err
				}
				if collides {
					return true, -1, nil
				}
				if dist < minDist {
					minDist = dist
				}
			}
			continue
		}

		// Internal node: push children
		stack = append(stack, stackEntry{node: entry.node.left}, stackEntry{node: entry.node.right})
	}

	return false, minDist, nil
}

// transformBVH returns a transformed copy of the BVH using the given pose.
// It avoids rebuilding the tree structure by transforming node AABBs and leaf geometries.
func transformBVH(node *bvhNode, pose Pose) *bvhNode {
	if tok, ok := bvhProfileStart(); ok {
		defer bvhProfileAddDuration(&bvhProfileTimeXformNanos, tok)
	}
	if node == nil {
		return nil
	}
	minPt, maxPt := transformAABB(node.min, node.max, pose)
	newNode := &bvhNode{
		min: minPt,
		max: maxPt,
	}
	if node.geoms != nil {
		newNode.geoms = make([]Geometry, len(node.geoms))
		for i, g := range node.geoms {
			newNode.geoms[i] = g.Transform(pose)
		}
		return newNode
	}
	newNode.left = transformBVH(node.left, pose)
	newNode.right = transformBVH(node.right, pose)
	return newNode
}

// leafDistanceFromLeaf computes the minimum distance between two sets of geometries.
func leafDistanceFromLeaf(geoms1, geoms2 []Geometry) float64 {
	minDist := math.Inf(1)

	for _, g1 := range geoms1 {
		for _, g2 := range geoms2 {
			// Use the Geometry interface's DistanceFrom method
			dist, _ := g1.DistanceFrom(g2)
			if dist < minDist {
				minDist = dist
			}
		}
	}

	return minDist
}
