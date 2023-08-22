package pointcloud

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/spatialmath"
)

const (
	internalNode = NodeType(iota)
	leafNodeEmpty
	leafNodeFilled
	// This value allows for high level of granularity in the octree while still allowing for fast access times
	// even on a pi.
	maxRecursionDepth = 1000
	nodeRegionOverlap = 1e-6
	// TODO (RSDK-3767): pass these in a different way.
	confidenceThreshold = 50    // value between 0-100, threshold sets the confidence level required for a point to be considered a collision
	buffer              = 150.0 // max distance from base to point for it to be considered a collision in mm
)

// NodeType represents the possible types of nodes in an octree.
type NodeType uint8

// BasicOctree is a data structure that represents a basic octree structure with information regarding center
// point, side length and node data. An octree is a data structure that recursively partitions 3D space into
// octants to represent occupancy. It is a storage format for a pointcloud that allows for better searchability
// and serialization.
type BasicOctree struct {
	node       basicOctreeNode
	center     r3.Vector
	sideLength float64
	size       int
	meta       MetaData
	label      string
}

// basicOctreeNode is a struct comprised of the type of node, children nodes (should they exist) and the pointcloud's
// PointAndData datatype representing a point in space.
type basicOctreeNode struct {
	nodeType NodeType
	children []*BasicOctree
	point    *PointAndData
	maxVal   int
}

// NewBasicOctree creates a new basic octree with specified center, side and metadata.
func NewBasicOctree(center r3.Vector, sideLength float64) (*BasicOctree, error) {
	if sideLength <= 0 {
		return nil, errors.Errorf("invalid side length (%.2f) for octree", sideLength)
	}

	octree := &BasicOctree{
		node:       newLeafNodeEmpty(),
		center:     center,
		sideLength: sideLength,
		size:       0,
		meta:       NewMetaData(),
	}

	return octree, nil
}

// Size returns the number of points stored in the octree's metadata.
func (octree *BasicOctree) Size() int {
	return octree.size
}

// MaxVal returns the max value of all children's data for the passed in octree.
func (octree *BasicOctree) MaxVal() int {
	return octree.node.maxVal
}

// Set recursively iterates through a basic octree, attempting to add a given point and data to the tree after
// ensuring it falls within the bounds of the given basic octree.
func (octree *BasicOctree) Set(p r3.Vector, d Data) error {
	_, err := octree.helperSet(p, d, 0)
	return err
}

// At traverses a basic octree to see if a point exists at the specified location. If a point does exist, its data
// is returned along with true. If a point does not exist, no data is returned and the boolean is returned false.
func (octree *BasicOctree) At(x, y, z float64) (Data, bool) {
	// Check if point could exist in octree given bounds
	if !octree.checkPointPlacement(r3.Vector{X: x, Y: y, Z: z}) {
		return nil, false
	}

	switch octree.node.nodeType {
	case internalNode:
		for _, child := range octree.node.children {
			d, exists := child.At(x, y, z)
			if exists {
				return d, true
			}
		}

	case leafNodeFilled:
		if octree.node.point.P.ApproxEqual(r3.Vector{X: x, Y: y, Z: z}) {
			return octree.node.point.D, true
		}

	case leafNodeEmpty:
	}

	return nil, false
}

