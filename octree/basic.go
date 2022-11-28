package octree

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	pc "go.viam.com/rdk/pointcloud"
)

// basicOctree is a data structure that represents a basic octree structure with information regarding center
// point, side length and node data.
type basicOctree struct {
	logger     golog.Logger
	node       basicOctreeNode
	center     r3.Vector
	sideLength float64
	size       int32
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
	return int(octree.size)
}

// Set checks if the point to be added is a valid point for a basic octree to contain based on its center and side
// length. It then recursively iterates through the tree until it finds the appropriate node to add it to. If the
// found node contains a point already, it will split the node into octants and will add both the old point and new
// one to the newly created children trees.
func (octree *basicOctree) Set(p r3.Vector, d pc.Data) error {
	if (pc.PointAndData{P: p, D: d} == pc.PointAndData{}) {
		octree.logger.Debug("no data given, skipping insertion")
		return nil
	}

	if !octree.checkPointPlacement(p) {
		return errors.New("error point is outside the bounds of this octree")
	}

	switch octree.node.nodeType {
	case InternalNode:
		for _, childNode := range octree.node.children {
			if childNode.checkPointPlacement(p) {
				err := childNode.Set(p, d)
				if err == nil {
					// Update metadata
					octree.meta.Merge(p, d)
					octree.size++
				}
				return err
			}
		}
		return errors.New("error invalid internal node detected, please check your tree")

	case LeafNodeFilled:
		if _, exists := octree.At(p.X, p.Y, p.Z); exists {
			// Update data in point
			octree.node.point.D = d
			return nil
		}
		if err := octree.splitIntoOctants(); err != nil {
			return errors.Errorf("error in splitting octree into new octants: %v", err)
		}
		// No update of metadata as the set call below will lead to the InternalNode case due to the octant split
		return octree.Set(p, d)

	case LeafNodeEmpty:
		// Update metadata
		octree.meta.Merge(p, d)
		octree.size++
		octree.node = newLeafNodeFilled(p, d)
	}

	return nil
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

// Iterate TODO: Implement iterate for octree.
func (octree *basicOctree) Iterate(numBatches, myBatch int, fn func(p r3.Vector, d pc.Data) bool) {
}

// MarshalOctree TODO: Implement marshalling for octree.
func (octree *basicOctree) MarshalOctree() ([]byte, error) {
	return nil, nil
}

// Metadata returns the metadata of the pointcloud stored in the octree.
func (octree *basicOctree) MetaData() pc.MetaData {
	return octree.meta
}
