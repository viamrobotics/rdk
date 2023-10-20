package pointcloud

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/spatialmath"
)

// Helper function for generating a new empty octree.
func createNewOctree(center r3.Vector, side float64) (*BasicOctree, error) {
	basicOct, err := NewBasicOctree(center, side)
	if err != nil {
		return nil, err
	}

	return basicOct, err
}

// Helper function that adds a list of points to a given basic octree.
func addPoints(basicOct *BasicOctree, pointsAndData []PointAndData) error {
	for _, pd := range pointsAndData {
		if err := basicOct.Set(pd.P, pd.D); err != nil {
			return err
		}
	}
	return nil
}

// Helper function that checks that all valid points from the given list have been added to the basic octree.
func checkPoints(t *testing.T, basicOct *BasicOctree, pointsAndData []PointAndData) {
	t.Helper()

	for _, point := range pointsAndData {
		d, ok := basicOct.At(point.P.X, point.P.Y, point.P.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, point.D)
	}
}

// Helper function that makes and returns a PointCloud of a given type from an artifact path.
func makeFullPointCloudFromArtifact(t *testing.T, artifactPath string, pcType PCType) (PointCloud, error) {
	t.Helper()

	path := filepath.Clean(artifact.MustPath(artifactPath))
	pcdFile, err := os.Open(path)
	defer utils.UncheckedErrorFunc(pcdFile.Close)
	if err != nil {
		return nil, err
	}

	var PC PointCloud
	switch pcType {
	case BasicType:
		PC, err = ReadPCD(pcdFile)
	case BasicOctreeType:
		PC, err = ReadPCDToBasicOctree(pcdFile)
	}

	return PC, err
}

// Test the creation of new basic octrees.
func TestBasicOctreeNew(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	sideInvalid := 0.0
	_, err := createNewOctree(center, sideInvalid)
	test.That(t, err, test.ShouldBeError, errors.Errorf("invalid side length (%.2f) for octree", sideInvalid))

	sideInvalid = -2.0
	_, err = createNewOctree(center, sideInvalid)
	test.That(t, err, test.ShouldBeError, errors.Errorf("invalid side length (%.2f) for octree", sideInvalid))

	sideValid := 1.0
	basicOct, err := createNewOctree(center, sideValid)
	test.That(t, err, test.ShouldBeNil)

	t.Run("New Octree as basic octree", func(t *testing.T) {
		test.That(t, basicOct.node, test.ShouldResemble, newLeafNodeEmpty())
		test.That(t, basicOct.center, test.ShouldResemble, r3.Vector{X: 0, Y: 0, Z: 0})
		test.That(t, basicOct.sideLength, test.ShouldAlmostEqual, sideValid)
		test.That(t, basicOct.meta, test.ShouldResemble, NewMetaData())
		test.That(t, basicOct.MetaData(), test.ShouldResemble, NewMetaData())
	})

	validateBasicOctree(t, basicOct, center, sideValid)
}

