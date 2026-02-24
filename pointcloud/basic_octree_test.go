package pointcloud

import (
	"math"
	"path/filepath"
	"sync"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/spatialmath"
)

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
func makeFullPointCloudFromArtifact(t *testing.T, artifactPath, pcType string) (PointCloud, error) {
	t.Helper()

	path := filepath.Clean(artifact.MustPath(artifactPath))
	return NewFromFile(path, pcType)
}

// Test the creation of new basic octrees.
func TestBasicOctreeNew(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	sideValid := 1.0
	basicOct := newBasicOctree(center, sideValid, defaultConfidenceThreshold)

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
	d1 := 99
	d2 := 100
	d3 := 75
	d4 := 10

	t.Run("Set point into empty leaf node into basic octree", func(t *testing.T) {
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)
		test.That(t, basicOct.Size(), test.ShouldEqual, 0)

		point1 := r3.Vector{X: 0.1, Y: 0, Z: 0}
		data1 := NewValueData(1)
		err := basicOct.Set(point1, data1)
		test.That(t, err, test.ShouldBeNil)
		node := newLeafNodeFilled(point1, data1)
		test.That(t, basicOct.node, test.ShouldResemble, node)
		test.That(t, basicOct.Size(), test.ShouldEqual, 1)

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Set point into filled leaf node into basic octree", func(t *testing.T) {
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		err := basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(d1))
		test.That(t, err, test.ShouldBeNil)
		mp := basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, d1)

		err = basicOct.Set(r3.Vector{X: -.5, Y: 0, Z: 0}, NewValueData(d2))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.node.nodeType, test.ShouldResemble, internalNode)
		test.That(t, basicOct.Size(), test.ShouldEqual, side)
		mp = basicOct.node.children[0].MaxVal()
		test.That(t, mp, test.ShouldEqual, int(math.Max(float64(d1), float64(d2))))
		mp = basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, int(math.Max(float64(d1), float64(d2))))

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Set point into internal node node into basic octree", func(t *testing.T) {
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		err := basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(d3))
		test.That(t, err, test.ShouldBeNil)
		mp := basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, d3)

		err = basicOct.Set(r3.Vector{X: -.5, Y: 0, Z: 0}, NewValueData(d2))
		test.That(t, err, test.ShouldBeNil)
		mp = basicOct.node.children[0].MaxVal()
		test.That(t, mp, test.ShouldEqual, int(math.Max(float64(d2), float64(d3))))

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
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		err := basicOct.Set(r3.Vector{X: 2, Y: 0, Z: 0}, NewValueData(1))
		test.That(t, err, test.ShouldBeError, errors.New("error point is outside the bounds of this octree"))

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Set point at intersection of multiple basic octree nodes", func(t *testing.T) {
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		err := basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(d1))
		test.That(t, err, test.ShouldBeNil)
		mp := basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, d1)

		err = basicOct.Set(r3.Vector{X: -.5, Y: 0, Z: 0}, NewValueData(d2))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.size, test.ShouldEqual, 2)
		mp = basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, int(math.Max(float64(d1), float64(d2))))

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Set same point with new data in basic octree", func(t *testing.T) {
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		err := basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(d1))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.node.point.D.Value(), test.ShouldEqual, d1)
		mp := basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, d1)

		err = basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(d2))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, basicOct.node.point.D.Value(), test.ShouldEqual, d2)
		test.That(t, basicOct.Size(), test.ShouldEqual, 1)
		mp = basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, int(math.Max(float64(d1), float64(d2))))

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Set point into invalid internal node", func(t *testing.T) {
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		basicOct.node = newInternalNode([]*BasicOctree{})
		err := basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(1))
		test.That(t, err, test.ShouldBeError, errors.New("error invalid internal node detected, please check your tree"))
	})

	t.Run("Set point into invalid internal node", func(t *testing.T) {
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		basicOct.node = newInternalNode([]*BasicOctree{})
		err := basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(1))
		test.That(t, err, test.ShouldBeError, errors.New("error invalid internal node detected, please check your tree"))
	})

	t.Run("Set point, hit max recursion depth", func(t *testing.T) {
		side = 2.0
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		basicOct = createLopsidedOctree(basicOct, 0, maxRecursionDepth-1)

		err := basicOct.Set(r3.Vector{X: -1, Y: -1, Z: -1}, NewValueData(d1))
		test.That(t, err, test.ShouldBeNil)
		mp := basicOct.MaxVal()
		test.That(t, mp, test.ShouldEqual, d1)

		basicOct = createLopsidedOctree(basicOct, 0, maxRecursionDepth)
		err = basicOct.Set(r3.Vector{X: -1, Y: -1, Z: -1}, NewBasicData())
		test.That(t, err, test.ShouldBeError, errors.New("error max allowable recursion depth reached"))
	})

	t.Run("Set empty data point", func(t *testing.T) {
		side = 1.
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		pointAndData := PointAndData{}

		err := basicOct.Set(pointAndData.P, pointAndData.D)
		test.That(t, err, test.ShouldBeNil)
	})
}

