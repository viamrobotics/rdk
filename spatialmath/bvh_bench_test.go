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

func BenchmarkTriangleTriangleCollide(b *testing.B) {
	b.ReportAllocs()
	// Two triangles that are close but not overlapping — exercises the full
	// segment-segment fallback path inside collidesWithTriangle.
	t1 := NewTriangle(
		r3.Vector{X: 0, Y: 0, Z: 0},
		r3.Vector{X: 10, Y: 0, Z: 0},
		r3.Vector{X: 0, Y: 10, Z: 0},
	)
	t2 := NewTriangle(
		r3.Vector{X: 1, Y: 1, Z: 2},
		r3.Vector{X: 11, Y: 1, Z: 2},
		r3.Vector{X: 1, Y: 11, Z: 2},
	)
	for i := 0; i < b.N; i++ {
		collides, d, _ := t1.CollidesWith(t2, 0)
		benchmarkBoolSink = collides
		benchmarkFloatSink = d
	}
}

func BenchmarkSegmentSegmentClosest(b *testing.B) {
	b.ReportAllocs()
	a1 := r3.Vector{X: 0, Y: 0, Z: 0}
	a2 := r3.Vector{X: 10, Y: 0, Z: 0}
	b1 := r3.Vector{X: 5, Y: 1, Z: 1}
	b2 := r3.Vector{X: 5, Y: 1, Z: 10}
	for i := 0; i < b.N; i++ {
		p, q := ClosestPointsSegmentSegment(a1, a2, b1, b2)
		benchmarkVecSink = p
		benchmarkVecSink = q
	}
}

func BenchmarkSegmentPointClosest(b *testing.B) {
	b.ReportAllocs()
	pt1 := r3.Vector{X: 0, Y: 0, Z: 0}
	pt2 := r3.Vector{X: 10, Y: 0, Z: 0}
	// Cycle through queries that hit each branch of the function to avoid
	// branch-prediction bias on a single hot path.
	queries := [3]r3.Vector{
		{X: -5, Y: 1, Z: 0}, // t <= 0 (returns pt1)
		{X: 15, Y: 1, Z: 0}, // t >= 1 (returns pt2)
		{X: 5, Y: 3, Z: 0},  // interpolated
	}
	for i := 0; i < b.N; i++ {
		benchmarkVecSink = ClosestPointSegmentPoint(pt1, pt2, queries[i%3])
	}
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
	q := pose.Orientation().Quaternion()
	trans := pose.Point()

	b.Run("dual_quaternion_compose_point", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			benchmarkVecSink = Compose(pose, NewPoseFromPoint(pt)).Point()
		}
	})

	b.Run("quaternion_transform_point", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			benchmarkVecSink = TransformPoint(q, trans, pt)
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

	b.Run("distance_current_overlap", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d, err := bvhDistanceFromBVH(node1, node2, identity, identity)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkFloatSink = d
		}
	})

	// Realistic mid-range case: trees nearby but not colliding, forcing full traversal
	// before the no-collision verdict. This is the typical workload shape — neither
	// trivial-reject nor trivial-collide.
	nearMissPose := NewPose(r3.Vector{X: 1100, Y: 0, Z: 0}, NewZeroOrientation())
	b.Run("collides_near_miss", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collides, d, err := bvhCollidesWithBVH(node1, node2, identity, nearMissPose, 0)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkBoolSink = collides
			benchmarkFloatSink = d
		}
	})

	// Rotated pose: exercises the cached pose decomposition path under a non-identity
	// rotation matrix where the cache savings matter most.
	rotatedPose := NewPose(
		r3.Vector{X: 600, Y: 100, Z: 50},
		&OrientationVector{OX: 0.2, OY: 0.7, OZ: 0.7, Theta: 0.4},
	)
	b.Run("collides_rotated_partial", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collides, d, err := bvhCollidesWithBVH(node1, node2, identity, rotatedPose, 0)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkBoolSink = collides
			benchmarkFloatSink = d
		}
	})
}

// BenchmarkMeshEdgeInterpolation simulates the motion planner's edge-check
// workload: a sequence of collision queries between the same two meshes under
// slightly-different poses, mimicking interpolated states along one RRT edge.
// Without the witness cache each query traverses the full BVH; with it, the
// first cache write lets subsequent queries skip the BVH and just re-check the
// cached colliding triangle pair.
//
// Two mesh layouts:
//   - "scattered": background triangles + small overlap zone, like a robot link
//     vs an obstacle where only a few triangle pairs are actually in contact.
//   - "interpenetrating": meshes whose AABBs are mostly overlapping, like the
//     case where collision is "deep" and persists across pose perturbations.
func BenchmarkMeshEdgeInterpolation(b *testing.B) {
	const bgCount = 1024
	bg1 := benchmarkTriangleGeometries(bgCount)
	bg2 := benchmarkTriangleGeometries(bgCount)
	tris1 := make([]*Triangle, bgCount+1)
	tris2 := make([]*Triangle, bgCount+1)
	for i := 0; i < bgCount; i++ {
		tris1[i] = bg1[i].(*Triangle)
		tris2[i] = bg2[i].(*Triangle)
	}
	// Add a deliberately-interpenetrating triangle pair: two triangles that
	// cross each other so collisions are robust against small pose shifts
	// (unlike coplanar shifts of identical triangles, which are degenerate).
	tris1[bgCount] = NewTriangle(
		r3.Vector{X: 100, Y: 100, Z: 0},
		r3.Vector{X: 110, Y: 100, Z: 0},
		r3.Vector{X: 105, Y: 110, Z: 0},
	)
	tris2[bgCount] = NewTriangle(
		r3.Vector{X: 105, Y: 105, Z: -5},
		r3.Vector{X: 105, Y: 105, Z: 5},
		r3.Vector{X: 115, Y: 105, Z: 0},
	)

	// "Interpolation": tiny translations along a short trajectory. 32 steps
	// approximates the resolution checkPath uses per planner edge.
	const steps = 32
	poses := make([]Pose, steps)
	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps-1)
		poses[i] = NewPose(r3.Vector{X: t * 0.5, Y: t * 0.3, Z: t * 0.2}, NewZeroOrientation())
	}

	// Pre-transform once per pose to isolate the cache benefit from the
	// per-iter Mesh.Transform cost (which is identical between warm and cold
	// runs and would otherwise dominate).
	mesh1 := NewMesh(NewZeroPose(), tris1, "m1")
	mesh2 := NewMesh(NewZeroPose(), tris2, "m2")
	mesh1.ensureBVH()
	mesh2.ensureBVH()
	m2s := make([]*Mesh, steps)
	for i, p := range poses {
		m2s[i] = mesh2.Transform(p).(*Mesh)
	}

	b.Run("warm_cache", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			collides, d, err := mesh1.collidesWithMesh(m2s[i%steps], 0)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkBoolSink = collides
			benchmarkFloatSink = d
		}
	})

	// Cold-cache variant: clear the witness cache before each call so every
	// query falls through to the full BVH traversal.
	b.Run("cold_cache", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			mesh1.state.witnesses.Delete(m2s[i%steps].state)
			collides, d, err := mesh1.collidesWithMesh(m2s[i%steps], 0)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkBoolSink = collides
			benchmarkFloatSink = d
		}
	})
}
