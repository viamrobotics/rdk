package octree

import (
	"context"
	"errors"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	pc "go.viam.com/rdk/pointcloud"
)

// octree data is the structure of an octree.
type basicOctree struct {
	ctx    context.Context
	logger golog.Logger
	node   basicOctreeNode
	center r3.Vector
	side   float64
	meta   MetaData
}

type basicOctreeNode struct {
	nodeType NodeType
	tree     []basicOctree
	point    pc.PointAndData
}

// New creates a new basic octree with specified center, side and metadata.
func New(ctx context.Context, center r3.Vector, sideLength float64, logger golog.Logger) (Octree, error) {
	if sideLength <= 0 {
		return nil, errors.New("invalid side length for octree")
	}

	octree := &basicOctree{
		ctx:    ctx,
		logger: logger,
		node:   newLeafNodeEmpty(),
		center: center,
		side:   sideLength,
	}
	octree.meta = octree.NewMetaData()
	return octree, nil
}

// Size returns the number of points stored in the octree by traversing it and incrementing based on the number
// of nodes containing a point.
func (octree *basicOctree) Size() int {
	var totalSize int
	switch octree.node.nodeType {
	case InternalNode:
		for _, children := range octree.node.tree {
			totalSize += children.Size()
		}
	case LeafNodeFilled:
		totalSize = 1

	case LeafNodeEmpty:
		totalSize = 0
	}
	return totalSize
}

// Set checks if the point to be added is a valid point for the OCtree to hold based on its center and side length.
// It then recursively iterates through the tree until it finds the appropriate node to add it too. If the found node
// contains a point already, it will the node into octants and add both the old and new points to them respectively.
func (octree *basicOctree) Set(p r3.Vector, d pc.Data) error {
	if !checkPointPlacement(octree.center, octree.side, p) {
		return errors.New("error invalid point to add to octree")
	}

	octree.meta.Merge(p, d)

	switch octree.node.nodeType {
	case InternalNode:
		for _, childNode := range octree.node.tree {
			if checkPointPlacement(childNode.center, childNode.side, p) {
				return childNode.Set(p, d)
			}
		}
	case LeafNodeFilled:
		err := octree.splitIntoOctants()
		if err != nil {
			return errors.New("error in splitting octree into new octants")
		}
		return octree.Set(p, d)
	case LeafNodeEmpty:
		octree.node = newLeafNodeFilled(p, d)
	}
	return nil
}

// At traverses the octree to see in a point exists at the specified location. If a point does exist, its data is
// returned along with true, if one is not then no data is returned and the boolean is returned false.
func (octree *basicOctree) At(x, y, z float64) (pc.Data, bool) {
	switch octree.node.nodeType {
	case InternalNode:
		for _, child := range octree.node.tree {
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

// Iterate TODO: Implement iterate for octree.
func (octree *basicOctree) Iterate(numBatches, myBatch int, fn func(p r3.Vector, d pc.Data) bool) {
}

// MarshalOctree TODO: Implement marshalling for octree.
func (octree *basicOctree) MarshalOctree() ([]byte, error) {
	return nil, nil
}

// NewMetaData returns the metadata for.
func (octree *basicOctree) NewMetaData() MetaData {
	return MetaData{
		Version:    octreeVersion,
		CenterX:    octree.center.X,
		CenterY:    octree.center.Y,
		CenterZ:    octree.center.Z,
		Side:       octree.side,
		Size:       0,
		PCMetaData: pc.NewMetaData(),
	}
}

// OctreeMetaData returns the octree metadata.
func (octree *basicOctree) OctreeMetaData() MetaData {
	return octree.meta
}

// Metadata returns the metadata of the pointcloud stored in the octree.
func (octree *basicOctree) MetaData() pc.MetaData {
	return octree.meta.PCMetaData
}
