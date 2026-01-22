package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestBuildBVH(t *testing.T) {
	t.Run("empty triangles returns nil", func(t *testing.T) {
		bvh := buildBVH([]*Triangle{})
		test.That(t, bvh, test.ShouldBeNil)
	})

	t.Run("single triangle creates leaf node", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		bvh := buildBVH([]*Triangle{tri})

		test.That(t, bvh, test.ShouldNotBeNil)
		test.That(t, bvh.triangles, test.ShouldNotBeNil)
		test.That(t, len(bvh.triangles), test.ShouldEqual, 1)
		test.That(t, bvh.left, test.ShouldBeNil)
		test.That(t, bvh.right, test.ShouldBeNil)
	})

	t.Run("few triangles creates leaf node", func(t *testing.T) {
		triangles := []*Triangle{
			NewTriangle(r3.Vector{X: 0, Y: 0, Z: 0}, r3.Vector{X: 1, Y: 0, Z: 0}, r3.Vector{X: 0, Y: 1, Z: 0}),
			NewTriangle(r3.Vector{X: 1, Y: 0, Z: 0}, r3.Vector{X: 2, Y: 0, Z: 0}, r3.Vector{X: 1, Y: 1, Z: 0}),
			NewTriangle(r3.Vector{X: 2, Y: 0, Z: 0}, r3.Vector{X: 3, Y: 0, Z: 0}, r3.Vector{X: 2, Y: 1, Z: 0}),
		}
		bvh := buildBVH(triangles)

		test.That(t, bvh, test.ShouldNotBeNil)
		test.That(t, bvh.triangles, test.ShouldNotBeNil)
		test.That(t, len(bvh.triangles), test.ShouldEqual, 3)
		test.That(t, bvh.left, test.ShouldBeNil)
		test.That(t, bvh.right, test.ShouldBeNil)
	})

	t.Run("many triangles creates internal nodes", func(t *testing.T) {
		triangles := make([]*Triangle, 10)
		for i := 0; i < 10; i++ {
			x := float64(i)
			triangles[i] = NewTriangle(
				r3.Vector{X: x, Y: 0, Z: 0},
				r3.Vector{X: x + 1, Y: 0, Z: 0},
				r3.Vector{X: x, Y: 1, Z: 0},
			)
		}
		bvh := buildBVH(triangles)

		test.That(t, bvh, test.ShouldNotBeNil)
		test.That(t, bvh.triangles, test.ShouldBeNil)
		test.That(t, bvh.left, test.ShouldNotBeNil)
		test.That(t, bvh.right, test.ShouldNotBeNil)
	})
}

func TestComputeTrianglesAABB(t *testing.T) {
	t.Run("single triangle", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		min, max := computeTrianglesAABB([]*Triangle{tri})

		test.That(t, min, test.ShouldResemble, r3.Vector{X: 0, Y: 0, Z: 0})
		test.That(t, max, test.ShouldResemble, r3.Vector{X: 1, Y: 1, Z: 0})
	})

	t.Run("multiple triangles", func(t *testing.T) {
		triangles := []*Triangle{
			NewTriangle(
				r3.Vector{X: 0, Y: 0, Z: 0},
				r3.Vector{X: 1, Y: 0, Z: 0},
				r3.Vector{X: 0, Y: 1, Z: 0},
			),
			NewTriangle(
				r3.Vector{X: 5, Y: 5, Z: 5},
				r3.Vector{X: 6, Y: 5, Z: 5},
				r3.Vector{X: 5, Y: 6, Z: 5},
			),
			NewTriangle(
				r3.Vector{X: -2, Y: -3, Z: -1},
				r3.Vector{X: -1, Y: -3, Z: -1},
				r3.Vector{X: -2, Y: -2, Z: -1},
			),
		}
		min, max := computeTrianglesAABB(triangles)

		test.That(t, min, test.ShouldResemble, r3.Vector{X: -2, Y: -3, Z: -1})
		test.That(t, max, test.ShouldResemble, r3.Vector{X: 6, Y: 6, Z: 5})
	})
}

