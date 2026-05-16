package spatialmath

import (
	"fmt"
	"math"
	"sort"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/quat"
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
// Value of 4 balances tree depth vs leaf iteration cost - small enough to limit
// linear scans at leaves, large enough to avoid excessive tree overhead.
const maxGeomsPerLeaf = 4

type geometryCentroidSorter struct {
	geoms     []Geometry
	centroids []r3.Vector
	axis      int
}

func (s geometryCentroidSorter) Len() int {
	return len(s.geoms)
}

func (s geometryCentroidSorter) Less(i, j int) bool {
	ci := s.centroids[i]
	cj := s.centroids[j]
	switch s.axis {
	case 0:
		return ci.X < cj.X
	case 1:
		return ci.Y < cj.Y
	default:
		return ci.Z < cj.Z
	}
}

func (s geometryCentroidSorter) Swap(i, j int) {
	s.geoms[i], s.geoms[j] = s.geoms[j], s.geoms[i]
	s.centroids[i], s.centroids[j] = s.centroids[j], s.centroids[i]
}

// buildBVH constructs a BVH from a list of geometries.
func buildBVH(geoms []Geometry) *bvhNode {
	if len(geoms) == 0 {
		return nil
	}
	return buildBVHNode(geoms)
}

func buildBVHNode(geoms []Geometry) *bvhNode {
	node := &bvhNode{}

	// Compute AABB (axis aligned bounding box) for all geometries
	node.min, node.max = computeGeomsAABB(geoms)

	// If few enough geometries, make this a leaf node
	if len(geoms) <= maxGeomsPerLeaf {
		node.geoms = geoms
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

	// Precompute centroids so sort comparisons don't repeatedly allocate and compute orientation.
	centroids := make([]r3.Vector, len(geoms))
	for i, g := range geoms {
		centroids[i] = geometryCentroid(g)
	}
	sort.Sort(geometryCentroidSorter{
		geoms:     geoms,
		centroids: centroids,
		axis:      axis,
	})

	// Split at median
	mid := len(geoms) / 2
	node.left = buildBVHNode(geoms[:mid])
	node.right = buildBVHNode(geoms[mid:])

	return node
}

func geometryCentroid(g Geometry) r3.Vector {
	if tri, ok := g.(*Triangle); ok {
		return tri.Centroid()
	}
	return g.Pose().Point()
}

// computeGeometryAABB returns the axis-aligned bounding box for any Geometry.
// The returned min and max vectors define the AABB in world coordinates.
func computeGeometryAABB(g Geometry) (r3.Vector, r3.Vector) {
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
		// BVH stores bounds in local space. We must transform to world space via the mesh pose.
		// Previously this returned bvh.min/max directly (local space), which caused collision
		// checks to silently miss collisions after BVH initialization.
		bvh := geom.ensureBVH()
		if bvh != nil {
			return transformAABBToWorldSpace(bvh.min, bvh.max, geom.pose)
		}
		return computeMeshAABB(geom)
	default:
		panic(fmt.Errorf(
			"cannot construct AABB for: %v, %w",
			g,
			errGeometryTypeUnsupported,
		))
	}
}

// computeTriangleAABB computes the AABB for a single triangle.
func computeTriangleAABB(t *Triangle) (r3.Vector, r3.Vector) {
	pts := t.Points()
	minPt := r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	maxPt := r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}

	for _, pt := range pts {
		minPt, maxPt = expandAABB(minPt, maxPt, pt)
	}
	return minPt, maxPt
}

func expandAABB(minPt, maxPt, pt r3.Vector) (r3.Vector, r3.Vector) {
	newMinPt, newMaxPt := r3.Vector{}, r3.Vector{}
	newMinPt.X = math.Min(minPt.X, pt.X)
	newMinPt.Y = math.Min(minPt.Y, pt.Y)
	newMinPt.Z = math.Min(minPt.Z, pt.Z)
	newMaxPt.X = math.Max(maxPt.X, pt.X)
	newMaxPt.Y = math.Max(maxPt.Y, pt.Y)
	newMaxPt.Z = math.Max(maxPt.Z, pt.Z)
	return newMinPt, newMaxPt
}

