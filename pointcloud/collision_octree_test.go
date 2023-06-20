package pointcloud

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/spatialmath"
)

// Helper function for generating a new empty collision octree.
func createNewCollisionOctree(center r3.Vector, side float64, confidenceThreshold int, buffer float64) (*CollisionOctree, error) {
	collisionOct, err := NewCollisionOctree(center, side, confidenceThreshold, buffer)
	if err != nil {
		return nil, err
	}

	return collisionOct, err
}

// Test the Transform()function which adds transforms an octree by a pose.
func TestCollisionOctreeTransform(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0

	t.Run("Set point into empty leaf node into basic octree and transform", func(t *testing.T) {
		collisionOct, err := createNewCollisionOctree(center, side, 50, 0)
		test.That(t, err, test.ShouldBeNil)

		point1 := r3.Vector{X: 0.1, Y: 0, Z: 0}
		data1 := NewValueData(1)
		err = collisionOct.Set(point1, data1)
		test.That(t, err, test.ShouldBeNil)

		transformedOct := collisionOct.Transform(spatialmath.NewPoseFromPoint(r3.Vector{X: 1.0, Y: 2.0, Z: 0}))
		newCenter := r3.Vector{X: 1.0, Y: 2.0, Z: 0}
		test.That(t, collisionOct.MaxVal(), test.ShouldEqual, transformedOct.(*CollisionOctree).MaxVal())
		validateBasicOctree(t, transformedOct.(*CollisionOctree).BasicOctree, newCenter, side)
	})

	t.Run("Set point into internal node node into basic octree and transform", func(t *testing.T) {
		basicOct, err := createNewCollisionOctree(center, side, 50, 0)
		test.That(t, err, test.ShouldBeNil)

		d3 := 3
		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(d3))
		test.That(t, err, test.ShouldBeNil)

		d2 := 2
		err = basicOct.Set(r3.Vector{X: -.5, Y: 0, Z: 0}, NewValueData(d2))
		test.That(t, err, test.ShouldBeNil)

		d4 := 4
		err = basicOct.Set(r3.Vector{X: -.4, Y: 0, Z: 0}, NewValueData(d4))
		test.That(t, err, test.ShouldBeNil)

		transformedOct := basicOct.Transform(spatialmath.NewPoseFromPoint(r3.Vector{X: 1.0, Y: 2.0, Z: 0}))
		newCenter := r3.Vector{X: 1.0, Y: 2.0, Z: 0}

		validateBasicOctree(t, transformedOct.(*CollisionOctree).BasicOctree, newCenter, side)
	})
}