func TestTriangleCentroid(t *testing.T) {
	t.Run("origin-based triangle", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 3, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 3, Z: 0},
		)
		centroid := tri.Centroid()

		test.That(t, centroid, test.ShouldResemble, r3.Vector{X: 1, Y: 1, Z: 0})
	})

	t.Run("offset triangle", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 1, Y: 1, Z: 1},
			r3.Vector{X: 4, Y: 1, Z: 1},
			r3.Vector{X: 1, Y: 4, Z: 1},
		)
		centroid := tri.Centroid()

		test.That(t, centroid, test.ShouldResemble, r3.Vector{X: 2, Y: 2, Z: 1})
	})
}

func TestAABBOverlap(t *testing.T) {
	t.Run("identical boxes overlap", func(t *testing.T) {
		min1 := r3.Vector{X: 0, Y: 0, Z: 0}
		max1 := r3.Vector{X: 1, Y: 1, Z: 1}
		overlap := aabbOverlap(min1, max1, min1, max1)
		test.That(t, overlap, test.ShouldBeTrue)
	})

	t.Run("adjacent boxes overlap (touching faces)", func(t *testing.T) {
		min1 := r3.Vector{X: 0, Y: 0, Z: 0}
		max1 := r3.Vector{X: 1, Y: 1, Z: 1}
		min2 := r3.Vector{X: 1, Y: 0, Z: 0}
		max2 := r3.Vector{X: 2, Y: 1, Z: 1}
		overlap := aabbOverlap(min1, max1, min2, max2)
		test.That(t, overlap, test.ShouldBeTrue)
	})

	t.Run("overlapping boxes", func(t *testing.T) {
		min1 := r3.Vector{X: 0, Y: 0, Z: 0}
		max1 := r3.Vector{X: 2, Y: 2, Z: 2}
		min2 := r3.Vector{X: 1, Y: 1, Z: 1}
		max2 := r3.Vector{X: 3, Y: 3, Z: 3}
		overlap := aabbOverlap(min1, max1, min2, max2)
		test.That(t, overlap, test.ShouldBeTrue)
	})

	t.Run("separated boxes X axis", func(t *testing.T) {
		min1 := r3.Vector{X: 0, Y: 0, Z: 0}
		max1 := r3.Vector{X: 1, Y: 1, Z: 1}
		min2 := r3.Vector{X: 2, Y: 0, Z: 0}
		max2 := r3.Vector{X: 3, Y: 1, Z: 1}
		overlap := aabbOverlap(min1, max1, min2, max2)
		test.That(t, overlap, test.ShouldBeFalse)
	})

	t.Run("separated boxes Y axis", func(t *testing.T) {
		min1 := r3.Vector{X: 0, Y: 0, Z: 0}
		max1 := r3.Vector{X: 1, Y: 1, Z: 1}
		min2 := r3.Vector{X: 0, Y: 2, Z: 0}
		max2 := r3.Vector{X: 1, Y: 3, Z: 1}
		overlap := aabbOverlap(min1, max1, min2, max2)
		test.That(t, overlap, test.ShouldBeFalse)
	})

	t.Run("separated boxes Z axis", func(t *testing.T) {
		min1 := r3.Vector{X: 0, Y: 0, Z: 0}
		max1 := r3.Vector{X: 1, Y: 1, Z: 1}
		min2 := r3.Vector{X: 0, Y: 0, Z: 2}
		max2 := r3.Vector{X: 1, Y: 1, Z: 3}
		overlap := aabbOverlap(min1, max1, min2, max2)
		test.That(t, overlap, test.ShouldBeFalse)
	})

	t.Run("one box contains other", func(t *testing.T) {
		min1 := r3.Vector{X: 0, Y: 0, Z: 0}
		max1 := r3.Vector{X: 10, Y: 10, Z: 10}
		min2 := r3.Vector{X: 2, Y: 2, Z: 2}
		max2 := r3.Vector{X: 3, Y: 3, Z: 3}
		overlap := aabbOverlap(min1, max1, min2, max2)
		test.That(t, overlap, test.ShouldBeTrue)
	})
}

