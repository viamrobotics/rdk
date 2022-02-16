package spatialmath

import (
	"encoding/json"
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
)

// PointCreator implements the VolumeCreator interface for point structs.
type pointCreator struct {
	offset Pose
}

// point is a collision geometry that represents a single point in 3D space that occupies no volume.
type point struct {
	pose Pose
}

// NewPoint instantiates a PointCreator class, which allows instantiating pointers given only a pose which is applied
// at the specified offset from the pose. These pointers have dimensions given by the provided halfSize vector.
func NewPoint(offset Pose) VolumeCreator {
	return &pointCreator{offset}
}

// NewVolume instantiates a new point from a PointCreator class.
func (pc *pointCreator) NewVolume(pose Pose) Volume {
	p := &point{pc.offset}
	p.Transform(pose)
	return p
}

func (pc *pointCreator) MarshalJSON() ([]byte, error) {
	config, err := NewVolumeConfig(pc.offset)
	if err != nil {
		return nil, err
	}
	config.Type = "point"
	return json.Marshal(config)
}

// Pose returns the pose of the point.
func (pt *point) Pose() Pose {
	return pt.pose
}

// Vertices returns the vertices defining the point.
func (pt *point) Vertices() []r3.Vector {
	return []r3.Vector{pt.pose.Point()}
}

// AlmostEqual compares the point with another volume and checks if they are equivalent.
func (pt *point) AlmostEqual(v Volume) bool {
	other, ok := v.(*point)
	if !ok {
		return false
	}
	return PoseAlmostEqual(pt.pose, other.pose)
}

// Transform premultiplies the point pose with a transform, allowing the point to be moved in space.
func (pt *point) Transform(toPremultiply Pose) {
	pt.pose = Compose(toPremultiply, pt.pose)
}

// CollidesWith checks if the given point collides with the given volume and returns true if it does.
func (pt *point) CollidesWith(v Volume) (bool, error) {
	if other, ok := v.(*box); ok {
		return pointVsBoxCollision(other, pt.pose.Point()), nil
	}
	if other, ok := v.(*sphere); ok {
		return sphereVsPointDistance(other, pt.pose.Point()) <= 0, nil
	}
	if other, ok := v.(*point); ok {
		return pt.AlmostEqual(other), nil
	}
	return true, errors.Errorf("collisions between point and %T are not supported", v)
}

// CollidesWith checks if the given point collides with the given volume and returns true if it does.
func (pt *point) DistanceFrom(v Volume) (float64, error) {
	if other, ok := v.(*box); ok {
		return pointVsBoxDistance(other, pt.pose.Point()), nil
	}
	if other, ok := v.(*sphere); ok {
		return sphereVsPointDistance(other, pt.pose.Point()), nil
	}
	if other, ok := v.(*point); ok {
		return pt.pose.Point().Sub(other.pose.Point()).Norm(), nil
	}
	return math.Inf(-1), errors.Errorf("collisions between point and %T are not supported", v)
}

// pointVsBoxCollision takes a box and a point as arguments and returns a bool describing if they are in collision. \
// true == collision / false == no collision.
func pointVsBoxCollision(b *box, pt r3.Vector) bool {
	return b.closestPoint(pt).Sub(pt).Norm() <= 0
}

// pointVsBoxDistance takes a box and a point as arguments and returns a floating point number.  If this number is nonpositive it represents
// the penetration depth of the point within the box.  If the returned float is positive it represents the separation distance between the
// point and the box, which are not in collision.
func pointVsBoxDistance(b *box, pt r3.Vector) float64 {
	distance := b.closestPoint(pt).Sub(pt).Norm()
	if distance > 0 {
		return distance
	}
	return -b.penetrationDepth(pt)
}
