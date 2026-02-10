package spatialmath

import (
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/golang/geo/r3"
)

var (
	benchmarkVecSink   r3.Vector
	benchmarkFloatSink float64
	benchmarkBoolSink  bool
	benchmarkNodeSink  *bvhNode
)

func benchmarkTriangleGeometries(count int) []Geometry {
	rng := rand.New(rand.NewSource(42))
	geoms := make([]Geometry, count)
	for i := 0; i < count; i++ {
		cx := rng.Float64() * 1000
		cy := rng.Float64() * 1000
		cz := rng.Float64() * 1000
		// Keep triangles non-degenerate while varying shape/orientation.
		p0 := r3.Vector{X: cx, Y: cy, Z: cz}
		p1 := r3.Vector{X: cx + 5 + rng.Float64()*3, Y: cy + rng.Float64()*2, Z: cz + rng.Float64()*2}
		p2 := r3.Vector{X: cx + rng.Float64()*2, Y: cy + 5 + rng.Float64()*3, Z: cz + rng.Float64()*2}
		geoms[i] = NewTriangle(p0, p1, p2)
	}
	return geoms
}

func benchmarkSortByPosePoint(geoms []Geometry, axis int) {
	sort.Slice(geoms, func(i, j int) bool {
		ci := geoms[i].Pose().Point()
		cj := geoms[j].Pose().Point()
		switch axis {
		case 0:
			return ci.X < cj.X
		case 1:
			return ci.Y < cj.Y
		default:
			return ci.Z < cj.Z
		}
	})
}

func benchmarkSortByCachedCentroid(geoms []Geometry, axis int) {
	centroids := make([]r3.Vector, len(geoms))
	for i, g := range geoms {
		centroids[i] = geometryCentroid(g)
	}
	sort.Sort(geometryCentroidSorter{
		geoms:     geoms,
		centroids: centroids,
		axis:      axis,
	})
}

func benchmarkLegacyTransformAABB(minPt, maxPt r3.Vector, pose Pose) (r3.Vector, r3.Vector) {
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
		newMin, newMax = expandAABB(newMin, newMax, worldPt)
	}
	return newMin, newMax
}

