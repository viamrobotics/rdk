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

// Test creation of empty leaf node, filled leaf node and internal node.
func TestNodeCreation(t *testing.T) {
	t.Run("Create empty leaf node", func(t *testing.T) {
		basicOct := newLeafNodeEmpty()

		test.That(t, basicOct.nodeType, test.ShouldResemble, LeafNodeEmpty)
		test.That(t, basicOct.point, test.ShouldResemble, pc.PointAndData{})
		test.That(t, basicOct.children, test.ShouldBeNil)
	})

	t.Run("Create filled leaf node", func(t *testing.T) {
		p := r3.Vector{X: 0, Y: 0, Z: 0}
		d := pc.NewValueData(1.0)
		basicOct := newLeafNodeFilled(p, d)

		test.That(t, basicOct.nodeType, test.ShouldResemble, LeafNodeFilled)
		test.That(t, basicOct.point, test.ShouldResemble, pc.PointAndData{P: p, D: d})
		test.That(t, basicOct.children, test.ShouldBeNil)
	})

	t.Run("Create internal node", func(t *testing.T) {
		var children []*basicOctree
		basicOct := newInternalNode(children)

		test.That(t, basicOct.nodeType, test.ShouldResemble, InternalNode)
		test.That(t, basicOct.point, test.ShouldResemble, pc.PointAndData{})
		test.That(t, basicOct.children, test.ShouldResemble, children)
	})
}

// Tests that splitting a filled octree node results in seven empty leaf nodes and
// one filled one as well as the splitting of an empty octree node will result in
// eight empty children nodes.
func TestSplitIntoOctants(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 1.0

	t.Run("Splitting empty octree node into octants", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.splitIntoOctants()
		test.That(t, err, test.ShouldBeError, errors.New("error attempted to split empty leaf node"))
	})

	t.Run("Splitting filled basic octree node into octants", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		p := r3.Vector{X: 0, Y: 0, Z: 0}
		d := pc.NewValueData(1.0)

		err = basicOct.Set(p, d)
		test.That(t, err, test.ShouldBeNil)

		err = basicOct.splitIntoOctants()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(basicOct.node.children), test.ShouldEqual, 8)
		test.That(t, basicOct.node.nodeType, test.ShouldResemble, InternalNode)
		test.That(t, basicOct.node.point, test.ShouldResemble, pc.PointAndData{})

		filledLeaves := []*basicOctree{}
		emptyLeaves := []*basicOctree{}
		internalLeaves := []*basicOctree{}

		for _, child := range basicOct.node.children {
			switch child.node.nodeType {
			case LeafNodeFilled:
				filledLeaves = append(filledLeaves, child)
			case LeafNodeEmpty:
				emptyLeaves = append(emptyLeaves, child)
			case InternalNode:
				internalLeaves = append(internalLeaves, child)
			}
		}
		test.That(t, len(filledLeaves), test.ShouldEqual, 1)
		test.That(t, len(emptyLeaves), test.ShouldEqual, 7)
		test.That(t, len(internalLeaves), test.ShouldEqual, 0)
		test.That(t, filledLeaves[0].node.point, test.ShouldResemble, pc.PointAndData{P: p, D: d})
		test.That(t, filledLeaves[0].checkPointPlacement(p), test.ShouldBeTrue)

		validateBasicOctree(t, basicOct, center, side)
	})

	t.Run("Splitting internal basic octree node with point into octants", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		p1 := r3.Vector{X: 0, Y: 0, Z: 0}
		d1 := pc.NewValueData(1.0)
		basicOct.Set(p1, d1)
		test.That(t, err, test.ShouldBeNil)

		p2 := r3.Vector{X: .5, Y: 0, Z: 0}
		d2 := pc.NewValueData(2.0)
		basicOct.Set(p2, d2)
		test.That(t, err, test.ShouldBeNil)

		d, ok := basicOct.At(p1.X, p1.Y, p1.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, d1)

		d, ok = basicOct.At(p2.X, p2.Y, p2.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, d, test.ShouldResemble, d2)

		_, ok = basicOct.At(0, 1, .5)
		test.That(t, ok, test.ShouldBeFalse)

		err = basicOct.splitIntoOctants()
		test.That(t, err, test.ShouldBeError, errors.New("error attempted to split internal node"))
	})

	t.Run("Splitting invalid basic octree node", func(t *testing.T) {
		basicOct, err := createNewOctree(ctx, center, side, logger)
		test.That(t, err, test.ShouldBeNil)

		basicOct.node = newLeafNodeFilled(r3.Vector{X: 0, Y: 0, Z: 10}, pc.NewValueData(1.0))
		err = basicOct.splitIntoOctants()
		test.That(t, err, test.ShouldBeError, errors.New("error point is outside the bounds of this octree"))

		basicOct.node = newLeafNodeFilled(r3.Vector{X: 0, Y: 0, Z: 10}, pc.NewValueData(1.0))
		err1 := basicOct.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewValueData(1.0))
		test.That(t, err1, test.ShouldBeError, errors.Errorf("error in splitting octree into new octants: %v", err))
	})
}

