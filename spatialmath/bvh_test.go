package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

// trianglesToGeometries is a test helper to convert triangles to geometries (variadic).
func trianglesToGeometries(tris ...*Triangle) []Geometry {
	geoms := make([]Geometry, len(tris))
	for i, t := range tris {
		geoms[i] = t
	}
	return geoms
}

// triangleSliceToGeometries is a test helper to convert a slice of triangles to geometries.
func triangleSliceToGeometries(tris []*Triangle) []Geometry {
	geoms := make([]Geometry, len(tris))
	for i, t := range tris {
		geoms[i] = t
	}
	return geoms
}

func TestBuildBVH(t *testing.T) {
	t.Run("empty triangles returns nil", func(t *testing.T) {
		bvh := buildBVH([]Geometry{})
		test.That(t, bvh, test.ShouldBeNil)
	})

	t.Run("single triangle creates leaf node", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		bvh := buildBVH(trianglesToGeometries(tri))

		test.That(t, bvh, test.ShouldNotBeNil)
		test.That(t, bvh.geoms, test.ShouldNotBeNil)
		test.That(t, len(bvh.geoms), test.ShouldEqual, 1)
		test.That(t, bvh.left, test.ShouldBeNil)
		test.That(t, bvh.right, test.ShouldBeNil)
	})

	t.Run("few triangles creates leaf node", func(t *testing.T) {
		triangles := []*Triangle{
			NewTriangle(r3.Vector{X: 0, Y: 0, Z: 0}, r3.Vector{X: 1, Y: 0, Z: 0}, r3.Vector{X: 0, Y: 1, Z: 0}),
			NewTriangle(r3.Vector{X: 1, Y: 0, Z: 0}, r3.Vector{X: 2, Y: 0, Z: 0}, r3.Vector{X: 1, Y: 1, Z: 0}),
			NewTriangle(r3.Vector{X: 2, Y: 0, Z: 0}, r3.Vector{X: 3, Y: 0, Z: 0}, r3.Vector{X: 2, Y: 1, Z: 0}),
		}
		bvh := buildBVH(triangleSliceToGeometries(triangles))

		test.That(t, bvh, test.ShouldNotBeNil)
		test.That(t, bvh.geoms, test.ShouldNotBeNil)
		test.That(t, len(bvh.geoms), test.ShouldEqual, 3)
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
		bvh := buildBVH(triangleSliceToGeometries(triangles))

		test.That(t, bvh, test.ShouldNotBeNil)
		test.That(t, bvh.geoms, test.ShouldBeNil)
		test.That(t, bvh.left, test.ShouldNotBeNil)
		test.That(t, bvh.right, test.ShouldNotBeNil)
	})
}

func TestComputeTriangleAABB(t *testing.T) {
	t.Run("single triangle", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		minPt, maxPt := computeTriangleAABB(tri)

		test.That(t, minPt, test.ShouldResemble, r3.Vector{X: 0, Y: 0, Z: 0})
		test.That(t, maxPt, test.ShouldResemble, r3.Vector{X: 1, Y: 1, Z: 0})
	})

	t.Run("multiple triangles via computeGeomsAABB", func(t *testing.T) {
		geoms := []Geometry{
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
		minPt, maxPt := computeGeomsAABB(geoms)

		test.That(t, minPt, test.ShouldResemble, r3.Vector{X: -2, Y: -3, Z: -1})
		test.That(t, maxPt, test.ShouldResemble, r3.Vector{X: 6, Y: 6, Z: 5})
	})
}

