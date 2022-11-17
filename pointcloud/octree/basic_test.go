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

func addPoints(basicOct *basicOctree, pointsAndData []pc.PointAndData) error {
	for _, pd := range pointsAndData {
		if err := basicOct.Set(pd.P, pd.D); err != nil {
			return err
		}
	}
	return nil
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

		pointsAndData := []pc.PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: pc.NewValueData(1)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		d, ok := basicOct.At(pointsAndData[0].P.X, pointsAndData[0].P.Y, pointsAndData[0].P.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, pointsAndData[0].D)

		d, ok = basicOct.At(0.0001, 0, 0)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, d, test.ShouldBeNil)
	})

	t.Run("At check of multi level basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []pc.PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: pc.NewValueData(1)},
			{P: r3.Vector{X: -.5, Y: 0, Z: 0}, D: pc.NewValueData(2)},
			{P: r3.Vector{X: -0.4, Y: 0, Z: 0}, D: pc.NewValueData(3)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		d, ok := basicOct.At(pointsAndData[0].P.X, pointsAndData[0].P.Y, pointsAndData[0].P.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, pointsAndData[0].D)

		d, ok = basicOct.At(pointsAndData[1].P.X, pointsAndData[1].P.Y, pointsAndData[1].P.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, pointsAndData[1].D)

		d, ok = basicOct.At(pointsAndData[2].P.X, pointsAndData[2].P.Y, pointsAndData[2].P.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, pointsAndData[2].D)

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

// Test the Iterate() function which will apply a specified  function to every point in an octree until one returns a
// false value.
func TestBasicOctreeIterate(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 1.0

	t.Run("Iterate zero batch check of an empty basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		total := 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, 0)
	})

	t.Run("Iterate zero batch check of an filled basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []pc.PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: pc.NewValueData(2)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		// All true (full iteration)
		total := 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value())

		// False return (partial iteration)
		total = 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
			if d.Value() == 1 {
				total += d.Value()
			}
			return true
		})
		test.That(t, total, test.ShouldNotEqual, pointsAndData[0].D.Value())
	})

	t.Run("Iterate zero batch check of an multi-level basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []pc.PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: pc.NewValueData(1)},
			{P: r3.Vector{X: .5, Y: 0, Z: 0}, D: pc.NewValueData(2)},
			{P: r3.Vector{X: .6, Y: 0, Z: 0}, D: pc.NewValueData(1)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		// All true (full iteration)
		total := 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value()+
			pointsAndData[1].D.Value()+
			pointsAndData[2].D.Value())

		// False return (partial iteration)
		total = 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
			if d.Value() == 1 {
				total += d.Value()
				return true
			}
			return false
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value())
	})

	t.Run("Iterate non-zero batch check of an filled basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []pc.PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: pc.NewValueData(2)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		// Matching batch id
		total := 0
		basicOct.Iterate(1, 0, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value())

		// Non-matching batch id
		total = 0
		basicOct.Iterate(1, 1, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, 0)
	})

	t.Run("Iterate non-zero batch check of an multi-level basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []pc.PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: pc.NewValueData(1)},
			{P: r3.Vector{X: .5, Y: 0, Z: 0}, D: pc.NewValueData(2)},
			{P: r3.Vector{X: .6, Y: 0, Z: 0}, D: pc.NewValueData(3)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		// Batched processing with match for first data point
		total := 0
		basicOct.Iterate(9, 1, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value())

		// Batched processing with match for second data point
		total = 0
		basicOct.Iterate(9, 0, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[1].D.Value())

		// Batched processing no matching data point
		total = 0
		basicOct.Iterate(9, 2, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, 0)

		// Batched processing all matching data points
		total = 0
		basicOct.Iterate(1, 0, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value()+
			pointsAndData[1].D.Value()+
			pointsAndData[2].D.Value())
	})
}
