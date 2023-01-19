package spatialmath

import (
	"github.com/golang/geo/r3"
)

// This file incorporates work covered by the Brax project -- https://github.com/google/brax/blob/main/LICENSE.
// Copyright 2021 The Brax Authors, which is licensed under the Apache License Version 2.0 (the “License”).
// You may obtain a copy of the license at http://www.apache.org/licenses/LICENSE-2.0.

// mesh is a collision geometry that represents a set of triangles that represent a mesh.
type mesh struct {
	pose      Pose
	triangles []*triangle
}

type triangle struct {
	p0 r3.Vector
	p1 r3.Vector
	p2 r3.Vector

	normal r3.Vector
}

func newTriangle(p0, p1, p2 r3.Vector) *triangle {
	return &triangle{
		p0:     p0,
		p1:     p1,
		p2:     p2,
		normal: PlaneNormal(p0, p1, p2),
	}
}

// closestPointToCoplanarPoint takes a point, and returns the closest point on the triangle to the given point
// The given point *MUST* be coplanar with the triangle. If it is known ahead of time that the point is coplanar, this is faster.
func (t *triangle) closestPointToCoplanarPoint(pt r3.Vector) r3.Vector {
	// Determine whether point is inside all triangle edges:
	c0 := pt.Sub(t.p0).Cross(t.p1.Sub(t.p0))
	c1 := pt.Sub(t.p1).Cross(t.p2.Sub(t.p1))
	c2 := pt.Sub(t.p2).Cross(t.p0.Sub(t.p2))
	inside := c0.Dot(t.normal) <= 0 && c1.Dot(t.normal) <= 0 && c2.Dot(t.normal) <= 0

	if inside {
		return pt
	}

	// Edge 1:
	refPt := ClosestPointSegmentPoint(t.p0, t.p1, pt)
	bestDist := pt.Sub(refPt).Norm2()

	// Edge 2:
	point2 := ClosestPointSegmentPoint(t.p1, t.p2, pt)
	if distsq := pt.Sub(point2).Norm2(); distsq < bestDist {
		refPt = point2
		bestDist = distsq
	}

	// Edge 3:
	point3 := ClosestPointSegmentPoint(t.p2, t.p0, pt)
	if distsq := pt.Sub(point3).Norm2(); distsq < bestDist {
		return point3
	}
	return refPt
}

// closestPointToPoint takes a point, and returns the closest point on the triangle to the given point, as well as whether the point
// is on the edge of the triangle.
// This is slower than closestPointToCoplanarPoint.
func (t *triangle) closestPointToPoint(point r3.Vector) r3.Vector {
	closestPtInside, inside := t.closestInsidePoint(point)
	if inside {
		return closestPtInside
	}

	// If the closest point is outside the triangle, it must be on an edge, so we
	// check each triangle edge for a closest point to the point pt.
	closestPt := ClosestPointSegmentPoint(t.p0, t.p1, point)
	bestDist := point.Sub(closestPt).Norm2()

	newPt := ClosestPointSegmentPoint(t.p1, t.p2, point)
	if newDist := point.Sub(newPt).Norm2(); newDist < bestDist {
		closestPt = newPt
		bestDist = newDist
	}

	newPt = ClosestPointSegmentPoint(t.p2, t.p0, point)
	if newDist := point.Sub(newPt).Norm2(); newDist < bestDist {
		return newPt
	}
	return closestPt
}

// closestInsidePoint returns the closest point on a triangle IF AND ONLY IF the query point's projection overlaps the triangle.
// Otherwise it will return the query point.
// To visualize this- if one draws a tetrahedron using the triangle and the query point, all angles from the triangle to the query point
// must be <= 90 degrees.
func (t *triangle) closestInsidePoint(point r3.Vector) (r3.Vector, bool) {
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
	inside := (0 <= u) && (u <= 1) && (0 <= v) && (v <= 1) && (u+v <= 1)
	return t.p0.Add(e0.Mul(u)).Add(e1.Mul(v)), inside
}
