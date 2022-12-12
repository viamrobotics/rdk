package octree

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	pc "go.viam.com/rdk/pointcloud"
)

const (
	// This value allows for high level of granularity in the octree while still allowing for fast access times
	// even on a pi.
	maxRecursionDepth = 1000
)

// basicOctree is a data structure that represents a basic octree structure with information regarding center
// point, side length and node data.
type basicOctree struct {
	logger     golog.Logger
	node       basicOctreeNode
	center     r3.Vector
	sideLength float64
	size       int
	meta       pc.MetaData
}

// basicOctreeNode is a struct comprised of the type of node, children nodes (should they exist) and the pointcloud's
// PointAndData datatype representing a point in space.
type basicOctreeNode struct {
	nodeType NodeType
	children []*basicOctree
	point    pc.PointAndData
}

// New creates a new basic octree with specified center, side and metadata.
func New(ctx context.Context, center r3.Vector, sideLength float64, logger golog.Logger) (Octree, error) {
	if sideLength <= 0 {
		return nil, errors.Errorf("invalid side length (%.2f) for octree", sideLength)
	}

	octree := &basicOctree{
		logger:     logger,
		node:       newLeafNodeEmpty(),
		center:     center,
		sideLength: sideLength,
		size:       0,
		meta:       pc.NewMetaData(),
	}

	return octree, nil
}

// Size returns the number of points stored in the octree's metadata.
func (octree *basicOctree) Size() int {
	return octree.size
}

// Set recursively iterates through a basic octree, attempting to add a given point and data to the tree after
// ensuring it falls within the bounds of the given basic octree.
func (octree *basicOctree) Set(p r3.Vector, d pc.Data) error {
	return octree.helperSet(p, d, 0)
}

// At traverses a basic octree to see if a point exists at the specified location. If a point does exist, its data
// is returned along with true. If a point does not exist, no data is returned and the boolean is returned false.
func (octree *basicOctree) At(x, y, z float64) (pc.Data, bool) {
	// Check if point could exist in octree given bounds
	if !octree.checkPointPlacement(r3.Vector{X: x, Y: y, Z: z}) {
		return nil, false
	}

	switch octree.node.nodeType {
	case InternalNode:
		for _, child := range octree.node.children {
			d, exists := child.At(x, y, z)
			if exists {
				return d, true
			}
		}

	case LeafNodeFilled:
		if octree.node.point.P.ApproxEqual(r3.Vector{X: x, Y: y, Z: z}) {
			return octree.node.point.D, true
		}

	case LeafNodeEmpty:
	}

	return nil, false
}

// Iterate is a batchable process that will go through a basic octree and applies a specified function
// to either all the data points or a subset of them based on the given numBatches and currentBatch
// inputs. If any of the applied functions returns a false value, iteration will stop and no further
// points will be processed.
func (octree *basicOctree) Iterate(numBatches, currentBatch int, fn func(p r3.Vector, d pc.Data) bool) {
	if numBatches < 0 || currentBatch < 0 || (numBatches > 0 && currentBatch >= numBatches) {
		return
	}

	lowerBound := 0
	upperBound := octree.Size()

	if numBatches > 0 {
		batchSize := (octree.Size() + numBatches - 1) / numBatches
		lowerBound = currentBatch * batchSize
		upperBound = (currentBatch + 1) * batchSize
	}
	if upperBound > octree.Size() {
		upperBound = octree.Size()
	}

	octree.helperIterate(lowerBound, upperBound, 0, fn)
}

// MarshalOctree TODO: Implement marshalling for octree.
func (octree *basicOctree) MarshalOctree() ([]byte, error) {
	return nil, nil
}

// Metadata returns the metadata of the pointcloud stored in the octree.
func (octree *basicOctree) MetaData() pc.MetaData {
	return octree.meta
}
