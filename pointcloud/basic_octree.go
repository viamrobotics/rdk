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
	maxRecursionDepth   = 1000
	nodeRegionOverlap   = 0.000001
	confidenceThreshold = 60   // value between 0-100, threshold sets the confidence level required for a point to be considered a collision
	buffer              = 60.0 // max distance from base to point for it to be considered a collision in mm
)

// NodeType represents the possible types of nodes in an octree.
type NodeType uint8

// BasicOctree is a data structure that represents a basic octree structure with information regarding center
// point, side length and node data. An octree is a data structure that recursively partitions 3D space into
// octants to represent occupancy. It is a storage format for a pointcloud that allows for better searchability
// and serialization.
type BasicOctree struct {
	node       basicOctreeNode
	center     spatialmath.Pose
	sideLength float64
	size       int
	meta       MetaData
}

// basicOctreeNode is a struct comprised of the type of node, children nodes (should they exist) and the pointcloud's
// PointAndData datatype representing a point in space.
type basicOctreeNode struct {
	nodeType NodeType
	children []*BasicOctree
	point    *PointAndData
	maxVal   int
	label    string
}

// NewBasicOctree creates a new basic octree with specified center, side and metadata.
func NewBasicOctree(center spatialmath.Pose, sideLength float64) (*BasicOctree, error) {
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

// CollidesWithGeometry will return whether a given geometry is in collision with a given point.
func (octree *BasicOctree) CollidesWithGeometry(geom spatialmath.Geometry, threshold int, buffer float64) (bool, error) {
	if octree.MaxVal() < threshold {
		return false, nil
	}
	switch octree.node.nodeType {
	case internalNode:
		ocbox, err := spatialmath.NewBox(
			octree.center,
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
			collide, err = child.CollidesWithGeometry(geom, threshold, buffer)
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

// Pose returns the pose of the octree.
func (octree *BasicOctree) Pose() spatialmath.Pose {
	return octree.center
}

// AlmostEqual compares the octree with another geometry and checks if they are equivalent.
func (octree *BasicOctree) AlmostEqual(geom spatialmath.Geometry) bool {
	// TODO
	return false
}

// Transform recursively steps through the octree and transforms it by the given pose.
func (octree *BasicOctree) Transform(p spatialmath.Pose) spatialmath.Geometry {
	var transformedOctree *BasicOctree
	switch octree.node.nodeType {
	case internalNode:
		newCenter := spatialmath.Compose(octree.center, p)

		newTotalX := 0.0
		newTotalY := 0.0
		newTotalZ := 0.0

		newChildren := make([]*BasicOctree, 0)

		for _, child := range octree.node.children {
			transformedChild := child.Transform(p).(*BasicOctree)
			newChildren = append(newChildren, transformedChild)

			newTotalX += transformedChild.meta.totalX
			newTotalY += transformedChild.meta.totalY
			newTotalZ += transformedChild.meta.totalZ
		}

		transformPoint := p.Point()
		newMetaData := newTransformedMetaData(octree.meta, transformPoint)

		newMetaData.totalX = newTotalX
		newMetaData.totalY = newTotalY
		newMetaData.totalZ = newTotalZ

		transformedOctree = &BasicOctree{
			newInternalNode(newChildren),
			newCenter,
			octree.sideLength,
			octree.size,
			newMetaData,
		}
	case leafNodeFilled:
		transformPoint := p.Point()
		newCenter := spatialmath.Compose(octree.center, p)
		newPoint := &PointAndData{P: octree.node.point.P.Add(transformPoint), D: octree.node.point.D}

		newMetaData := newTransformedMetaData(octree.meta, transformPoint)

		newMetaData.totalX = octree.meta.totalX + transformPoint.X
		newMetaData.totalY = octree.meta.totalY + transformPoint.Y
		newMetaData.totalZ = octree.meta.totalZ + transformPoint.Z

		transformedOctree = &BasicOctree{
			newLeafNodeFilled(newPoint.P, newPoint.D),
			newCenter,
			octree.sideLength,
			octree.size,
			newMetaData,
		}
	case leafNodeEmpty:
		transformedOctree = &BasicOctree{
			newLeafNodeEmpty(),
			spatialmath.Compose(octree.center, p),
			octree.sideLength,
			octree.size,
			octree.meta,
		}
	}
	return transformedOctree
}

// newTransformedMetaData returns a new MetaData with min and max values of originalMeta transformed by transformPoint
func newTransformedMetaData(originalMeta MetaData, transformPoint r3.Vector) MetaData {
	newMetaData := NewMetaData()
	newMetaData.MaxX = originalMeta.MaxX + transformPoint.X
	newMetaData.MinX = originalMeta.MinX + transformPoint.X
	newMetaData.MaxY = originalMeta.MaxY + transformPoint.Y
	newMetaData.MinY = originalMeta.MinY + transformPoint.Y
	newMetaData.MaxZ = originalMeta.MaxZ + transformPoint.Z
	newMetaData.MinZ = originalMeta.MinZ + transformPoint.Z

	newMetaData.HasColor = originalMeta.HasColor
	newMetaData.HasValue = originalMeta.HasValue

	return newMetaData
}

// ToProtobuf converts the octree to a Geometry proto message.
func (octree *BasicOctree) ToProtobuf() *commonpb.Geometry {
	// TODO
	return nil
}

// CollidesWith checks if the given octree collides with the given geometry and returns true if it does.
func (octree *BasicOctree) CollidesWith(geom spatialmath.Geometry) (bool, error) {
	return octree.CollidesWithGeometry(geom, confidenceThreshold, buffer)
}

// DistanceFrom returns the distance from the given octree to the given geometry.
func (octree *BasicOctree) DistanceFrom(geom spatialmath.Geometry) (float64, error) {
	// TODO: currently implemented as the bare minimum but needs to be changed to correct implementation
	collides, err := octree.CollidesWith(geom)
	if err != nil {
		return -math.Inf(-1), err
	}
	if collides {
		return -1, nil
	}
	return 1, nil
}

// EncompassedBy returns true if the given octree is within the given geometry.
func (octree *BasicOctree) EncompassedBy(geom spatialmath.Geometry) (bool, error) {
	// TODO
	return false, errors.New("not implemented")
}

// SetLabel sets the label of this octree.
func (octree *BasicOctree) SetLabel(label string) {
	// Label returns the label of this octree.
	octree.node.label = label
}

// Label returns the label of this octree.
func (octree *BasicOctree) Label() string {
	return octree.node.label
}

// String returns a human readable string that represents this octree.
func (octree *BasicOctree) String() string {
	return fmt.Sprintf("octree with center at %v and side length of %v", octree.center, octree.sideLength)
}

// ToPoints converts an octree geometry into []r3.Vector.
func (octree *BasicOctree) ToPoints(resolution float64) []r3.Vector {
	// TODO
	return nil
}

// MarshalJSON marshals JSON from the octree.
func (octree *BasicOctree) MarshalJSON() ([]byte, error) {
	// TODO
	return nil, errors.New("not implemented")
}
