// Package octree implements a octree representation of pointclouds for easy traversal and storage of
// probability and color data
package octree

import (
	pc "go.viam.com/rdk/pointcloud"
)

// Each node in the octree is either an internal node which links to other nodes, is an empty node with
// no points or further links, or is an occupied node which contains a single point of data that
// includes location, color and probability information.
const (
	InternalNode = NodeType(iota)
	LeafNodeEmpty
	LeafNodeFilled
	octreeVersion = 1.0
)

// NodeType represents the possible types of nodes in an octree.
type NodeType uint8

// Octree is a data structure that recursively partitions 3D space into octants to represent occupancy. It
// is a storage format for a pointcloud that allows for better searchability and serialization. Each node
// is either an internal node, empty node or child node. This implementation of an octree is compatible with
// the pointcloud representation and includes a marshaling function.
type Octree interface {
	pc.PointCloud
	Marshaler
}

// Marshaler will convert an octree into a serialized array of bytes.
type Marshaler interface {
	MarshalOctree() ([]byte, error)
}

// Unmarshaler will convert a serialized octree into an Octree datatype.
type Unmarshaler interface {
	UnmarshalOctree() (Octree, error)
}
