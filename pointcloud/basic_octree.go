package pointcloud

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/spatialmath"
)

const octreeMagicSideLength = -17

// BasicOctreeType octree string.
const BasicOctreeType = "octree"

// BasicOctreeConfig the type.
var BasicOctreeConfig = TypeConfig{
	StructureType: BasicOctreeType,
	NewWithParams: func(size int) PointCloud {
		return newBasicOctree(r3.Vector{}, octreeMagicSideLength, defaultConfidenceThreshold)
	},
}

func init() {
	Register(BasicOctreeConfig)
}

const (
	internalNode = NodeType(iota)
	leafNodeEmpty
	leafNodeFilled
	// This value allows for high level of granularity in the octree while still allowing for fast access times
	// even on a pi.
	maxRecursionDepth          = 250  // This gives us enough resolution to model the observable universe in planck lengths.
	floatEpsilon               = 1e-6 // This is also effectively half of the minimum side length.
	nodeRegionOverlap          = floatEpsilon / 2
	defaultConfidenceThreshold = 50
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

	// value between 0-100 that sets a threshold which is the confidence level required for a point to be considered a collision
	confidenceThreshold int

	toStore PointCloud // this is temporary when building when sideLength == -1
}

// basicOctreeNode is a struct comprised of the type of node, children nodes (should they exist) and the pointcloud's
// PointAndData datatype representing a point in space.
type basicOctreeNode struct {
	nodeType NodeType
	children []*BasicOctree
	point    *PointAndData
	maxVal   int
}

// NewFromMesh returns an octree representation of the Mesh geometry.
func NewFromMesh(mesh *spatialmath.Mesh) (*BasicOctree, error) {
	meshPts := mesh.ToPoints(0)
	pc := NewBasicPointCloud(len(meshPts))
	for _, pt := range meshPts {
		if err := pc.Set(pt, NewBasicData()); err != nil {
			return nil, err
		}
	}
	octree, err := ToBasicOctree(pc, 0)
	if err != nil {
		return nil, err
	}
	octree.SetLabel(mesh.Label())
	return octree, nil
}

// ToBasicOctree takes a pointcloud object and converts it into a basic octree.
func ToBasicOctree(cloud PointCloud, confidenceThreshold int) (*BasicOctree, error) {
	if basicOctree, ok := cloud.(*BasicOctree); ok && (basicOctree.confidenceThreshold == confidenceThreshold) {
		return basicOctree, nil
	}

	meta := cloud.MetaData()
	center := meta.Center()
	maxSideLength := meta.MaxSideLength()
	basicOctree := newBasicOctree(center, maxSideLength, defaultConfidenceThreshold)

	var err error
	cloud.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		if err = basicOctree.Set(p, d); err != nil {
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return basicOctree, nil
}

func newBasicOctree(center r3.Vector, sideLength float64, confidenceThreshold int) *BasicOctree {
	if sideLength <= 0 && sideLength != octreeMagicSideLength {
		sideLength = 1
	}

	octree := &BasicOctree{
		node:                newLeafNodeEmpty(),
		center:              center,
		sideLength:          sideLength,
		size:                0,
		meta:                NewMetaData(),
		confidenceThreshold: confidenceThreshold,
	}

	return octree
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
	if octree.sideLength == octreeMagicSideLength {
		if octree.toStore == nil {
			octree.toStore = NewBasicPointCloud(0)
		}
		return octree.toStore.Set(p, d)
	}
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
		if pointsAlmostEqualEpsilon(octree.node.point.P, r3.Vector{X: x, Y: y, Z: z}, floatEpsilon) {
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
// Note that this checks that the *geometry* is equal; that is, both octrees have the same number of points and in the same locations.
// This is agnostic to things like the label, the centerpoint (as the individual points have locations), the side lengths, etc.
func (octree *BasicOctree) AlmostEqual(geom spatialmath.Geometry) bool {
	otherOctree, ok := geom.(*BasicOctree)
	if !ok {
		return false
	}
	if octree.size != otherOctree.size {
		return false
	}
	allExist := true
	octree.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		_, exists := otherOctree.At(p.X, p.Y, p.Z)
		if !exists {
			allExist = false
			return false
		}
		return true
	})
	return allExist
}

// Transform recursively steps through the octree and transforms it by the given pose.
func (octree *BasicOctree) Transform(pose spatialmath.Pose) spatialmath.Geometry {
	newCenter := spatialmath.Compose(pose, spatialmath.NewPoseFromPoint(octree.center))

	// New sidelength is the diagonal of octree to guarantee fit
	newOctree := newBasicOctree(newCenter.Point(), octree.sideLength*math.Sqrt(3), octree.confidenceThreshold)

	newOctree.label = octree.label
	newOctree.meta = octree.meta

	octree.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		tformPt := spatialmath.Compose(pose, spatialmath.NewPoseFromPoint(p)).Point()
		// We don't do anything with this error, as returning false here merely silently truncates the pointcloud.
		// Preference is to lose one point than the rest of them.
		err := newOctree.Set(tformPt, d)
		_ = err
		return true
	})
	return newOctree
}