func benchmarkLegacyLeafDistanceFromLeaf(geoms1, geoms2 []Geometry, pose1, pose2 Pose) (float64, error) {
	minDist := math.Inf(1)
	for _, g1 := range geoms1 {
		worldG1 := g1.Transform(pose1)
		for _, g2 := range geoms2 {
			worldG2 := g2.Transform(pose2)
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

func benchmarkLegacyLeafCollidesWithLeaf(geoms1, geoms2 []Geometry, pose1, pose2 Pose, collisionBufferMM float64) (bool, float64, error) {
	minDist := math.Inf(1)
	for _, g1 := range geoms1 {
		worldG1 := g1.Transform(pose1)
		for _, g2 := range geoms2 {
			worldG2 := g2.Transform(pose2)
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

func benchmarkBuildBVHLegacy(geoms []Geometry) *bvhNode {
	if len(geoms) == 0 {
		return nil
	}
	return benchmarkBuildBVHLegacyNode(geoms)
}

func benchmarkBuildBVHLegacyNode(geoms []Geometry) *bvhNode {
	node := &bvhNode{}
	node.min, node.max = computeGeomsAABB(geoms)

	if len(geoms) <= maxGeomsPerLeaf {
		node.geoms = geoms
		return node
	}

	extent := node.max.Sub(node.min)
	axis := 0
	if extent.Y > extent.X && extent.Y > extent.Z {
		axis = 1
	} else if extent.Z > extent.X && extent.Z > extent.Y {
		axis = 2
	}

	sort.Slice(geoms, func(i, j int) bool {
		ci := geoms[i].Pose().Point()
		cj := geoms[j].Pose().Point()
		switch axis {
		case 0:
			return ci.X < cj.X
		case 1:
			return ci.Y < cj.Y
		default:
			return ci.Z < cj.Z
		}
	})

	mid := len(geoms) / 2
	node.left = benchmarkBuildBVHLegacyNode(geoms[:mid])
	node.right = benchmarkBuildBVHLegacyNode(geoms[mid:])
	return node
}

func benchmarkBvhCollidesWithBVHLegacyLeaves(node1, node2 *bvhNode, pose1, pose2 Pose, collisionBufferMM float64) (bool, float64, error) {
	if node1 == nil || node2 == nil {
		return false, math.Inf(1), nil
	}

	min1, max1 := transformAABB(node1.min, node1.max, pose1)
	min2, max2 := transformAABB(node2.min, node2.max, pose2)

	min1.X -= collisionBufferMM
	min1.Y -= collisionBufferMM
	min1.Z -= collisionBufferMM
	max1.X += collisionBufferMM
	max1.Y += collisionBufferMM
	max1.Z += collisionBufferMM

	if !aabbOverlap(min1, max1, min2, max2) {
		return false, aabbDistance(min1, max1, min2, max2), nil
	}

	if node1.geoms != nil && node2.geoms != nil {
		return benchmarkLegacyLeafCollidesWithLeaf(node1.geoms, node2.geoms, pose1, pose2, collisionBufferMM)
	}

	if node1.geoms != nil {
		leftCollide, leftDist, err := benchmarkBvhCollidesWithBVHLegacyLeaves(node1, node2.left, pose1, pose2, collisionBufferMM)
		if err != nil {
			return false, 0, err
		}
		if leftCollide {
			return true, leftDist, nil
		}
		rightCollide, rightDist, err := benchmarkBvhCollidesWithBVHLegacyLeaves(node1, node2.right, pose1, pose2, collisionBufferMM)
		if err != nil {
			return false, 0, err
		}
		if rightCollide {
			return true, rightDist, nil
		}
		return false, math.Min(leftDist, rightDist), nil
	}

	if node2.geoms != nil {
		leftCollide, leftDist, err := benchmarkBvhCollidesWithBVHLegacyLeaves(node1.left, node2, pose1, pose2, collisionBufferMM)
		if err != nil {
			return false, 0, err
		}
		if leftCollide {
			return true, leftDist, nil
		}
		rightCollide, rightDist, err := benchmarkBvhCollidesWithBVHLegacyLeaves(node1.right, node2, pose1, pose2, collisionBufferMM)
		if err != nil {
			return false, 0, err
		}
		if rightCollide {
			return true, rightDist, nil
		}
		return false, math.Min(leftDist, rightDist), nil
	}

	minDist := math.Inf(1)
	pairs := [][2]*bvhNode{
		{node1.left, node2.left},
		{node1.left, node2.right},
		{node1.right, node2.left},
		{node1.right, node2.right},
	}

	for _, pair := range pairs {
		collide, dist, err := benchmarkBvhCollidesWithBVHLegacyLeaves(pair[0], pair[1], pose1, pose2, collisionBufferMM)
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

func benchmarkBvhDistanceFromBVHLegacyLeaves(node1, node2 *bvhNode, pose1, pose2 Pose) (float64, error) {
	if node1 == nil || node2 == nil {
		return math.Inf(1), nil
	}

	min1, max1 := transformAABB(node1.min, node1.max, pose1)
	min2, max2 := transformAABB(node2.min, node2.max, pose2)
	if !aabbOverlap(min1, max1, min2, max2) {
		return aabbDistance(min1, max1, min2, max2), nil
	}

	if node1.geoms != nil && node2.geoms != nil {
		return benchmarkLegacyLeafDistanceFromLeaf(node1.geoms, node2.geoms, pose1, pose2)
	}

	if node1.geoms != nil {
		leftDist, err := benchmarkBvhDistanceFromBVHLegacyLeaves(node1, node2.left, pose1, pose2)
		if err != nil {
			return 0, err
		}
		rightDist, err := benchmarkBvhDistanceFromBVHLegacyLeaves(node1, node2.right, pose1, pose2)
		if err != nil {
			return 0, err
		}
		return math.Min(leftDist, rightDist), nil
	}

	if node2.geoms != nil {
		leftDist, err := benchmarkBvhDistanceFromBVHLegacyLeaves(node1.left, node2, pose1, pose2)
		if err != nil {
			return 0, err
		}
		rightDist, err := benchmarkBvhDistanceFromBVHLegacyLeaves(node1.right, node2, pose1, pose2)
		if err != nil {
			return 0, err
		}
		return math.Min(leftDist, rightDist), nil
	}

	minDist := math.Inf(1)
	pairs := [][2]*bvhNode{
		{node1.left, node2.left},
		{node1.left, node2.right},
		{node1.right, node2.left},
		{node1.right, node2.right},
	}
	for _, pair := range pairs {
		dist, err := benchmarkBvhDistanceFromBVHLegacyLeaves(pair[0], pair[1], pose1, pose2)
		if err != nil {
			return 0, err
		}
		if dist < minDist {
			minDist = dist
		}
	}
	return minDist, nil
}

func BenchmarkTriangleCentroidExtraction(b *testing.B) {
	b.ReportAllocs()
	tri := NewTriangle(
		r3.Vector{X: 1, Y: 2, Z: 3},
		r3.Vector{X: 4, Y: 5, Z: 6},
		r3.Vector{X: 7, Y: 8, Z: 9},
	)

	b.Run("pose_point", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			benchmarkVecSink = tri.Pose().Point()
		}
	})

	b.Run("centroid", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			benchmarkVecSink = tri.Centroid()
		}
	})
}

func BenchmarkBVHCentroidSort(b *testing.B) {
	b.ReportAllocs()
	const triCount = 7492
	src := benchmarkTriangleGeometries(triCount)

	b.Run("legacy_pose_point_in_comparator", func(b *testing.B) {
		geoms := make([]Geometry, triCount)
		for i := 0; i < b.N; i++ {
			copy(geoms, src)
			benchmarkSortByPosePoint(geoms, 0)
		}
		benchmarkVecSink = geoms[0].Pose().Point()
	})

	b.Run("cached_centroid", func(b *testing.B) {
		geoms := make([]Geometry, triCount)
		for i := 0; i < b.N; i++ {
			copy(geoms, src)
			benchmarkSortByCachedCentroid(geoms, 0)
		}
		benchmarkVecSink = geoms[0].Pose().Point()
	})
}

func BenchmarkBVHBuild(b *testing.B) {
	b.ReportAllocs()
	const triCount = 7492
	src := benchmarkTriangleGeometries(triCount)

	b.Run("legacy_pose_point_sort", func(b *testing.B) {
		geoms := make([]Geometry, triCount)
		for i := 0; i < b.N; i++ {
			copy(geoms, src)
			benchmarkNodeSink = benchmarkBuildBVHLegacy(geoms)
		}
	})

	b.Run("cached_centroid_sort", func(b *testing.B) {
		geoms := make([]Geometry, triCount)
		for i := 0; i < b.N; i++ {
			copy(geoms, src)
			benchmarkNodeSink = buildBVH(geoms)
		}
	})
}

func BenchmarkPointTransform(b *testing.B) {
	b.ReportAllocs()
	pose := NewPose(
		r3.Vector{X: 300, Y: -120, Z: 40},
		&OrientationVector{OX: 0.3, OY: -0.2, OZ: 0.9, Theta: 1.2},
	)
	pt := r3.Vector{X: -12, Y: 8, Z: 5}
	rm := pose.Orientation().RotationMatrix()
	trans := pose.Point()

	b.Run("dual_quaternion_compose_point", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			benchmarkVecSink = Compose(pose, NewPoseFromPoint(pt)).Point()
		}
	})

	b.Run("rotation_matrix_transform_point", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			benchmarkVecSink = TransformPoint(rm, trans, pt)
		}
	})
}