func TestAABBDistance(t *testing.T) {
	t.Run("overlapping boxes have zero distance", func(t *testing.T) {
		min1 := r3.Vector{X: 0, Y: 0, Z: 0}
		max1 := r3.Vector{X: 2, Y: 2, Z: 2}
		min2 := r3.Vector{X: 1, Y: 1, Z: 1}
		max2 := r3.Vector{X: 3, Y: 3, Z: 3}
		dist := aabbDistance(min1, max1, min2, max2)
		test.That(t, dist, test.ShouldEqual, 0)
	})

	t.Run("separated along X axis", func(t *testing.T) {
		min1 := r3.Vector{X: 0, Y: 0, Z: 0}
		max1 := r3.Vector{X: 1, Y: 1, Z: 1}
		min2 := r3.Vector{X: 3, Y: 0, Z: 0}
		max2 := r3.Vector{X: 4, Y: 1, Z: 1}
		dist := aabbDistance(min1, max1, min2, max2)
		test.That(t, dist, test.ShouldEqual, 2)
	})

	t.Run("separated along Y axis", func(t *testing.T) {
		min1 := r3.Vector{X: 0, Y: 0, Z: 0}
		max1 := r3.Vector{X: 1, Y: 1, Z: 1}
		min2 := r3.Vector{X: 0, Y: 4, Z: 0}
		max2 := r3.Vector{X: 1, Y: 5, Z: 1}
		dist := aabbDistance(min1, max1, min2, max2)
		test.That(t, dist, test.ShouldEqual, 3)
	})

	t.Run("separated along Z axis", func(t *testing.T) {
		min1 := r3.Vector{X: 0, Y: 0, Z: 0}
		max1 := r3.Vector{X: 1, Y: 1, Z: 1}
		min2 := r3.Vector{X: 0, Y: 0, Z: 6}
		max2 := r3.Vector{X: 1, Y: 1, Z: 7}
		dist := aabbDistance(min1, max1, min2, max2)
		test.That(t, dist, test.ShouldEqual, 5)
	})

	t.Run("separated diagonally", func(t *testing.T) {
		min1 := r3.Vector{X: 0, Y: 0, Z: 0}
		max1 := r3.Vector{X: 1, Y: 1, Z: 1}
		min2 := r3.Vector{X: 4, Y: 5, Z: 1}
		max2 := r3.Vector{X: 5, Y: 6, Z: 2}
		// Distance should be sqrt(3^2 + 4^2 + 0^2) = 5
		dist := aabbDistance(min1, max1, min2, max2)
		test.That(t, dist, test.ShouldEqual, 5)
	})
}

func TestTransformAABB(t *testing.T) {
	t.Run("identity transform", func(t *testing.T) {
		min := r3.Vector{X: 0, Y: 0, Z: 0}
		max := r3.Vector{X: 1, Y: 1, Z: 1}
		newMin, newMax := transformAABB(min, max, NewZeroPose())

		test.That(t, R3VectorAlmostEqual(newMin, min, 1e-9), test.ShouldBeTrue)
		test.That(t, R3VectorAlmostEqual(newMax, max, 1e-9), test.ShouldBeTrue)
	})

	t.Run("translation only", func(t *testing.T) {
		min := r3.Vector{X: 0, Y: 0, Z: 0}
		max := r3.Vector{X: 1, Y: 1, Z: 1}
		pose := NewPose(r3.Vector{X: 5, Y: 3, Z: 2}, NewZeroOrientation())
		newMin, newMax := transformAABB(min, max, pose)

		test.That(t, R3VectorAlmostEqual(newMin, r3.Vector{X: 5, Y: 3, Z: 2}, 1e-9), test.ShouldBeTrue)
		test.That(t, R3VectorAlmostEqual(newMax, r3.Vector{X: 6, Y: 4, Z: 3}, 1e-9), test.ShouldBeTrue)
	})

	t.Run("90 degree rotation around Z", func(t *testing.T) {
		min := r3.Vector{X: 0, Y: 0, Z: 0}
		max := r3.Vector{X: 2, Y: 1, Z: 1}
		pose := NewPose(r3.Vector{}, &OrientationVector{OZ: 1, Theta: math.Pi / 2})
		newMin, newMax := transformAABB(min, max, pose)

		// A 2x1x1 box rotated 90 degrees around Z becomes 1x2x1
		test.That(t, R3VectorAlmostEqual(newMin, r3.Vector{X: -1, Y: 0, Z: 0}, 1e-9), test.ShouldBeTrue)
		test.That(t, R3VectorAlmostEqual(newMax, r3.Vector{X: 0, Y: 2, Z: 1}, 1e-9), test.ShouldBeTrue)
	})
}