func TestComputeGeometryAABB(t *testing.T) {
	t.Run("triangle AABB", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 1, Y: 2, Z: 3},
			r3.Vector{X: 4, Y: 5, Z: 6},
			r3.Vector{X: 7, Y: 1, Z: 2},
		)
		minPt, maxPt := computeGeometryAABB(tri)

		test.That(t, minPt.X, test.ShouldEqual, 1)
		test.That(t, minPt.Y, test.ShouldEqual, 1)
		test.That(t, minPt.Z, test.ShouldEqual, 2)
		test.That(t, maxPt.X, test.ShouldEqual, 7)
		test.That(t, maxPt.Y, test.ShouldEqual, 5)
		test.That(t, maxPt.Z, test.ShouldEqual, 6)
	})

	t.Run("sphere AABB", func(t *testing.T) {
		s, err := NewSphere(NewPose(r3.Vector{X: 10, Y: 20, Z: 30}, NewZeroOrientation()), 5, "")
		test.That(t, err, test.ShouldBeNil)
		minPt, maxPt := computeGeometryAABB(s)

		test.That(t, minPt.X, test.ShouldEqual, 5)
		test.That(t, minPt.Y, test.ShouldEqual, 15)
		test.That(t, minPt.Z, test.ShouldEqual, 25)
		test.That(t, maxPt.X, test.ShouldEqual, 15)
		test.That(t, maxPt.Y, test.ShouldEqual, 25)
		test.That(t, maxPt.Z, test.ShouldEqual, 35)
	})

	t.Run("axis-aligned box AABB", func(t *testing.T) {
		b, err := NewBox(NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, NewZeroOrientation()), r3.Vector{X: 4, Y: 6, Z: 8}, "")
		test.That(t, err, test.ShouldBeNil)
		minPt, maxPt := computeGeometryAABB(b)

		test.That(t, minPt.X, test.ShouldAlmostEqual, -2, 1e-9)
		test.That(t, minPt.Y, test.ShouldAlmostEqual, -3, 1e-9)
		test.That(t, minPt.Z, test.ShouldAlmostEqual, -4, 1e-9)
		test.That(t, maxPt.X, test.ShouldAlmostEqual, 2, 1e-9)
		test.That(t, maxPt.Y, test.ShouldAlmostEqual, 3, 1e-9)
		test.That(t, maxPt.Z, test.ShouldAlmostEqual, 4, 1e-9)
	})

	t.Run("rotated box AABB", func(t *testing.T) {
		// A 2x2x2 box rotated 45 degrees around Z axis
		b, err := NewBox(
			NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, &OrientationVector{OZ: 1, Theta: math.Pi / 4}),
			r3.Vector{X: 2, Y: 2, Z: 2},
			"",
		)
		test.That(t, err, test.ShouldBeNil)
		minPt, maxPt := computeGeometryAABB(b)

		// Rotated 45 degrees, the AABB should be larger than 2x2
		// sqrt(2) * 1 = ~1.414 for each half
		test.That(t, minPt.X, test.ShouldBeLessThan, -1)
		test.That(t, maxPt.X, test.ShouldBeGreaterThan, 1)
		test.That(t, minPt.Y, test.ShouldBeLessThan, -1)
		test.That(t, maxPt.Y, test.ShouldBeGreaterThan, 1)
		// Z should be unaffected
		test.That(t, minPt.Z, test.ShouldAlmostEqual, -1, 1e-9)
		test.That(t, maxPt.Z, test.ShouldAlmostEqual, 1, 1e-9)
	})

	t.Run("capsule AABB", func(t *testing.T) {
		// Capsule along Z axis, centered at origin
		c, err := NewCapsule(NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, NewZeroOrientation()), 2, 10, "")
		test.That(t, err, test.ShouldBeNil)
		minPt, maxPt := computeGeometryAABB(c)

		// Capsule has length 10 along Z, radius 2
		// segA and segB are at Z = -3 and Z = 3 (length/2 - radius = 5-2 = 3)
		// AABB should be from -2 to 2 in X and Y, and -5 to 5 in Z
		test.That(t, minPt.X, test.ShouldAlmostEqual, -2, 1e-9)
		test.That(t, maxPt.X, test.ShouldAlmostEqual, 2, 1e-9)
		test.That(t, minPt.Y, test.ShouldAlmostEqual, -2, 1e-9)
		test.That(t, maxPt.Y, test.ShouldAlmostEqual, 2, 1e-9)
		test.That(t, minPt.Z, test.ShouldAlmostEqual, -5, 1e-9)
		test.That(t, maxPt.Z, test.ShouldAlmostEqual, 5, 1e-9)
	})

	t.Run("point AABB", func(t *testing.T) {
		p := NewPoint(r3.Vector{X: 5, Y: 10, Z: 15}, "")
		minPt, maxPt := computeGeometryAABB(p)

		test.That(t, minPt, test.ShouldResemble, r3.Vector{X: 5, Y: 10, Z: 15})
		test.That(t, maxPt, test.ShouldResemble, r3.Vector{X: 5, Y: 10, Z: 15})
	})
}