// rotatedAABBExtents computes world-space AABB extents using Arvo's abs(R) * extents.
// Note: RotationMatrix stores R^T (transpose) in row-major order, so we read columns of rm
// to get rows of R: R[i][j] = rm.At(j, i).
func rotatedAABBExtents(rm *RotationMatrix, extents r3.Vector) r3.Vector {
	return r3.Vector{
		X: math.Abs(rm.At(0, 0))*extents.X + math.Abs(rm.At(1, 0))*extents.Y + math.Abs(rm.At(2, 0))*extents.Z,
		Y: math.Abs(rm.At(0, 1))*extents.X + math.Abs(rm.At(1, 1))*extents.Y + math.Abs(rm.At(2, 1))*extents.Z,
		Z: math.Abs(rm.At(0, 2))*extents.X + math.Abs(rm.At(1, 2))*extents.Y + math.Abs(rm.At(2, 2))*extents.Z,
	}
}

func aabbFromCenterExtents(center, extents r3.Vector) (r3.Vector, r3.Vector) {
	return center.Sub(extents), center.Add(extents)
}

// computeSphereAABB computes the AABB for a sphere.
func computeSphereAABB(s *sphere) (r3.Vector, r3.Vector) {
	center := s.Pose().Point()
	r := s.radius
	return r3.Vector{X: center.X - r, Y: center.Y - r, Z: center.Z - r},
		r3.Vector{X: center.X + r, Y: center.Y + r, Z: center.Z + r}
}

func computeBoxAABB(b *box) (r3.Vector, r3.Vector) {
	rm := b.center.Orientation().RotationMatrix()
	center := b.center.Point()
	halfSize := r3.Vector{X: b.halfSize[0], Y: b.halfSize[1], Z: b.halfSize[2]}
	worldExtents := rotatedAABBExtents(rm, halfSize)
	return aabbFromCenterExtents(center, worldExtents)
}

// computeCapsuleAABB computes the AABB for a capsule.
// A capsule is defined by two endpoints (segA, segB) and a radius.
func computeCapsuleAABB(c *capsule) (r3.Vector, r3.Vector) {
	r := c.radius
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
	q := m.pose.Orientation().Quaternion()
	trans := m.pose.Point()

	for _, tri := range m.triangles {
		minPt, maxPt = expandAABB(minPt, maxPt, TransformPoint(q, trans, tri.p0))
		minPt, maxPt = expandAABB(minPt, maxPt, TransformPoint(q, trans, tri.p1))
		minPt, maxPt = expandAABB(minPt, maxPt, TransformPoint(q, trans, tri.p2))
	}
	return minPt, maxPt
}

// transformAABBToWorldSpace transforms a local-space AABB to world space by
// transforming all 8 corners and computing a new axis-aligned bounding box.
// The result is a valid but potentially looser world-space AABB.
func transformAABBToWorldSpace(localMin, localMax r3.Vector, pose Pose) (r3.Vector, r3.Vector) {
	q := pose.Orientation().Quaternion()
	t := pose.Point()

	worldMin := r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	worldMax := r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}

	for _, x := range [2]float64{localMin.X, localMax.X} {
		for _, y := range [2]float64{localMin.Y, localMax.Y} {
			for _, z := range [2]float64{localMin.Z, localMax.Z} {
				pt := TransformPoint(q, t, r3.Vector{X: x, Y: y, Z: z})
				worldMin, worldMax = expandAABB(worldMin, worldMax, pt)
			}
		}
	}
	return worldMin, worldMax
}