func TestBVHCollidesWithBVH(t *testing.T) {
	t.Run("nil nodes do not collide", func(t *testing.T) {
		collides, dist := bvhCollidesWithBVH(nil, NewZeroPose(), nil, NewZeroPose(), 0)
		test.That(t, collides, test.ShouldBeFalse)
		test.That(t, math.IsInf(dist, 1), test.ShouldBeTrue)
	})

	t.Run("one nil node does not collide", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		bvh := buildBVH([]*Triangle{tri})

		collides, dist := bvhCollidesWithBVH(bvh, NewZeroPose(), nil, NewZeroPose(), 0)
		test.That(t, collides, test.ShouldBeFalse)
		test.That(t, math.IsInf(dist, 1), test.ShouldBeTrue)

		collides, dist = bvhCollidesWithBVH(nil, NewZeroPose(), bvh, NewZeroPose(), 0)
		test.That(t, collides, test.ShouldBeFalse)
		test.That(t, math.IsInf(dist, 1), test.ShouldBeTrue)
	})

	t.Run("identical triangles collide", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		bvh1 := buildBVH([]*Triangle{tri})
		bvh2 := buildBVH([]*Triangle{tri})

		collides, _ := bvhCollidesWithBVH(bvh1, NewZeroPose(), bvh2, NewZeroPose(), 0)
		test.That(t, collides, test.ShouldBeTrue)
	})

	t.Run("separated triangles do not collide", func(t *testing.T) {
		tri1 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		tri2 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 10},
			r3.Vector{X: 1, Y: 0, Z: 10},
			r3.Vector{X: 0, Y: 1, Z: 10},
		)
		bvh1 := buildBVH([]*Triangle{tri1})
		bvh2 := buildBVH([]*Triangle{tri2})

		collides, dist := bvhCollidesWithBVH(bvh1, NewZeroPose(), bvh2, NewZeroPose(), 0)
		test.That(t, collides, test.ShouldBeFalse)
		test.That(t, dist, test.ShouldBeGreaterThan, 0)
	})

	t.Run("triangles collide with collision buffer", func(t *testing.T) {
		tri1 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		tri2 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0.5},
			r3.Vector{X: 1, Y: 0, Z: 0.5},
			r3.Vector{X: 0, Y: 1, Z: 0.5},
		)
		bvh1 := buildBVH([]*Triangle{tri1})
		bvh2 := buildBVH([]*Triangle{tri2})

		// Without buffer, no collision
		collides, _ := bvhCollidesWithBVH(bvh1, NewZeroPose(), bvh2, NewZeroPose(), 0)
		test.That(t, collides, test.ShouldBeFalse)

		// With buffer >= 0.5, collision
		collides, _ = bvhCollidesWithBVH(bvh1, NewZeroPose(), bvh2, NewZeroPose(), 0.5)
		test.That(t, collides, test.ShouldBeTrue)
	})

	t.Run("collision with pose transformation", func(t *testing.T) {
		tri1 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		tri2 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		bvh1 := buildBVH([]*Triangle{tri1})
		bvh2 := buildBVH([]*Triangle{tri2})

		// Move second triangle far away
		pose2 := NewPose(r3.Vector{X: 100, Y: 100, Z: 100}, NewZeroOrientation())
		collides, _ := bvhCollidesWithBVH(bvh1, NewZeroPose(), bvh2, pose2, 0)
		test.That(t, collides, test.ShouldBeFalse)

		// Move second triangle to overlap
		pose2 = NewPose(r3.Vector{X: 0.1, Y: 0.1, Z: 0}, NewZeroOrientation())
		collides, _ = bvhCollidesWithBVH(bvh1, NewZeroPose(), bvh2, pose2, 0)
		test.That(t, collides, test.ShouldBeTrue)
	})

	t.Run("large BVH collision", func(t *testing.T) {
		// Create two meshes with many triangles
		triangles1 := make([]*Triangle, 20)
		triangles2 := make([]*Triangle, 20)
		for i := 0; i < 20; i++ {
			x := float64(i)
			triangles1[i] = NewTriangle(
				r3.Vector{X: x, Y: 0, Z: 0},
				r3.Vector{X: x + 1, Y: 0, Z: 0},
				r3.Vector{X: x, Y: 1, Z: 0},
			)
			triangles2[i] = NewTriangle(
				r3.Vector{X: x, Y: 0, Z: 10},
				r3.Vector{X: x + 1, Y: 0, Z: 10},
				r3.Vector{X: x, Y: 1, Z: 10},
			)
		}
		bvh1 := buildBVH(triangles1)
		bvh2 := buildBVH(triangles2)

		// Should not collide (separated in Z)
		collides, dist := bvhCollidesWithBVH(bvh1, NewZeroPose(), bvh2, NewZeroPose(), 0)
		test.That(t, collides, test.ShouldBeFalse)
		test.That(t, dist, test.ShouldBeGreaterThan, 0)

		// Move them together
		pose2 := NewPose(r3.Vector{X: 0, Y: 0, Z: -10}, NewZeroOrientation())
		collides, _ = bvhCollidesWithBVH(bvh1, NewZeroPose(), bvh2, pose2, 0)
		test.That(t, collides, test.ShouldBeTrue)
	})
}

