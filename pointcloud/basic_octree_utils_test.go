package pointcloud

import (
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
)

// Test creation of empty leaf node, filled leaf node and internal node.
func TestNodeCreation(t *testing.T) {
	t.Run("Create empty leaf node", func(t *testing.T) {
		basicOct := newLeafNodeEmpty()

		test.That(t, basicOct.nodeType, test.ShouldResemble, leafNodeEmpty)
		test.That(t, basicOct.point, test.ShouldResemble, PointAndData{})
		test.That(t, basicOct.children, test.ShouldBeNil)
	})

	t.Run("Create filled leaf node", func(t *testing.T) {
		p := r3.Vector{X: 0, Y: 0, Z: 0}
		d := NewValueData(1.0)
		basicOct := newLeafNodeFilled(p, d)

		test.That(t, basicOct.nodeType, test.ShouldResemble, leafNodeFilled)
		test.That(t, basicOct.point, test.ShouldResemble, PointAndData{P: p, D: d})
		test.That(t, basicOct.children, test.ShouldBeNil)
	})

	t.Run("Create internal node", func(t *testing.T) {
		var children []*BasicOctree
		basicOct := newInternalNode(children)

		test.That(t, basicOct.nodeType, test.ShouldResemble, internalNode)
		test.That(t, basicOct.point, test.ShouldResemble, PointAndData{})
		test.That(t, basicOct.children, test.ShouldResemble, children)
	})
}

// Tests that splitting a filled octree node results in seven empty leaf nodes and
// one filled one as well as the splitting of an empty octree node will result in
// eight empty children nodes.
func TestSplitIntoOctants(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 1.0

	t.Run("Splitting empty octree node into octants", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.splitIntoOctants()
		test.That(t, err, test.ShouldBeError, errors.New("error attempted to split empty leaf node"))
	})

	t.Run("Splitting filled basic octree node into octants", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(1)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.splitIntoOctants()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(basicOct.node.children), test.ShouldEqual, 8)
		test.That(t, basicOct.node.nodeType, test.ShouldResemble, internalNode)
		test.That(t, basicOct.node.point, test.ShouldResemble, PointAndData{})

		filledLeaves := []*BasicOctree{}
		emptyLeaves := []*BasicOctree{}
		internalLeaves := []*BasicOctree{}

		for _, child := range basicOct.node.children {
			switch child.node.nodeType {
			case leafNodeFilled:
				filledLeaves = append(filledLeaves, child)
			case leafNodeEmpty:
				emptyLeaves = append(emptyLeaves, child)
			case internalNode:
				internalLeaves = append(internalLeaves, child)
			}
		}
		test.That(t, len(filledLeaves), test.ShouldEqual, 1)
		test.That(t, len(emptyLeaves), test.ShouldEqual, 7)
		test.That(t, len(internalLeaves), test.ShouldEqual, 0)
		test.That(t, filledLeaves[0].checkPointPlacement(pointsAndData[0].P), test.ShouldBeTrue)

		checkPoints(t, basicOct, pointsAndData)

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Splitting internal basic octree node with point into octants", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		pointsAndData := []PointAndData{
			{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(1)},
			{P: r3.Vector{X: .5, Y: 0, Z: 0}, D: NewValueData(2)},
		}

		err = addPoints(basicOct, pointsAndData)
		test.That(t, err, test.ShouldBeNil)

		checkPoints(t, basicOct, pointsAndData)

		d, ok := basicOct.At(0, 1, .5)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, d, test.ShouldBeNil)

		err = basicOct.splitIntoOctants()
		test.That(t, err, test.ShouldBeError, errors.New("error attempted to split internal node"))
	})

	t.Run("Splitting invalid basic octree node", func(t *testing.T) {
		basicOct, err := createNewOctree(center, side)
		test.That(t, err, test.ShouldBeNil)

		basicOct.node = newLeafNodeFilled(r3.Vector{X: 0, Y: 0, Z: 10}, NewValueData(1.0))
		err = basicOct.splitIntoOctants()
		test.That(t, err, test.ShouldBeError, errors.New("error point is outside the bounds of this octree"))

		basicOct.node = newLeafNodeFilled(r3.Vector{X: 0, Y: 0, Z: 10}, NewValueData(1.0))
		err1 := basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, NewValueData(1.0))
		test.That(t, err1, test.ShouldBeError, errors.Errorf("error in splitting octree into new octants: %v", err))
	})
}

// Test the function responsible for checking if the specified point will fit in the octree given its center and side.
func TestCheckPointPlacement(t *testing.T) {
	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0

	basicOct, err := createNewOctree(center, side)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, basicOct.checkPointPlacement(r3.Vector{X: 0, Y: 0, Z: 0}), test.ShouldBeTrue)
	test.That(t, basicOct.checkPointPlacement(r3.Vector{X: .25, Y: .25, Z: .25}), test.ShouldBeTrue)
	test.That(t, basicOct.checkPointPlacement(r3.Vector{X: .5, Y: .5, Z: .5}), test.ShouldBeTrue)
	test.That(t, basicOct.checkPointPlacement(r3.Vector{X: 1.01, Y: 0, Z: 0}), test.ShouldBeFalse)
	test.That(t, basicOct.checkPointPlacement(r3.Vector{X: 1.00, Y: 1.01, Z: 0}), test.ShouldBeFalse)
	test.That(t, basicOct.checkPointPlacement(r3.Vector{X: 0.99, Y: 0, Z: 1.01}), test.ShouldBeFalse)
	test.That(t, basicOct.checkPointPlacement(r3.Vector{X: 2, Y: 0, Z: 0}), test.ShouldBeFalse)
	test.That(t, basicOct.checkPointPlacement(r3.Vector{X: -1000, Y: 0, Z: 0}), test.ShouldBeFalse)

	center = r3.Vector{X: 1000, Y: -1000, Z: 10}
	side = 24.0

	basicOct, err = createNewOctree(center, side)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, basicOct.checkPointPlacement(r3.Vector{X: 1000, Y: -1000, Z: 5}), test.ShouldBeTrue)
	test.That(t, basicOct.checkPointPlacement(r3.Vector{X: 1000, Y: -994, Z: .5}), test.ShouldBeTrue)
	test.That(t, basicOct.checkPointPlacement(r3.Vector{X: -1000, Y: 0, Z: 0}), test.ShouldBeFalse)

	validateBasicOctree(t, basicOct, center, side)
}