// computeGeomsAABB computes the AABB encompassing all given geometries.
func computeGeomsAABB(geoms []Geometry) (r3.Vector, r3.Vector) {
	minPt := r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	maxPt := r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}

	for _, g := range geoms {
		gMin, gMax := computeGeometryAABB(g)
		minPt, maxPt = expandAABB(minPt, maxPt, gMin)
		minPt, maxPt = expandAABB(minPt, maxPt, gMax)
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

func transformAABB(minPt, maxPt r3.Vector, pose Pose) (r3.Vector, r3.Vector) {
	pc := newBVHPoseCache(pose)
	return transformAABBCached(minPt, maxPt, &pc)
}

// bvhPoseCache holds a pose pre-decomposed into the forms used during BVH traversal.
// Avoids repeatedly calling Orientation().RotationMatrix()/Quaternion() (which allocate
// and recompute) on every recursive entry.
type bvhPoseCache struct {
	pose  Pose
	q     quat.Number
	rm    *RotationMatrix
	trans r3.Vector
}

func newBVHPoseCache(p Pose) bvhPoseCache {
	o := p.Orientation()
	return bvhPoseCache{
		pose:  p,
		q:     o.Quaternion(),
		rm:    o.RotationMatrix(),
		trans: p.Point(),
	}
}

func transformAABBCached(minPt, maxPt r3.Vector, pc *bvhPoseCache) (r3.Vector, r3.Vector) {
	center := minPt.Add(maxPt).Mul(0.5)
	extents := maxPt.Sub(minPt).Mul(0.5)
	worldCenter := TransformPoint(pc.q, pc.trans, center)
	worldExtents := rotatedAABBExtents(pc.rm, extents)
	return aabbFromCenterExtents(worldCenter, worldExtents)
}

func expandAABBBuffer(minPt, maxPt r3.Vector, buffer float64) (r3.Vector, r3.Vector) {
	return r3.Vector{X: minPt.X - buffer, Y: minPt.Y - buffer, Z: minPt.Z - buffer},
		r3.Vector{X: maxPt.X + buffer, Y: maxPt.Y + buffer, Z: maxPt.Z + buffer}
}

// bvhCollidesWithBVH checks if two BVH trees collide, using the given poses to transform them.
// The BVH nodes store geometries in local space; poses are applied lazily during traversal.
func bvhCollidesWithBVH(node1, node2 *bvhNode, pose1, pose2 Pose, collisionBufferMM float64) (bool, float64, error) {
	collides, dist, _, err := bvhCollidesWithBVHTracked(node1, node2, pose1, pose2, collisionBufferMM)
	return collides, dist, err
}

// bvhCollidesWithBVHTracked is the witness-tracking variant of bvhCollidesWithBVH.
// On a true return it additionally provides the colliding triangle pair (when
// available) as witness — used by (*Mesh).collidesWithMesh to seed a temporal-
// coherence cache so subsequent queries with nearby poses can re-check the same
// pair directly and skip the BVH entirely. witness is the zero value when no
// collision is found or when the colliding geometries weren't both Triangles.
func bvhCollidesWithBVHTracked(node1, node2 *bvhNode, pose1, pose2 Pose, collisionBufferMM float64) (bool, float64, [2]*Triangle, error) {
	var witness [2]*Triangle
	if node1 == nil || node2 == nil {
		return false, math.Inf(1), witness, nil
	}

	pc1 := newBVHPoseCache(pose1)
	pc2 := newBVHPoseCache(pose2)

	min1, max1 := transformAABBCached(node1.min, node1.max, &pc1)
	min1, max1 = expandAABBBuffer(min1, max1, collisionBufferMM)
	min2, max2 := transformAABBCached(node2.min, node2.max, &pc2)

	collides, dist, err := bvhCollidesWithBVHRec(node1, min1, max1, node2, min2, max2, &pc1, &pc2, collisionBufferMM, &witness)
	return collides, dist, witness, err
}

// bvhCollidesWithBVHRec is the recursive worker for bvhCollidesWithBVH.
// Caller invariants:
//   - min1/max1 is node1's world-space AABB pre-expanded by collisionBufferMM.
//   - min2/max2 is node2's world-space AABB (unexpanded).
//
// This lets each node's AABB be transformed exactly once across the traversal,
// instead of recomputing both AABBs at every recursive entry.
func bvhCollidesWithBVHRec(
	node1 *bvhNode, min1, max1 r3.Vector,
	node2 *bvhNode, min2, max2 r3.Vector,
	pc1, pc2 *bvhPoseCache,
	collisionBufferMM float64,
	witness *[2]*Triangle,
) (bool, float64, error) {
	if !aabbOverlap(min1, max1, min2, max2) {
		return false, aabbDistance(min1, max1, min2, max2), nil
	}

	if node1.geoms != nil && node2.geoms != nil {
		return leafCollidesWithLeaf(node1.geoms, node2.geoms, pc1.pose, pc2.pose, collisionBufferMM, witness)
	}

	if node1.geoms != nil {
		// node1 is leaf; transform node2's children once at this level.
		l2Min, l2Max := transformAABBCached(node2.left.min, node2.left.max, pc2)
		leftCollide, leftDist, err := bvhCollidesWithBVHRec(node1, min1, max1, node2.left, l2Min, l2Max, pc1, pc2, collisionBufferMM, witness)
		if err != nil {
			return false, 0, err
		}
		if leftCollide {
			return true, leftDist, nil
		}
		r2Min, r2Max := transformAABBCached(node2.right.min, node2.right.max, pc2)
		rightCollide, rightDist, err := bvhCollidesWithBVHRec(node1, min1, max1, node2.right, r2Min, r2Max, pc1, pc2, collisionBufferMM, witness)
		if err != nil {
			return false, 0, err
		}
		if rightCollide {
			return true, rightDist, nil
		}
		return false, math.Min(leftDist, rightDist), nil
	}

	if node2.geoms != nil {
		// node2 is leaf; transform node1's children once at this level.
		l1Min, l1Max := transformAABBCached(node1.left.min, node1.left.max, pc1)
		l1Min, l1Max = expandAABBBuffer(l1Min, l1Max, collisionBufferMM)
		leftCollide, leftDist, err := bvhCollidesWithBVHRec(node1.left, l1Min, l1Max, node2, min2, max2, pc1, pc2, collisionBufferMM, witness)
		if err != nil {
			return false, 0, err
		}
		if leftCollide {
			return true, leftDist, nil
		}
		r1Min, r1Max := transformAABBCached(node1.right.min, node1.right.max, pc1)
		r1Min, r1Max = expandAABBBuffer(r1Min, r1Max, collisionBufferMM)
		rightCollide, rightDist, err := bvhCollidesWithBVHRec(node1.right, r1Min, r1Max, node2, min2, max2, pc1, pc2, collisionBufferMM, witness)
		if err != nil {
			return false, 0, err
		}
		if rightCollide {
			return true, rightDist, nil
		}
		return false, math.Min(leftDist, rightDist), nil
	}

	// Both internal: transform all 4 children once, then evaluate overlapping pairs
	// first so collision short-circuits don't get blocked by guaranteed-miss pairs.
	// Non-overlapping pairs need not recurse — their AABB distance is the best info
	// available and feeds the minDist return.
	lmin1, lmax1 := transformAABBCached(node1.left.min, node1.left.max, pc1)
	lmin1, lmax1 = expandAABBBuffer(lmin1, lmax1, collisionBufferMM)
	rmin1, rmax1 := transformAABBCached(node1.right.min, node1.right.max, pc1)
	rmin1, rmax1 = expandAABBBuffer(rmin1, rmax1, collisionBufferMM)
	lmin2, lmax2 := transformAABBCached(node2.left.min, node2.left.max, pc2)
	rmin2, rmax2 := transformAABBCached(node2.right.min, node2.right.max, pc2)

	type pairEntry struct {
		n1, n2                     *bvhNode
		pMin1, pMax1, pMin2, pMax2 r3.Vector
		overlap                    bool
	}
	pairs := [4]pairEntry{
		{node1.left, node2.left, lmin1, lmax1, lmin2, lmax2, aabbOverlap(lmin1, lmax1, lmin2, lmax2)},
		{node1.left, node2.right, lmin1, lmax1, rmin2, rmax2, aabbOverlap(lmin1, lmax1, rmin2, rmax2)},
		{node1.right, node2.left, rmin1, rmax1, lmin2, lmax2, aabbOverlap(rmin1, rmax1, lmin2, lmax2)},
		{node1.right, node2.right, rmin1, rmax1, rmin2, rmax2, aabbOverlap(rmin1, rmax1, rmin2, rmax2)},
	}

	minDist := math.Inf(1)
	// First pass: recurse only into overlapping pairs (where collisions are possible).
	for i := range pairs {
		p := &pairs[i]
		if !p.overlap {
			continue
		}
		collide, dist, err := bvhCollidesWithBVHRec(p.n1, p.pMin1, p.pMax1, p.n2, p.pMin2, p.pMax2, pc1, pc2, collisionBufferMM, witness)
		if err != nil {
			return false, 0, err
		}
		if collide {
			return true, dist, nil
		}
		if dist < minDist {
			minDist = dist
		}
	}
	// Second pass: non-overlapping pairs contribute only to minDist.
	for i := range pairs {
		p := &pairs[i]
		if p.overlap {
			continue
		}
		dist := aabbDistance(p.pMin1, p.pMax1, p.pMin2, p.pMax2)
		if dist < minDist {
			minDist = dist
		}
	}
	return false, minDist, nil
}

// leafCollidesWithLeaf performs collision checks between two leaf nodes using the Geometry interface.
// Geometries are stored in local space and transformed on-demand using the provided poses.
// When a collision is found and witness != nil, the two colliding (local-space) geometries
// are recorded — but only when both are *Triangle, which is the only case (*Mesh).collidesWithMesh
// uses the cache for.
func leafCollidesWithLeaf(
	geoms1, geoms2 []Geometry, pose1, pose2 Pose, collisionBufferMM float64, witness *[2]*Triangle,
) (bool, float64, error) {
	// Fast path: triangle-triangle leaves (the dominant case for Mesh-Mesh checks).
	// Transforms triangles into stack-allocated Triangle values instead of allocating
	// a new *Triangle per Geometry.Transform call. Eliminates ~8 heap allocations per
	// leaf pair in the hot path.
	if allLeafTriangles(geoms1) && allLeafTriangles(geoms2) {
		return triangleLeafCollide(geoms1, geoms2, pose1, pose2, collisionBufferMM, witness)
	}

	minDist := math.Inf(1)
	// Pre-transform geoms2 once to avoid redundant transforms in inner loop.
	worldGeoms2 := make([]Geometry, len(geoms2))
	for i, g := range geoms2 {
		worldGeoms2[i] = g.Transform(pose2)
	}
	for _, g1 := range geoms1 {
		worldG1 := g1.Transform(pose1)
		for j, worldG2 := range worldGeoms2 {
			collides, dist, err := worldG1.CollidesWith(worldG2, collisionBufferMM)
			if err != nil {
				return false, 0, err
			}
			if collides {
				// Record the local-space triangle pair (not the world-space copies)
				// when both leaves are triangles, so the witness cache can re-check
				// under future poses.
				if witness != nil {
					t1, ok1 := g1.(*Triangle)
					t2, ok2 := geoms2[j].(*Triangle)
					if ok1 && ok2 {
						witness[0] = t1
						witness[1] = t2
					}
				}
				return true, -1, nil
			}
			if dist < minDist {
				minDist = dist
			}
		}
	}

	return false, minDist, nil
}

// allLeafTriangles reports whether every geometry in the slice is a *Triangle.
// Cheap (≤ maxGeomsPerLeaf iterations) and lets leafCollidesWithLeaf take the
// allocation-free triangle path when both leaves are all triangles.
func allLeafTriangles(geoms []Geometry) bool {
	for _, g := range geoms {
		if _, ok := g.(*Triangle); !ok {
			return false
		}
	}
	return true
}

// triangleLeafCollide is the allocation-free fast path for triangle-vs-triangle leaf checks.
// Transforms each triangle into a stack-local Triangle value (escape analysis keeps the
// pointers off-heap because collidesWithTriangle does not retain the receivers) and
// avoids the *Triangle allocation that Triangle.Transform performs.
// When witness != nil, records the local-space colliding pair so (*Mesh).collidesWithMesh
// can re-check it directly on subsequent queries with similar poses.
func triangleLeafCollide(
	geoms1, geoms2 []Geometry, pose1, pose2 Pose, collisionBufferMM float64, witness *[2]*Triangle,
) (bool, float64, error) {
	q1 := pose1.Orientation().Quaternion()
	t1 := pose1.Point()
	q2 := pose2.Orientation().Quaternion()
	t2 := pose2.Point()

	// maxGeomsPerLeaf bounds the leaf size, so a fixed-size stack array is safe.
	// triLocals[i] is the original *Triangle from geoms2[i]; recorded into witness
	// on collision so callers get a stable handle (not the world-space copy).
	// triMin/triMax are the world-space 3-point AABBs of each triangle, used
	// as a cheap lower-bound pre-filter on the inner triangle-pair loop.
	var worldT2s [maxGeomsPerLeaf]Triangle
	var triLocals2 [maxGeomsPerLeaf]*Triangle
	var t2Min, t2Max [maxGeomsPerLeaf]r3.Vector
	n2 := len(geoms2)
	bufferN2 := collisionBufferMM * collisionBufferMM
	for i := 0; i < n2; i++ {
		tri := geoms2[i].(*Triangle)
		triLocals2[i] = tri
		worldT2s[i] = Triangle{
			p0:     TransformPoint(q2, t2, tri.p0),
			p1:     TransformPoint(q2, t2, tri.p1),
			p2:     TransformPoint(q2, t2, tri.p2),
			normal: transformDirection(q2, tri.normal),
		}
		t2Min[i], t2Max[i] = triAABB(&worldT2s[i])
	}

	minDist := math.Inf(1)
	for _, g1 := range geoms1 {
		tri1 := g1.(*Triangle)
		worldT1 := Triangle{
			p0:     TransformPoint(q1, t1, tri1.p0),
			p1:     TransformPoint(q1, t1, tri1.p1),
			p2:     TransformPoint(q1, t1, tri1.p2),
			normal: transformDirection(q1, tri1.normal),
		}
		t1MinV, t1MaxV := triAABB(&worldT1)
		for i := 0; i < n2; i++ {
			// AABB pre-filter: skip the (much more expensive) triangle-triangle
			// distance check when the triangles' AABBs are already separated by
			// more than minDist and more than the collision buffer — neither
			// the collision verdict nor the minDist return can improve.
			dx := math.Max(0, math.Max(t1MinV.X-t2Max[i].X, t2Min[i].X-t1MaxV.X))
			dy := math.Max(0, math.Max(t1MinV.Y-t2Max[i].Y, t2Min[i].Y-t1MaxV.Y))
			dz := math.Max(0, math.Max(t1MinV.Z-t2Max[i].Z, t2Min[i].Z-t1MaxV.Z))
			lbN2 := dx*dx + dy*dy + dz*dz
			if lbN2 > bufferN2 && lbN2 >= minDist {
				continue
			}
			collides, dist := worldT1.collidesWithTriangle(&worldT2s[i], collisionBufferMM)
			if collides {
				if witness != nil {
					witness[0] = tri1
					witness[1] = triLocals2[i]
				}
				return true, -1, nil
			}
			// Use squared distance to match the squared lower-bound semantics.
			dist2 := dist * dist
			if dist2 < minDist {
				minDist = dist2
			}
		}
	}
	if math.IsInf(minDist, 1) {
		return false, minDist, nil
	}
	return false, math.Sqrt(minDist), nil
}

// triAABB returns the 3-point AABB of a triangle in whatever space its points are in.
func triAABB(t *Triangle) (r3.Vector, r3.Vector) {
	return r3.Vector{
			X: math.Min(math.Min(t.p0.X, t.p1.X), t.p2.X),
			Y: math.Min(math.Min(t.p0.Y, t.p1.Y), t.p2.Y),
			Z: math.Min(math.Min(t.p0.Z, t.p1.Z), t.p2.Z),
		}, r3.Vector{
			X: math.Max(math.Max(t.p0.X, t.p1.X), t.p2.X),
			Y: math.Max(math.Max(t.p0.Y, t.p1.Y), t.p2.Y),
			Z: math.Max(math.Max(t.p0.Z, t.p1.Z), t.p2.Z),
		}
}

// bvhDistanceFromBVH computes the minimum distance between two BVH trees.
// The BVH nodes store geometries in local space; poses are applied lazily during traversal.
func bvhDistanceFromBVH(node1, node2 *bvhNode, pose1, pose2 Pose) (float64, error) {
	if node1 == nil || node2 == nil {
		return math.Inf(1), nil
	}

	// Transform AABBs to world space
	min1, max1 := transformAABB(node1.min, node1.max, pose1)
	min2, max2 := transformAABB(node2.min, node2.max, pose2)

	// Check if AABBs overlap
	if !aabbOverlap(min1, max1, min2, max2) {
		// If AABBs don't overlap, the AABB distance is a lower bound
		// For distant meshes, this is good enough
		return aabbDistance(min1, max1, min2, max2), nil
	}

	// Both are leaves - compute exact distance
	if node1.geoms != nil && node2.geoms != nil {
		return leafDistanceFromLeaf(node1.geoms, node2.geoms, pose1, pose2)
	}

	// Recurse into children
	if node1.geoms != nil {
		leftDist, err := bvhDistanceFromBVH(node1, node2.left, pose1, pose2)
		if err != nil {
			return 0, err
		}
		rightDist, err := bvhDistanceFromBVH(node1, node2.right, pose1, pose2)
		if err != nil {
			return 0, err
		}
		return math.Min(leftDist, rightDist), nil
	}

	if node2.geoms != nil {
		leftDist, err := bvhDistanceFromBVH(node1.left, node2, pose1, pose2)
		if err != nil {
			return 0, err
		}
		rightDist, err := bvhDistanceFromBVH(node1.right, node2, pose1, pose2)
		if err != nil {
			return 0, err
		}
		return math.Min(leftDist, rightDist), nil
	}

	// Both are internal nodes
	minDist := math.Inf(1)
	pairs := [][2]*bvhNode{
		{node1.left, node2.left},
		{node1.left, node2.right},
		{node1.right, node2.left},
		{node1.right, node2.right},
	}

	for _, pair := range pairs {
		dist, err := bvhDistanceFromBVH(pair[0], pair[1], pose1, pose2)
		if err != nil {
			return 0, err
		}
		if dist < minDist {
			minDist = dist
		}
	}

	return minDist, nil
}

// bvhCollidesWithGeometry traverses the BVH checking against a single geometry.
// The BVH stores geometries in local space; bvhPose is applied lazily during traversal.
// The 'other' geometry is assumed to already be in world space.
func bvhCollidesWithGeometry(
	node *bvhNode,
	bvhPose Pose,
	other Geometry,
	otherMin,
	otherMax r3.Vector,
	buffer float64,
) (bool, float64, error) {
	if node == nil {
		return false, math.Inf(1), nil
	}

	// Transform node AABB to world space
	nodeMin, nodeMax := transformAABB(node.min, node.max, bvhPose)

	// Expand node AABB by buffer
	nodeMin.X -= buffer
	nodeMin.Y -= buffer
	nodeMin.Z -= buffer
	nodeMax.X += buffer
	nodeMax.Y += buffer
	nodeMax.Z += buffer

	// Early exit if AABBs don't overlap
	if !aabbOverlap(nodeMin, nodeMax, otherMin, otherMax) {
		return false, aabbDistance(nodeMin, nodeMax, otherMin, otherMax), nil
	}

	// Leaf node: check each geometry against other
	if node.geoms != nil {
		minDist := math.Inf(1)
		for _, g := range node.geoms {
			// Transform geometry to world space
			worldG := g.Transform(bvhPose)
			collides, dist, err := worldG.CollidesWith(other, buffer)
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
		return false, minDist, nil
	}

	// Internal node: recurse
	leftCollide, leftDist, err := bvhCollidesWithGeometry(node.left, bvhPose, other, otherMin, otherMax, buffer)
	if err != nil || leftCollide {
		return leftCollide, leftDist, err
	}
	rightCollide, rightDist, err := bvhCollidesWithGeometry(node.right, bvhPose, other, otherMin, otherMax, buffer)
	if err != nil || rightCollide {
		return rightCollide, rightDist, err
	}
	return false, math.Min(leftDist, rightDist), nil
}

// leafDistanceFromLeaf computes the minimum distance between two sets of geometries.
// Geometries are stored in local space and transformed on-demand using the provided poses.
func leafDistanceFromLeaf(geoms1, geoms2 []Geometry, pose1, pose2 Pose) (float64, error) {
	minDist := math.Inf(1)
	worldGeoms1 := make([]Geometry, len(geoms1))
	worldGeoms2 := make([]Geometry, len(geoms2))

	for i, g := range geoms1 {
		worldGeoms1[i] = g.Transform(pose1)
	}
	for i, g := range geoms2 {
		worldGeoms2[i] = g.Transform(pose2)
	}

	for _, worldG1 := range worldGeoms1 {
		for _, worldG2 := range worldGeoms2 {
			// Use the Geometry interface's DistanceFrom method
			dist, err := worldG1.DistanceFrom(worldG2)
			if err != nil {
				return 0, err
			}
			if dist < minDist {
				minDist = dist
			}
		}
	}

	return minDist, nil
}
