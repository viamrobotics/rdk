package spatialmath

import (
	"math"
	"sort"

	"github.com/golang/geo/r3"
)

// bvhNode represents a node in a Bounding Volume Hierarchy tree.
// Each node has an axis-aligned bounding box (AABB) and either:
// - Two children (internal node), or
// - A list of triangles (leaf node)
// wikipedia.org/wiki/Bounding_volume_hierarchy
type bvhNode struct {
	min, max   r3.Vector   // AABB bounds
	left       *bvhNode    // left child (nil for leaf)
	right      *bvhNode    // right child (nil for leaf)
	triangles  []*Triangle // triangles (only for leaf nodes)
}

// maxTrianglesPerLeaf is the threshold for splitting BVH nodes.
const maxTrianglesPerLeaf = 4

// buildBVH constructs a BVH from a list of triangles.
func buildBVH(triangles []*Triangle) *bvhNode {
	if len(triangles) == 0 {
		return nil
	}
	return buildBVHNode(triangles)
}

func buildBVHNode(triangles []*Triangle) *bvhNode {
	node := &bvhNode{}

	// Compute AABB for all triangles
	node.min, node.max = computeTrianglesAABB(triangles)

	// If few enough triangles, make this a leaf node
	if len(triangles) <= maxTrianglesPerLeaf {
		node.triangles = triangles
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

	// Sort triangles by centroid along the chosen axis
	sort.Slice(triangles, func(i, j int) bool {
		ci := triangleCentroid(triangles[i])
		cj := triangleCentroid(triangles[j])
		switch axis {
		case 0:
			return ci.X < cj.X
		case 1:
			return ci.Y < cj.Y
		default:
			return ci.Z < cj.Z
		}
	})

	// Split at median
	mid := len(triangles) / 2
	node.left = buildBVHNode(triangles[:mid])
	node.right = buildBVHNode(triangles[mid:])

	return node
}

// computeTrianglesAABB computes the AABB encompassing all given triangles.
func computeTrianglesAABB(triangles []*Triangle) (min, max r3.Vector) {
	min = r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	max = r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}

	for _, tri := range triangles {
		for _, pt := range tri.Points() {
			min.X = math.Min(min.X, pt.X)
			min.Y = math.Min(min.Y, pt.Y)
			min.Z = math.Min(min.Z, pt.Z)
			max.X = math.Max(max.X, pt.X)
			max.Y = math.Max(max.Y, pt.Y)
			max.Z = math.Max(max.Z, pt.Z)
		}
	}
	return min, max
}