// Test the At() function for basic octrees which returns the data at a specific location should it exist.
func TestBasicOctreeAt(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0

	t.Run("At check of single node basic octree", func(t *testing.T) {
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(1)},
		}

		err := addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		checkPoints(t, basicOct, pointsAndData)

		d, ok := basicOct.At(0.0001, 0, 0)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, d, test.ShouldBeNil)

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("At check of multi level basic octree", func(t *testing.T) {
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(51)},
			{P: r3.Vector{X: -.5, Y: 0, Z: 0}, D: NewValueData(52)},
			{P: r3.Vector{X: -0.4, Y: 0, Z: 0}, D: NewValueData(53)},
		}

		err := addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		checkPoints(t, basicOct, pointsAndData)

		d, ok := basicOct.At(-.6, 0, 0)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, d, test.ShouldBeNil)

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("At check of empty basic octree", func(t *testing.T) {
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		d, ok := basicOct.At(0, 0, 0)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, d, test.ShouldBeNil)

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("At check of point outside octree bounds", func(t *testing.T) {
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

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
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		total := 0
		basicOct.Iterate(0, 0, func(p r3.Vector, d Data) bool {
			total += d.Value()
			return true
		})
		test.That(t, total, test.ShouldEqual, 0)

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Iterate zero batch check of a filled basic octree", func(t *testing.T) {
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(2)},
		}

		err := addPoints(basicOct, pointsAndData)
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
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(50)},
			{P: r3.Vector{X: .5, Y: 0, Z: 0}, D: NewValueData(51)},
			{P: r3.Vector{X: .6, Y: 0, Z: 0}, D: NewValueData(50)},
		}

		err := addPoints(basicOct, pointsAndData)
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
			if d.Value() == pointsAndData[0].D.Value() {
				total += d.Value()
				return true
			}
			return false
		})
		test.That(t, total, test.ShouldEqual, pointsAndData[0].D.Value())

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Iterate non-zero batch check of an filled basic octree", func(t *testing.T) {
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(2)},
		}

		err := addPoints(basicOct, pointsAndData)
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
		basicOct := newBasicOctree(center, side, defaultConfidenceThreshold)

		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(1)},
			{P: r3.Vector{X: .5, Y: 0, Z: 0}, D: NewValueData(2)},
			{P: r3.Vector{X: .6, Y: 0, Z: 0}, D: NewValueData(3)},
		}

		err := addPoints(basicOct, pointsAndData)
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

	meta := startPC.MetaData()
	center := meta.Center()
	maxSideLength := meta.MaxSideLength()

	basicOct := newBasicOctree(center, maxSideLength, defaultConfidenceThreshold)

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

func createExampleOctree() (*BasicOctree, error) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0
	octree := newBasicOctree(center, side, defaultConfidenceThreshold)

	pointsAndData := []PointAndData{
		{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(52)},
		{P: r3.Vector{X: .5, Y: 0, Z: 0}, D: NewValueData(53)},
		{P: r3.Vector{X: .5, Y: 0, Z: .5}, D: NewValueData(60)},
		{P: r3.Vector{X: .5, Y: .5, Z: 0}, D: NewValueData(51)},
		{P: r3.Vector{X: .55, Y: .55, Z: 0}, D: NewValueData(54)},
		{P: r3.Vector{X: -.55, Y: -.55, Z: 0}, D: NewValueData(55)},
		{P: r3.Vector{X: .755, Y: .755, Z: 0}, D: NewValueData(56)},
	}

	err := addPoints(octree, pointsAndData)
	if err != nil {
		return nil, err
	}
	return octree, nil
}

func TestCachedMaxProbability(t *testing.T) {
	t.Run("get the max val from an octree", func(t *testing.T) {
		octree, err := createExampleOctree()
		test.That(t, err, test.ShouldBeNil)

		validateBasicOctree(t, octree, octree.center, octree.sideLength)

		mp := octree.MaxVal()
		test.That(t, mp, test.ShouldEqual, 60)

		mp = octree.node.children[0].MaxVal()
		test.That(t, mp, test.ShouldEqual, 55)
	})

	t.Run("cannot set arbitrary values into the octree", func(t *testing.T) {
		d := &basicData{value: 0, hasValue: false}
		node := newLeafNodeFilled(r3.Vector{1, 2, 3}, d)
		filledNode := basicOctreeNode{
			children:     nil,
			nodeType:     leafNodeFilled,
			point:        &PointAndData{P: r3.Vector{1, 2, 3}, D: d},
			maxVal:       defaultConfidenceThreshold,
			pointGeoOnce: &sync.Once{},
		}
		test.That(t, node, test.ShouldResemble, filledNode)
	})
}