// Test the function responsible for checking if the specified point will fit in the octree given its center and side.
func TestCheckPointPlacement(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	center := r3.Vector{X: 0, Y: 0, Z: 0}
	side := 2.0

	basicOct, err := createNewOctree(ctx, center, side, logger)
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

	basicOct, err = createNewOctree(ctx, center, side, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, basicOct.checkPointPlacement(r3.Vector{X: 1000, Y: -1000, Z: 5}), test.ShouldBeTrue)
	test.That(t, basicOct.checkPointPlacement(r3.Vector{X: 1000, Y: -994, Z: .5}), test.ShouldBeTrue)
	test.That(t, basicOct.checkPointPlacement(r3.Vector{X: -1000, Y: 0, Z: 0}), test.ShouldBeFalse)

	validateBasicOctree(t, basicOct, center, side)
}

// Helper function that recursively checks a basic octree's structure and metadata.
func validateBasicOctree(t *testing.T, bOct *basicOctree, center r3.Vector, sideLength float64) int32 {
	t.Helper()

	test.That(t, sideLength, test.ShouldEqual, bOct.sideLength)
	test.That(t, center, test.ShouldResemble, bOct.center)

	// TODO: add check of internal metadata once iterate function is available for easy looping over all points.

	var size int32

	switch bOct.node.nodeType {
	case InternalNode:
		test.That(t, len(bOct.node.children), test.ShouldEqual, 8)
		test.That(t, bOct.node.point, test.ShouldResemble, pc.PointAndData{})

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
			case InternalNode:
				numInternalNodes++
			case LeafNodeFilled:
				numLeafNodeFilledNodes++
			case LeafNodeEmpty:
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
	case LeafNodeFilled:
		test.That(t, len(bOct.node.children), test.ShouldEqual, 0)
		test.That(t, bOct.node.point, test.ShouldNotResemble, pc.PointAndData{})
		test.That(t, bOct.checkPointPlacement(bOct.node.point.P), test.ShouldBeTrue)
		test.That(t, bOct.size, test.ShouldEqual, 1)
		size = bOct.size
	case LeafNodeEmpty:
		test.That(t, len(bOct.node.children), test.ShouldEqual, 0)
		test.That(t, bOct.node.point, test.ShouldResemble, pc.PointAndData{})
		test.That(t, bOct.size, test.ShouldEqual, 0)
		size = bOct.size
	}
	return size
}

// Helper functions for visualizing octree during testing
//
//nolint:unused
func stringBasicOctreeNodeType(n NodeType) string {
	switch n {
	case InternalNode:
		return "InternalNode"
	case LeafNodeEmpty:
		return "LeafNodeEmpty"
	case LeafNodeFilled:
		return "LeafNodeFilled"
	}
	return ""
}

//nolint:unused
func printBasicOctree(bOct *basicOctree, s string) {
	bOct.logger.Infof("%v %.2f %.2f %.2f - %v | Children: %v Side: %v Size: %v\n", s,
		bOct.center.X, bOct.center.Y, bOct.center.Z,
		stringBasicOctreeNodeType(bOct.node.nodeType), len(bOct.node.children), bOct.sideLength, bOct.size)

	if bOct.node.nodeType == LeafNodeFilled {
		bOct.logger.Infof("%s (%.2f %.2f %.2f) - Val: %v\n", s,
			bOct.node.point.P.X, bOct.node.point.P.Y, bOct.node.point.P.Z, bOct.node.point.D.Value())
	}
	for _, v := range bOct.node.children {
		printBasicOctree(v, s+"-+-")
	}
}