// triangleCentroid returns the centroid of a triangle.
func triangleCentroid(t *Triangle) r3.Vector {
	pts := t.Points()
	return r3.Vector{
		X: (pts[0].X + pts[1].X + pts[2].X) / 3,
		Y: (pts[0].Y + pts[1].Y + pts[2].Y) / 3,
		Z: (pts[0].Z + pts[1].Z + pts[2].Z) / 3,
	}
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
func transformAABB(min, max r3.Vector, pose Pose) (newMin, newMax r3.Vector) {
	// Get the 8 corners of the AABB
	corners := []r3.Vector{
		{X: min.X, Y: min.Y, Z: min.Z},
		{X: min.X, Y: min.Y, Z: max.Z},
		{X: min.X, Y: max.Y, Z: min.Z},
		{X: min.X, Y: max.Y, Z: max.Z},
		{X: max.X, Y: min.Y, Z: min.Z},
		{X: max.X, Y: min.Y, Z: max.Z},
		{X: max.X, Y: max.Y, Z: min.Z},
		{X: max.X, Y: max.Y, Z: max.Z},
	}

	newMin = r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	newMax = r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}

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

// bvhCollidesWithBVH checks if two BVH trees collide, using the given poses to transform them.
// Returns (collides, minDistance).
func bvhCollidesWithBVH(node1 *bvhNode, pose1 Pose, node2 *bvhNode, pose2 Pose, collisionBufferMM float64) (bool, float64) {
	if node1 == nil || node2 == nil {
		return false, math.Inf(1)
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
		return false, aabbDistance(min1, max1, min2, max2)
	}

	// Both are leaves - do triangle-triangle checks
	if node1.triangles != nil && node2.triangles != nil {
		return leafCollidesWithLeaf(node1.triangles, pose1, node2.triangles, pose2, collisionBufferMM)
	}

	// Recurse into children
	// Strategy: descend into the larger node first for better culling
	if node1.triangles != nil {
		// node1 is leaf, recurse into node2's children
		leftCollide, leftDist := bvhCollidesWithBVH(node1, pose1, node2.left, pose2, collisionBufferMM)
		if leftCollide {
			return true, -1
		}
		rightCollide, rightDist := bvhCollidesWithBVH(node1, pose1, node2.right, pose2, collisionBufferMM)
		if rightCollide {
			return true, -1
		}
		return false, math.Min(leftDist, rightDist)
	}

	if node2.triangles != nil {
		// node2 is leaf, recurse into node1's children
		leftCollide, leftDist := bvhCollidesWithBVH(node1.left, pose1, node2, pose2, collisionBufferMM)
		if leftCollide {
			return true, -1
		}
		rightCollide, rightDist := bvhCollidesWithBVH(node1.right, pose1, node2, pose2, collisionBufferMM)
		if rightCollide {
			return true, -1
		}
		return false, math.Min(leftDist, rightDist)
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
		collide, dist := bvhCollidesWithBVH(pair[0], pose1, pair[1], pose2, collisionBufferMM)
		if collide {
			return true, -1
		}
		if dist < minDist {
			minDist = dist
		}
	}

	return false, minDist
}

// leafCollidesWithLeaf performs triangle-triangle collision between two leaf nodes.
func leafCollidesWithLeaf(tris1 []*Triangle, pose1 Pose, tris2 []*Triangle, pose2 Pose, collisionBufferMM float64) (bool, float64) {
	minDist := math.Inf(1)

	for _, t1 := range tris1 {
		worldTri1 := t1.Transform(pose1)
		p1 := worldTri1.Points()

		for _, t2 := range tris2 {
			worldTri2 := t2.Transform(pose2)
			p2 := worldTri2.Points()

			// Check segments from tri1 against tri2
			for i := 0; i < 3; i++ {
				start := p1[i]
				end := p1[(i+1)%3]
				bestSegPt, bestTriPt := ClosestPointsSegmentTriangle(start, end, worldTri2)
				dist := bestSegPt.Sub(bestTriPt).Norm()
				if dist <= collisionBufferMM {
					return true, -1
				}
				if dist < minDist {
					minDist = dist
				}
			}

			// Check segments from tri2 against tri1
			for i := 0; i < 3; i++ {
				start := p2[i]
				end := p2[(i+1)%3]
				bestSegPt, bestTriPt := ClosestPointsSegmentTriangle(start, end, worldTri1)
				dist := bestSegPt.Sub(bestTriPt).Norm()
				if dist <= collisionBufferMM {
					return true, -1
				}
				if dist < minDist {
					minDist = dist
				}
			}
		}
	}

	return false, minDist
}

// bvhDistanceFromBVH computes the minimum distance between two BVH trees.
func bvhDistanceFromBVH(node1 *bvhNode, pose1 Pose, node2 *bvhNode, pose2 Pose) float64 {
	if node1 == nil || node2 == nil {
		return math.Inf(1)
	}

	// Transform AABBs to world space
	min1, max1 := transformAABB(node1.min, node1.max, pose1)
	min2, max2 := transformAABB(node2.min, node2.max, pose2)

	// Check if AABBs overlap
	if !aabbOverlap(min1, max1, min2, max2) {
		// If AABBs don't overlap, the AABB distance is a lower bound
		// For distant meshes, this is good enough
		return aabbDistance(min1, max1, min2, max2)
	}

	// Both are leaves - compute exact distance
	if node1.triangles != nil && node2.triangles != nil {
		return leafDistanceFromLeaf(node1.triangles, pose1, node2.triangles, pose2)
	}

	// Recurse into children
	if node1.triangles != nil {
		leftDist := bvhDistanceFromBVH(node1, pose1, node2.left, pose2)
		rightDist := bvhDistanceFromBVH(node1, pose1, node2.right, pose2)
		return math.Min(leftDist, rightDist)
	}

	if node2.triangles != nil {
		leftDist := bvhDistanceFromBVH(node1.left, pose1, node2, pose2)
		rightDist := bvhDistanceFromBVH(node1.right, pose1, node2, pose2)
		return math.Min(leftDist, rightDist)
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
		dist := bvhDistanceFromBVH(pair[0], pose1, pair[1], pose2)
		if dist < minDist {
			minDist = dist
		}
	}

	return minDist
}

// leafDistanceFromLeaf computes the minimum distance between two sets of triangles.
func leafDistanceFromLeaf(tris1 []*Triangle, pose1 Pose, tris2 []*Triangle, pose2 Pose) float64 {
	minDist := math.Inf(1)

	for _, t1 := range tris1 {
		worldTri1 := t1.Transform(pose1)
		p1 := worldTri1.Points()

		for _, t2 := range tris2 {
			worldTri2 := t2.Transform(pose2)
			p2 := worldTri2.Points()

			// Check segments from tri1 against tri2
			for i := 0; i < 3; i++ {
				start := p1[i]
				end := p1[(i+1)%3]
				bestSegPt, bestTriPt := ClosestPointsSegmentTriangle(start, end, worldTri2)
				dist := bestSegPt.Sub(bestTriPt).Norm()
				if dist < minDist {
					minDist = dist
				}
			}

			// Check segments from tri2 against tri1
			for i := 0; i < 3; i++ {
				start := p2[i]
				end := p2[(i+1)%3]
				bestSegPt, bestTriPt := ClosestPointsSegmentTriangle(start, end, worldTri1)
				dist := bestSegPt.Sub(bestTriPt).Norm()
				if dist < minDist {
					minDist = dist
				}
			}
		}
	}

	return minDist
}