// Helper function that recursively checks a basic octree's structure and metadata.
func validateBasicOctree(t *testing.T, bOct *BasicOctree, center r3.Vector, sideLength float64) int {
	t.Helper()

	test.That(t, sideLength, test.ShouldEqual, bOct.sideLength)
	test.That(t, center, test.ShouldResemble, bOct.center)

	validateMetadata(t, bOct)

	var size int
	switch bOct.node.nodeType {
	case internalNode:
		test.That(t, len(bOct.node.children), test.ShouldEqual, 8)
		test.That(t, bOct.node.point, test.ShouldResemble, PointAndData{})

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

			size += validateBasicOctree(t, child, r3.Vector{
				X: center.X + i*sideLength/4.,
				Y: center.Y + j*sideLength/4.,
				Z: center.Z + k*sideLength/4.,
			}, sideLength/2.)
		}
		test.That(t, size, test.ShouldEqual, bOct.size)
		test.That(t, numInternalNodes+numLeafNodeEmptyNodes+numLeafNodeFilledNodes, test.ShouldEqual, 8)
	case leafNodeFilled:
		test.That(t, len(bOct.node.children), test.ShouldEqual, 0)
		test.That(t, bOct.node.point, test.ShouldNotResemble, PointAndData{})
		test.That(t, bOct.checkPointPlacement(bOct.node.point.P), test.ShouldBeTrue)
		test.That(t, bOct.size, test.ShouldEqual, 1)
		size = bOct.size
	case leafNodeEmpty:
		test.That(t, len(bOct.node.children), test.ShouldEqual, 0)
		test.That(t, bOct.node.point, test.ShouldResemble, PointAndData{})
		test.That(t, bOct.size, test.ShouldEqual, 0)
		size = bOct.size
	}
	return size
}

// Helper function for checking basic octree metadata.
func validateMetadata(t *testing.T, bOct *BasicOctree) {
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
	test.That(t, bOct.meta.TotalX(), test.ShouldAlmostEqual, metadata.TotalX())
	test.That(t, bOct.meta.TotalY(), test.ShouldAlmostEqual, metadata.TotalY())
	test.That(t, bOct.meta.TotalZ(), test.ShouldAlmostEqual, metadata.TotalZ())
}

// Helper function to create lopsided octree for testing of recursion depth limit.
func createLopsidedOctree(oct *BasicOctree, i, max int) *BasicOctree {
	if i >= max {
		return oct
	}

	children := []*BasicOctree{}
	newSideLength := oct.sideLength / 2
	for _, i := range []float64{-1.0, 1.0} {
		for _, j := range []float64{-1.0, 1.0} {
			for _, k := range []float64{-1.0, 1.0} {
				centerOffset := r3.Vector{
					X: i * newSideLength / 2.,
					Y: j * newSideLength / 2.,
					Z: k * newSideLength / 2.,
				}
				newCenter := oct.center.Add(centerOffset)

				// Create a new basic octree child
				child := &BasicOctree{
					center:     newCenter,
					sideLength: newSideLength,
					size:       0,
					node:       newLeafNodeEmpty(),
					meta:       NewMetaData(),
				}
				children = append(children, child)
			}
		}
	}
	oct.node = newInternalNode(children)
	oct.node.children[0] = createLopsidedOctree(oct.node.children[0], i+1, max)
	return oct
}

// Helper functions for visualizing octree during testing
//
//nolint:unused
func stringBasicOctreeNodeType(n NodeType) string {
	switch n {
	case internalNode:
		return "InternalNode"
	case leafNodeEmpty:
		return "LeafNodeEmpty"
	case leafNodeFilled:
		return "LeafNodeFilled"
	}
	return ""
}

//nolint:unused
func printBasicOctree(logger golog.Logger, bOct *BasicOctree, s string) {
	logger.Infof("%v %e %e %e - %v | Children: %v Side: %v Size: %v\n", s,
		bOct.center.X, bOct.center.Y, bOct.center.Z,
		stringBasicOctreeNodeType(bOct.node.nodeType), len(bOct.node.children), bOct.sideLength, bOct.size)

	if bOct.node.nodeType == leafNodeFilled {
		logger.Infof("%s (%e %e %e) - Val: %v\n", s,
			bOct.node.point.P.X, bOct.node.point.P.Y, bOct.node.point.P.Z, bOct.node.point.D.Value())
	}
	for _, v := range bOct.node.children {
		printBasicOctree(logger, v, s+"-+-")
	}
}