// ToProtobuf converts the octree to a Geometry proto message.
// TODO (RSDK-3743): Implement BasicOctree Geometry functions.
func (octree *BasicOctree) ToProtobuf() *commonpb.Geometry {
	return nil
}

// CollidesWith checks if the given octree collides with the given geometry and returns true if it does.
// A point is in collision if its stored probability is >= confidenceThreshold and if it is at most collisionBufferMM distance away.
func (octree *BasicOctree) CollidesWith(geom spatialmath.Geometry, collisionBufferMM float64) (bool, error) {
	if octree.MaxVal() < octree.confidenceThreshold {
		return false, nil
	}
	switch octree.node.nodeType {
	case internalNode:
		ocbox, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(octree.center),
			r3.Vector{
				X: octree.sideLength + collisionBufferMM,
				Y: octree.sideLength + collisionBufferMM,
				Z: octree.sideLength + collisionBufferMM,
			},
			"",
		)
		if err != nil {
			return false, err
		}

		// Check whether our geom collides with the area represented by the octree. If false, we can skip
		collide, err := geom.CollidesWith(ocbox, collisionBufferMM)
		if err != nil {
			return false, err
		}
		if !collide {
			return false, nil
		}
		for _, child := range octree.node.children {
			collide, err = child.CollidesWith(geom, collisionBufferMM)
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
		return geom.CollidesWith(spatialmath.NewPoint(octree.node.point.P, ""), collisionBufferMM)
	}
	return false, errors.New("unknown octree node type")
}

// DistanceFrom returns the distance from the given octree to the given geometry.
// TODO (RSDK-3743): Implement BasicOctree Geometry functions.
func (octree *BasicOctree) DistanceFrom(geom spatialmath.Geometry) (float64, error) {
	collides, err := octree.CollidesWith(geom, floatEpsilon)
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
	points := make([]r3.Vector, 0, octree.size)
	octree.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		points = append(points, p)
		return true
	})
	return points
}

// MarshalJSON marshals JSON from the octree.
// TODO (RSDK-3743): Implement BasicOctree Geometry functions.
func (octree *BasicOctree) MarshalJSON() ([]byte, error) {
	return nil, errors.New("not implemented")
}

// FinalizeAfterReading fix it all.
func (octree *BasicOctree) FinalizeAfterReading() (PointCloud, error) {
	if octree.sideLength != octreeMagicSideLength {
		return octree, nil
	}

	meta := octree.toStore.MetaData()
	octree.center = meta.Center()
	octree.sideLength = meta.MaxSideLength()

	var err error
	octree.toStore.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		err = octree.Set(p, d)
		return err == nil
	})

	octree.toStore = nil
	return octree, nil
}

// CreateNewRecentered re-size and center.
func (octree *BasicOctree) CreateNewRecentered(offset spatialmath.Pose) PointCloud {
	center := offset.Point().Add(octree.center)
	return newBasicOctree(center, octree.sideLength, octree.confidenceThreshold)
}
