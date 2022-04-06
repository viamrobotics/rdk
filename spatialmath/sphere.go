package spatialmath

import (
	"encoding/json"
	"math"

	"github.com/golang/geo/r3"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/utils"
)

// SphereCreator implements the GeometryCreator interface for sphere structs.
type sphereCreator struct {
	radius float64
	offset Pose
}

// sphere is a collision geometry that represents a sphere, it has a pose and a radius that fully define it.
type sphere struct {
	radius float64
	pose   Pose
}

// NewSphereCreator instantiates a SphereCreator class, which allows instantiating spheres given only a pose which is applied
// at the specified offset from the pose. These spheres have a radius specified by the radius argument.
func NewSphereCreator(radius float64, offset Pose) (GeometryCreator, error) {
	if radius <= 0 {
		return nil, newBadGeometryDimensionsError(&sphere{})
	}
	return &sphereCreator{radius, offset}, nil
}

// NewGeometry instantiates a new sphere from a SphereCreator class.
func (sc *sphereCreator) NewGeometry(pose Pose) Geometry {
	s := &sphere{sc.radius, sc.offset}
	s.Transform(pose)
	return s
}

func (sc *sphereCreator) MarshalJSON() ([]byte, error) {
	config, err := NewGeometryConfig(sc.offset)
	if err != nil {
		return nil, err
	}
	config.Type = "sphere"
	config.R = sc.radius
	return json.Marshal(config)
}

// NewSphere instantiates a new sphere Geometry.
func NewSphere(pt r3.Vector, radius float64) (Geometry, error) {
	if radius < 0 {
		return nil, newBadGeometryDimensionsError(&sphere{})
	}
	return &sphere{radius, NewPoseFromPoint(pt)}, nil
}

// Pose returns the pose of the sphere.
func (s *sphere) Pose() Pose {
	return s.pose
}

// Vertices returns the point defining the center of the sphere (because it would take an infinite number of points to accurately define a
// the bounding geometry of a sphere, the center point returned in this function along with the known radius should be used).
func (s *sphere) Vertices() []r3.Vector {
	return []r3.Vector{s.pose.Point()}
}

// AlmostEqual compares the sphere with another geometry and checks if they are equivalent.
func (s *sphere) AlmostEqual(g Geometry) bool {
	other, ok := g.(*sphere)
	if !ok {
		return false
	}
	return PoseAlmostEqual(s.pose, other.pose) && utils.Float64AlmostEqual(s.radius, other.radius, 1e-8)
}

// Transform premultiplies the sphere pose with a transform, allowing the sphere to be moved in space.
func (s *sphere) Transform(toPremultiply Pose) {
	s.pose = Compose(toPremultiply, s.pose)
}

// ToProto converts the sphere to a Geometry proto message.
func (s *sphere) ToProtobuf() *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: PoseToProtobuf(s.pose),
		GeometryType: &commonpb.Geometry_Sphere{
			Sphere: &commonpb.Sphere{
				RadiusMm: s.radius,
			},
		},
	}
}

// CollidesWith checks if the given sphere collides with the given geometry and returns true if it does.
func (s *sphere) CollidesWith(g Geometry) (bool, error) {
	if other, ok := g.(*sphere); ok {
		return sphereVsSphereDistance(s, other) <= 0, nil
	}
	if other, ok := g.(*box); ok {
		return sphereVsBoxCollision(s, other), nil
	}
	if other, ok := g.(*point); ok {
		return sphereVsPointDistance(s, other.pose.Point()) <= 0, nil
	}
	return true, newCollisionTypeUnsupportedError(s, g)
}

// CollidesWith checks if the given sphere collides with the given geometry and returns true if it does.
func (s *sphere) DistanceFrom(g Geometry) (float64, error) {
	if other, ok := g.(*box); ok {
		return sphereVsBoxDistance(s, other), nil
	}
	if other, ok := g.(*sphere); ok {
		return sphereVsSphereDistance(s, other), nil
	}
	if other, ok := g.(*point); ok {
		return sphereVsPointDistance(s, other.pose.Point()), nil
	}
	return math.Inf(-1), newCollisionTypeUnsupportedError(s, g)
}

func (s *sphere) EncompassedBy(g Geometry) (bool, error) {
	if other, ok := g.(*sphere); ok {
		return sphereInSphere(s, other), nil
	}
	if other, ok := g.(*box); ok {
		return sphereInBox(s, other), nil
	}
	if _, ok := g.(*point); ok {
		return false, nil
	}
	return true, newCollisionTypeUnsupportedError(s, g)
}

// sphereVsPointDistance takes a sphere and a point as arguments and returns a floating point number.  If this number is nonpositive it
// represents the penetration depth of the point within the sphere.  If the returned float is positive it represents the separation
// distance between the point and the sphere, which are not in collision.
func sphereVsPointDistance(s *sphere, pt r3.Vector) float64 {
	return s.pose.Point().Sub(pt).Norm() - s.radius
}

// sphereVsSphereCollision takes two spheres as arguments and returns a floating point number.  If this number is nonpositive it represents
// the penetration depth for the two spheres, which are in collision.  If the returned float is positive it represents the diestance
// between the spheres, which are not in collision
// reference: https://studiofreya.com/3d-math-and-physics/simple-sphere-sphere-collision-detection-and-collision-response/
func sphereVsSphereDistance(a, s *sphere) float64 {
	return a.pose.Point().Sub(s.pose.Point()).Norm() - (a.radius + s.radius)
}

// sphereVsBoxDistance takes a box and a sphere as arguments and returns a bool describing if they are in collision
// true == collision / false == no collision.
// Reference: https://github.com/gszauer/GamePhysicsCookbook/blob/a0b8ee0c39fed6d4b90bb6d2195004dfcf5a1115/Code/Geometry3D.cpp#L326
func sphereVsBoxCollision(s *sphere, b *box) bool {
	return s.pose.Point().Sub(b.closestPoint(s.pose.Point())).Norm() <= s.radius
}

// sphereVsBoxDistance takes a box and a sphere as arguments and returns a floating point number.  If this number is nonpositive it
// represents the penetration depth for the two geometries, which are in collision.  If the returned float is positive it represents the
// separation distance for the two geometries, which are not in collision.
func sphereVsBoxDistance(s *sphere, b *box) float64 {
	return pointVsBoxDistance(b, s.pose.Point()) - s.radius
}

// sphereInSphere returns a bool describing if the inner sphere is fully encompassed by the outer sphere.
func sphereInSphere(inner, outer *sphere) bool {
	return inner.pose.Point().Sub(outer.pose.Point()).Norm()+inner.radius <= outer.radius
}

// sphereInBox returns a bool describing if the given sphere is fully encompassed by the given box.
func sphereInBox(s *sphere, b *box) bool {
	return -pointVsBoxDistance(b, s.pose.Point()) >= s.radius
}