// Test the Set()function which adds points and associated data to an octree.
func TestBasicOctreeSet(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0

	t.Run("Set point into empty leaf node into basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.Size(), test.ShouldEqual, 0)

		point1 := r3.Vector{X: 0.1, Y: 0, Z: 0}
		data1 := NewValueData(1)
		err = basicOct.Set(point1, data1)
		test.That(t, err, test.ShouldBeNil)
		node := newLeafNodeFilled(point1, data1)
		test.That(t, basicOct.node, test.ShouldResemble, node)
		test.That(t, basicOct.Size(), test.ShouldEqual, 1)

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Set point into filled leaf node into basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		d1 := 1
		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(d1))
		test.That(t, err, test.ShouldBeNil)
		mp := basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, d1)

		d2 := 2
		err = basicOct.Set(r3.Vector{X: -.5, Y: 0, Z: 0}, NewValueData(d2))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.node.nodeType, test.ShouldResemble, internalNode)
		test.That(t, basicOct.Size(), test.ShouldEqual, 2)
		mp = basicOct.node.children[0].MaxVal()
		test.That(t, mp, test.ShouldEqual, int(math.Max(float64(d1), float64(d2))))
		mp = basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, int(math.Max(float64(d1), float64(d2))))

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Set point into internal node node into basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		d3 := 3
		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(d3))
		test.That(t, err, test.ShouldBeNil)
		mp := basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, d3)

		d2 := 2
		err = basicOct.Set(r3.Vector{X: -.5, Y: 0, Z: 0}, NewValueData(d2))
		test.That(t, err, test.ShouldBeNil)
		mp = basicOct.node.children[0].MaxVal()
		test.That(t, mp, test.ShouldEqual, int(math.Max(float64(d2), float64(d3))))

		d4 := 4
		err = basicOct.Set(r3.Vector{X: -.4, Y: 0, Z: 0}, NewValueData(d4))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.node.nodeType, test.ShouldResemble, internalNode)
		test.That(t, basicOct.Size(), test.ShouldEqual, 3)
		mp = basicOct.node.children[0].MaxVal()
		greatest := int(math.Max(math.Max(float64(d2), float64(d3)), float64(d4)))
		test.That(t, mp, test.ShouldEqual, greatest)

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Set point that lies outside the basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.Set(r3.Vector{X: 2, Y: 0, Z: 0}, NewValueData(1))
		test.That(t, err, test.ShouldBeError, errors.New("error point is outside the bounds of this octree"))

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Set point at intersection of multiple basic octree nodes", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		d1 := 1
		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(d1))
		test.That(t, err, test.ShouldBeNil)
		mp := basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, d1)

		d2 := 2
		err = basicOct.Set(r3.Vector{X: -.5, Y: 0, Z: 0}, NewValueData(d2))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.size, test.ShouldEqual, 2)
		mp = basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, int(math.Max(float64(d1), float64(d2))))

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Set same point with new data in basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		d1 := 1
		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(d1))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.node.point.D.Value(), test.ShouldEqual, d1)
		mp := basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, d1)

		d2 := 2
		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(d2))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.node.point.D.Value(), test.ShouldEqual, d2)
		test.That(t, basicOct.Size(), test.ShouldEqual, 1)
		mp = basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, int(math.Max(float64(d1), float64(d2))))

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Set point into invalid internal node", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		basicOct.node = newInternalNode([]*BasicOctree{})
		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(1))
		test.That(t, err, test.ShouldBeError, errors.New("error invalid internal node detected, please check your tree"))
	})

	t.Run("Set point into invalid internal node", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		basicOct.node = newInternalNode([]*BasicOctree{})
		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(1))
		test.That(t, err, test.ShouldBeError, errors.New("error invalid internal node detected, please check your tree"))
	})

	t.Run("Set point, hit max recursion depth", func(t *testing.T) {
		side = 2.
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		basicOct = createLopsidedOctree(basicOct, 0, maxRecursionDepth-1)

		d1 := 1
		err = basicOct.Set(r3.Vector{X: -1, Y: -1, Z: -1}, NewValueData(d1))
		test.That(t, err, test.ShouldBeNil)
		mp := basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, d1)

		basicOct = createLopsidedOctree(basicOct, 0, maxRecursionDepth)
		err = basicOct.Set(r3.Vector{X: -1, Y: -1, Z: -1}, NewBasicData())
		test.That(t, err, test.ShouldBeError, errors.New("error max allowable recursion depth reached"))
	})

	t.Run("Set empty data point", func(t *testing.T) {
		side = 1.
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		pointAndData := PointAndData{}

		err = basicOct.Set(pointAndData.P, pointAndData.D)
		test.That(t, err, test.ShouldBeNil)
	})
}

