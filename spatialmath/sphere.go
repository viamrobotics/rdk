package spatialmath

import (
	"encoding/json"
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/utils"
)

// SphereCreator implements the VolumeCreator interface for sphere structs.
type sphereCreator struct {
	radius float64
	offset Pose
}

// sphere is a collision geometry that represents a sphere, it has a pose and a radius that fully define it.
type sphere struct {
	radius float64
	pose   Pose
}

// NewSphere instantiates a SphereCreator class, which allows instantiating spheres given only a pose which is applied
// at the specified offset from the pose. These spheres have a radius specified by the radius argument.
func NewSphere(radius float64, offset Pose) (VolumeCreator, error) {
	if radius <= 0 {
		return nil, errors.New("sphere dimensions can not be zero")
	}
	return &sphereCreator{radius, offset}, nil
}

// NewVolume instantiates a new sphere from a SphereCreator class.
func (sc *sphereCreator) NewVolume(pose Pose) Volume {
	s := &sphere{sc.radius, sc.offset}
	s.Transform(pose)
	return s
}

func (sc *sphereCreator) MarshalJSON() ([]byte, error) {
	return json.Marshal(VolumeConfig{
		Type: "sphere",
		R:    sc.radius,
	})
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

// AlmostEqual compares the sphere with another volume and checks if they are equivalent.
func (s *sphere) AlmostEqual(v Volume) bool {
	other, ok := v.(*sphere)
	if !ok {
		return false
	}
	return PoseAlmostEqual(s.pose, other.pose) && utils.Float64AlmostEqual(s.radius, other.radius, 1e-8)
}

// Transform premultiplies the sphere pose with a transform, allowing the sphere to be moved in space.
func (s *sphere) Transform(toPremultiply Pose) {
	s.pose = Compose(toPremultiply, s.pose)
}

// CollidesWith checks if the given sphere collides with the given volume and returns true if it does.
func (s *sphere) CollidesWith(v Volume) (bool, error) {
	if other, ok := v.(*sphere); ok {
		return sphereVsSphereDistance(s, other) <= 0, nil
	}
	if other, ok := v.(*box); ok {
		return sphereVsBoxCollision(s, other), nil
	}
	return true, errors.Errorf("collisions between sphere and %T are not supported", v)
}

// CollidesWith checks if the given sphere collides with the given volume and returns true if it does.
func (s *sphere) DistanceFrom(v Volume) (float64, error) {
	if other, ok := v.(*sphere); ok {
		return sphereVsSphereDistance(s, other), nil
	}
	if other, ok := v.(*box); ok {
		return sphereVsBoxDistance(s, other), nil
	}
	return math.Inf(-1), errors.Errorf("collisions between sphere and %T are not supported", v)
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
// represents the penetration depth for the two volumes, which are in collision.  If the returned float is positive it represents the
// separation distance for the two volumes, which are not in collision.
func sphereVsBoxDistance(s *sphere, b *box) float64 {
	closestPoint := b.closestPoint(s.pose.Point())
	sphereToBoxDistance := s.pose.Point().Sub(closestPoint)
	boxToFaceDistance := 0.
	if R3VectorAlmostEqual(sphereToBoxDistance, r3.Vector{}, 1e-3) {
		boxToFaceDistance = boxVsPointDistance(b, closestPoint)
	}
	return sphereToBoxDistance.Norm() - (s.radius + boxToFaceDistance)
}
