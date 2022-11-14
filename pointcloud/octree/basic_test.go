package octree

import (
	"context"
	"errors"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	pc "go.viam.com/rdk/pointcloud"
)

func createNewOctree(t *testing.T, center r3.Vector, side float64) (Octree, error) {
	t.Helper()

	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	return New(ctx, center, side, logger)
}

func TestNewOctree(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	sideInvalid := 0.0
	_, err := createNewOctree(t, center, sideInvalid)
	test.That(t, err, test.ShouldBeError, errors.New("invalid side length for octree"))

	sideInvalid = -2.0
	_, err = createNewOctree(t, center, sideInvalid)
	test.That(t, err, test.ShouldBeError, errors.New("invalid side length for octree"))

	sideValid := 1.0
	octree, err := createNewOctree(t, center, sideValid)
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
		basicOct := octree.(*basicOctree)

		test.That(t, basicOct.node, test.ShouldResemble, newLeafNodeEmpty())
		test.That(t, basicOct.center, test.ShouldResemble, r3.Vector{X: 0, Y: 0, Z: 0})
		test.That(t, basicOct.side, test.ShouldAlmostEqual, sideValid)
		test.That(t, basicOct.meta, test.ShouldResemble, newOctreeMetadata)
	})

	t.Run("Check new octree metadata creation", func(t *testing.T) {
		basicOct := octree.(*basicOctree)

		meta := basicOct.NewMetaData()
		test.That(t, meta, test.ShouldResemble, newOctreeMetadata)
	})

	t.Run("Update metadata when merging a new point into a new basic octree", func(t *testing.T) {
		basicOct := octree.(*basicOctree)
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

func TestBasicOctreeSet(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 1.0

	octree, err := createNewOctree(t, center, side)
	test.That(t, err, test.ShouldBeNil)

	basicOct := octree.(*basicOctree)

	point1 := r3.Vector{X: 0, Y: 0, Z: 0}
	data1 := pc.NewValueData(1)
	err = basicOct.Set(point1, data1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, basicOct.node, test.ShouldResemble, newLeafNodeFilled(point1, data1))

	// Setting into:
	// - internal node good
	// - internal node bad
	// - empty leaf node good (done)
	// - empty leaf node bad
	// - filled leaf node good
	// - filled leaf node bad

	// Add metadata checks at end
}

func TestBasicOctreeAt(t *testing.T) {
	// At into:
	// - internal node good
	// - internal node bad
	// - empty leaf node good
	// - empty leaf node bad
	// - filled leaf node good
	// - filled leaf node bad

	// Add metadata checks at end
}

func TestBasicOctreeSize(t *testing.T) {
	// Size of:
	// - internal node good
	// - internal node bad
	// - empty leaf node good
	// - empty leaf node bad
	// - filled leaf node good
	// - filled leaf node bad

	// Add metadata checks at end
}