func TestLeafCollidesWithLeaf(t *testing.T) {
	t.Run("overlapping triangles collide", func(t *testing.T) {
		tri1 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		tri2 := NewTriangle(
			r3.Vector{X: 0.5, Y: 0.5, Z: -0.5},
			r3.Vector{X: 0.5, Y: 0.5, Z: 0.5},
			r3.Vector{X: -0.5, Y: 0.5, Z: 0},
		)

		collides, _ := leafCollidesWithLeaf([]*Triangle{tri1}, NewZeroPose(), []*Triangle{tri2}, NewZeroPose(), 0)
		test.That(t, collides, test.ShouldBeTrue)
	})

	t.Run("separated triangles do not collide", func(t *testing.T) {
		tri1 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		tri2 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 5},
			r3.Vector{X: 1, Y: 0, Z: 5},
			r3.Vector{X: 0, Y: 1, Z: 5},
		)

		collides, dist := leafCollidesWithLeaf([]*Triangle{tri1}, NewZeroPose(), []*Triangle{tri2}, NewZeroPose(), 0)
		test.That(t, collides, test.ShouldBeFalse)
		test.That(t, dist, test.ShouldAlmostEqual, 5, 1e-9)
	})

	t.Run("collision with buffer", func(t *testing.T) {
		tri1 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		tri2 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 1},
			r3.Vector{X: 1, Y: 0, Z: 1},
			r3.Vector{X: 0, Y: 1, Z: 1},
		)

		// No collision without buffer
		collides, _ := leafCollidesWithLeaf([]*Triangle{tri1}, NewZeroPose(), []*Triangle{tri2}, NewZeroPose(), 0)
		test.That(t, collides, test.ShouldBeFalse)

		// Collision with buffer
		collides, _ = leafCollidesWithLeaf([]*Triangle{tri1}, NewZeroPose(), []*Triangle{tri2}, NewZeroPose(), 1)
		test.That(t, collides, test.ShouldBeTrue)
	})
}

