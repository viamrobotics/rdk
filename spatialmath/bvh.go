package spatialmath

import (
	"fmt"
	"math"
	"sort"

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
		// Use existing BVH bounds if available
		if geom.bvh != nil {
			return geom.bvh.min, geom.bvh.max
		}
		// Fallback: compute from triangles
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
func rotatedAABBExtents(rm *RotationMatrix, extents r3.Vector) r3.Vector {
	return r3.Vector{
		X: math.Abs(rm.At(0, 0))*extents.X + math.Abs(rm.At(0, 1))*extents.Y + math.Abs(rm.At(0, 2))*extents.Z,
		Y: math.Abs(rm.At(1, 0))*extents.X + math.Abs(rm.At(1, 1))*extents.Y + math.Abs(rm.At(1, 2))*extents.Z,
		Z: math.Abs(rm.At(2, 0))*extents.X + math.Abs(rm.At(2, 1))*extents.Y + math.Abs(rm.At(2, 2))*extents.Z,
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
	rm := pose.Orientation().RotationMatrix()
	q := pose.Orientation().Quaternion()
	trans := pose.Point()

	center := minPt.Add(maxPt).Mul(0.5)
	extents := maxPt.Sub(minPt).Mul(0.5)

	worldCenter := TransformPoint(q, trans, center)
	worldExtents := rotatedAABBExtents(rm, extents)
	return aabbFromCenterExtents(worldCenter, worldExtents)
}

// bvhCollidesWithBVH checks if two BVH trees collide, using the given poses to transform them.
// The BVH nodes store geometries in local space; poses are applied lazily during traversal.
func bvhCollidesWithBVH(node1, node2 *bvhNode, pose1, pose2 Pose, collisionBufferMM float64) (bool, float64, error) {
	if node1 == nil || node2 == nil {
		return false, math.Inf(1), nil
	}

	// Transform AABBs to world space
	min1, max1 := transformAABB(node1.min, node1.max, pose1)
	min2, max2 := transformAABB(node2.min, node2.max, pose2)

	// Expand first AABB by collision buffer
	min1.X -= collisionBufferMM
	min1.Y -= collisionBufferMM
	min1.Z -= collisionBufferMM
	max1.X += collisionBufferMM
	max1.Y += collisionBufferMM
	max1.Z += collisionBufferMM

	// Check if AABBs overlap
	if !aabbOverlap(min1, max1, min2, max2) {
		return false, aabbDistance(min1, max1, min2, max2), nil
	}

	// Both are leaves - do geometry-geometry checks
	if node1.geoms != nil && node2.geoms != nil {
		return leafCollidesWithLeaf(node1.geoms, node2.geoms, pose1, pose2, collisionBufferMM)
	}

	// Recurse into children
	// Strategy: descend into the larger node first for better culling
	if node1.geoms != nil {
		// node1 is leaf, recurse into node2's children
		leftCollide, leftDist, err := bvhCollidesWithBVH(node1, node2.left, pose1, pose2, collisionBufferMM)
		if err != nil {
			return false, 0, err
		}
		if leftCollide {
			return true, leftDist, nil
		}
		rightCollide, rightDist, err := bvhCollidesWithBVH(node1, node2.right, pose1, pose2, collisionBufferMM)
		if err != nil {
			return false, 0, err
		}
		if rightCollide {
			return true, rightDist, nil
		}
		return false, math.Min(leftDist, rightDist), nil
	}

	if node2.geoms != nil {
		// node2 is leaf, recurse into node1's children
		leftCollide, leftDist, err := bvhCollidesWithBVH(node1.left, node2, pose1, pose2, collisionBufferMM)
		if err != nil {
			return false, 0, err
		}
		if leftCollide {
			return true, leftDist, nil
		}
		rightCollide, rightDist, err := bvhCollidesWithBVH(node1.right, node2, pose1, pose2, collisionBufferMM)
		if err != nil {
			return false, 0, err
		}
		if rightCollide {
			return true, rightDist, nil
		}
		return false, math.Min(leftDist, rightDist), nil
	}

	// Both are internal nodes - check all 4 combinations
	minDist := math.Inf(1)
	pairs := [][2]*bvhNode{
		{node1.left, node2.left},
		{node1.left, node2.right},
		{node1.right, node2.left},
		{node1.right, node2.right},
	}

	for _, pair := range pairs {
		collide, dist, err := bvhCollidesWithBVH(pair[0], pair[1], pose1, pose2, collisionBufferMM)
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

	return false, minDist, nil
}

// leafCollidesWithLeaf performs collision checks between two leaf nodes using the Geometry interface.
// Geometries are stored in local space and transformed on-demand using the provided poses.
func leafCollidesWithLeaf(geoms1, geoms2 []Geometry, pose1, pose2 Pose, collisionBufferMM float64) (bool, float64, error) {
	minDist := math.Inf(1)
	for _, g1 := range geoms1 {
		// Transform geometry to world space.
		worldG1 := g1.Transform(pose1)
		for _, g2 := range geoms2 {
			// Transform geometry to world space.
			worldG2 := g2.Transform(pose2)
			// Use the Geometry interface's CollidesWith method
			collides, dist, err := worldG1.CollidesWith(worldG2, collisionBufferMM)
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
	}

	return false, minDist, nil
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
