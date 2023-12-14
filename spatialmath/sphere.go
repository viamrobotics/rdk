package spatialmath

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/utils"
)

// Always use at least this many points to describe a sphere.
const defaultMinSpherePoints = 10.

// sphere is a collision geometry that represents a sphere, it has a pose and a radius that fully define it.
type sphere struct {
	pose   Pose
	radius float64
	label  string
}

// NewSphere instantiates a new sphere Geometry.
func NewSphere(offset Pose, radius float64, label string) (Geometry, error) {
	if radius <= 0 {
		return nil, newBadGeometryDimensionsError(&sphere{})
	}
	return &sphere{offset, radius, label}, nil
}

func (s sphere) MarshalJSON() ([]byte, error) {
	config, err := NewGeometryConfig(&s)
	if err != nil {
		return nil, err
	}
	config.Type = "sphere"
	config.R = s.radius
	return json.Marshal(config)
}

// String returns a human readable string that represents the sphere.
func (s *sphere) String() string {
	return fmt.Sprintf("Type: Sphere | Position: X:%.1f, Y:%.1f, Z:%.1f | Radius: %.0f",
		s.pose.Point().X, s.pose.Point().Y, s.pose.Point().Z, s.radius)
}

// Label returns the label of this sphere.
func (s *sphere) Label() string {
	return s.label
}

// SetLabel sets the label of this sphere.
func (s *sphere) SetLabel(label string) {
	if s != nil {
		s.label = label
	}
}

// Pose returns the pose of the sphere.
func (s *sphere) Pose() Pose {
	return s.pose
}

// AlmostEqual compares the sphere with another geometry and checks if they are equivalent.
func (s *sphere) AlmostEqual(g Geometry) bool {
	other, ok := g.(*sphere)
	if !ok {
		return false
	}
	return PoseAlmostEqualEps(s.pose, other.pose, 1e-6) && utils.Float64AlmostEqual(s.radius, other.radius, 1e-8)
}

// Transform premultiplies the sphere pose with a transform, allowing the sphere to be moved in space.
func (s *sphere) Transform(toPremultiply Pose) Geometry {
	return &sphere{Compose(toPremultiply, s.pose), s.radius, s.label}
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
		Label: s.label,
	}
}

// CollidesWith checks if the given sphere collides with the given geometry and returns true if it does.
func (s *sphere) CollidesWith(g Geometry) (bool, error) {
	if other, ok := g.(*sphere); ok {
		return sphereVsSphereDistance(s, other) <= CollisionBuffer, nil
	}
	if other, ok := g.(*capsule); ok {
		return capsuleVsSphereDistance(other, s) <= CollisionBuffer, nil
	}
	if other, ok := g.(*box); ok {
		return sphereVsBoxCollision(s, other), nil
	}
	if other, ok := g.(*point); ok {
		return sphereVsPointDistance(s, other.position) <= CollisionBuffer, nil
	}
	return true, newCollisionTypeUnsupportedError(s, g)
}

func (s *sphere) DistanceFrom(g Geometry) (float64, error) {
	if other, ok := g.(*box); ok {
		return sphereVsBoxDistance(s, other), nil
	}
	if other, ok := g.(*sphere); ok {
		return sphereVsSphereDistance(s, other), nil
	}
	if other, ok := g.(*capsule); ok {
		return capsuleVsSphereDistance(other, s), nil
	}
	if other, ok := g.(*point); ok {
		return sphereVsPointDistance(s, other.position), nil
	}
	return math.Inf(-1), newCollisionTypeUnsupportedError(s, g)
}

func (s *sphere) EncompassedBy(g Geometry) (bool, error) {
	if other, ok := g.(*sphere); ok {
		return sphereInSphere(s, other), nil
	}
	if other, ok := g.(*capsule); ok {
		return sphereInCapsule(s, other), nil
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
	return s.pose.Point().Sub(b.closestPoint(s.pose.Point())).Norm() <= s.radius+CollisionBuffer
}

// sphereVsBoxDistance takes a box and a sphere as arguments and returns a floating point number.  If this number is nonpositive it
// represents the penetration depth for the two geometries, which are in collision.  If the returned float is positive it represents the
// separation distance for the two geometries, which are not in collision.
func sphereVsBoxDistance(s *sphere, b *box) float64 {
	return pointVsBoxDistance(s.pose.Point(), b) - s.radius
}

// sphereInSphere returns a bool describing if the inner sphere is fully encompassed by the outer sphere.
func sphereInSphere(inner, outer *sphere) bool {
	return inner.pose.Point().Sub(outer.pose.Point()).Norm()+inner.radius <= outer.radius
}

// sphereInBox returns a bool describing if the given sphere is fully encompassed by the given box.
func sphereInBox(s *sphere, b *box) bool {
	return -pointVsBoxDistance(s.pose.Point(), b) >= s.radius
}

// sphereInCapsule returns a bool describing if the given sphere is fully encompassed by the given capsule.
func sphereInCapsule(s *sphere, c *capsule) bool {
	return -capsuleVsPointDistance(c, s.pose.Point()) >= s.radius
}

// ToPoints converts a sphere geometry into []r3.Vector. This method takes one argument which determines
// how many points per sqmm should be on the sphere's surface. If the argument is set to 0. we automatically
// substitute the value with defaultPointDensity.
func (s *sphere) ToPoints(resolution float64) []r3.Vector {
	// check for user defined spacing
	var iter float64
	area := 4. * math.Pi * s.radius * s.radius
	if resolution != 0. {
		iter = area * resolution
	} else {
		iter = area / defaultPointDensity // default spacing
	}
	if iter < defaultMinSpherePoints {
		iter = defaultMinSpherePoints
	}
	// code taken from: https://stackoverflow.com/questions/9600801/evenly-distributing-n-points-on-a-sphere
	// we want the number of points on the sphere's surface to grow in proportion with the sphere's radius
	phi := math.Pi * (3.0 - math.Sqrt(5.0)) // golden angle in radians
	iterInt := int(iter)
	var vecList []r3.Vector
	for i := 0; i < iterInt; i++ {
		y := 1 - (float64(i)/float64(iterInt-1))*2 // y goes from 1 to -1
		radius := math.Sqrt(1 - y*y)               // radius at y
		// Account for floating point error
		if y*y > 1 {
			radius = 0
		}
		theta := phi * float64(i) // golden angle increment
		x := (math.Cos(theta) * radius) * s.radius
		z := (math.Sin(theta) * radius) * s.radius
		vec := r3.Vector{x, y * s.radius, z}
		vecList = append(vecList, vec)
	}
	return transformPointsToPose(vecList, s.Pose())
}