func TestBVHDistanceFromBVH(t *testing.T) {
	t.Run("nil nodes return infinity", func(t *testing.T) {
		dist := bvhDistanceFromBVH(nil, NewZeroPose(), nil, NewZeroPose())
		test.That(t, math.IsInf(dist, 1), test.ShouldBeTrue)
	})

	t.Run("one nil node returns infinity", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		bvh := buildBVH([]*Triangle{tri})

		dist := bvhDistanceFromBVH(bvh, NewZeroPose(), nil, NewZeroPose())
		test.That(t, math.IsInf(dist, 1), test.ShouldBeTrue)

		dist = bvhDistanceFromBVH(nil, NewZeroPose(), bvh, NewZeroPose())
		test.That(t, math.IsInf(dist, 1), test.ShouldBeTrue)
	})

	t.Run("overlapping triangles have zero distance", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		bvh1 := buildBVH([]*Triangle{tri})
		bvh2 := buildBVH([]*Triangle{tri})

		dist := bvhDistanceFromBVH(bvh1, NewZeroPose(), bvh2, NewZeroPose())
		test.That(t, dist, test.ShouldEqual, 0)
	})

	t.Run("parallel triangles separated in Z", func(t *testing.T) {
		tri1 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		tri2 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 5},
			r3.Vector{X: 1, Y: 0, Z: 5},
			r3.Vector{X: 0, Y: 1, Z: 5},
		)
		bvh1 := buildBVH([]*Triangle{tri1})
		bvh2 := buildBVH([]*Triangle{tri2})

		dist := bvhDistanceFromBVH(bvh1, NewZeroPose(), bvh2, NewZeroPose())
		test.That(t, dist, test.ShouldAlmostEqual, 5, 1e-9)
	})

	t.Run("distance with pose transformation", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		bvh1 := buildBVH([]*Triangle{tri})
		bvh2 := buildBVH([]*Triangle{tri})

		// Move second BVH away
		pose2 := NewPose(r3.Vector{X: 0, Y: 0, Z: 10}, NewZeroOrientation())
		dist := bvhDistanceFromBVH(bvh1, NewZeroPose(), bvh2, pose2)
		test.That(t, dist, test.ShouldAlmostEqual, 10, 1e-9)
	})

	t.Run("large BVH distance", func(t *testing.T) {
		triangles1 := make([]*Triangle, 20)
		triangles2 := make([]*Triangle, 20)
		for i := 0; i < 20; i++ {
			x := float64(i)
			triangles1[i] = NewTriangle(
				r3.Vector{X: x, Y: 0, Z: 0},
				r3.Vector{X: x + 1, Y: 0, Z: 0},
				r3.Vector{X: x, Y: 1, Z: 0},
			)
			triangles2[i] = NewTriangle(
				r3.Vector{X: x, Y: 0, Z: 7},
				r3.Vector{X: x + 1, Y: 0, Z: 7},
				r3.Vector{X: x, Y: 1, Z: 7},
			)
		}
		bvh1 := buildBVH(triangles1)
		bvh2 := buildBVH(triangles2)

		dist := bvhDistanceFromBVH(bvh1, NewZeroPose(), bvh2, NewZeroPose())
		test.That(t, dist, test.ShouldAlmostEqual, 7, 1e-9)
	})
}

func TestLeafDistanceFromLeaf(t *testing.T) {
	t.Run("overlapping triangles", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)

		dist := leafDistanceFromLeaf([]*Triangle{tri}, NewZeroPose(), []*Triangle{tri}, NewZeroPose())
		test.That(t, dist, test.ShouldEqual, 0)
	})

	t.Run("parallel triangles", func(t *testing.T) {
		tri1 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		tri2 := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 3},
			r3.Vector{X: 1, Y: 0, Z: 3},
			r3.Vector{X: 0, Y: 1, Z: 3},
		)

		dist := leafDistanceFromLeaf([]*Triangle{tri1}, NewZeroPose(), []*Triangle{tri2}, NewZeroPose())
		test.That(t, dist, test.ShouldAlmostEqual, 3, 1e-9)
	})

	t.Run("multiple triangles returns minimum distance", func(t *testing.T) {
		tris1 := []*Triangle{
			NewTriangle(r3.Vector{X: 0, Y: 0, Z: 0}, r3.Vector{X: 1, Y: 0, Z: 0}, r3.Vector{X: 0, Y: 1, Z: 0}),
			NewTriangle(r3.Vector{X: 5, Y: 0, Z: 0}, r3.Vector{X: 6, Y: 0, Z: 0}, r3.Vector{X: 5, Y: 1, Z: 0}),
		}
		tris2 := []*Triangle{
			NewTriangle(r3.Vector{X: 0, Y: 0, Z: 10}, r3.Vector{X: 1, Y: 0, Z: 10}, r3.Vector{X: 0, Y: 1, Z: 10}),
			NewTriangle(r3.Vector{X: 5, Y: 0, Z: 2}, r3.Vector{X: 6, Y: 0, Z: 2}, r3.Vector{X: 5, Y: 1, Z: 2}),
		}

		dist := leafDistanceFromLeaf(tris1, NewZeroPose(), tris2, NewZeroPose())
		// Minimum distance should be between tris1[1] and tris2[1] = 2
		test.That(t, dist, test.ShouldAlmostEqual, 2, 1e-9)
	})
}