// Test the At() function for basic octrees which returns the data at a specific location should it exist.
func TestBasicOctreeAt(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0

	t.Run("At check of single node basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(1)},
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
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(1)},
			{P: r3.Vector{X: -.5, Y: 0, Z: 0}, D: NewValueData(2)},
			{P: r3.Vector{X: -0.4, Y: 0, Z: 0}, D: NewValueData(3)},
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
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		d, ok := basicOct.At(0, 0, 0)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, d, test.ShouldBeNil)

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("At check of point outside octree bounds", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
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
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0

	t.Run("Iterate zero batch check of an empty basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		total := 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, 0)

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Iterate zero batch check of a filled basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(2)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		// Full iteration - applies function to all points
		total := 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value())

		// Partial iteration - applies function to no points
		total = 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d Data) bool {
			if d.Value() == 1 {
				total += d.Value()
			}
			return true
		})
		test.That(t, total, test.ShouldNotEqual, pointsAndData[0].D.Value())

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Iterate zero batch check of an multi-level basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(1)},
			{P: r3.Vector{X: .5, Y: 0, Z: 0}, D: NewValueData(2)},
			{P: r3.Vector{X: .6, Y: 0, Z: 0}, D: NewValueData(1)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		// Full iteration - applies function to all points
		total := 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value()+
			pointsAndData[1].D.Value()+
			pointsAndData[2].D.Value())

		// Partial iteration - applies function to only first point
		total = 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d Data) bool {
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
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(2)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		total := 0
		basicOct.Iterate(1, 0, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value())

		// Invalid batching
		total = 0
		basicOct.Iterate(1, 2, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, 0)

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Iterate non-zero batch check of an multi-level basic octree", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(1)},
			{P: r3.Vector{X: .5, Y: 0, Z: 0}, D: NewValueData(2)},
			{P: r3.Vector{X: .6, Y: 0, Z: 0}, D: NewValueData(3)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		// Batched process (numBatches = octree size, currentBatch = 0)
		total := 0
		basicOct.Iterate(3, 0, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value())

		// Batched process (numBatches = octree size, currentBatch = 1)
		total = 0
		basicOct.Iterate(3, 1, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[1].D.Value())

		// Batched process (numBatches = octree size, currentBatch = 2)
		total = 0
		basicOct.Iterate(3, 2, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[2].D.Value())

		// Batched process (numBatches = octree size, currentBatch = 3)
		total = 0
		basicOct.Iterate(3, 3, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, 0)

		// Batched process (numBatches > octree size, currentBatch = 0)
		total = 0
		basicOct.Iterate(4, 0, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value())

		// Batched process (numBatches > octree size, currentBatch = 1)
		total = 0
		basicOct.Iterate(4, 1, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[1].D.Value())

		// Batched process (numBatches > octree size, currentBatch = 2)
		total = 0
		basicOct.Iterate(4, 2, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[2].D.Value())

		// Batched process (numBatches > octree size, currentBatch = 3)
		total = 0
		basicOct.Iterate(4, 3, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, 0)

		// Batched process (numBatches < octree size, currentBatch = 0)
		total = 0
		basicOct.Iterate(2, 0, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value()+
			pointsAndData[1].D.Value())

		// Batched process (numBatches < octree size, currentBatch = 1)
		total = 0
		basicOct.Iterate(2, 1, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[2].D.Value())

		// Batched process (apply function to all data)
		total = 0
		basicOct.Iterate(1, 0, func(p r3.Vector, d Data) bool {
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
	startPC, err := makeFullPointCloudFromArtifact(t, "pointcloud/test_short.pcd", BasicType)
	test.That(t, err, test.ShouldBeNil)

	center := getCenterFromPcMetaData(startPC.MetaData())
	maxSideLength := getMaxSideLengthFromPcMetaData(startPC.MetaData())

	basicOct, err := NewBasicOctree(center, maxSideLength)
	test.That(t, err, test.ShouldBeNil)

	startPC.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		if err = basicOct.Set(p, d); err != nil {
			return false
		}
		return true
	})

	test.That(t, startPC.Size(), test.ShouldEqual, basicOct.Size())
	test.That(t, startPC.MetaData(), test.ShouldResemble, basicOct.meta)

	// Check all points from the pointcloud have been properly added to the new basic octree
	startPC.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		dOct, ok := basicOct.At(p.X, p.Y, p.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, dOct)
		return true
	})

	validateBasicOctree(t, basicOct, center, maxSideLength)
}

// Test the functionalities involved with converting a pcd into a basic octree.
func TestReadBasicOctreeFromPCD(t *testing.T) {
	t.Run("reading from binary PCD to octree", func(t *testing.T) {
		binaryArtifactPath := "slam/rplidar_data/rplidar_data_0.pcd"
		testPCDToBasicOctree(t, binaryArtifactPath)
	})

	t.Run("reading from ascii PCD to octree", func(t *testing.T) {
		asciiArtifactPath := "slam/mock_lidar/0.pcd"
		testPCDToBasicOctree(t, asciiArtifactPath)
	})
}

// Helper function for testing basic octree creation from a given artifact.
func testPCDToBasicOctree(t *testing.T, artifactPath string) {
	basicPC, err := makeFullPointCloudFromArtifact(t, artifactPath, BasicType)
	test.That(t, err, test.ShouldBeNil)
	basic, ok := basicPC.(*basicPointCloud)
	test.That(t, ok, test.ShouldBeTrue)

	basicOctPC, err := makeFullPointCloudFromArtifact(t, artifactPath, BasicOctreeType)
	test.That(t, err, test.ShouldBeNil)
	basicOct, ok := basicOctPC.(*BasicOctree)
	test.That(t, ok, test.ShouldBeTrue)

	test.That(t, basic.Size(), test.ShouldEqual, basicOct.Size())
	test.That(t, basic.MetaData(), test.ShouldResemble, basicOct.MetaData())

	// Check all points from the pcd have been properly added to the new basic octree
	basic.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		dOct, ok := basicOct.At(p.X, p.Y, p.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, dOct)
		return true
	})

	validateBasicOctree(t, basicOct, basicOct.center, basicOct.sideLength)
}