func TestComputeGeomsAABB(t *testing.T) {
	t.Run("multiple geometries", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		s, _ := NewSphere(NewPose(r3.Vector{X: 10, Y: 10, Z: 10}, NewZeroOrientation()), 2, "")
		p := NewPoint(r3.Vector{X: -5, Y: -5, Z: -5}, "")

		geoms := []Geometry{tri, s, p}
		minPt, maxPt := computeGeomsAABB(geoms)

		test.That(t, minPt.X, test.ShouldEqual, -5)
		test.That(t, minPt.Y, test.ShouldEqual, -5)
		test.That(t, minPt.Z, test.ShouldEqual, -5)
		test.That(t, maxPt.X, test.ShouldEqual, 12) // sphere center 10 + radius 2
		test.That(t, maxPt.Y, test.ShouldEqual, 12)
		test.That(t, maxPt.Z, test.ShouldEqual, 12)
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
		minPt := r3.Vector{X: 0, Y: 0, Z: 0}
		maxPt := r3.Vector{X: 1, Y: 1, Z: 1}
		newMin, newMax := transformAABB(minPt, maxPt, NewZeroPose())

		test.That(t, R3VectorAlmostEqual(newMin, minPt, 1e-9), test.ShouldBeTrue)
		test.That(t, R3VectorAlmostEqual(newMax, maxPt, 1e-9), test.ShouldBeTrue)
	})

	t.Run("translation only", func(t *testing.T) {
		minPt := r3.Vector{X: 0, Y: 0, Z: 0}
		maxPt := r3.Vector{X: 1, Y: 1, Z: 1}
		pose := NewPose(r3.Vector{X: 5, Y: 3, Z: 2}, NewZeroOrientation())
		newMin, newMax := transformAABB(minPt, maxPt, pose)

		test.That(t, R3VectorAlmostEqual(newMin, r3.Vector{X: 5, Y: 3, Z: 2}, 1e-9), test.ShouldBeTrue)
		test.That(t, R3VectorAlmostEqual(newMax, r3.Vector{X: 6, Y: 4, Z: 3}, 1e-9), test.ShouldBeTrue)
	})

	t.Run("90 degree rotation around Z", func(t *testing.T) {
		minPt := r3.Vector{X: 0, Y: 0, Z: 0}
		maxPt := r3.Vector{X: 2, Y: 1, Z: 1}
		pose := NewPose(r3.Vector{}, &OrientationVector{OZ: 1, Theta: math.Pi / 2})
		newMin, newMax := transformAABB(minPt, maxPt, pose)

		// A 2x1x1 box rotated 90 degrees around Z becomes 1x2x1
		test.That(t, R3VectorAlmostEqual(newMin, r3.Vector{X: -1, Y: 0, Z: 0}, 1e-9), test.ShouldBeTrue)
		test.That(t, R3VectorAlmostEqual(newMax, r3.Vector{X: 0, Y: 2, Z: 1}, 1e-9), test.ShouldBeTrue)
	})
}

