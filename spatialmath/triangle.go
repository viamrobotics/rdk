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
