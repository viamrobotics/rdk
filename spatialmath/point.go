package spatialmath

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
)

// point is a collision geometry that represents a single point in 3D space that occupies no geometry.
type point struct {
	position r3.Vector
	label    string
}

// NewPoint instantiates a new point Geometry.
func NewPoint(pt r3.Vector, label string) Geometry {
	return &point{pt, label}
}

// String returns a human readable string that represents the pointCreator.
func (pt *point) String() string {
	p := pt.position
	return fmt.Sprintf("Type: Point, Location X:%.0f, Y:%.0f, Z:%.0f", p.X, p.Y, p.Z)
}

func (pt point) MarshalJSON() ([]byte, error) {
	config, err := NewGeometryConfig(&pt)
	if err != nil {
		return nil, err
	}
	return json.Marshal(config)
}

// Label returns the labels of this point.
func (pt *point) Label() string {
	return pt.label
}

// SetLabel sets the label of this point.
func (pt *point) SetLabel(label string) {
	pt.label = label
}

// Pose returns the pose of the point.
func (pt *point) Pose() Pose {
	return NewPoseFromPoint(pt.position)
}

// AlmostEqual compares the point with another geometry and checks if they are equivalent.
func (pt *point) AlmostEqual(g Geometry) bool {
	other, ok := g.(*point)
	if !ok {
		return false
	}
	return pt.position.ApproxEqual(other.position)
}

// Transform premultiplies the point pose with a transform, allowing the point to be moved in space.
func (pt *point) Transform(toPremultiply Pose) Geometry {
	return &point{Compose(toPremultiply, NewPoseFromPoint(pt.position)).Point(), pt.label}
}

// ToProto converts the point to a Geometry proto message.
func (pt *point) ToProtobuf() *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: PoseToProtobuf(NewPoseFromPoint(pt.position)),
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
		return pointVsBoxCollision(pt.position, other), nil
	}
	if other, ok := g.(*sphere); ok {
		return sphereVsPointDistance(other, pt.position) <= 0, nil
	}
	if other, ok := g.(*capsule); ok {
		return capsuleVsPointDistance(other, pt.position) <= 0, nil
	}
	if other, ok := g.(*point); ok {
		return pt.AlmostEqual(other), nil
	}
	return true, newCollisionTypeUnsupportedError(pt, g)
}

// CollidesWith checks if the given point collides with the given geometry and returns true if it does.
func (pt *point) DistanceFrom(g Geometry) (float64, error) {
	if other, ok := g.(*box); ok {
		return pointVsBoxDistance(pt.position, other), nil
	}
	if other, ok := g.(*sphere); ok {
		return sphereVsPointDistance(other, pt.position), nil
	}
	if other, ok := g.(*capsule); ok {
		return capsuleVsPointDistance(other, pt.position), nil
	}
	if other, ok := g.(*point); ok {
		return pt.position.Sub(other.position).Norm(), nil
	}
	return math.Inf(-1), newCollisionTypeUnsupportedError(pt, g)
}

// EncompassedBy returns a bool describing if the given point is completely encompassed by the given geometry.
func (pt *point) EncompassedBy(g Geometry) (bool, error) {
	return pt.CollidesWith(g)
}

// pointVsBoxCollision takes a box and a point as arguments and returns a bool describing if they are in collision. \
// true == collision / false == no collision.
func pointVsBoxCollision(pt r3.Vector, b *box) bool {
	//~ fmt.Println("close", b.closestPoint(pt).Sub(pt))
	//~ fmt.Println("depth", b.pointPenetrationDepth(pt))
	return b.closestPoint(pt).Sub(pt).Norm() <= CollisionBuffer
}

// pointVsBoxDistance takes a box and a point as arguments and returns a floating point number.  If this number is nonpositive it represents
// the penetration depth of the point within the box.  If the returned float is positive it represents the separation distance between the
// point and the box, which are not in collision.
func pointVsBoxDistance(pt r3.Vector, b *box) float64 {
	distance := b.closestPoint(pt).Sub(pt).Norm()
	if distance > 0 {
		return distance
	}
	return -b.pointPenetrationDepth(pt)
}

// ToPointCloud converts a point geometry into a []r3.Vector.
func (pt *point) ToPoints(resolution float64) []r3.Vector {
	return []r3.Vector{pt.position}
}
