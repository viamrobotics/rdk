package spatialmath

import (
	"github.com/golang/geo/r3"
)

// Triangle is three points and a normal vector.
type Triangle struct {
	p0 r3.Vector
	p1 r3.Vector
	p2 r3.Vector

	normal r3.Vector
}

// NewTriangle creates a Triangle from three points. The Normal is computed; directionality is random.
func NewTriangle(p0, p1, p2 r3.Vector) *Triangle {
	return &Triangle{
		p0:     p0,
		p1:     p1,
		p2:     p2,
		normal: PlaneNormal(p0, p1, p2),
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
func (t *Triangle) Transform(toPremultiply Pose) *Triangle {
	return NewTriangle(
		Compose(toPremultiply, NewPoseFromPoint(t.p0)).Point(),
		Compose(toPremultiply, NewPoseFromPoint(t.p1)).Point(),
		Compose(toPremultiply, NewPoseFromPoint(t.p2)).Point(),
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
