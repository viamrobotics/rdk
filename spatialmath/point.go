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
func (pt *point) almostEqual(g Geometry) bool {
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

// CollidesWith checks if the given point collides with the given geometry and returns true if it
// does. If there's no collision, the method will return the distance between the point and input
// geometry. If there is a collision, a negative number is returned.
func (pt *point) CollidesWith(g Geometry, collisionBufferMM float64) (bool, float64, error) {
	switch other := g.(type) {
	case *Mesh:
		return other.CollidesWith(pt, collisionBufferMM)
	case *box:
		c, d := pointVsBoxCollision(pt.position, other, collisionBufferMM)
		return c, d, nil
	case *sphere:
		// Point-sphere distance is cheap
		dist := sphereVsPointDistance(other, pt.position)
		if dist <= collisionBufferMM {
			return true, -1, nil
		}
		return false, dist, nil
	case *capsule:
		// Point-capsule distance is cheap
		dist := capsuleVsPointDistance(other, pt.position)
		if dist <= collisionBufferMM {
			return true, -1, nil
		}
		return false, dist, nil
	case *point:
		// Point-point distance is cheap
		dist := pt.position.Sub(other.position).Norm()
		if dist <= collisionBufferMM {
			return true, -1, nil
		}
		return false, dist, nil
	default:
		return true, collisionBufferMM, newCollisionTypeUnsupportedError(pt, g)
	}
}

// CollidesWith checks if the given point collides with the given geometry and returns true if it does.
func (pt *point) DistanceFrom(g Geometry) (float64, error) {
	switch other := g.(type) {
	case *Mesh:
		return other.DistanceFrom(pt)
	case *box:
		return pointVsBoxDistance(pt.position, other), nil
	case *sphere:
		return sphereVsPointDistance(other, pt.position), nil
	case *capsule:
		return capsuleVsPointDistance(other, pt.position), nil
	case *point:
		return pt.position.Sub(other.position).Norm(), nil
	default:
		return math.Inf(-1), newCollisionTypeUnsupportedError(pt, g)
	}
}

// EncompassedBy returns a bool describing if the given point is completely encompassed by the given geometry.
func (pt *point) EncompassedBy(g Geometry) (bool, error) {
	collides, _, err := pt.CollidesWith(g, defaultCollisionBufferMM)
	return collides, err
}

// pointVsBoxCollision takes a box and a point as arguments and returns a bool describing if they are in collision. \
// true == collision / false == no collision.
func pointVsBoxCollision(pt r3.Vector, b *box, collisionBufferMM float64) (bool, float64) {
	d := b.closestPoint(pt).Sub(pt).Norm()
	return d <= collisionBufferMM, d
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

// Hash returns a hash value for this point.
func (pt *point) Hash() int {
	hash := 0
	pos := pt.position
	hash += (5 * (int(pos.X*10) + 1000)) * 2
	hash += (6 * (int(pos.Y*10) + 10221)) * 3
	hash += (7 * (int(pos.Z*10) + 2124)) * 4
	hash += hashString(pt.label) * 11
	return hash
}

func hashString(s string) int {
	hash := 0
	for idx, c := range s {
		hash += ((idx + 1) * 7) + ((int(c) + 12) * 12)
	}
	return hash
}

// ToPointCloud converts a point geometry into a []r3.Vector.
func (pt *point) ToPoints(resolution float64) []r3.Vector {
	return []r3.Vector{pt.position}
}
