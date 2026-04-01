package spatialmath

import (
	"math"

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
func ClosestPointsSegmentSegment(a1, a2, b1, b2 r3.Vector) (r3.Vector, r3.Vector) {
	// Proceed according to this math.stackexchange answer: https://math.stackexchange.com/a/2812513
	// The stack solution has a sign error, which we correct
	// These (ugly) variable names were chosen to match those in the stack exchange answer, for easy reference

	d3121 := b1.Sub(a1).Dot(a2.Sub(a1))
	d4321 := b2.Sub(b1).Dot(a2.Sub(a1))
	d4331 := b2.Sub(b1).Dot(b1.Sub(a1))
	r1pow2 := a2.Sub(a1).Norm2()
	r2pow2 := b2.Sub(b1).Norm2()

	denom := r1pow2*r2pow2 - d4321*d4321
	// If denom is 0, the segments are parallel, and we can jump to the endpt case below
	// If denom is not 0, we finish our algebra
	if math.Abs(denom) > floatEpsilon {
		// Find the closest points on the lines each segment define
		// If s or t lie outside of [0,1], the closest points on the lines are NOT on the finite segments
		s := (d3121*r2pow2 - d4331*d4321) / denom
		t := (d3121*d4321 - d4331*r1pow2) / denom
		if 0 <= s && s <= 1 && 0 <= t && t <= 1 {
			bestSeg1Pt := a1.Add(a2.Sub(a1).Mul(s))
			bestSeg2Pt := b1.Add(b2.Sub(b1).Mul(t))
			return bestSeg1Pt, bestSeg2Pt
		}
	}

	// If we're here, the lines are either parallel or their closest points lie off at least one of the segments
	// It suffices to just check each segment against the other segment's endpoints
	bestSeg1Pt := a1
	bestSeg2Pt := ClosestPointSegmentPoint(b1, b2, a1)
	bestDist := bestSeg1Pt.Sub(bestSeg2Pt).Norm2() // actually squared distance, but doesn't matter
	if bestDist < defaultCollisionBufferMM {       // this could be problematic if a lesser collision buffer is ever desired
		return bestSeg1Pt, bestSeg2Pt
	}
	seg1Pt := a2
	seg2Pt := ClosestPointSegmentPoint(b1, b2, a2)
	dist := seg2Pt.Sub(seg1Pt).Norm2()
	if dist < defaultCollisionBufferMM {
		return seg1Pt, seg2Pt
	}
	if dist < bestDist {
		bestSeg1Pt, bestSeg2Pt = seg1Pt, seg2Pt
		bestDist = dist
	}
	seg1Pt = ClosestPointSegmentPoint(a1, a2, b1)
	seg2Pt = b1
	dist = seg2Pt.Sub(seg1Pt).Norm2()
	if dist < defaultCollisionBufferMM {
		return seg1Pt, seg2Pt
	}
	if dist < bestDist {
		bestSeg1Pt, bestSeg2Pt = seg1Pt, seg2Pt
		bestDist = dist
	}
	seg1Pt = ClosestPointSegmentPoint(a1, a2, b2)
	seg2Pt = b2
	dist = seg2Pt.Sub(seg1Pt).Norm2()
	if dist < bestDist {
		return seg1Pt, seg2Pt
	}

	return bestSeg1Pt, bestSeg2Pt
}

// PlaneNormal returns the plane normal of the triangle defined by the three given points.
func PlaneNormal(p0, p1, p2 r3.Vector) r3.Vector {
	return p1.Sub(p0).Cross(p2.Sub(p0)).Normalize()
}

// BoundingSphere returns a spherical geometry centered on the point (0, 0, 0) that will encompass the given geometry
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
		return nil, errGeometryTypeUnsupported
	}
	return NewSphere(NewZeroPose(), r, geometry.Label())
}

// ClosestPointsSegmentTriangle takes a line segment and a triangle, and returns the point on each closest to the other.
func ClosestPointsSegmentTriangle(ap1, ap2 r3.Vector, t *Triangle) (bestSegPt, bestTriPt r3.Vector) {
	// The closest triangle point is either on the edge or within the triangle.

	// First, handle the case where the closest triangle point is inside the
	// triangle. This returns a good value if either the line segment intersects the triangle or a segment
	// endpoint is closest to a point inside the triangle.
	// If the line overlaps the triangle and is parallel to the triangle plane,
	// the chosen triangle point is arbitrary.
	segPt, _ := ClosestPointsSegmentPlane(ap1, ap2, t.p0, t.normal)
	triPt, inside := ClosestTriangleInsidePoint(t, segPt)
	if inside {
		// If inside is false, then these will not be the best points, because they are based on the segment-plane intersection
		return segPt, triPt
	}

	// If not inside, check triangle edges for the closest point.
	bestSegPt, bestTriPt = ClosestPointsSegmentSegment(ap1, ap2, t.p0, t.p1)
	bestDist := bestSegPt.Sub(bestTriPt).Norm2()
	if bestDist < defaultCollisionBufferMM {
		return bestSegPt, bestTriPt
	}
	segPt2, triPt2 := ClosestPointsSegmentSegment(ap1, ap2, t.p1, t.p2)
	d2 := segPt2.Sub(triPt2).Norm2()
	if d2 < defaultCollisionBufferMM {
		return segPt2, triPt2
	}
	if d2 < bestDist {
		bestSegPt, bestTriPt = segPt2, triPt2
		bestDist = d2
	}
	segPt3, triPt3 := ClosestPointsSegmentSegment(ap1, ap2, t.p0, t.p2)
	d3 := segPt3.Sub(triPt3).Norm2()
	if d3 < bestDist {
		return segPt3, triPt3
	}

	return bestSegPt, bestTriPt
}

// ClosestPointsSegmentPlane takes a line segment, plus a plane defined by a point and a normal vector, and returns the point on the
// segment which is closest to the plane, as well as the coplanar point in line with the line.
func ClosestPointsSegmentPlane(ap1, ap2, planePt, planeNormal r3.Vector) (segPt, coplanarPt r3.Vector) {
	// If a line segment is parametrized as S(t) = a + t * (b - a), we can
	// plug it into the plane equation dot(n, S(t)) - d = 0, then solve for t to
	// get the line-plane intersection. We then clip t to be in [0, 1] to be on
	// the line segment.

	segVec := ap2.Sub(ap1)
	d := planePt.Dot(planeNormal)
	denom := planeNormal.Dot(segVec)
	if math.Abs(denom) < floatEpsilon {
		coplanarPt = ap1.Sub(planeNormal.Mul(planeNormal.Dot(ap1) - d))
		return ap1, coplanarPt
	}

	t := (d - planeNormal.Dot(ap1)) / denom // do not pad denom with 1e-k, small error can significantly mess up collisions
	coplanarPt = segVec.Mul(t).Add(ap1)
	if t <= 0 {
		return ap1, coplanarPt
	}
	if t >= 1 {
		return ap2, coplanarPt
	}
	return coplanarPt, coplanarPt
}
