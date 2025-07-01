package spatialmath

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
)

// points is a collision geometry that represents a collection of 3D points, it has a pose and points that fully define it.
type points struct {
	pose   Pose
	label  string
	points []r3.Vector
}

// NewPoints instantiates a new points Geometry.
func NewPoints(pose Pose, pts []r3.Vector, label string) (Geometry, error) {
	if pose == nil {
		return nil, newBadGeometryDimensionsError(&points{})
	}
	if pts == nil {
		return nil, newBadGeometryDimensionsError(&points{})
	}
	return &points{
		pose:   pose,
		points: pts,
		label:  label,
	}, nil
}

// String returns a human readable string that represents the points.
func (pts *points) String() string {
	return fmt.Sprintf("Type: Points | Position: X:%.1f, Y:%.1f, Z:%.1f | Points: %v",
		pts.pose.Point().X, pts.pose.Point().Y, pts.pose.Point().Z, pts.points)
}

// MarshalJSON converts the points to a JSON object.
func (pts *points) MarshalJSON() ([]byte, error) {
	config, err := NewGeometryConfig(pts)
	if err != nil {
		return nil, err
	}
	return json.Marshal(config)
}

// SetLabel sets the label of this points.
func (pts *points) SetLabel(label string) {
	pts.label = label
}

// Label returns the label of this points.
func (pts *points) Label() string {
	return pts.label
}

// Pose returns the pose of the points.
func (pts *points) Pose() Pose {
	return pts.pose
}

// AlmostEqual compares the points with another geometry and checks if they are equivalent.
func (pts *points) almostEqual(g Geometry) bool {
	other, ok := g.(*points)
	if !ok {
		return false
	}
	return PoseAlmostEqualEps(pts.pose, other.pose, 1e-6)
}

// Transform premultiplies the points pose with a transform, allowing the points to be moved in space.
func (pts *points) Transform(toPremultiply Pose) Geometry {
	transformedPoints := make([]r3.Vector, len(pts.points))
	for i, p := range pts.points {
		transformedPoints[i] = Compose(toPremultiply, NewPoseFromPoint(p)).Point()
	}

	return &points{
		pose:   Compose(toPremultiply, pts.pose),
		points: transformedPoints,
		label:  pts.label,
	}
}

// ToProtobuf converts the points to a Points proto message.
func (pts *points) ToProtobuf() *commonpb.Geometry {
	length := len(pts.points)
	array := make([]float32, length*3)
	for i := range length {
		array[i*3] = float32(pts.points[i].X)
		array[i*3+1] = float32(pts.points[i].Y)
		array[i*3+2] = float32(pts.points[i].Z)
	}
	return &commonpb.Geometry{
		Center: PoseToProtobuf(pts.pose),
		GeometryType: &commonpb.Geometry_Points{
			Points: &commonpb.Points{
				Array: array,
			},
		},
		Label: pts.label,
	}
}

// CollidesWith checks if the given points collides with the given geometry and returns true if it does.
func (pts *points) CollidesWith(geometry Geometry, collisionBufferMM float64) (bool, error) {
	switch other := geometry.(type) {
	case *Mesh:
		return other.CollidesWith(pts, collisionBufferMM)
	case *box:
		return pointsVsBoxCollision(pts, other, collisionBufferMM), nil
	case *sphere:
		return pointsVsSphereDistance(pts, other) <= collisionBufferMM, nil
	case *capsule:
		return pointsVsCapsuleDistance(pts, other) <= collisionBufferMM, nil
	case *point:
		return pointsVsPointDistance(pts, other) <= collisionBufferMM, nil
	case *points:
		return pointsVsPointsDistance(pts, other) <= collisionBufferMM, nil
	case *line:
		return pointsVsLineDistance(pts, other) <= collisionBufferMM, nil
	default:
		return true, newCollisionTypeUnsupportedError(pts, geometry)
	}
}

// DistanceFrom calculates the distance from the points to the given geometry.
func (pts *points) DistanceFrom(geometry Geometry) (float64, error) {
	switch other := geometry.(type) {
	case *Mesh:
		return other.DistanceFrom(pts)
	case *box:
		return pointsVsBoxDistance(pts, other), nil
	case *sphere:
		return pointsVsSphereDistance(pts, other), nil
	case *capsule:
		return pointsVsCapsuleDistance(pts, other), nil
	case *point:
		return pointsVsPointDistance(pts, other), nil
	case *points:
		return pointsVsPointsDistance(pts, other), nil
	case *line:
		return pointsVsLineDistance(pts, other), nil
	default:
		return math.Inf(-1), newCollisionTypeUnsupportedError(pts, geometry)
	}
}