func TestBVHCollidesWithBVH(t *testing.T) {
	zeroPose := NewZeroPose()

	t.Run("nil nodes do not collide", func(t *testing.T) {
		collides, dist, err := bvhCollidesWithBVH(nil, nil, zeroPose, zeroPose, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeFalse)
		test.That(t, math.IsInf(dist, 1), test.ShouldBeTrue)
	})

	t.Run("one nil node does not collide", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		bvh := buildBVH(trianglesToGeometries(tri))

		collides, dist, err := bvhCollidesWithBVH(bvh, nil, zeroPose, zeroPose, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeFalse)
		test.That(t, math.IsInf(dist, 1), test.ShouldBeTrue)

		collides, dist, err = bvhCollidesWithBVH(nil, bvh, zeroPose, zeroPose, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeFalse)
		test.That(t, math.IsInf(dist, 1), test.ShouldBeTrue)
	})

	t.Run("identical triangles collide", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		bvh1 := buildBVH(trianglesToGeometries(tri))
		bvh2 := buildBVH(trianglesToGeometries(tri))

		collides, _, err := bvhCollidesWithBVH(bvh1, bvh2, zeroPose, zeroPose, 0)
		test.That(t, err, test.ShouldBeNil)
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
		bvh1 := buildBVH(trianglesToGeometries(tri1))
		bvh2 := buildBVH(trianglesToGeometries(tri2))

		collides, dist, err := bvhCollidesWithBVH(bvh1, bvh2, zeroPose, zeroPose, 0)
		test.That(t, err, test.ShouldBeNil)
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
		bvh1 := buildBVH(trianglesToGeometries(tri1))
		bvh2 := buildBVH(trianglesToGeometries(tri2))

		// Without buffer, no collision
		collides, _, err := bvhCollidesWithBVH(bvh1, bvh2, zeroPose, zeroPose, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeFalse)

		// With buffer >= 0.5, collision
		collides, _, err = bvhCollidesWithBVH(bvh1, bvh2, zeroPose, zeroPose, 0.5)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeTrue)
	})

	t.Run("collision with pose transformation", func(t *testing.T) {
		// Both BVHs store triangles at origin in local space
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		bvh1 := buildBVH(trianglesToGeometries(tri))
		bvh2 := buildBVH(trianglesToGeometries(tri))

		// Move second BVH far away via pose (lazy transform)
		pose2 := NewPose(r3.Vector{X: 100, Y: 100, Z: 100}, NewZeroOrientation())
		collides, _, err := bvhCollidesWithBVH(bvh1, bvh2, zeroPose, pose2, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeFalse)

		// Move second BVH to overlap via pose
		pose2 = NewPose(r3.Vector{X: 0.1, Y: 0.1, Z: 0}, NewZeroOrientation())
		collides, _, err = bvhCollidesWithBVH(bvh1, bvh2, zeroPose, pose2, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeTrue)
	})

	t.Run("large BVH collision", func(t *testing.T) {
		// Create two meshes with many triangles in local space
		triangles1 := make([]*Triangle, 20)
		triangles2 := make([]*Triangle, 20)
		for i := 0; i < 20; i++ {
			x := float64(i)
			triangles1[i] = NewTriangle(
				r3.Vector{X: x, Y: 0, Z: 0},
				r3.Vector{X: x + 1, Y: 0, Z: 0},
				r3.Vector{X: x, Y: 1, Z: 0},
			)
			// triangles2 at Z=10 in local space
			triangles2[i] = NewTriangle(
				r3.Vector{X: x, Y: 0, Z: 10},
				r3.Vector{X: x + 1, Y: 0, Z: 10},
				r3.Vector{X: x, Y: 1, Z: 10},
			)
		}
		bvh1 := buildBVH(triangleSliceToGeometries(triangles1))
		bvh2 := buildBVH(triangleSliceToGeometries(triangles2))

		// Should not collide (separated in Z)
		collides, dist, err := bvhCollidesWithBVH(bvh1, bvh2, zeroPose, zeroPose, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeFalse)
		test.That(t, dist, test.ShouldBeGreaterThan, 0)

		// Move bvh2 down by 10 via pose to make them overlap
		pose2 := NewPose(r3.Vector{X: 0, Y: 0, Z: -10}, NewZeroOrientation())
		collides, _, err = bvhCollidesWithBVH(bvh1, bvh2, zeroPose, pose2, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeTrue)
	})
}

