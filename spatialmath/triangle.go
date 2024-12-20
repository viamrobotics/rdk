package spatialmath

import (
	"math"

	"github.com/golang/geo/r3"
)

type Triangle struct {
	p0 r3.Vector
	p1 r3.Vector
	p2 r3.Vector

	normal r3.Vector
}

func NewTriangle(p0, p1, p2 r3.Vector) *Triangle {
	return &Triangle{
		p0:     p0,
		p1:     p1,
		p2:     p2,
		normal: PlaneNormal(p0, p1, p2),
	}
}

// closestPointToCoplanarPoint takes a point, and returns the closest point on the triangle to the given point
// The given point *MUST* be coplanar with the triangle. If it is known ahead of time that the point is coplanar, this is faster.
func (t *Triangle) ClosestPointToCoplanarPoint(pt r3.Vector) r3.Vector {
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

// closestPointToPoint takes a point, and returns the closest point on the triangle to the given point.
// This is slower than closestPointToCoplanarPoint.
func (t *Triangle) ClosestPointToPoint(point r3.Vector) r3.Vector {
	closestPtInside, inside := t.ClosestInsidePoint(point)
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
func (t *Triangle) ClosestInsidePoint(point r3.Vector) (r3.Vector, bool) {
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

func (t *Triangle) Points() []r3.Vector {
	return []r3.Vector{t.p0, t.p1, t.p2}
}

func (t *Triangle) Normal() r3.Vector {
	return t.normal
}

// IntersectsPlane determines if the triangle intersects with a plane defined by a point and normal vector.
// Returns true if the triangle intersects with or lies on the plane.
func (t *Triangle) IntersectsPlane(planePt, planeNormal r3.Vector) bool {
	// Calculate signed distances from each triangle vertex to the plane
	d0 := planeNormal.Dot(t.p0.Sub(planePt))
	d1 := planeNormal.Dot(t.p1.Sub(planePt))
	d2 := planeNormal.Dot(t.p2.Sub(planePt))

	// If all points are on the same side of the plane (all distances positive or all negative),
	// there is no intersection
	if (d0 > floatEpsilon && d1 > floatEpsilon && d2 > floatEpsilon) ||
		(d0 < -floatEpsilon && d1 < -floatEpsilon && d2 < -floatEpsilon) {
		return false
	}

	// If any point is very close to the plane or points lie on different sides,
	// there is an intersection
	return true
}

// TrianglePlaneIntersectingSegment determines the line segment where a triangle intersects with a plane.
// Returns the two points defining the intersection line segment and whether an intersection exists.
// If the triangle only touches the plane at a point, both returned points will be the same.
// If the triangle lies in the plane, it returns two points representing the longest edge of the triangle.
func (t *Triangle) TrianglePlaneIntersectingSegment(planePt, planeNormal r3.Vector) (r3.Vector, r3.Vector, bool) {
	// First check if there's an intersection
	if !t.IntersectsPlane(planePt, planeNormal) {
		return r3.Vector{}, r3.Vector{}, false
	}

	// Calculate signed distances from each vertex to the plane
	d0 := planeNormal.Dot(t.p0.Sub(planePt))
	d1 := planeNormal.Dot(t.p1.Sub(planePt))
	d2 := planeNormal.Dot(t.p2.Sub(planePt))

	// If triangle lies in plane (all distances are approximately zero)
	if math.Abs(d0) < floatEpsilon && math.Abs(d1) < floatEpsilon && math.Abs(d2) < floatEpsilon {
		// Return the longest edge of the triangle
		e1 := t.p1.Sub(t.p0).Norm2()
		e2 := t.p2.Sub(t.p1).Norm2()
		e3 := t.p0.Sub(t.p2).Norm2()
		if e1 >= e2 && e1 >= e3 {
			return t.p0, t.p1, true
		} else if e2 >= e1 && e2 >= e3 {
			return t.p1, t.p2, true
		}
		return t.p2, t.p0, true
	}

	// Find the two edges that intersect with the plane
	var intersections []r3.Vector
	edges := [][2]r3.Vector{
		{t.p0, t.p1},
		{t.p1, t.p2},
		{t.p2, t.p0},
	}
	dists := []float64{d0, d1, d2}

	for i := 0; i < 3; i++ {
		j := (i + 1) % 3
		// If edge crosses plane (distances have different signs)
		if (dists[i] * dists[j]) < 0 {
			// Calculate intersection point using linear interpolation
			t := dists[i] / (dists[i] - dists[j])
			edge := edges[i]
			intersection := edge[0].Add(edge[1].Sub(edge[0]).Mul(t))
			intersections = append(intersections, intersection)
		} else if math.Abs(dists[i]) < floatEpsilon {
			// Vertex lies on plane
			intersections = append(intersections, edges[i][0])
		}
	}

	// We should have exactly two intersection points
	if len(intersections) < 2 {
		// Handle degenerate case where triangle only touches plane at a point
		return intersections[0], intersections[0], true
	}
	return intersections[0], intersections[1], true
}