// Test the various geometry-specific interface methods.
func TestBasicOctreeGeometryFunctions(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0

	octree := newBasicOctree(center, side, defaultConfidenceThreshold)
	pointsAndData := []PointAndData{
		{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(2)},
		{P: r3.Vector{X: 1, Y: 0, Z: 0}, D: NewValueData(3)},
		{P: r3.Vector{X: 1, Y: 1, Z: 1}, D: NewValueData(5)},
	}
	err := addPoints(octree, pointsAndData)
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

		dupOctree := newBasicOctree(pts[0].P, side, defaultConfidenceThreshold)
		err := addPoints(dupOctree, pts)
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

	octree := newBasicOctree(center, side, defaultConfidenceThreshold)
	pointsAndData := []PointAndData{
		{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(2)},
		{P: r3.Vector{X: 1, Y: 0, Z: 0}, D: NewValueData(3)},
		{P: r3.Vector{X: 1, Y: 1, Z: 1}, D: NewValueData(5)},
	}
	err := addPoints(octree, pointsAndData)
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

func TestBasicOctreePointsCollidingWith(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 20.0
	octree := newBasicOctree(center, side, 50) // confidence threshold of 50

	// Add points with different confidence values
	pointsAndData := []PointAndData{
		{P: r3.Vector{X: 1, Y: 1, Z: 1}, D: NewValueData(60)},    // above threshold
		{P: r3.Vector{X: 2, Y: 2, Z: 2}, D: NewValueData(40)},    // below threshold
		{P: r3.Vector{X: 3, Y: 3, Z: 3}, D: NewValueData(70)},    // above threshold
		{P: r3.Vector{X: -1, Y: -1, Z: -1}, D: NewValueData(80)}, // above threshold, outside box
		{P: r3.Vector{X: 10, Y: 10, Z: 10}, D: NewValueData(90)}, // above threshold, outside box
	}
	err := addPoints(octree, pointsAndData)
	test.That(t, err, test.ShouldBeNil)

	// Create a box that encompasses some points
	box, err := spatialmath.NewBox(
		spatialmath.NewPoseFromPoint(r3.Vector{X: 2, Y: 2, Z: 2}),
		r3.Vector{X: 4, Y: 4, Z: 4}, // box centered at (2,2,2), so extends from (0,0,0) to (4,4,4)
		"test_box",
	)
	test.That(t, err, test.ShouldBeNil)

	// Test with the box
	geometries := []spatialmath.Geometry{box}
	collidingPoints := octree.PointsCollidingWith(geometries, 0)

	// Should find points (1,1,1) and (3,3,3) since they're above threshold and within the box
	// Point (2,2,2) is below threshold so should be excluded
	// Point (-1,-1,-1) is above threshold but outside the box
	expectedPoints := []r3.Vector{
		{X: 1, Y: 1, Z: 1},
		{X: 3, Y: 3, Z: 3},
	}

	test.That(t, len(collidingPoints), test.ShouldEqual, len(expectedPoints))
	for _, expected := range expectedPoints {
		found := false
		for _, actual := range collidingPoints {
			if actual.Sub(expected).Norm() < 1e-6 {
				found = true
				break
			}
		}
		test.That(t, found, test.ShouldBeTrue)
	}
}

func TestBasicOctreePointsWithinRadius(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 10.0
	octree := newBasicOctree(center, side, 50) // confidence threshold of 50

	// Add points with different confidence values
	pointsAndData := []PointAndData{
		{P: r3.Vector{X: 1, Y: 0, Z: 0}, D: NewValueData(60)}, // distance 1 from origin, above threshold
		{P: r3.Vector{X: 2, Y: 0, Z: 0}, D: NewValueData(40)}, // distance 2 from origin, below threshold
		{P: r3.Vector{X: 0, Y: 3, Z: 0}, D: NewValueData(70)}, // distance 3 from origin, above threshold
		{P: r3.Vector{X: 0, Y: 0, Z: 5}, D: NewValueData(80)}, // distance 5 from origin, above threshold
	}
	err := addPoints(octree, pointsAndData)
	test.That(t, err, test.ShouldBeNil)

	// Test with radius of 2.5 from origin
	queryCenter := r3.Vector{X: 0, Y: 0, Z: 0}
	radius := 2.5
	pointsWithinRadius, err := octree.PointsWithinRadius(queryCenter, radius)
	test.That(t, err, test.ShouldBeNil)

	expectedPoints := []r3.Vector{
		{X: 1, Y: 0, Z: 0},
	}

	test.That(t, len(pointsWithinRadius), test.ShouldEqual, len(expectedPoints))
	for _, expected := range expectedPoints {
		found := false
		for _, actual := range pointsWithinRadius {
			if actual.Sub(expected).Norm() < 1e-6 {
				found = true
				break
			}
		}
		test.That(t, found, test.ShouldBeTrue)
	}

	// Test with larger radius
	radius = 4.0
	pointsWithinRadius, err = octree.PointsWithinRadius(queryCenter, radius)
	test.That(t, err, test.ShouldBeNil)

	// Should find points at distance 1 and distance 3 (both above threshold and within radius)
	expectedPoints = []r3.Vector{
		{X: 1, Y: 0, Z: 0}, // distance 1
		{X: 0, Y: 3, Z: 0}, // distance 3
	}

	test.That(t, len(pointsWithinRadius), test.ShouldEqual, len(expectedPoints))
	for _, expected := range expectedPoints {
		found := false
		for _, actual := range pointsWithinRadius {
			if actual.Sub(expected).Norm() < 1e-6 {
				found = true
				break
			}
		}
		test.That(t, found, test.ShouldBeTrue)
	}
}