func TestCachedMaxProbability(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0

	t.Run("get the max val from an octree", func(t *testing.T) {
		octree, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)
		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(2)},
			{P: r3.Vector{X: .5, Y: 0, Z: 0}, D: NewValueData(3)},
			{P: r3.Vector{X: .5, Y: 0, Z: .5}, D: NewValueData(10)},
			{P: r3.Vector{X: .5, Y: .5, Z: 0}, D: NewValueData(1)},
			{P: r3.Vector{X: .55, Y: .55, Z: 0}, D: NewValueData(4)},
			{P: r3.Vector{X: -.55, Y: -.55, Z: 0}, D: NewValueData(5)},
			{P: r3.Vector{X: .755, Y: .755, Z: 0}, D: NewValueData(6)},
		}

		err = addPoints(octree, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		validateBasicOctree(t, octree, octree.center, octree.sideLength)

		mp := octree.MaxVal()
		test.That(t, mp, test.ShouldEqual, 10)

		mp = octree.node.children[0].MaxVal()
		test.That(t, mp, test.ShouldEqual, 5)
	})

	t.Run("cannot set arbitrary values into the octree", func(t *testing.T) {
		d := &basicData{value: 0, hasValue: false}
		node := newLeafNodeFilled(r3.Vector{1, 2, 3}, d)
		filledNode := basicOctreeNode{
			children: nil,
			nodeType: leafNodeFilled,
			point:    &PointAndData{P: r3.Vector{1, 2, 3}, D: d},
			maxVal:   emptyProb,
		}
		test.That(t, node, test.ShouldResemble, filledNode)
	})

	t.Run("setting negative values", func(t *testing.T) {
		octree, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)
		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(-2)},
			{P: r3.Vector{X: .5, Y: 0, Z: 0}, D: NewValueData(-3)},
			{P: r3.Vector{X: .5, Y: 0, Z: .5}, D: NewValueData(-10)},
			{P: r3.Vector{X: .5, Y: .5, Z: 0}, D: NewValueData(-1)},
			{P: r3.Vector{X: .55, Y: .55, Z: 0}, D: NewValueData(-4)},
			{P: r3.Vector{X: -.55, Y: -.55, Z: 0}, D: NewValueData(-5)},
			{P: r3.Vector{X: .755, Y: .755, Z: 0}, D: NewValueData(-6)},
		}

		err = addPoints(octree, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		validateBasicOctree(t, octree, octree.center, octree.sideLength)

		mp := octree.MaxVal()
		test.That(t, mp, test.ShouldEqual, -1)

		mp = octree.node.children[0].MaxVal()
		test.That(t, mp, test.ShouldEqual, -2)
	})
}