func BenchmarkTransformAABB(b *testing.B) {
	b.ReportAllocs()
	minPt := r3.Vector{X: -100, Y: -80, Z: -60}
	maxPt := r3.Vector{X: 100, Y: 80, Z: 60}
	pose := NewPose(
		r3.Vector{X: 250, Y: -75, Z: 30},
		&OrientationVector{OX: 0.1, OY: 0.7, OZ: 0.7, Theta: 0.9},
	)

	b.Run("legacy_corners_compose", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			benchmarkVecSink, _ = benchmarkLegacyTransformAABB(minPt, maxPt, pose)
		}
	})

	b.Run("arvo_extents", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			benchmarkVecSink, _ = transformAABB(minPt, maxPt, pose)
		}
	})
}

func BenchmarkLeafDistanceFromLeaf(b *testing.B) {
	b.ReportAllocs()
	geoms1 := benchmarkTriangleGeometries(4)
	geoms2 := benchmarkTriangleGeometries(4)
	pose1 := NewPose(
		r3.Vector{X: 0, Y: 0, Z: 0},
		&OrientationVector{OX: 0, OY: 0, OZ: 1, Theta: 0.2},
	)
	pose2 := NewPose(
		r3.Vector{X: 30, Y: -15, Z: 10},
		&OrientationVector{OX: 1, OY: 1, OZ: 0, Theta: 0.5},
	)

	b.Run("legacy_transform_inside_inner_loop", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d, err := benchmarkLegacyLeafDistanceFromLeaf(geoms1, geoms2, pose1, pose2)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkFloatSink = d
		}
	})

	b.Run("pretransform_leaves_once", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d, err := leafDistanceFromLeaf(geoms1, geoms2, pose1, pose2)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkFloatSink = d
		}
	})
}

func BenchmarkBVHVsBVHQuery(b *testing.B) {
	b.ReportAllocs()
	const triCount = 1024
	src := benchmarkTriangleGeometries(triCount)

	geoms1 := make([]Geometry, triCount)
	geoms2 := make([]Geometry, triCount)
	copy(geoms1, src)
	copy(geoms2, src)
	node1 := buildBVH(geoms1)
	node2 := buildBVH(geoms2)

	identity := NewZeroPose()
	farPose := NewPose(r3.Vector{X: 10000, Y: 10000, Z: 10000}, NewZeroOrientation())

	b.Run("collides_current_far_reject", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collides, d, err := bvhCollidesWithBVH(node1, node2, identity, farPose, 0)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkBoolSink = collides
			benchmarkFloatSink = d
		}
	})

	b.Run("collides_current_overlap", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collides, d, err := bvhCollidesWithBVH(node1, node2, identity, identity, 0)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkBoolSink = collides
			benchmarkFloatSink = d
		}
	})

	b.Run("collides_legacy_leaf_loop_overlap", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collides, d, err := benchmarkBvhCollidesWithBVHLegacyLeaves(node1, node2, identity, identity, 0)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkBoolSink = collides
			benchmarkFloatSink = d
		}
	})

	b.Run("distance_current_overlap", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d, err := bvhDistanceFromBVH(node1, node2, identity, identity)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkFloatSink = d
		}
	})

	b.Run("distance_legacy_leaf_loop_overlap", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d, err := benchmarkBvhDistanceFromBVHLegacyLeaves(node1, node2, identity, identity)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkFloatSink = d
		}
	})
}
