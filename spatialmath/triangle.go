package spatialmath

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
)

// Triangle is three points and a normal vector.
// Triangle implements the Geometry interface.
type Triangle struct {
	p0 r3.Vector
	p1 r3.Vector
	p2 r3.Vector

	normal r3.Vector
	label  string
}

// NewTriangle creates a Triangle from three points. The Normal is computed; directionality is random.
func NewTriangle(p0, p1, p2 r3.Vector) *Triangle {
	return &Triangle{
		p0:     p0,
		p1:     p1,
		p2:     p2,
		normal: PlaneNormal(p0, p1, p2),
		label:  "",
	}
}

// NewLabeledTriangle creates a Triangle with a label.
func NewLabeledTriangle(p0, p1, p2 r3.Vector, label string) *Triangle {
	return &Triangle{
		p0:     p0,
		p1:     p1,
		p2:     p2,
		normal: PlaneNormal(p0, p1, p2),
		label:  label,
	}
}

// Points returns the three points associated with the triangle.
func (t *Triangle) Points() []r3.Vector {
	return []r3.Vector{t.p0, t.p1, t.p2}
}

// Normal returns the triangle's normal vector.
func (t *Triangle) Normal() r3.Vector {
	return t.normal
}

// Area calculates the area of a triangle using the cross product method.
// Returns the area as a float64.
func (t *Triangle) Area() float64 {
	pts := t.Points()
	// Calculate two edges
	edge1 := pts[1].Sub(pts[0])
	edge2 := pts[2].Sub(pts[0])
	// Area is half the magnitude of the cross product of two edges
	return edge1.Cross(edge2).Norm() / 2.0
}

// Centroid calculates the point that represents the centroid of the triangle.
func (t *Triangle) Centroid() r3.Vector {
	return t.p0.Add(t.p1).Add(t.p2).Mul(1. / 3.)
}

// Transform premultiplies the triangle's points with a transform, allowing the triangle to be moved in space.
func (t *Triangle) Transform(toPremultiply Pose) Geometry {
	return NewLabeledTriangle(
		Compose(toPremultiply, NewPoseFromPoint(t.p0)).Point(),
		Compose(toPremultiply, NewPoseFromPoint(t.p1)).Point(),
		Compose(toPremultiply, NewPoseFromPoint(t.p2)).Point(),
		t.label,
	)
}

// ClosestTriangleInsidePoint returns the closest point on a triangle IF AND ONLY IF the query point's projection overlaps the triangle.
// Otherwise it will return the query point.
// To visualize this- if one draws a tetrahedron using the triangle and the query point, all angles from the triangle to the query point
// must be <= 90 degrees.
func ClosestTriangleInsidePoint(t *Triangle, point r3.Vector) (r3.Vector, bool) {
	eps := 1e-6

	// Parametrize the triangle s.t. a point inside the triangle is
	// Q = p0 + u * e0 + v * e1, when 0 <= u <= 1, 0 <= v <= 1, and
	// 0 <= u + v <= 1. Let e0 = (p1 - p0) and e1 = (p2 - p0).
	// We analytically minimize the distance between the point pt and Q.
	e0 := t.p1.Sub(t.p0)
	e1 := t.p2.Sub(t.p0)
	a := e0.Norm2()
	b := e0.Dot(e1)
	c := e1.Norm2()
	d := point.Sub(t.p0)
	// The determinant is 0 only if the angle between e1 and e0 is 0
	// (i.e. the triangle has overlapping lines).
	det := (a*c - b*b)
	u := (c*e0.Dot(d) - b*e1.Dot(d)) / det
	v := (-b*e0.Dot(d) + a*e1.Dot(d)) / det
	inside := (0 <= u+eps) && (u <= 1+eps) && (0 <= v+eps) && (v <= 1+eps) && (u+v <= 1+eps)
	return t.p0.Add(e0.Mul(u)).Add(e1.Mul(v)), inside
}

// ClosestPointTrianglePoint takes a point, and returns the closest point on the triangle to the given point.
func ClosestPointTrianglePoint(t *Triangle, point r3.Vector) r3.Vector {
	pts := t.Points()
	closestPtInside, inside := ClosestTriangleInsidePoint(t, point)
	if inside {
		return closestPtInside
	}

	// If the closest point is outside the triangle, it must be on an edge, so we
	// check each triangle edge for a closest point to the point pt.
	closestPt := ClosestPointSegmentPoint(pts[0], pts[1], point)
	bestDist := point.Sub(closestPt).Norm2()

	newPt := ClosestPointSegmentPoint(pts[1], pts[2], point)
	if newDist := point.Sub(newPt).Norm2(); newDist < bestDist {
		closestPt = newPt
		bestDist = newDist
	}

	newPt = ClosestPointSegmentPoint(pts[2], pts[0], point)
	if newDist := point.Sub(newPt).Norm2(); newDist < bestDist {
		return newPt
	}
	return closestPt
}

