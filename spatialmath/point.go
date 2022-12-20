package spatialmath

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
)

// PointCreator implements the GeometryCreator interface for point structs.
type pointCreator struct {
	offset r3.Vector
	label  string
}

// point is a collision geometry that represents a single point in 3D space that occupies no geometry.
type point struct {
	pose  r3.Vector
	label string
}

// NewPointCreator instantiates a PointCreator class, which allows instantiating point geometries given only a pose which is applied
// at the specified offset from the pose. These pointers have dimensions given by the provided halfSize vector.
func NewPointCreator(offset r3.Vector, label string) GeometryCreator {
	return &pointCreator{offset, label}
}

// NewGeometry instantiates a new point from a PointCreator class.
func (pc *pointCreator) NewGeometry(pose Pose) Geometry {
	return &point{pose.Point(), pc.label}
}

func (pc *pointCreator) Offset() Pose {
	return NewPoseFromPoint(pc.offset)
}

// String returns a human readable string that represents the pointCreator.
func (pc *pointCreator) String() string {
	pt := pc.offset
	return fmt.Sprintf("Type: Point, Location X:%.0f, Y:%.0f, Z:%.0f", pt.X, pt.Y, pt.Z)
}

func (pc *pointCreator) MarshalJSON() ([]byte, error) {
	config, err := NewGeometryConfig(pc)
	if err != nil {
		return nil, err
	}
	return json.Marshal(config)
}

// ToProto converts the point to a Geometry proto message.
func (pc *pointCreator) ToProtobuf() *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: PoseToProtobuf(NewPoseFromPoint(pc.offset)),
		GeometryType: &commonpb.Geometry_Sphere{
			Sphere: &commonpb.Sphere{
				RadiusMm: 0,
			},
		},
		Label: pc.label,
	}
}

// NewPoint instantiates a new point Geometry.
func NewPoint(pt r3.Vector, label string) Geometry {
	return &point{pt, label}
}

// Label returns the labels of this point.
func (pt *point) Label() string {
	if pt != nil {
		return pt.label
	}
	return ""
}

// Pose returns the pose of the point.
func (pt *point) Pose() Pose {
	return NewPoseFromPoint(pt.pose)
}

// Vertices returns the vertices defining the point.
func (pt *point) Vertices() []r3.Vector {
	return []r3.Vector{pt.pose}
}

// AlmostEqual compares the point with another geometry and checks if they are equivalent.
func (pt *point) AlmostEqual(g Geometry) bool {
	other, ok := g.(*point)
	if !ok {
		return false
	}
	return PoseAlmostEqual(NewPoseFromPoint(pt.pose), NewPoseFromPoint(other.pose))
}

// Transform premultiplies the point pose with a transform, allowing the point to be moved in space.
func (pt *point) Transform(toPremultiply Pose) Geometry {
	return &point{Compose(toPremultiply, NewPoseFromPoint(pt.pose)).Point(), pt.label}
}

// ToProto converts the point to a Geometry proto message.
func (pt *point) ToProtobuf() *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: PoseToProtobuf(NewPoseFromPoint(pt.pose)),
		GeometryType: &commonpb.Geometry_Sphere{
			Sphere: &commonpb.Sphere{
				RadiusMm: 0,
			},
		},
		Label: pt.label,
	}
}

// CollidesWith checks if the given point collides with the given geometry and returns true if it does.
func (pt *point) CollidesWith(g Geometry) (bool, error) {
	if other, ok := g.(*box); ok {
		return pointVsBoxCollision(other, pt.pose), nil
	}
	if other, ok := g.(*sphere); ok {
		return sphereVsPointDistance(other, pt.pose) <= 0, nil
	}
	if other, ok := g.(*point); ok {
		return pt.AlmostEqual(other), nil
	}
	return true, newCollisionTypeUnsupportedError(pt, g)
}

// CollidesWith checks if the given point collides with the given geometry and returns true if it does.
func (pt *point) DistanceFrom(g Geometry) (float64, error) {
	if other, ok := g.(*box); ok {
		return pointVsBoxDistance(other, pt.pose), nil
	}
	if other, ok := g.(*sphere); ok {
		return sphereVsPointDistance(other, pt.pose), nil
	}
	if other, ok := g.(*point); ok {
		return pt.pose.Sub(other.pose).Norm(), nil
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

// ToPointCloud converts a point geometry into a []r3.Vector
func (pt *point) ToPointCloud(options map[string]interface{}) []r3.Vector {
	return []r3.Vector{pt.pose}
}
