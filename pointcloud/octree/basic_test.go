package octree

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"

	pc "go.viam.com/rdk/pointcloud"
)

// Helper function for generating a new empty octree.
func createNewOctree(ctx context.Context, center r3.Vector, side float64, logger golog.Logger) (*basicOctree, error) {
	octree, err := New(ctx, center, side, logger)
	if err != nil {
		return nil, err
	}

	basicOct := octree.(*basicOctree)
	return basicOct, err
}

// Test the creation of new octrees.
func TestNewOctree(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	center := r3.Vector{X: 0, Y: 0, Z: 0}
	sideInvalid := 0.0
	_, err := createNewOctree(ctx, center, sideInvalid, logger)
	test.That(t, err, test.ShouldBeError, errors.Errorf("invalid side length (%.2f) for octree", sideInvalid))

	sideInvalid = -2.0
	_, err = createNewOctree(ctx, center, sideInvalid, logger)
	test.That(t, err, test.ShouldBeError, errors.Errorf("invalid side length (%.2f) for octree", sideInvalid))

	sideValid := 1.0
	basicOct, err := createNewOctree(ctx, center, sideValid, logger)
	test.That(t, err, test.ShouldBeNil)

	newOctreeMetadata := MetaData{
		Version:    octreeVersion,
		CenterX:    center.X,
		CenterY:    center.Y,
		CenterZ:    center.Z,
		Side:       sideValid,
		Size:       0,
		PCMetaData: pc.NewMetaData(),
	}

	t.Run("New Octree as basic octree", func(t *testing.T) {
		test.That(t, basicOct.node, test.ShouldResemble, newLeafNodeEmpty())
		test.That(t, basicOct.center, test.ShouldResemble, r3.Vector{X: 0, Y: 0, Z: 0})
		test.That(t, basicOct.side, test.ShouldAlmostEqual, sideValid)
		test.That(t, basicOct.meta, test.ShouldResemble, newOctreeMetadata)
	})

	t.Run("Check new octree metadata creation", func(t *testing.T) {
		meta := basicOct.NewMetaData()
		test.That(t, meta, test.ShouldResemble, newOctreeMetadata)
	})

	t.Run("Update metadata when merging a new point into a new basic octree", func(t *testing.T) {
		p := r3.Vector{X: 0, Y: 0, Z: 0}
		d := pc.NewBasicData()

		newOctreeMetadata.Size = 1
		newOctreeMetadata.PCMetaData.Merge(p, d)

		basicOct.meta.Merge(p, d)
		// meta value after merge function call
		test.That(t, basicOct.meta, test.ShouldResemble, newOctreeMetadata)
		// OctreeMetData function call
		test.That(t, basicOct.MetaData(), test.ShouldResemble, newOctreeMetadata.PCMetaData)
		// MetaData function call
		test.That(t, basicOct.OctreeMetaData(), test.ShouldResemble, newOctreeMetadata)
	})
}

// Test the Set()function which adds points and associated data to an octree.
func TestBasicOctreeSet(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 1.0

	t.Run("Set point into empty leaf node into basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.Size(), test.ShouldEqual, 0)

		point1 := r3.Vector{X: 0.1, Y: 0, Z: 0}
		data1 := pc.NewValueData(1)
		err = basicOct.Set(point1, data1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.node, test.ShouldResemble, newLeafNodeFilled(point1, data1))
		test.That(t, basicOct.Size(), test.ShouldEqual, 1)
	})

	t.Run("Set point into filled leaf node into basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewValueData(1))
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.Set(r3.Vector{X: -.5, Y: 0, Z: 0}, pc.NewValueData(1))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.node.nodeType, test.ShouldResemble, InternalNode)
		test.That(t, basicOct.Size(), test.ShouldEqual, 2)
		test.That(t, basicOct.OctreeMetaData().Size, test.ShouldEqual, 2)
	})

	t.Run("Set point into internal node node into basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewValueData(1))
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.Set(r3.Vector{X: -.5, Y: 0, Z: 0}, pc.NewValueData(1))
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.Set(r3.Vector{X: -.4, Y: 0, Z: 0}, pc.NewValueData(1))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.node.nodeType, test.ShouldResemble, InternalNode)
		test.That(t, basicOct.Size(), test.ShouldEqual, 3)
		test.That(t, basicOct.OctreeMetaData().Size, test.ShouldEqual, 3)
	})

	t.Run("Set point that lies outside the basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.Set(r3.Vector{X: 2, Y: 0, Z: 0}, pc.NewValueData(1))
		test.That(t, err, test.ShouldBeError, errors.New("error point is outside the bounds of this octree"))
	})

	t.Run("Set point at split in basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewValueData(1))
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.Set(r3.Vector{X: -.5, Y: 0, Z: 0}, pc.NewValueData(1))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.OctreeMetaData().Size, test.ShouldEqual, basicOct.Size())
	})

	t.Run("Set same point with new data in basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		val := 1
		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewValueData(val))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.node.point.D.Value(), test.ShouldEqual, val)

		val = 2
		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewValueData(val))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.node.point.D.Value(), test.ShouldEqual, val)
		test.That(t, basicOct.Size(), test.ShouldEqual, 1)
		test.That(t, basicOct.OctreeMetaData().Size, test.ShouldEqual, 1)
	})

	t.Run("Set point into invalid internal node", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		basicOct.node = newInternalNode([]*basicOctree{})
		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewValueData(1))
		test.That(t, err, test.ShouldBeError, errors.New("error invalid internal node detected, please check your tree"))
	})
}

// Test the At() function which returns the data at a specific location should it exist.
func TestBasicOctreeAt(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 1.0

	t.Run("At check of single node basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		p1 := r3.Vector{X: 0, Y: 0, Z: 0}
		d1 := pc.NewValueData(1)
		err = basicOct.Set(p1, d1)
		test.That(t, err, test.ShouldBeNil)

		d, ok := basicOct.At(0, 0, 0)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, d1)

		d, ok = basicOct.At(0.0001, 0, 0)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, d, test.ShouldBeNil)
	})

	t.Run("At check of multi level basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		p1 := r3.Vector{X: 0, Y: 0, Z: 0}
		d1 := pc.NewValueData(1)
		err = basicOct.Set(p1, d1)
		test.That(t, err, test.ShouldBeNil)

		p2 := r3.Vector{X: -.5, Y: 0, Z: 0}
		d2 := pc.NewValueData(2)
		err = basicOct.Set(p2, d2)
		test.That(t, err, test.ShouldBeNil)

		p3 := r3.Vector{X: -.4, Y: 0, Z: 0}
		d3 := pc.NewValueData(3)
		err = basicOct.Set(p3, d3)
		test.That(t, err, test.ShouldBeNil)

		d, ok := basicOct.At(p1.X, p1.Y, p1.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, d1)

		d, ok = basicOct.At(p2.X, p2.Y, p2.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, d2)

		d, ok = basicOct.At(p3.X, p3.Y, p3.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, d3)

		d, ok = basicOct.At(-.6, 0, 0)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, d, test.ShouldBeNil)
	})

	t.Run("At check of empty basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		d, ok := basicOct.At(0, 0, 0)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, d, test.ShouldBeNil)
	})
}