// Pose returns a pose representing the triangle's position and orientation.
// Position is the centroid, orientation is derived from the normal vector.
func (t *Triangle) Pose() Pose {
	return NewPose(t.Centroid(), &OrientationVector{OX: t.normal.X, OY: t.normal.Y, OZ: t.normal.Z})
}

// Label returns the label of this triangle.
func (t *Triangle) Label() string {
	return t.label
}

// SetLabel sets the label of this triangle.
func (t *Triangle) SetLabel(label string) {
	if t != nil {
		t.label = label
	}
}

// String returns a human readable string that represents the triangle.
func (t *Triangle) String() string {
	return fmt.Sprintf("Type: Triangle | P0: (%.1f, %.1f, %.1f) | P1: (%.1f, %.1f, %.1f) | P2: (%.1f, %.1f, %.1f)",
		t.p0.X, t.p0.Y, t.p0.Z, t.p1.X, t.p1.Y, t.p1.Z, t.p2.X, t.p2.Y, t.p2.Z)
}

// CollidesWith checks if this triangle collides with another geometry.
func (t *Triangle) CollidesWith(g Geometry, collisionBufferMM float64) (bool, float64, error) {
	switch other := g.(type) {
	case *Triangle:
		collides, dist := t.collidesWithTriangle(other, collisionBufferMM)
		return collides, dist, nil
	case *sphere:
		collides, dist := t.collidesWithSphere(other, collisionBufferMM)
		return collides, dist, nil
	case *box:
		return t.collidesWithBox(other, collisionBufferMM)
	case *capsule:
		collides, dist := t.collidesWithCapsule(other, collisionBufferMM)
		return collides, dist, nil
	case *point:
		return t.collidesWithPoint(other, collisionBufferMM)
	case *Mesh:
		// Delegate to mesh (which iterates its triangles)
		return other.CollidesWith(t, collisionBufferMM)
	default:
		return true, collisionBufferMM, newCollisionTypeUnsupportedError(t, g)
	}
}

// collidesWithTriangle checks collision between two triangles.
func (t *Triangle) collidesWithTriangle(other *Triangle, collisionBufferMM float64) (bool, float64) {
	p1 := t.Points()
	p2 := other.Points()
	minDist := math.Inf(1)

	// Check segments from t against other
	for i := 0; i < 3; i++ {
		start := p1[i]
		end := p1[(i+1)%3]
		bestSegPt, bestTriPt := ClosestPointsSegmentTriangle(start, end, other)
		dist := bestSegPt.Sub(bestTriPt).Norm()
		if dist <= collisionBufferMM {
			return true, -1
		}
		if dist < minDist {
			minDist = dist
		}
	}

	// Check segments from other against t
	for i := 0; i < 3; i++ {
		start := p2[i]
		end := p2[(i+1)%3]
		bestSegPt, bestTriPt := ClosestPointsSegmentTriangle(start, end, t)
		dist := bestSegPt.Sub(bestTriPt).Norm()
		if dist <= collisionBufferMM {
			return true, -1
		}
		if dist < minDist {
			minDist = dist
		}
	}

	return false, minDist
}

// collidesWithSphere checks if triangle collides with a sphere.
func (t *Triangle) collidesWithSphere(s *sphere, collisionBufferMM float64) (bool, float64) {
	closestPt := ClosestPointTrianglePoint(t, s.Pose().Point())
	dist := closestPt.Sub(s.Pose().Point()).Norm() - s.radius
	if dist <= collisionBufferMM {
		return true, -1
	}
	return false, dist
}

// collidesWithBox checks if triangle collides with a box.
func (t *Triangle) collidesWithBox(b *box, collisionBufferMM float64) (bool, float64, error) {
	// Convert box to mesh and check collision
	boxMesh := b.toMesh()
	return boxMesh.CollidesWith(t, collisionBufferMM)
}