func TestBVHAxisSplitting(t *testing.T) {
	t.Run("splits along X when X extent is largest", func(t *testing.T) {
		triangles := make([]*Triangle, 10)
		for i := 0; i < 10; i++ {
			x := float64(i * 10) // Large X spread
			triangles[i] = NewTriangle(
				r3.Vector{X: x, Y: 0, Z: 0},
				r3.Vector{X: x + 1, Y: 0, Z: 0},
				r3.Vector{X: x, Y: 1, Z: 0},
			)
		}
		bvh := buildBVH(triangles)

		// Verify structure is split (internal node)
		test.That(t, bvh.left, test.ShouldNotBeNil)
		test.That(t, bvh.right, test.ShouldNotBeNil)
	})

	t.Run("splits along Y when Y extent is largest", func(t *testing.T) {
		triangles := make([]*Triangle, 10)
		for i := 0; i < 10; i++ {
			y := float64(i * 10) // Large Y spread
			triangles[i] = NewTriangle(
				r3.Vector{X: 0, Y: y, Z: 0},
				r3.Vector{X: 1, Y: y, Z: 0},
				r3.Vector{X: 0, Y: y + 1, Z: 0},
			)
		}
		bvh := buildBVH(triangles)

		test.That(t, bvh.left, test.ShouldNotBeNil)
		test.That(t, bvh.right, test.ShouldNotBeNil)
	})

	t.Run("splits along Z when Z extent is largest", func(t *testing.T) {
		triangles := make([]*Triangle, 10)
		for i := 0; i < 10; i++ {
			z := float64(i * 10) // Large Z spread
			triangles[i] = NewTriangle(
				r3.Vector{X: 0, Y: 0, Z: z},
				r3.Vector{X: 1, Y: 0, Z: z},
				r3.Vector{X: 0, Y: 1, Z: z},
			)
		}
		bvh := buildBVH(triangles)

		test.That(t, bvh.left, test.ShouldNotBeNil)
		test.That(t, bvh.right, test.ShouldNotBeNil)
	})
}

func TestBVHNodeBounds(t *testing.T) {
	t.Run("leaf node bounds encompass all triangles", func(t *testing.T) {
		triangles := []*Triangle{
			NewTriangle(r3.Vector{X: -1, Y: -1, Z: -1}, r3.Vector{X: 0, Y: -1, Z: -1}, r3.Vector{X: -1, Y: 0, Z: -1}),
			NewTriangle(r3.Vector{X: 5, Y: 5, Z: 5}, r3.Vector{X: 6, Y: 5, Z: 5}, r3.Vector{X: 5, Y: 6, Z: 5}),
		}
		bvh := buildBVH(triangles)

		test.That(t, bvh.min.X, test.ShouldBeLessThanOrEqualTo, -1)
		test.That(t, bvh.min.Y, test.ShouldBeLessThanOrEqualTo, -1)
		test.That(t, bvh.min.Z, test.ShouldBeLessThanOrEqualTo, -1)
		test.That(t, bvh.max.X, test.ShouldBeGreaterThanOrEqualTo, 6)
		test.That(t, bvh.max.Y, test.ShouldBeGreaterThanOrEqualTo, 6)
		test.That(t, bvh.max.Z, test.ShouldBeGreaterThanOrEqualTo, 5)
	})

	t.Run("internal node bounds encompass children", func(t *testing.T) {
		triangles := make([]*Triangle, 20)
		for i := 0; i < 20; i++ {
			x := float64(i - 10) // Spread from -10 to 9
			triangles[i] = NewTriangle(
				r3.Vector{X: x, Y: 0, Z: 0},
				r3.Vector{X: x + 1, Y: 0, Z: 0},
				r3.Vector{X: x, Y: 1, Z: 0},
			)
		}
		bvh := buildBVH(triangles)

		// Root should encompass all triangles
		test.That(t, bvh.min.X, test.ShouldBeLessThanOrEqualTo, -10)
		test.That(t, bvh.max.X, test.ShouldBeGreaterThanOrEqualTo, 10)

		// Children should be subsets
		if bvh.left != nil && bvh.right != nil {
			test.That(t, bvh.left.min.X, test.ShouldBeGreaterThanOrEqualTo, bvh.min.X)
			test.That(t, bvh.left.max.X, test.ShouldBeLessThanOrEqualTo, bvh.max.X)
			test.That(t, bvh.right.min.X, test.ShouldBeGreaterThanOrEqualTo, bvh.min.X)
			test.That(t, bvh.right.max.X, test.ShouldBeLessThanOrEqualTo, bvh.max.X)
		}
	})
}
