package pointcloud

import (
	"math"
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
		validateCollisionOctree(t, transformedOct.(*CollisionOctree), newCenter, side)
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

		transformedOctree := basicOct.Transform(spatialmath.NewPoseFromPoint(r3.Vector{X: 1.0, Y: 2.0, Z: 0}))
		newCenter := r3.Vector{X: 1.0, Y: 2.0, Z: 0}

		validateCollisionOctree(t, transformedOctree.(*CollisionOctree), newCenter, side)
	})
}

// Helper function that recursively checks a basic octree's structure and metadata.
func validateCollisionOctree(t *testing.T, bOct *CollisionOctree, center r3.Vector, sideLength float64) (int, int) {
	t.Helper()

	test.That(t, sideLength, test.ShouldEqual, bOct.sideLength)
	test.That(t, center, test.ShouldResemble, bOct.center)

	validateCollisionMetadata(t, bOct)

	var size int
	maxVal := emptyProb
	switch bOct.node.nodeType {
	case internalNode:
		test.That(t, len(bOct.node.children), test.ShouldEqual, 8)
		test.That(t, bOct.node.point, test.ShouldBeNil)

		numLeafNodeFilledNodes := 0
		numLeafNodeEmptyNodes := 0
		numInternalNodes := 0
		for c, child := range bOct.node.children {
			var i, j, k float64
			if c%8 < 4 {
				i = -1
			} else {
				i = 1
			}
			if c%4 < 2 {
				j = -1
			} else {
				j = 1
			}
			if c%2 < 1 {
				k = -1
			} else {
				k = 1
			}

			switch child.node.nodeType {
			case internalNode:
				numInternalNodes++
			case leafNodeFilled:
				numLeafNodeFilledNodes++
			case leafNodeEmpty:
				numLeafNodeEmptyNodes++
			}

			childSize, childMaxProb := validateBasicOctree(t, child, r3.Vector{
				X: center.X + i*sideLength/4.,
				Y: center.Y + j*sideLength/4.,
				Z: center.Z + k*sideLength/4.,
			}, sideLength/2.)
			size += childSize
			maxVal = int(math.Max(float64(maxVal), float64(childMaxProb)))
		}
		test.That(t, size, test.ShouldEqual, bOct.size)
		test.That(t, bOct.node.maxVal, test.ShouldEqual, maxVal)
		test.That(t, bOct.node.maxVal, test.ShouldEqual, bOct.MaxVal())
		test.That(t, numInternalNodes+numLeafNodeEmptyNodes+numLeafNodeFilledNodes, test.ShouldEqual, 8)
	case leafNodeFilled:
		test.That(t, len(bOct.node.children), test.ShouldEqual, 0)
		test.That(t, bOct.node.point, test.ShouldNotBeNil)
		test.That(t, bOct.checkPointPlacement(bOct.node.point.P), test.ShouldBeTrue)
		test.That(t, bOct.size, test.ShouldEqual, 1)
		size = bOct.size
		maxVal = bOct.node.maxVal
	case leafNodeEmpty:
		test.That(t, len(bOct.node.children), test.ShouldEqual, 0)
		test.That(t, bOct.node.point, test.ShouldBeNil)
		test.That(t, bOct.size, test.ShouldEqual, 0)
		size = bOct.size
	}
	return size, maxVal
}

// Helper function for checking basic octree metadata.
func validateCollisionMetadata(t *testing.T, bOct *CollisionOctree) {
	t.Helper()

	metadata := NewMetaData()
	bOct.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		metadata.Merge(p, d)
		return true
	})

	test.That(t, bOct.meta.HasColor, test.ShouldEqual, metadata.HasColor)
	test.That(t, bOct.meta.HasValue, test.ShouldEqual, metadata.HasValue)
	test.That(t, bOct.meta.MaxX, test.ShouldEqual, metadata.MaxX)
	test.That(t, bOct.meta.MinX, test.ShouldEqual, metadata.MinX)
	test.That(t, bOct.meta.MaxY, test.ShouldEqual, metadata.MaxY)
	test.That(t, bOct.meta.MinY, test.ShouldEqual, metadata.MinY)
	test.That(t, bOct.meta.MaxZ, test.ShouldEqual, metadata.MaxZ)
	test.That(t, bOct.meta.MinZ, test.ShouldEqual, metadata.MinZ)

	// tolerance value to handle uncertainties in float point calculations
	tolerance := 0.0001
	test.That(t, bOct.meta.TotalX(), test.ShouldBeBetween, metadata.TotalX()-tolerance, metadata.TotalX()+tolerance)
	test.That(t, bOct.meta.TotalY(), test.ShouldBeBetween, metadata.TotalY()-tolerance, metadata.TotalY()+tolerance)
	test.That(t, bOct.meta.TotalZ(), test.ShouldBeBetween, metadata.TotalZ()-tolerance, metadata.TotalZ()+tolerance)
}