// collidesWithCapsule checks if triangle collides with a capsule.
func (t *Triangle) collidesWithCapsule(c *capsule, collisionBufferMM float64) (bool, float64) {
	// Find closest points between capsule's central segment and triangle
	segA := c.segA
	segB := c.segB
	bestSegPt, bestTriPt := ClosestPointsSegmentTriangle(segA, segB, t)
	dist := bestSegPt.Sub(bestTriPt).Norm() - c.radius
	if dist <= collisionBufferMM {
		return true, -1
	}
	return false, dist
}

// collidesWithPoint checks if triangle collides with a point.
func (t *Triangle) collidesWithPoint(p *point, collisionBufferMM float64) (bool, float64, error) {
	closestPt := ClosestPointTrianglePoint(t, p.position)
	dist := closestPt.Sub(p.position).Norm()
	if dist <= collisionBufferMM {
		return true, -1, nil
	}
	return false, dist, nil
}

// DistanceFrom returns the minimum distance from this triangle to another geometry.
func (t *Triangle) DistanceFrom(g Geometry) (float64, error) {
	switch other := g.(type) {
	case *Triangle:
		_, dist := t.collidesWithTriangle(other, 0)
		return dist, nil
	case *sphere:
		_, dist := t.collidesWithSphere(other, 0)
		return dist, nil
	case *point:
		_, dist, err := t.collidesWithPoint(other, 0)
		if err != nil {
			return math.Inf(1), err
		}
		return dist, nil
	case *capsule:
		_, dist := t.collidesWithCapsule(other, 0)
		return dist, nil
	case *box:
		_, dist, err := t.collidesWithBox(other, 0)
		if err != nil {
			return math.Inf(1), err
		}
		return dist, err
	case *Mesh:
		return other.DistanceFrom(t)
	default:
		return math.Inf(-1), newCollisionTypeUnsupportedError(t, g)
	}
}

// EncompassedBy returns true if all three vertices are inside the other geometry.
func (t *Triangle) EncompassedBy(g Geometry) (bool, error) {
	switch other := g.(type) {
	case *point:
		return false, nil
	case *Mesh:
		return false, nil // Meshes have no volume
	case *Triangle:
		return false, nil // Triangles have no volume
	case *sphere, *box, *capsule:
		// Check if all 3 points collide with the geometry (are inside)
		for _, pt := range t.Points() {
			pointGeom := NewPoint(pt, "")
			collides, _, err := pointGeom.CollidesWith(other, defaultCollisionBufferMM)
			if err != nil {
				return false, err
			}
			if !collides {
				return false, nil
			}
		}
		return true, nil
	default:
		return false, newCollisionTypeUnsupportedError(t, g)
	}
}

// ToPoints converts a triangle geometry into []r3.Vector.
// Returns the three vertices of the triangle.
func (t *Triangle) ToPoints(resolution float64) []r3.Vector {
	return t.Points()
}

// ToProtobuf converts the triangle to a Geometry proto message.
// Triangles don't have a dedicated protobuf type, so we return nil.
func (t *Triangle) ToProtobuf() *commonpb.Geometry {
	// Triangles are not directly representable in the protobuf Geometry type.
	// They are typically part of a Mesh. Return nil.
	return nil
}

// MarshalJSON serializes the triangle to JSON.
func (t *Triangle) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":   "triangle",
		"p0":     map[string]float64{"x": t.p0.X, "y": t.p0.Y, "z": t.p0.Z},
		"p1":     map[string]float64{"x": t.p1.X, "y": t.p1.Y, "z": t.p1.Z},
		"p2":     map[string]float64{"x": t.p2.X, "y": t.p2.Y, "z": t.p2.Z},
		"normal": map[string]float64{"x": t.normal.X, "y": t.normal.Y, "z": t.normal.Z},
		"label":  t.label,
	})
}

// Hash returns a hash value for this triangle.
func (t *Triangle) Hash() int {
	hash := 0
	hash += (5 * (int(t.p0.X*10) + 1000)) * 2
	hash += (6 * (int(t.p0.Y*10) + 10221)) * 3
	hash += (7 * (int(t.p0.Z*10) + 2124)) * 4
	hash += (8 * (int(t.p1.X*10) + 5000)) * 5
	hash += (9 * (int(t.p1.Y*10) + 6000)) * 6
	hash += (10 * (int(t.p1.Z*10) + 7000)) * 7
	hash += (11 * (int(t.p2.X*10) + 8000)) * 8
	hash += (12 * (int(t.p2.Y*10) + 9000)) * 9
	hash += (13 * (int(t.p2.Z*10) + 10000)) * 10
	return hash
}
