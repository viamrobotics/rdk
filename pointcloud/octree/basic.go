package octree

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	pc "go.viam.com/rdk/pointcloud"
)

// basicOctree is a data structure that represents a basic octree structure with information regarding center
// point, side length and node data. Node data is comprised of a PointAndData (should one exist) from

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
	tree     []*basicOctree
	point    pc.PointAndData
}

// New creates a new basic octree with specified center, side and metadata.
func New(ctx context.Context, center r3.Vector, sideLength float64, logger golog.Logger) (Octree, error) {
	if sideLength <= 0 {
		return nil, errors.Errorf("invalid side length (%.2f) for octree", sideLength)
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

// Set checks if the point to be added is a valid point for the octree to hold based on its center and side length.
// It then recursively iterates through the tree until it finds the appropriate node to add it too. If the found node
// contains a point already, it will split the node into octants and add both the old and new points to them respectively.
func (octree *basicOctree) Set(p r3.Vector, d pc.Data) error {
	if (pc.PointAndData{P: p, D: d} == pc.PointAndData{}) {
		octree.logger.Debug("no data given, skipping insertion")
		return nil
	}

	if !checkPointPlacement(octree.center, octree.side, p) {
		return errors.New("error point is outside the bounds of this octree")
	}

	switch octree.node.nodeType {
	case InternalNode:
		for _, childNode := range octree.node.tree {
			if checkPointPlacement(childNode.center, childNode.side, p) {
				// Iterate through children nodes
				err := childNode.Set(p, d)
				if err == nil {
					// Update metadata
					octree.meta.Merge(p, d)
				}
				return err
			}
		}
		return errors.New("error invalid internal node detected, please check your tree")

	case LeafNodeFilled:
		_, exists := octree.At(p.X, p.Y, p.Z)
		if exists {
			// Update data in point
			octree.node.point.D = d
			return nil
		}
		err := octree.splitIntoOctants()
		if err != nil {
			return errors.Errorf("error in splitting octree into new octants: %v", err)
		}
		// No update of metadata as the set call below will lead to the InternalNode case due to the octant split
		return octree.Set(p, d)

	case LeafNodeEmpty:
		// Update metadata
		octree.meta.Merge(p, d)
		octree.node = newLeafNodeFilled(p, d)
	}

	return nil
}

// At traverses the octree to see if a point exists at the specified location. If a point does exist, its data is
// returned along with true. If a point does not exist, no data is returned and the boolean is returned false.
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

// NewMetaData creates and return the octree MetaData associated with a new basic octree.
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