// EncompassedBy checks if the points are completely contained within the given geometry.
func (pts *points) EncompassedBy(geometry Geometry) (bool, error) {
	switch other := geometry.(type) {
	case *Mesh:
		return false, nil
	case *box:
		for _, p := range pts.points {
			if pointVsBoxDistance(p, other) > defaultCollisionBufferMM {
				return false, nil
			}
		}
		return true, nil
	case *sphere:
		for _, p := range pts.points {
			if sphereVsPointDistance(other, p) > defaultCollisionBufferMM {
				return false, nil
			}
		}
		return true, nil
	case *capsule:
		for _, p := range pts.points {
			if capsuleVsPointDistance(other, p) > defaultCollisionBufferMM {
				return false, nil
			}
		}
		return true, nil
	case *point:
		return false, nil
	case *points:
		return false, nil
	case *line:
		return false, nil
	default:
		return false, newCollisionTypeUnsupportedError(pts, geometry)
	}
}

// ToPoints returns the points of the points.
func (pts *points) ToPoints(resolution float64) []r3.Vector {
	return pts.points
}

// pointsVsBoxCollision checks if any point is on or inside the box.
func pointsVsBoxCollision(pts *points, box *box, collisionBufferMM float64) bool {
	for _, p := range pts.points {
		if pointVsBoxCollision(p, box, collisionBufferMM) {
			return true
		}
	}
	return false
}

// pointsVsBoxDistance calculates the distance from the points to the given box.
func pointsVsBoxDistance(pts *points, box *box) float64 {
	var minDistance float64 = math.Inf(1)
	for _, p := range pts.points {
		distance := pointVsBoxDistance(p, box)
		if distance <= 0 {
			return 0
		}
		if distance < minDistance {
			minDistance = distance
		}
	}
	return minDistance
}

// pointsVsSphereDistance calculates the distance from the points to the given sphere.
func pointsVsSphereDistance(pts *points, sphere *sphere) float64 {
	var minDistance float64 = math.Inf(1)
	for _, p := range pts.points {
		distance := sphereVsPointDistance(sphere, p)
		if distance <= 0 {
			return 0
		}
		if distance < minDistance {
			minDistance = distance
		}
	}
	return minDistance
}

// pointsVsCapsuleDistance calculates the distance from the points to the given capsule.
func pointsVsCapsuleDistance(pts *points, capsule *capsule) float64 {
	var minDistance float64 = math.Inf(1)
	for _, p := range pts.points {
		distance := capsuleVsPointDistance(capsule, p)
		if distance <= 0 {
			return 0
		}
		if distance < minDistance {
			minDistance = distance
		}
	}
	return minDistance
}

// pointsVsPointDistance calculates the distance from the points to the given point.
func pointsVsPointDistance(pts *points, targetPoint *point) float64 {
	var minDistance float64 = math.Inf(1)
	Q := targetPoint.position
	for _, p := range pts.points {
		distance := p.Sub(Q).Norm()
		if distance == 0 {
			return 0
		}
		if distance < minDistance {
			minDistance = distance
		}
	}
	return minDistance
}

// pointsVsPointsDistance calculates the distance from the points to the given points.
func pointsVsPointsDistance(pts *points, other *points) float64 {
	var minDistance float64 = math.Inf(1)
	for _, p := range pts.points {
		for _, q := range other.points {
			distance := p.Sub(q).Norm()
			if distance == 0 {
				return 0
			}
			if distance < minDistance {
				minDistance = distance
			}
		}
	}
	return minDistance
}

// pointsVsLineDistance calculates the distance from the points to the given line.
func pointsVsLineDistance(pts *points, line *line) float64 {
	var minDistance float64 = math.Inf(1)
	for _, p := range pts.points {
		// Check distance to each line segment
		for i := range len(line.segments) - 1 {
			p0 := line.segments[i]
			p1 := line.segments[i+1]
			distance := DistToLineSegment(p0, p1, p)
			if distance == 0 {
				return 0
			}
			if distance < minDistance {
				minDistance = distance
			}
		}
	}
	return minDistance
}
