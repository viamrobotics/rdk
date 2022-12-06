package octree

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	pc "go.viam.com/rdk/pointcloud"
)

// Creates a new LeafNodeEmpty.
func newLeafNodeEmpty() basicOctreeNode {
	octNode := basicOctreeNode{
		children: nil,
		nodeType: LeafNodeEmpty,
		point:    pc.PointAndData{},
	}
	return octNode
}

// Creates a new InternalNode with specified children nodes.
func newInternalNode(tree []*basicOctree) basicOctreeNode {
	octNode := basicOctreeNode{
		children: tree,
		nodeType: InternalNode,
		point:    pc.PointAndData{},
	}
	return octNode
}

// Creates a new LeafNodeFilled and stores specified position and data.
func newLeafNodeFilled(p r3.Vector, d pc.Data) basicOctreeNode {
	octNode := basicOctreeNode{
		children: nil,
		nodeType: LeafNodeFilled,
		point:    pc.PointAndData{P: p, D: d},
	}
	return octNode
}

// Splits a basic octree into multiple octants and will place any stored point in appropriate child
// node. Note: splitIntoOctants should only be called when an octree is a LeafNodeFilled.
func (octree *basicOctree) splitIntoOctants() error {
	switch octree.node.nodeType {
	case InternalNode:
		return errors.New("error attempted to split internal node")
	case LeafNodeEmpty:
		return errors.New("error attempted to split empty leaf node")
	case LeafNodeFilled:

		children := []*basicOctree{}
		newSideLength := octree.sideLength / 2
		for _, i := range []float64{-1.0, 1.0} {
			for _, j := range []float64{-1.0, 1.0} {
				for _, k := range []float64{-1.0, 1.0} {
					centerOffset := r3.Vector{
						X: i * newSideLength / 2.,
						Y: j * newSideLength / 2.,
						Z: k * newSideLength / 2.,
					}
					newCenter := octree.center.Add(centerOffset)

					// Create a new basic octree child
					child := &basicOctree{
						center:     newCenter,
						sideLength: newSideLength,
						size:       0,
						logger:     octree.logger,
						node:       newLeafNodeEmpty(),
						meta:       pc.NewMetaData(),
					}
					children = append(children, child)
				}
			}
		}

		// Extract data before redefining node as InternalNode with eight new children nodes
		p := octree.node.point.P
		d := octree.node.point.D
		octree.node = newInternalNode(children)
		octree.meta = pc.NewMetaData()
		octree.size = 0
		return octree.Set(p, d)
	}
	return errors.Errorf("error attempted to split invalid node type (%v)", octree.node.nodeType)
}

// Checks that a point should be inside a basic octree based on its center and defined side length.
func (octree *basicOctree) checkPointPlacement(p r3.Vector) bool {
	return ((math.Abs(octree.center.X-p.X) <= octree.sideLength/2.) &&
		(math.Abs(octree.center.Y-p.Y) <= octree.sideLength/2.) &&
		(math.Abs(octree.center.Z-p.Z) <= octree.sideLength/2.))
}

func stringBasicOctreeNodeType2(n NodeType) string {
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

// iterateRecursive is a helper function for iterating through a basic octree. If an internal node is found it will be
// called recursively after updating the idx value to correspond to the id of the child node. If a leaf node with a point
// is found and the myBatch number matches the idx%numBatches then the function will be performed on the point and
// associated data. If the function returns false, the iteration will end.
func (octree *basicOctree) iterateRecursive(numBatches, currentBatch int, idx uint, fn func(p r3.Vector, d pc.Data) bool) (uint, bool) {
	currIdx := idx
	ok := true
	switch octree.node.nodeType {
	case InternalNode:
		for _, child := range octree.node.children {
			if idx, ok = child.iterateRecursive(numBatches, currentBatch, currIdx, fn); !ok {
				ok = false
				break
			}
			currIdx += uint(child.size)
		}

	case LeafNodeFilled:
		fmt.Printf("%v\n", currIdx)
		if numBatches == 0 || idx%uint(numBatches) == uint(currentBatch) {
			ok = fn(octree.node.point.P, octree.node.point.D)
		}

	case LeafNodeEmpty:
	}

	return idx, ok
}