// Test the various geometry-specific interface methods.
func TestBasicOctreeGeometryFunctions(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0

	octree, err := createNewOctree(center, side)
	test.That(t, err, test.ShouldBeNil)
	pointsAndData := []PointAndData{
		{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(2)},
		{P: r3.Vector{X: 1, Y: 0, Z: 0}, D: NewValueData(3)},
		{P: r3.Vector{X: 1, Y: 1, Z: 1}, D: NewValueData(5)},
	}
	err = addPoints(octree, pointsAndData)
	test.That(t, err, test.ShouldBeNil)

	checkExpectedPoints := func(geom spatialmath.Geometry, pts []PointAndData) {
		geomPts := geom.ToPoints(0)
		octree, ok := geom.(*BasicOctree)
		test.That(t, ok, test.ShouldBeTrue)

		for _, geomPt := range geomPts {
			d, ok := octree.At(geomPt.X, geomPt.Y, geomPt.Z)
			test.That(t, ok, test.ShouldBeTrue)
			anyEqual := false
			for _, pd := range pts {
				if pointsAlmostEqualEpsilon(geomPt, pd.P, floatEpsilon) {
					anyEqual = true
					test.That(t, d, test.ShouldResemble, pd.D)
				}
			}
			test.That(t, anyEqual, test.ShouldBeTrue)
		}

		dupOctree, err := createNewOctree(pts[0].P, side)
		test.That(t, err, test.ShouldBeNil)
		err = addPoints(dupOctree, pts)
		test.That(t, err, test.ShouldBeNil)
		equal := dupOctree.AlmostEqual(geom)
		test.That(t, equal, test.ShouldBeTrue)
	}

	t.Run("identity transform", func(t *testing.T) {
		expected := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(2)},
			{P: r3.Vector{X: 1, Y: 0, Z: 0}, D: NewValueData(3)},
			{P: r3.Vector{X: 1, Y: 1, Z: 1}, D: NewValueData(5)},
		}
		movedOctree := octree.Transform(spatialmath.NewZeroPose())
		checkExpectedPoints(movedOctree, expected)
	})

	t.Run("translate XY", func(t *testing.T) {
		expected := []PointAndData{
			{P: r3.Vector{X: -3, Y: 5, Z: 0}, D: NewValueData(2)},
			{P: r3.Vector{X: -2, Y: 5, Z: 0}, D: NewValueData(3)},
			{P: r3.Vector{X: -2, Y: 6, Z: 1}, D: NewValueData(5)},
		}
		movedOctree := octree.Transform(spatialmath.NewPoseFromPoint(r3.Vector{-3, 5, 0}))
		checkExpectedPoints(movedOctree, expected)
	})

	t.Run("rotate", func(t *testing.T) {
		expected := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(2)},
			{P: r3.Vector{X: -1, Y: 0, Z: 0}, D: NewValueData(3)},
			{P: r3.Vector{X: -1, Y: 1, Z: -1}, D: NewValueData(5)},
		}
		movedOctree := octree.Transform(spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVector{OZ: -1}))
		checkExpectedPoints(movedOctree, expected)
	})

	t.Run("rotate and translate", func(t *testing.T) {
		expected := []PointAndData{
			{P: r3.Vector{X: -10, Y: 5, Z: 10}, D: NewValueData(2)},
			{P: r3.Vector{X: -11, Y: 5, Z: 10}, D: NewValueData(3)},
			{P: r3.Vector{X: -11, Y: 6, Z: 9}, D: NewValueData(5)},
		}
		movedOctree := octree.Transform(spatialmath.NewPose(
			r3.Vector{-10, 5, 10},
			&spatialmath.OrientationVector{OZ: -1},
		))
		checkExpectedPoints(movedOctree, expected)
	})

	t.Run("rotate and translate twice", func(t *testing.T) {
		expected := []PointAndData{
			{P: r3.Vector{X: -35, Y: 60, Z: 110}, D: NewValueData(2)},
			{P: r3.Vector{X: -35, Y: 60, Z: 111}, D: NewValueData(3)},
			{P: r3.Vector{X: -36, Y: 59, Z: 111}, D: NewValueData(5)},
		}
		movedOctree1 := octree.Transform(spatialmath.NewPose(
			r3.Vector{-10, 5, 10},
			&spatialmath.OrientationVector{OZ: -1},
		))
		movedOctree2 := movedOctree1.Transform(spatialmath.NewPose(
			r3.Vector{-30, 50, 100},
			&spatialmath.OrientationVector{OY: 1},
		))
		checkExpectedPoints(movedOctree2, expected)
	})
}

func TestBasicOctreeAlmostEqual(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0

	octree, err := createNewOctree(center, side)
	test.That(t, err, test.ShouldBeNil)
	pointsAndData := []PointAndData{
		{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(2)},
		{P: r3.Vector{X: 1, Y: 0, Z: 0}, D: NewValueData(3)},
		{P: r3.Vector{X: 1, Y: 1, Z: 1}, D: NewValueData(5)},
	}
	err = addPoints(octree, pointsAndData)
	test.That(t, err, test.ShouldBeNil)

	equal := octree.AlmostEqual(octree)
	test.That(t, equal, test.ShouldBeTrue)

	movedOctree := octree.Transform(spatialmath.NewZeroPose())
	// Confirm that an identity transform adjusts the side length but still yields equality
	movedOctreeReal, ok := movedOctree.(*BasicOctree)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, movedOctreeReal.sideLength, test.ShouldNotEqual, octree.sideLength)
	equal = octree.AlmostEqual(movedOctree)
	test.That(t, equal, test.ShouldBeTrue)

	octree.Set(r3.Vector{-1, -1, -1}, NewValueData(999))
	equal = octree.AlmostEqual(movedOctree)
	test.That(t, equal, test.ShouldBeFalse)

	movedOctree = octree.Transform(spatialmath.NewPoseFromPoint(r3.Vector{-3, 5, 0}))
	equal = octree.AlmostEqual(movedOctree)
	test.That(t, equal, test.ShouldBeFalse)
}
