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

// Helper function that adds a list of points to a given basic octree.
func addPoints(basicOct *basicOctree, pointsAndData []pc.PointAndData) error {
	for _, pd := range pointsAndData {
		if err := basicOct.Set(pd.P, pd.D); err != nil {
			return err
		}
	}
	return nil
}

// Helper function that checks that all valid points from the given list have been added to the basic octree.
func checkPoints(t *testing.T, basicOct *basicOctree, pointsAndData []pc.PointAndData) {
	t.Helper()

	for _, point := range pointsAndData {
		d, ok := basicOct.At(point.P.X, point.P.Y, point.P.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, point.D)
	}
}

// Helper function that makes and returns a PointCloud from an artifact path.
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
		test.That(t, basicOct.MetaData(), test.ShouldResemble, pc.NewMetaData())
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

	t.Run("Set point, hit max recursion depth", func(t *testing.T) {
		side = 2.
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		basicOct = createLopsidedOctree(basicOct, 0, maxRecursionDepth-1)

		err = basicOct.Set(r3.Vector{X: -1, Y: -1, Z: -1}, pc.NewBasicData())
		test.That(t, err, test.ShouldBeNil)

		basicOct = createLopsidedOctree(basicOct, 0, maxRecursionDepth)
		err = basicOct.Set(r3.Vector{X: -1, Y: -1, Z: -1}, pc.NewBasicData())

		test.That(t, err, test.ShouldBeError, errors.New("error max allowable recursion depth reached"))
	})

	t.Run("Set empty data point", func(t *testing.T) {
		side = 1.
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		pointAndData := pc.PointAndData{}

		err = basicOct.Set(pointAndData.P, pointAndData.D)
		test.That(t, err, test.ShouldBeNil)
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

		pointsAndData := []pc.PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: pc.NewValueData(1)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		checkPoints(t, basicOct, pointsAndData)

		d, ok := basicOct.At(0.0001, 0, 0)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, d, test.ShouldBeNil)

		validateBasicOctree(t, basicOct, center, side)
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

		checkPoints(t, basicOct, pointsAndData)

		d, ok := basicOct.At(-.6, 0, 0)
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

// Test the Iterate() function, which will apply a specified function to every point in a basic octree until
// the function returns a false value.
func TestBasicOctreeIterate(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0

	t.Run("Iterate zero batch check of an empty basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		total := 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, 0)

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Iterate zero batch check of a filled basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []pc.PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: pc.NewValueData(2)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		// Full iteration - applies function to all points
		total := 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value())

		// Partial iteration - applies function to no points
		total = 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
			if d.Value() == 1 {
				total += d.Value()
			}
			return true
		})
		test.That(t, total, test.ShouldNotEqual, pointsAndData[0].D.Value())

		validateBasicOctree(t, basicOct, center, side)
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

		// Full iteration - applies function to all points
		total := 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value()+
			pointsAndData[1].D.Value()+
			pointsAndData[2].D.Value())

		// Partial iteration - applies function to only first point
		total = 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
			if d.Value() == 1 {
				total += d.Value()
				return true
			}
			return false
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value())

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Iterate non-zero batch check of an filled basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []pc.PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: pc.NewValueData(2)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		total := 0
		basicOct.Iterate(1, 0, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value())

		// Invalid batching
		total = 0
		basicOct.Iterate(1, 2, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, 0)

		validateBasicOctree(t, basicOct, center, side)
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

		// Batched process (numBatches = octree size, currentBatch = 0)
		total := 0
		basicOct.Iterate(3, 0, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value())

		// Batched process (numBatches = octree size, currentBatch = 1)
		total = 0
		basicOct.Iterate(3, 1, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[1].D.Value())

		// Batched process (numBatches = octree size, currentBatch = 2)
		total = 0
		basicOct.Iterate(3, 2, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[2].D.Value())

		// Batched process (numBatches = octree size, currentBatch = 3)
		total = 0
		basicOct.Iterate(3, 3, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, 0)

		// Batched process (numBatches > octree size, currentBatch = 0)
		total = 0
		basicOct.Iterate(4, 0, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value())

		// Batched process (numBatches > octree size, currentBatch = 1)
		total = 0
		basicOct.Iterate(4, 1, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[1].D.Value())

		// Batched process (numBatches > octree size, currentBatch = 2)
		total = 0
		basicOct.Iterate(4, 2, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[2].D.Value())

		// Batched process (numBatches > octree size, currentBatch = 3)
		total = 0
		basicOct.Iterate(4, 3, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, 0)

		// Batched process (numBatches < octree size, currentBatch = 0)
		total = 0
		basicOct.Iterate(2, 0, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value()+
			pointsAndData[1].D.Value())

		// Batched process (numBatches < octree size, currentBatch = 1)
		total = 0
		basicOct.Iterate(2, 1, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[2].D.Value())

		// Batched process (apply function to all data)
		total = 0
		basicOct.Iterate(1, 0, func(p r3.Vector, d pc.Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value()+
			pointsAndData[1].D.Value()+
			pointsAndData[2].D.Value())

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

	// Check all points from the pointcloud have been properly added to the new basic octree
	startPC.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
		dOct, ok := basicOct.At(p.X, p.Y, p.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, dOct)
		return true
	})

	validateBasicOctree(t, basicOct, center, side)
}
