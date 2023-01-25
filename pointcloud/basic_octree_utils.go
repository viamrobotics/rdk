package pointcloud

import (
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
)

// Creates a new LeafNodeEmpty.
func newLeafNodeEmpty() basicOctreeNode {
	octNode := basicOctreeNode{
		children: nil,
		nodeType: leafNodeEmpty,
		point:    PointAndData{},
	}
	return octNode
}

// Creates a new InternalNode with specified children nodes.
func newInternalNode(tree []*BasicOctree) basicOctreeNode {
	octNode := basicOctreeNode{
		children: tree,
		nodeType: internalNode,
		point:    PointAndData{},
	}
	return octNode
}

// Creates a new LeafNodeFilled and stores specified position and data.
func newLeafNodeFilled(p r3.Vector, d Data) basicOctreeNode {
	octNode := basicOctreeNode{
		children: nil,
		nodeType: leafNodeFilled,
		point:    PointAndData{P: p, D: d},
	}
	return octNode
}

// Splits a basic octree into multiple octants and will place any stored point in appropriate child
// node. Note: splitIntoOctants should only be called when an octree is a LeafNodeFilled.
func (octree *BasicOctree) splitIntoOctants() error {
	switch octree.node.nodeType {
	case internalNode:
		return errors.New("error attempted to split internal node")
	case leafNodeEmpty:
		return errors.New("error attempted to split empty leaf node")
	case leafNodeFilled:

		children := []*BasicOctree{}
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

		// Extract data before redefining node as InternalNode with eight new children nodes
		p := octree.node.point.P
		d := octree.node.point.D
		octree.node = newInternalNode(children)
		octree.meta = NewMetaData()
		octree.size = 0
		return octree.Set(p, d)
	}
	return errors.Errorf("error attempted to split invalid node type (%v)", octree.node.nodeType)
}

// Checks that a point should be inside a basic octree based on its center and defined side length.
func (octree *BasicOctree) checkPointPlacement(p r3.Vector) bool {
	return ((math.Abs(octree.center.X-p.X) <= octree.sideLength/2.) &&
		(math.Abs(octree.center.Y-p.Y) <= octree.sideLength/2.) &&
		(math.Abs(octree.center.Z-p.Z) <= octree.sideLength/2.))
}

// helperSet is used by Set to recursive move through a basic octree while tracking recursion depth.
// If the maximum recursion depth is reached before a valid node is found for the point it will return
// an error.
func (octree *BasicOctree) helperSet(p r3.Vector, d Data, recursionDepth int) error {
	if recursionDepth >= maxRecursionDepth {
		return errors.New("error max allowable recursion depth reached")
	}
	if (PointAndData{P: p, D: d} == PointAndData{}) {
		return nil
	}

	if !octree.checkPointPlacement(p) {
		return errors.New("error point is outside the bounds of this octree")
	}

	switch octree.node.nodeType {
	case internalNode:
		for _, childNode := range octree.node.children {
			if childNode.checkPointPlacement(p) {
				err := childNode.helperSet(p, d, recursionDepth+1)
				if err == nil {
					// Update metadata
					octree.meta.Merge(p, d)
					octree.size++
				}
				return err
			}
		}
		return errors.New("error invalid internal node detected, please check your tree")

	case leafNodeFilled:
		if _, exists := octree.At(p.X, p.Y, p.Z); exists {
			// Update data in point
			octree.node.point.D = d
			return nil
		}
		if err := octree.splitIntoOctants(); err != nil {
			return errors.Errorf("error in splitting octree into new octants: %v", err)
		}
		// No update of metadata as the set call below will lead to the InternalNode case due to the octant split
		return octree.helperSet(p, d, recursionDepth+1)

	case leafNodeEmpty:
		// Update metadata
		octree.meta.Merge(p, d)
		octree.size++
		octree.node = newLeafNodeFilled(p, d)
	}

	return nil
}

// helperIterate is a recursive helper function for iterating through a basic octree that returns
// the result of the specified boolean function. Batching is done using the calculated upper and
// lower bounds and the tracking of the index, this allows for only a subset of the basic octree
// to be searched through. If the applied function ever returns false, the iteration will end.
func (octree *BasicOctree) helperIterate(lowerBound, upperBound, idx int, fn func(p r3.Vector, d Data) bool) bool {
	ok := true
	switch octree.node.nodeType {
	case internalNode:
		for _, child := range octree.node.children {
			numPoints := child.size

			if (idx+numPoints > lowerBound) && (idx < upperBound) {
				if ok = child.helperIterate(lowerBound, upperBound, idx, fn); !ok {
					break
				}
			}
			idx += numPoints
		}

	case leafNodeFilled:
		ok = fn(octree.node.point.P, octree.node.point.D)

	case leafNodeEmpty:
	}

	return ok
}

// Helper function for calculating the center of a pointcloud based on its metadata.
func getCenterFromPcMetaData(meta MetaData) r3.Vector {
	return r3.Vector{
		X: (meta.MaxX + meta.MinX) / 2,
		Y: (meta.MaxY + meta.MinY) / 2,
		Z: (meta.MaxZ + meta.MinZ) / 2,
	}
}

// Helper function for calculating the max side length of a pointcloud based on its metadata.
func getMaxSideLengthFromPcMetaData(meta MetaData) float64 {
	return math.Max((meta.MaxX - meta.MinX), math.Max((meta.MaxY-meta.MinY), (meta.MaxZ-meta.MinZ)))
}
