package octree

import (
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	pc "go.viam.com/rdk/pointcloud"
)

// Creates a new EmptyLeafNode.
func newLeafNodeEmpty() basicOctreeNode {
	octNode := basicOctreeNode{
		tree:     nil,
		nodeType: LeafNodeEmpty,
		point:    pc.PointAndData{},
	}
	return octNode
}

// Creates a new InternalNode with specified children nodes.
func newInternalNode(tree []*basicOctree) basicOctreeNode {
	octNode := basicOctreeNode{
		tree:     tree,
		nodeType: InternalNode,
		point:    pc.PointAndData{},
	}
	return octNode
}

// Creates a new ChildLeafNode and stores specified position and data.
func newLeafNodeFilled(p r3.Vector, d pc.Data) basicOctreeNode {
	octNode := basicOctreeNode{
		tree:     nil,
		nodeType: LeafNodeFilled,
		point:    pc.PointAndData{P: p, D: d},
	}
	return octNode
}

// Splits the octree into multiple octants and place stored point in appropriate child
// Note: splitOctants should only be called when an octree is a ChildLeafNode.
func (octree *basicOctree) splitIntoOctants() error {
	if octree.node.nodeType == InternalNode {
		return errors.New("error attempted to split internal node")
	}

	children := []*basicOctree{}
	newSideLength := octree.side / 2
	for _, i := range []float64{-1.0, 1.0} {
		for _, j := range []float64{-1.0, 1.0} {
			for _, k := range []float64{-1.0, 1.0} {
				centerOffset := r3.Vector{
					X: i * newSideLength,
					Y: j * newSideLength,
					Z: k * newSideLength,
				}
				newCenter := octree.center.Add(centerOffset)

				// Create new basic octree children
				child := &basicOctree{
					ctx:    octree.ctx,
					center: newCenter,
					side:   newSideLength,
					logger: octree.logger,
					node:   newLeafNodeEmpty(),
					meta:   octree.NewMetaData(),
				}
				children = append(children, child)
			}
		}
	}

	// Extract potential data before redefining node as InternalNode with eight new children
	// nodes
	p := octree.node.point.P
	d := octree.node.point.D
	octree.node = newInternalNode(children)
	octree.meta = octree.NewMetaData()
	return octree.Set(p, d)
}

// Checks that point should be inside octree based on octree center and defined side length.
func checkPointPlacement(center r3.Vector, sideLength float64, p r3.Vector) bool {
	return ((math.Abs(center.X-p.X) <= sideLength) &&
		(math.Abs(center.Y-p.Y) <= sideLength) &&
		(math.Abs(center.Z-p.Z) <= sideLength))
}

// iterateRecursive is a helper function for iterating through a basic octree. If an internal node is found it will be
// called recursively after updating the idx value to correspond to the id of the child node. If a leaf node with a point
// is found and the myBatch number matches the idx%numBatches then the function will be performed on the point and
// associated data. If the function returns false, the iteration will end.
func (octree *basicOctree) iterateRecursive(numBatches, myBatch, idx int, fn func(p r3.Vector, d pc.Data) bool) (int, bool) {
	ok := true
	switch octree.node.nodeType {
	case InternalNode:
		for _, child := range octree.node.tree {
			idx, ok = child.iterateRecursive(numBatches, myBatch, idx+1, fn)
			if !ok {
				ok = false
				break
			}
		}

	case LeafNodeFilled:
		if numBatches == 0 || idx%numBatches == myBatch {
			ok = fn(octree.node.point.P, octree.node.point.D)
		}

	case LeafNodeEmpty:
	}

	return idx, ok
}