func TestLeafCollidesWithLeaf(t *testing.T) {
	zeroPose := NewZeroPose()

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

		collides, _, err := leafCollidesWithLeaf(trianglesToGeometries(tri1), trianglesToGeometries(tri2), zeroPose, zeroPose, 0)
		test.That(t, collides, test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)
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

		collides, dist, err := leafCollidesWithLeaf(trianglesToGeometries(tri1), trianglesToGeometries(tri2), zeroPose, zeroPose, 0)
		test.That(t, collides, test.ShouldBeFalse)
		test.That(t, dist, test.ShouldAlmostEqual, 5, 1e-9)
		test.That(t, err, test.ShouldBeNil)
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
		collides, _, err := leafCollidesWithLeaf(trianglesToGeometries(tri1), trianglesToGeometries(tri2), zeroPose, zeroPose, 0)
		test.That(t, collides, test.ShouldBeFalse)
		test.That(t, err, test.ShouldBeNil)

		// Collision with buffer
		collides, _, err = leafCollidesWithLeaf(trianglesToGeometries(tri1), trianglesToGeometries(tri2), zeroPose, zeroPose, 1)
		test.That(t, collides, test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestBVHDistanceFromBVH(t *testing.T) {
	zeroPose := NewZeroPose()

	t.Run("nil nodes return infinity", func(t *testing.T) {
		dist, err := bvhDistanceFromBVH(nil, nil, zeroPose, zeroPose)
		test.That(t, math.IsInf(dist, 1), test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("one nil node returns infinity", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		bvh := buildBVH(trianglesToGeometries(tri))

		dist, err := bvhDistanceFromBVH(bvh, nil, zeroPose, zeroPose)
		test.That(t, math.IsInf(dist, 1), test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)

		dist, err = bvhDistanceFromBVH(nil, bvh, zeroPose, zeroPose)
		test.That(t, math.IsInf(dist, 1), test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("overlapping triangles have zero or negative distance", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		bvh1 := buildBVH(trianglesToGeometries(tri))
		bvh2 := buildBVH(trianglesToGeometries(tri))

		dist, err := bvhDistanceFromBVH(bvh1, bvh2, zeroPose, zeroPose)
		// Overlapping geometries return negative distance (penetration depth) per Geometry interface
		test.That(t, dist, test.ShouldBeLessThanOrEqualTo, 0)
		test.That(t, err, test.ShouldBeNil)
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
		bvh1 := buildBVH(trianglesToGeometries(tri1))
		bvh2 := buildBVH(trianglesToGeometries(tri2))

		dist, err := bvhDistanceFromBVH(bvh1, bvh2, zeroPose, zeroPose)
		test.That(t, dist, test.ShouldAlmostEqual, 5, 1e-9)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("distance with pose transformation", func(t *testing.T) {
		// Both BVHs store triangles at origin in local space
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)
		bvh1 := buildBVH(trianglesToGeometries(tri))
		bvh2 := buildBVH(trianglesToGeometries(tri))

		// Move second BVH away via pose (lazy transform)
		pose2 := NewPose(r3.Vector{X: 0, Y: 0, Z: 10}, NewZeroOrientation())
		dist, err := bvhDistanceFromBVH(bvh1, bvh2, zeroPose, pose2)
		test.That(t, dist, test.ShouldAlmostEqual, 10, 1e-9)
		test.That(t, err, test.ShouldBeNil)
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
		bvh1 := buildBVH(triangleSliceToGeometries(triangles1))
		bvh2 := buildBVH(triangleSliceToGeometries(triangles2))

		dist, err := bvhDistanceFromBVH(bvh1, bvh2, zeroPose, zeroPose)
		test.That(t, dist, test.ShouldAlmostEqual, 7, 1e-9)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestLeafDistanceFromLeaf(t *testing.T) {
	zeroPose := NewZeroPose()

	t.Run("overlapping triangles", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		)

		dist, err := leafDistanceFromLeaf(trianglesToGeometries(tri), trianglesToGeometries(tri), zeroPose, zeroPose)
		// Overlapping/identical triangles return negative distance (penetration depth) per Geometry interface
		test.That(t, dist, test.ShouldBeLessThanOrEqualTo, 0)
		test.That(t, err, test.ShouldBeNil)
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

		dist, err := leafDistanceFromLeaf(trianglesToGeometries(tri1), trianglesToGeometries(tri2), zeroPose, zeroPose)
		test.That(t, dist, test.ShouldAlmostEqual, 3, 1e-9)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("multiple triangles returns minimum distance", func(t *testing.T) {
		tris1 := trianglesToGeometries(
			NewTriangle(r3.Vector{X: 0, Y: 0, Z: 0}, r3.Vector{X: 1, Y: 0, Z: 0}, r3.Vector{X: 0, Y: 1, Z: 0}),
			NewTriangle(r3.Vector{X: 5, Y: 0, Z: 0}, r3.Vector{X: 6, Y: 0, Z: 0}, r3.Vector{X: 5, Y: 1, Z: 0}),
		)
		tris2 := trianglesToGeometries(
			NewTriangle(r3.Vector{X: 0, Y: 0, Z: 10}, r3.Vector{X: 1, Y: 0, Z: 10}, r3.Vector{X: 0, Y: 1, Z: 10}),
			NewTriangle(r3.Vector{X: 5, Y: 0, Z: 2}, r3.Vector{X: 6, Y: 0, Z: 2}, r3.Vector{X: 5, Y: 1, Z: 2}),
		)

		dist, err := leafDistanceFromLeaf(tris1, tris2, zeroPose, zeroPose)
		// Minimum distance should be between tris1[1] and tris2[1] = 2
		test.That(t, dist, test.ShouldAlmostEqual, 2, 1e-9)
		test.That(t, err, test.ShouldBeNil)
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
		bvh := buildBVH(triangleSliceToGeometries(triangles))

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
		bvh := buildBVH(triangleSliceToGeometries(triangles))

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
		bvh := buildBVH(triangleSliceToGeometries(triangles))

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
		bvh := buildBVH(triangleSliceToGeometries(triangles))

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
		bvh := buildBVH(triangleSliceToGeometries(triangles))

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

func TestGeometryCentroid(t *testing.T) {
	t.Run("triangle uses fast path", func(t *testing.T) {
		tri := NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 3, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 6, Z: 0},
		)
		c := geometryCentroid(tri)
		test.That(t, c.X, test.ShouldAlmostEqual, 1, 1e-10)
		test.That(t, c.Y, test.ShouldAlmostEqual, 2, 1e-10)
		test.That(t, c.Z, test.ShouldAlmostEqual, 0, 1e-10)
	})

	t.Run("non-triangle uses Pose().Point()", func(t *testing.T) {
		s, err := NewSphere(NewPose(r3.Vector{X: 5, Y: 10, Z: 15}, NewZeroOrientation()), 3, "")
		test.That(t, err, test.ShouldBeNil)
		c := geometryCentroid(s)
		test.That(t, c.X, test.ShouldEqual, 5)
		test.That(t, c.Y, test.ShouldEqual, 10)
		test.That(t, c.Z, test.ShouldEqual, 15)
	})
}

func TestRotatedAABBExtents(t *testing.T) {
	t.Run("identity rotation preserves extents", func(t *testing.T) {
		rm := &RotationMatrix{mat: [9]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}}
		extents := r3.Vector{X: 2, Y: 3, Z: 4}
		result := rotatedAABBExtents(rm, extents)
		test.That(t, result.X, test.ShouldAlmostEqual, 2, 1e-10)
		test.That(t, result.Y, test.ShouldAlmostEqual, 3, 1e-10)
		test.That(t, result.Z, test.ShouldAlmostEqual, 4, 1e-10)
	})

	t.Run("90 deg rotation around Z swaps X and Y extents", func(t *testing.T) {
		// 90 deg around Z: [0 -1 0; 1 0 0; 0 0 1]
		rm := &RotationMatrix{mat: [9]float64{0, -1, 0, 1, 0, 0, 0, 0, 1}}
		extents := r3.Vector{X: 2, Y: 3, Z: 4}
		result := rotatedAABBExtents(rm, extents)
		// abs(R)*e: X = 0*2 + 1*3 + 0*4 = 3, Y = 1*2 + 0*3 + 0*4 = 2, Z = 4
		test.That(t, result.X, test.ShouldAlmostEqual, 3, 1e-10)
		test.That(t, result.Y, test.ShouldAlmostEqual, 2, 1e-10)
		test.That(t, result.Z, test.ShouldAlmostEqual, 4, 1e-10)
	})

	t.Run("45 deg rotation around Z expands extents", func(t *testing.T) {
		s := math.Sin(math.Pi / 4)
		c := math.Cos(math.Pi / 4)
		rm := &RotationMatrix{mat: [9]float64{c, -s, 0, s, c, 0, 0, 0, 1}}
		extents := r3.Vector{X: 1, Y: 1, Z: 1}
		result := rotatedAABBExtents(rm, extents)
		// abs(R)*e: X = c+s = sqrt(2), Y = s+c = sqrt(2), Z = 1
		test.That(t, result.X, test.ShouldAlmostEqual, math.Sqrt(2), 1e-10)
		test.That(t, result.Y, test.ShouldAlmostEqual, math.Sqrt(2), 1e-10)
		test.That(t, result.Z, test.ShouldAlmostEqual, 1, 1e-10)
	})
}

func TestAABBFromCenterExtents(t *testing.T) {
	center := r3.Vector{X: 5, Y: 10, Z: 15}
	extents := r3.Vector{X: 2, Y: 3, Z: 4}
	minPt, maxPt := aabbFromCenterExtents(center, extents)
	test.That(t, minPt, test.ShouldResemble, r3.Vector{X: 3, Y: 7, Z: 11})
	test.That(t, maxPt, test.ShouldResemble, r3.Vector{X: 7, Y: 13, Z: 19})
}

func TestComputeMeshAABB(t *testing.T) {
	t.Run("identity pose", func(t *testing.T) {
		triangles := []*Triangle{
			NewTriangle(
				r3.Vector{X: 0, Y: 0, Z: 0},
				r3.Vector{X: 1, Y: 0, Z: 0},
				r3.Vector{X: 0, Y: 1, Z: 0},
			),
			NewTriangle(
				r3.Vector{X: 0, Y: 0, Z: 0},
				r3.Vector{X: 0, Y: 0, Z: 3},
				r3.Vector{X: 0, Y: 1, Z: 0},
			),
		}
		mesh := NewMesh(NewZeroPose(), triangles, "test")
		minPt, maxPt := computeMeshAABB(mesh)

		test.That(t, minPt.X, test.ShouldAlmostEqual, 0, 1e-10)
		test.That(t, minPt.Y, test.ShouldAlmostEqual, 0, 1e-10)
		test.That(t, minPt.Z, test.ShouldAlmostEqual, 0, 1e-10)
		test.That(t, maxPt.X, test.ShouldAlmostEqual, 1, 1e-10)
		test.That(t, maxPt.Y, test.ShouldAlmostEqual, 1, 1e-10)
		test.That(t, maxPt.Z, test.ShouldAlmostEqual, 3, 1e-10)
	})

	t.Run("with translation", func(t *testing.T) {
		triangles := []*Triangle{
			NewTriangle(
				r3.Vector{X: 0, Y: 0, Z: 0},
				r3.Vector{X: 1, Y: 0, Z: 0},
				r3.Vector{X: 0, Y: 1, Z: 0},
			),
		}
		pose := NewPose(r3.Vector{X: 10, Y: 20, Z: 30}, NewZeroOrientation())
		mesh := NewMesh(pose, triangles, "test")
		minPt, maxPt := computeMeshAABB(mesh)

		test.That(t, minPt.X, test.ShouldAlmostEqual, 10, 1e-10)
		test.That(t, minPt.Y, test.ShouldAlmostEqual, 20, 1e-10)
		test.That(t, minPt.Z, test.ShouldAlmostEqual, 30, 1e-10)
		test.That(t, maxPt.X, test.ShouldAlmostEqual, 11, 1e-10)
		test.That(t, maxPt.Y, test.ShouldAlmostEqual, 21, 1e-10)
		test.That(t, maxPt.Z, test.ShouldAlmostEqual, 30, 1e-10)
	})

	t.Run("with rotation", func(t *testing.T) {
		// Single triangle in XY plane, rotated 90 deg around Z
		triangles := []*Triangle{
			NewTriangle(
				r3.Vector{X: 0, Y: 0, Z: 0},
				r3.Vector{X: 2, Y: 0, Z: 0},
				r3.Vector{X: 0, Y: 1, Z: 0},
			),
		}
		pose := NewPose(r3.Vector{}, &OrientationVector{OZ: 1, Theta: math.Pi / 2})
		mesh := NewMesh(pose, triangles, "test")
		minPt, maxPt := computeMeshAABB(mesh)

		// After 90 deg around Z: (2,0,0)->(0,2,0), (0,1,0)->(-1,0,0)
		test.That(t, minPt.X, test.ShouldAlmostEqual, -1, 1e-9)
		test.That(t, minPt.Y, test.ShouldAlmostEqual, 0, 1e-9)
		test.That(t, minPt.Z, test.ShouldAlmostEqual, 0, 1e-9)
		test.That(t, maxPt.X, test.ShouldAlmostEqual, 0, 1e-9)
		test.That(t, maxPt.Y, test.ShouldAlmostEqual, 2, 1e-9)
		test.That(t, maxPt.Z, test.ShouldAlmostEqual, 0, 1e-9)
	})
}
