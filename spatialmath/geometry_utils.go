package spatialmath

import (
	"github.com/golang/geo/r3"
)

const floatEpsilon = 1e-6

// This file incorporates work covered by the Brax project -- https://github.com/google/brax/blob/main/LICENSE.
// Copyright 2021 The Brax Authors, which is licensed under the Apache License Version 2.0 (the “License”).
// You may obtain a copy of the license at http://www.apache.org/licenses/LICENSE-2.0.

// DistToLineSegment takes a line segment defined by pt1 and pt2, plus some query point, and returns the cartesian distance from the
// query point to the closest point on the line segment.
func DistToLineSegment(pt1, pt2, query r3.Vector) float64 {
	// The below is equivalent to running ClosestPointOnLineSegment(pt1, pt2, query).Sub(query).Norm()
	// but it is ~15% faster
	ab := pt1.Sub(pt2)
	av := query.Sub(pt2)

	if av.Dot(ab) <= 0.0 { // Point is lagging behind start of the segment, so perpendicular distance is not viable.
		return av.Norm() // Use distance to start of segment instead.
	}

	bv := query.Sub(pt1)

	if bv.Dot(ab) >= 0.0 { // Point is advanced past the end of the segment, so perpendicular distance is not viable.
		return bv.Norm()
	}
	return (ab.Cross(av)).Norm() / ab.Norm()
}

// ClosestPointSegmentPoint takes a line segment defined by pt1 and pt2, plus some query point, and returns the point on the line segment
// which is closest to the query point.
func ClosestPointSegmentPoint(pt1, pt2, query r3.Vector) r3.Vector {
	ab := pt2.Sub(pt1)
	t := query.Sub(pt1).Dot(ab.Mul(1 / ab.Norm2()))
	if t <= 0 {
		return pt1
	} else if t >= 1 {
		return pt2
	}
	return pt1.Add(ab.Mul(t))
}

// SegmentDistanceToSegment will compute the distance separating two line segments at their closest point.
func SegmentDistanceToSegment(ap1, ap2, bp1, bp2 r3.Vector) float64 {
	bestA, bestB := ClosestPointsSegmentSegment(ap1, ap2, bp1, bp2)
	return bestA.Sub(bestB).Norm()
}

// ClosestPointsSegmentSegment will return the points at which two line segments are closest to one another.
func ClosestPointsSegmentSegment(ap1, ap2, bp1, bp2 r3.Vector) (r3.Vector, r3.Vector) {
	// vectors between line endpoints:
	v0 := bp1.Sub(ap1)
	v1 := bp2.Sub(ap1)
	v2 := bp1.Sub(ap2)
	v3 := bp2.Sub(ap2)

	// squared distances:
	d0 := v0.Norm2()
	d1 := v1.Norm2()
	d2 := v2.Norm2()
	d3 := v3.Norm2()

	// select best potential endpoint on capsule A:
	bestA := ap1
	if d2 < d0 || d2 < d1 || d3 < d0 || d3 < d1 {
		bestA = ap2
	}
	// select point on capsule B line segment nearest to best potential endpoint on A capsule:
	bestB := ClosestPointSegmentPoint(bp1, bp2, bestA)

	// now do the same for capsule A segment:
	bestA = ClosestPointSegmentPoint(ap1, ap2, bestB)
	return bestA, bestB
}

// PlaneNormal returns the plane normal of the triangle defined by the three given points.
func PlaneNormal(p0, p1, p2 r3.Vector) r3.Vector {
	return p1.Sub(p0).Cross(p2.Sub(p0)).Normalize()
}

// Bounding sphere returns a spherical geometry centered on the point (0, 0, 0) that will encompass the given geometry
// if it were to be rotated 360 degrees about the Z axis.  The label of the new geometry is inherited from the given one.
func BoundingSphere(geometry Geometry) (Geometry, error) {
	r := geometry.Pose().Point().Norm()
	switch g := geometry.(type) {
	case *box:
		r += r3.Vector{X: g.halfSize[0], Y: g.halfSize[1], Z: g.halfSize[2]}.Norm()
	case *sphere:
		r += g.radius
	case *capsule:
		r += g.length / 2
	case *point:
	default:
		return nil, ErrGeometryTypeUnsupported
	}
	return NewSphere(NewZeroPose(), r, geometry.Label())
}

// closestSegmentTrianglePoints takes a line segment and a triangle, and returns the point on each closest to the other.
func closestPointsSegmentTriangle(ap1, ap2 r3.Vector, t *triangle) (bestSegPt, bestTriPt r3.Vector) {
	// The closest triangle point is either on the edge or within the triangle.

	// First, handle the case where the closest triangle point is inside the
	// triangle. This returns a good value if either the line segment intersects the triangle or a segment
	// endpoint is closest to a point inside the triangle.
	// If the line overlaps the triangle and is parallel to the triangle plane,
	// the chosen triangle point is arbitrary.
	segPt, _ := closestPointsSegmentPlane(ap1, ap2, t.p0, t.normal)
	triPt, inside := t.closestInsidePoint(segPt)
	if inside {
		// If inside is false, then these will not be the best points, because they are based on the segment-plane intersection
		return segPt, triPt
	}

	// If not inside, check triangle edges for the closest point.
	bestSegPt, bestTriPt = ClosestPointsSegmentSegment(ap1, ap2, t.p0, t.p1)
	bestDist := bestSegPt.Sub(bestTriPt).Norm2()
	if bestDist == 0 {
		return bestSegPt, bestTriPt
	}
	segPt2, triPt2 := ClosestPointsSegmentSegment(ap1, ap2, t.p1, t.p2)
	d2 := segPt2.Sub(triPt2).Norm2()
	if d2 == 0 {
		return segPt2, triPt2
	}
	if d2 < bestDist {
		bestSegPt, bestTriPt = segPt2, triPt2
		bestDist = d2
	}
	segPt3, triPt3 := ClosestPointsSegmentSegment(ap1, ap2, t.p0, t.p2)
	d3 := segPt3.Sub(triPt3).Norm2()

	if d3 == 0 {
		return segPt3, triPt3
	}
	if d3 < bestDist {
		return segPt3, triPt3
	}

	return bestSegPt, bestTriPt
}

// closestSegmentPointToPlane takes a line segment, plus a plane defined by a point and a normal vector, and returns the point on the
// segment which is closest to the plane, as well as the coplanar point in line with the line.
func closestPointsSegmentPlane(ap1, ap2, planePt, planeNormal r3.Vector) (segPt, coplanarPt r3.Vector) {
	// If a line segment is parametrized as S(t) = a + t * (b - a), we can
	// plug it into the plane equation dot(n, S(t)) - d = 0, then solve for t to
	// get the line-plane intersection. We then clip t to be in [0, 1] to be on
	// the line segment.
	segVec := ap2.Sub(ap1)
	d := planePt.Dot(planeNormal)
	denom := planeNormal.Dot(segVec)
	t := (d - planeNormal.Dot(ap1)) / (denom + 1e-6)
	coplanarPt = segVec.Mul(t).Add(ap1)
	if t <= 0 {
		return ap1, coplanarPt
	}
	if t >= 1 {
		return ap2, coplanarPt
	}
	return coplanarPt, coplanarPt
}
