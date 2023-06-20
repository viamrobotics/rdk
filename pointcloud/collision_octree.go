package pointcloud

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/spatialmath"
)

// CollisionOctree is a data structure that represents a collision octree structure with a BasicOctree as its
// backbone. CollisionOctree is for the specific case of using an octree to detect collisions during motion
// planning. confidenceThreshold defines the minimum value a point should have to be considered a collision.
// buffer defines the maximum distance a point can be for it to be considered a collision.
type CollisionOctree struct {
	*BasicOctree
	confidenceThreshold int
	buffer              float64
}

// NewCollisionOctree creates a new empty collision octree with specified center, side, confidenceThreshold, and buffer distance.
func NewCollisionOctree(center r3.Vector, sideLength float64, confidenceThreshold int, buffer float64) (*CollisionOctree, error) {
	if sideLength <= 0 {
		return nil, errors.Errorf("invalid side length (%.2f) for octree", sideLength)
	}

	basicOct, err := NewBasicOctree(center, sideLength)
	if err != nil {
		return nil, err
	}

	collisionOct := &CollisionOctree{
		BasicOctree:         basicOct,
		confidenceThreshold: confidenceThreshold,
		buffer:              buffer,
	}

	return collisionOct, nil
}

// NewCollisionOctreeFromBasicOctree creates a new collision octree from basicOct with a specified confidenceThreshold and buffer distance.
func NewCollisionOctreeFromBasicOctree(basicOct *BasicOctree, confidenceThreshold int, buffer float64) (*CollisionOctree, error) {
	collisionOct := &CollisionOctree{
		BasicOctree:         basicOct,
		confidenceThreshold: confidenceThreshold,
		buffer:              buffer,
	}

	return collisionOct, nil
}

// Pose returns the pose of the octree.
func (cOct *CollisionOctree) Pose() spatialmath.Pose {
	return spatialmath.NewPoseFromPoint(cOct.center)
}

// AlmostEqual compares the octree with another geometry and checks if they are equivalent.
func (cOct *CollisionOctree) AlmostEqual(geom spatialmath.Geometry) bool {
	// TODO
	return false
}

// Transform recursively steps through the octree and transforms it by the given pose.
func (cOct *CollisionOctree) Transform(p spatialmath.Pose) spatialmath.Geometry {
	if spatialmath.PoseAlmostEqual(p, spatialmath.NewZeroPose()) {
		return cOct
	}

	var transformedOctree *CollisionOctree
	switch cOct.node.nodeType {
	case internalNode:
		newCenter := cOct.center.Add(p.Point())

		newTotalX := 0.0
		newTotalY := 0.0
		newTotalZ := 0.0

		newChildren := make([]*BasicOctree, 0)

		for _, child := range cOct.node.children {
			transformedChild := child.Transform(p)
			newChildren = append(newChildren, transformedChild)

			newTotalX += transformedChild.meta.totalX
			newTotalY += transformedChild.meta.totalY
			newTotalZ += transformedChild.meta.totalZ
		}

		transformPoint := p.Point()
		newMetaData := newTransformedMetaData(cOct.meta, transformPoint)

		newMetaData.totalX = newTotalX
		newMetaData.totalY = newTotalY
		newMetaData.totalZ = newTotalZ

		transformedOctree = &CollisionOctree{
			BasicOctree: &BasicOctree{
				newInternalNode(newChildren),
				newCenter,
				cOct.sideLength,
				cOct.size,
				newMetaData,
			},
			confidenceThreshold: cOct.confidenceThreshold,
			buffer:              cOct.buffer,
		}

		transformedOctree.node.maxVal = cOct.node.maxVal

	case leafNodeFilled:
		transformPoint := p.Point()
		newCenter := cOct.center.Add(p.Point())
		newPoint := &PointAndData{P: cOct.node.point.P.Add(transformPoint), D: cOct.node.point.D}

		newMetaData := newTransformedMetaData(cOct.meta, transformPoint)

		newMetaData.totalX = cOct.meta.totalX + transformPoint.X
		newMetaData.totalY = cOct.meta.totalY + transformPoint.Y
		newMetaData.totalZ = cOct.meta.totalZ + transformPoint.Z

		transformedOctree = &CollisionOctree{
			BasicOctree: &BasicOctree{
				newLeafNodeFilled(newPoint.P, newPoint.D),
				newCenter,
				cOct.sideLength,
				cOct.size,
				newMetaData,
			},
			confidenceThreshold: cOct.confidenceThreshold,
			buffer:              cOct.buffer,
		}

		transformedOctree.node.maxVal = cOct.node.maxVal

	case leafNodeEmpty:
		transformedOctree = &CollisionOctree{
			BasicOctree: &BasicOctree{
				newLeafNodeEmpty(),
				cOct.center.Add(p.Point()),
				cOct.sideLength,
				cOct.size,
				cOct.meta,
			},
			confidenceThreshold: cOct.confidenceThreshold,
			buffer:              cOct.buffer,
		}
	}
	return transformedOctree
}

// ToProtobuf converts the octree to a Geometry proto message.
func (cOct *CollisionOctree) ToProtobuf() *commonpb.Geometry {
	// TODO
	return nil
}

// CollidesWith checks if the given octree collides with the given geometry and returns true if it does.
func (cOct *CollisionOctree) CollidesWith(geom spatialmath.Geometry) (bool, error) {
	if cOct.MaxVal() < cOct.confidenceThreshold {
		return false, nil
	}
	switch cOct.node.nodeType {
	case internalNode:
		ocbox, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(cOct.center),
			r3.Vector{cOct.sideLength + cOct.buffer, cOct.sideLength + cOct.buffer, cOct.sideLength + cOct.buffer},
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
		for _, child := range cOct.node.children {
			collide, err = child.CollidesWithGeometry(geom, cOct.confidenceThreshold, cOct.buffer)
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
		ptGeom, err := spatialmath.NewSphere(spatialmath.NewPoseFromPoint(cOct.node.point.P), cOct.buffer, "")
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

// DistanceFrom returns the distance from the given octree to the given geometry.
func (cOct *CollisionOctree) DistanceFrom(geom spatialmath.Geometry) (float64, error) {
	// TODO: currently implemented as the bare minimum but needs to be changed to correct implementation
	collides, err := cOct.CollidesWith(geom)
	if err != nil {
		return -math.Inf(-1), err
	}
	if collides {
		return -1, nil
	}
	return 1, nil
}

// EncompassedBy returns true if the given octree is within the given geometry.
func (cOct *CollisionOctree) EncompassedBy(geom spatialmath.Geometry) (bool, error) {
	// TODO
	return false, errors.New("not implemented")
}

// SetLabel sets the label of this octree.
func (cOct *CollisionOctree) SetLabel(label string) {
	// Label returns the label of this octree.
	cOct.node.label = label
}

// Label returns the label of this octree.
func (cOct *CollisionOctree) Label() string {
	return cOct.node.label
}

// String returns a human readable string that represents this octree.
func (cOct *CollisionOctree) String() string {
	return fmt.Sprintf("octree with center at %v and side length of %v", cOct.center, cOct.sideLength)
}

// ToPoints converts an octree geometry into []r3.Vector.
func (cOct *CollisionOctree) ToPoints(resolution float64) []r3.Vector {
	// TODO
	return nil
}

// MarshalJSON marshals JSON from the octree.
func (cOct *CollisionOctree) MarshalJSON() ([]byte, error) {
	// TODO
	return nil, errors.New("not implemented")
}
