package spatialmath

import (
	"encoding/json"
	"math"

	"github.com/golang/geo/r3"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
)

// PointCreator implements the GeometryCreator interface for point structs.
type pointCreator struct {
	offset Pose
}

// point is a collision geometry that represents a single point in 3D space that occupies no geometry.
type point struct {
	pose Pose
}

// NewPointCreator instantiates a PointCreator class, which allows instantiating point geometries given only a pose which is applied
// at the specified offset from the pose. These pointers have dimensions given by the provided halfSize vector.
func NewPointCreator(offset Pose) GeometryCreator {
	return &pointCreator{offset}
}

// NewGeometry instantiates a new point from a PointCreator class.
func (pc *pointCreator) NewGeometry(pose Pose) Geometry {
	return &point{Compose(pc.offset, pose)}
}

func (pc *pointCreator) MarshalJSON() ([]byte, error) {
	config, err := NewGeometryConfig(pc.offset)
	if err != nil {
		return nil, err
	}
	config.Type = "point"
	return json.Marshal(config)
}

// NewPoint instantiates a new point Geometry.
func NewPoint(pt r3.Vector) Geometry {
	return &point{NewPoseFromPoint(pt)}
}

// Pose returns the pose of the point.
func (pt *point) Pose() Pose {
	return pt.pose
}

// Vertices returns the vertices defining the point.
func (pt *point) Vertices() []r3.Vector {
	return []r3.Vector{pt.pose.Point()}
}

// AlmostEqual compares the point with another geometry and checks if they are equivalent.
func (pt *point) AlmostEqual(g Geometry) bool {
	other, ok := g.(*point)
	if !ok {
		return false
	}
	return PoseAlmostEqual(pt.pose, other.pose)
}

// Transform premultiplies the point pose with a transform, allowing the point to be moved in space.
func (pt *point) Transform(toPremultiply Pose) Geometry {
	return &point{Compose(toPremultiply, pt.pose)}
}

// ToProto converts the point to a Geometry proto message.
func (pt *point) ToProtobuf() *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: PoseToProtobuf(pt.pose),
		GeometryType: &commonpb.Geometry_Sphere{
			Sphere: &commonpb.Sphere{
				RadiusMm: 0,
			},
		},
	}
}

// CollidesWith checks if the given point collides with the given geometry and returns true if it does.
func (pt *point) CollidesWith(g Geometry) (bool, error) {
	if other, ok := g.(*box); ok {
		return pointVsBoxCollision(other, pt.pose.Point()), nil
	}
	if other, ok := g.(*sphere); ok {
		return sphereVsPointDistance(other, pt.pose.Point()) <= 0, nil
	}
	if other, ok := g.(*point); ok {
		return pt.AlmostEqual(other), nil
	}
	return true, newCollisionTypeUnsupportedError(pt, g)
}

// CollidesWith checks if the given point collides with the given geometry and returns true if it does.
func (pt *point) DistanceFrom(g Geometry) (float64, error) {
	if other, ok := g.(*box); ok {
		return pointVsBoxDistance(other, pt.pose.Point()), nil
	}
	if other, ok := g.(*sphere); ok {
		return sphereVsPointDistance(other, pt.pose.Point()), nil
	}
	if other, ok := g.(*point); ok {
		return pt.pose.Point().Sub(other.pose.Point()).Norm(), nil
	}
	return math.Inf(-1), newCollisionTypeUnsupportedError(pt, g)
}

// EncompassedBy returns a bool describing if the given point is completely encompassed by the given geometry.
func (pt *point) EncompassedBy(g Geometry) (bool, error) {
	return pt.CollidesWith(g)
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
