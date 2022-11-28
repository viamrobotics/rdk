package octree

import (
	"context"
	"math"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

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

// Makes and returns a PointCloud from an artifact path.
func makePointCloudFromArtifact(t *testing.T, artifactPath string, numPoints int) (pc.PointCloud, error) {
	t.Helper()
	pcdFile, err := os.Open(artifact.MustPath(artifactPath))
	if err != nil {
		return nil, err
	}
	pcd, err := pc.ReadPCD(pcdFile)
	if err != nil {
		return nil, err
	}

	if numPoints == 0 {
		return pcd, nil
	}

	shortenedPC := pc.NewWithPrealloc(numPoints)

	counter := numPoints
	pcd.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
		if counter > 0 {
			err = shortenedPC.Set(p, d)
			counter--
		}
		return err == nil
	})
	if err != nil {
		return nil, err
	}

	return shortenedPC, nil
}

// Test the creation of new basic octrees.
func TestBasicOctreeNew(t *testing.T) {
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

	t.Run("New Octree as basic octree", func(t *testing.T) {
		test.That(t, basicOct.node, test.ShouldResemble, newLeafNodeEmpty())
		test.That(t, basicOct.center, test.ShouldResemble, r3.Vector{X: 0, Y: 0, Z: 0})
		test.That(t, basicOct.sideLength, test.ShouldAlmostEqual, sideValid)
		test.That(t, basicOct.meta, test.ShouldResemble, pc.NewMetaData())
	})

	validateBasicOctree(t, basicOct, center, sideValid)
}

// Test the Set()function which adds points and associated data to an octree.
func TestBasicOctreeSet(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0

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

		validateBasicOctree(t, basicOct, center, side)
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

		validateBasicOctree(t, basicOct, center, side)
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

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Set point that lies outside the basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.Set(r3.Vector{X: 2, Y: 0, Z: 0}, pc.NewValueData(1))
		test.That(t, err, test.ShouldBeError, errors.New("error point is outside the bounds of this octree"))

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Set point at intersection of multiple basic octree nodes", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewValueData(1))
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.Set(r3.Vector{X: -.5, Y: 0, Z: 0}, pc.NewValueData(1))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.size, test.ShouldEqual, 2)

		validateBasicOctree(t, basicOct, center, side)
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

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Set point into invalid internal node", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		basicOct.node = newInternalNode([]*basicOctree{})
		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewValueData(1))
		test.That(t, err, test.ShouldBeError, errors.New("error invalid internal node detected, please check your tree"))
	})

	t.Run("Set point into invalid internal node", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		basicOct.node = newInternalNode([]*basicOctree{})
		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewValueData(1))
		test.That(t, err, test.ShouldBeError, errors.New("error invalid internal node detected, please check your tree"))
	})
}

// Test the At() function for basic octrees which returns the data at a specific location should it exist.
func TestBasicOctreeAt(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0

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

		validateBasicOctree(t, basicOct, center, side)
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

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("At check of empty basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		d, ok := basicOct.At(0, 0, 0)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, d, test.ShouldBeNil)

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("At check of point outside octree bounds", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		d, ok := basicOct.At(3, 0, 0)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, d, test.ShouldBeNil)

		validateBasicOctree(t, basicOct, center, side)
	})
}

// Test the functionalities involved with converting a pointcloud into a basic octree.
func TestBasicOctreePointcloudIngestion(t *testing.T) {
	startPC, err := makePointCloudFromArtifact(t, "pointcloud/test.pcd", 100)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	center := r3.Vector{
		X: startPC.MetaData().MinX + (startPC.MetaData().MaxX-startPC.MetaData().MinX)/2,
		Y: startPC.MetaData().MinY + (startPC.MetaData().MaxY-startPC.MetaData().MinY)/2,
		Z: startPC.MetaData().MinZ + (startPC.MetaData().MaxZ-startPC.MetaData().MinZ)/2,
	}

	side := math.Max((startPC.MetaData().MaxX-startPC.MetaData().MinX),
		math.Max((startPC.MetaData().MaxY-startPC.MetaData().MinY),
			(startPC.MetaData().MaxZ-startPC.MetaData().MinZ))) * 1.01

	basicOct, err := createNewOctree(ctx, center, side, logger)
	test.That(t, err, test.ShouldBeNil)

	startPC.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
		if err = basicOct.Set(p, d); err != nil {
			return false
		}
		return true
	})

	test.That(t, startPC.Size(), test.ShouldEqual, basicOct.Size())
	test.That(t, startPC.MetaData(), test.ShouldResemble, basicOct.meta)
	// TODO: Add iterate check of each point pointcloud to see if it is in octree (next JIRA ticket)

	validateBasicOctree(t, basicOct, center, side)
}
