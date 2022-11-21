package octree

import (
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	pc "go.viam.com/rdk/pointcloud"
)

// Creates a new LeafNodeEmpty.
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

// Creates a new LeafNodeFilled and stores specified position and data.
func newLeafNodeFilled(p r3.Vector, d pc.Data) basicOctreeNode {
	octNode := basicOctreeNode{
		tree:     nil,
		nodeType: LeafNodeFilled,
		point:    pc.PointAndData{P: p, D: d},
	}
	return octNode
}

// Splits a basic octree into multiple octants and will place any stored point in appropriate child
// node. Note: splitOctants should only be called when an octree is a LeafNodeFilled.
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
					center: newCenter,
					side:   newSideLength,
					logger: octree.logger,
					node:   newLeafNodeEmpty(),
					meta:   octree.newMetaData(),
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
	octree.meta = octree.newMetaData()
	return octree.Set(p, d)
}

// Checks that a point should be inside a basic octree based on its center and defined side length.
func checkPointPlacement(center r3.Vector, sideLength float64, p r3.Vector) bool {
	return ((math.Abs(center.X-p.X) <= sideLength) &&
		(math.Abs(center.Y-p.Y) <= sideLength) &&
		(math.Abs(center.Z-p.Z) <= sideLength))
}