// Iterate is a batchable process that will go through a basic octree and applies a specified function
// to either all the data points or a subset of them based on the given numBatches and currentBatch
// inputs. If any of the applied functions returns a false value, iteration will stop and no further
// points will be processed.
func (octree *BasicOctree) Iterate(numBatches, currentBatch int, fn func(p r3.Vector, d Data) bool) {
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

// MetaData returns the metadata of the pointcloud stored in the octree.
func (octree *BasicOctree) MetaData() MetaData {
	return octree.meta
}

// Pose returns the pose of the octree.
func (octree *BasicOctree) Pose() spatialmath.Pose {
	return spatialmath.NewPoseFromPoint(octree.center)
}

// AlmostEqual compares the octree with another geometry and checks if they are equivalent.
// TODO (RSDK-3743): Implement BasicOctree Geometry functions.
func (octree *BasicOctree) AlmostEqual(geom spatialmath.Geometry) bool {
	return false
}

// Transform recursively steps through the octree and transforms it by the given pose.
// TODO (RSDK-3743): Implement BasicOctree Geometry functions.
func (octree *BasicOctree) Transform(p spatialmath.Pose) spatialmath.Geometry {
	return nil
}

// ToProtobuf converts the octree to a Geometry proto message.
// TODO (RSDK-3743): Implement BasicOctree Geometry functions.
func (octree *BasicOctree) ToProtobuf() *commonpb.Geometry {
	return nil
}

// CollidesWithGeometry will return whether a given geometry is in collision with a given point.
// A point is in collision if its stored probability is >= confidenceThreshold and if it is at most buffer distance away.
func (octree *BasicOctree) CollidesWithGeometry(geom spatialmath.Geometry, confidenceThreshold int, buffer float64) (bool, error) {
	if octree.MaxVal() < confidenceThreshold {
		return false, nil
	}
	switch octree.node.nodeType {
	case internalNode:
		ocbox, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(octree.center),
			r3.Vector{octree.sideLength + buffer, octree.sideLength + buffer, octree.sideLength + buffer},
			"",
		)
		if err != nil {
			return false, err
		}

		// Check whether our geom collides with the area represented by the octree. If false, we can skip
		collide, err := geom.CollidesWith(ocbox)
		if err != nil {
			return false, err
		}
		if !collide {
			return false, nil
		}
		for _, child := range octree.node.children {
			collide, err = child.CollidesWithGeometry(geom, confidenceThreshold, buffer)
			if err != nil {
				return false, err
			}
			if collide {
				return true, nil
			}
		}
		return false, nil
	case leafNodeEmpty:
		return false, nil
	case leafNodeFilled:
		ptGeom, err := spatialmath.NewSphere(spatialmath.NewPoseFromPoint(octree.node.point.P), buffer, "")
		if err != nil {
			return false, err
		}

		ptCollide, err := geom.CollidesWith(ptGeom)
		if err != nil {
			return false, err
		}
		return ptCollide, nil
	}
	return false, errors.New("unknown octree node type")
}

// CollidesWith checks if the given octree collides with the given geometry and returns true if it does.
func (octree *BasicOctree) CollidesWith(geom spatialmath.Geometry) (bool, error) {
	return octree.CollidesWithGeometry(geom, confidenceThreshold, buffer)
}

// DistanceFrom returns the distance from the given octree to the given geometry.
// TODO (RSDK-3743): Implement BasicOctree Geometry functions.
func (octree *BasicOctree) DistanceFrom(geom spatialmath.Geometry) (float64, error) {
	collides, err := octree.CollidesWith(geom)
	if err != nil {
		return math.Inf(1), err
	}
	if collides {
		return -1, nil
	}
	return 1, nil
}

// EncompassedBy returns true if the given octree is within the given geometry.
// TODO (RSDK-3743): Implement BasicOctree Geometry functions.
func (octree *BasicOctree) EncompassedBy(geom spatialmath.Geometry) (bool, error) {
	return false, errors.New("not implemented")
}

// SetLabel sets the label of this octree.
func (octree *BasicOctree) SetLabel(label string) {
	octree.label = label
}

// Label returns the label of this octree.
func (octree *BasicOctree) Label() string {
	return octree.label
}

// String returns a human readable string that represents this octree.
// octree's children will not be represented in the string.
func (octree *BasicOctree) String() string {
	template := "octree of node type %s. center: %v, side length: %v, size: %v"
	switch octree.node.nodeType {
	case internalNode:
		return fmt.Sprintf(template, "internalNode", octree.center, octree.sideLength, octree.size)
	case leafNodeEmpty:
		return fmt.Sprintf(template, "leafNodeEmpty", octree.center, octree.sideLength, octree.size)
	case leafNodeFilled:
		return fmt.Sprintf(template, "leafNodeFilled", octree.center, octree.sideLength, octree.size)
	}
	return ""
}

// ToPoints converts an octree geometry into []r3.Vector.
func (octree *BasicOctree) ToPoints(resolution float64) []r3.Vector {
	// TODO (RSDK-3743)
	return nil
}

// MarshalJSON marshals JSON from the octree.
// TODO (RSDK-3743): Implement BasicOctree Geometry functions.
func (octree *BasicOctree) MarshalJSON() ([]byte, error) {
	return nil, errors.New("not implemented")
}
