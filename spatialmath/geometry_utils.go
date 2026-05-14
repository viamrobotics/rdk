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
	qb := query.Sub(pt1)
	// t = (qb . ab) / |ab|^2. Check the numerator sign first: if (qb . ab) <= 0,
	// the projection lies before pt1, so the closest segment point is pt1 itself.
	// This avoids computing |ab|^2 entirely for the most common path.
	qbDotAb := qb.Dot(ab)
	if qbDotAb <= 0 {
		return pt1
	}
	abN2 := ab.Norm2()
	if qbDotAb >= abN2 {
		return pt2
	}
	return pt1.Add(ab.Mul(qbDotAb / abN2))
}

// SegmentDistanceToSegment will compute the distance separating two line segments at their closest point.
func SegmentDistanceToSegment(ap1, ap2, bp1, bp2 r3.Vector) float64 {
	bestA, bestB := ClosestPointsSegmentSegment(ap1, ap2, bp1, bp2)
	return bestA.Sub(bestB).Norm()
}

// ClosestPointsSegmentSegment will return the points at which two line segments are closest to one another.
// See https://math.stackexchange.com/a/2812513 (with a sign-error correction). The actual math is in
// closestPointsSegmentSegmentCached; this wrapper just precomputes the first segment's edge vector.
func ClosestPointsSegmentSegment(a1, a2, b1, b2 r3.Vector) (r3.Vector, r3.Vector) {
	d21 := a2.Sub(a1)
	return closestPointsSegmentSegmentCached(a1, a2, d21, d21.Norm2(), b1, b2)
}

// closestOnSegmentCached returns the point on segment pt1->pt2 closest to a query
// whose precomputed projection numerator (qb·ab) and segment squared-length (|ab|²)
// are passed in. Equivalent to ClosestPointSegmentPoint but skips the redundant
// Sub/Dot/Norm2 work whenever the caller already has those values cached.
func closestOnSegmentCached(pt1, pt2, ab r3.Vector, qbDotAb, abN2 float64) r3.Vector {
	if qbDotAb <= 0 {
		return pt1
	}
	if qbDotAb >= abN2 {
		return pt2
	}
	return pt1.Add(ab.Mul(qbDotAb / abN2))
}

// closestPointsSegmentSegmentCached is the same as ClosestPointsSegmentSegment but
// with the first segment's edge vector (d21 = a2-a1) and squared-length (r1pow2)
// already computed by the caller. Saves a Sub + Norm2 per call — material when the
// same segment is checked against many other segments (e.g. one triangle edge vs.
// the three edges of another triangle).
func closestPointsSegmentSegmentCached(a1, a2, d21 r3.Vector, r1pow2 float64, b1, b2 r3.Vector) (r3.Vector, r3.Vector) {
	d43 := b2.Sub(b1)
	d31 := b1.Sub(a1)
	d3121 := d31.Dot(d21)
	d4321 := d43.Dot(d21)
	d4331 := d43.Dot(d31)
	r2pow2 := d43.Norm2()

	denom := r1pow2*r2pow2 - d4321*d4321
	if math.Abs(denom) > floatEpsilon {
		s := (d3121*r2pow2 - d4331*d4321) / denom
		t := (d3121*d4321 - d4331*r1pow2) / denom
		if 0 <= s && s <= 1 && 0 <= t && t <= 1 {
			return a1.Add(d21.Mul(s)), b1.Add(d43.Mul(t))
		}
	}

	bestSeg1Pt := a1
	bestSeg2Pt := closestOnSegmentCached(b1, b2, d43, -d4331, r2pow2)
	bestDist := bestSeg1Pt.Sub(bestSeg2Pt).Norm2()
	if bestDist < defaultCollisionBufferMM {
		return bestSeg1Pt, bestSeg2Pt
	}

	seg1Pt := a2
	seg2Pt := closestOnSegmentCached(b1, b2, d43, d4321-d4331, r2pow2)
	dist := seg2Pt.Sub(seg1Pt).Norm2()
	if dist < defaultCollisionBufferMM {
		return seg1Pt, seg2Pt
	}
	if dist < bestDist {
		bestSeg1Pt, bestSeg2Pt = seg1Pt, seg2Pt
		bestDist = dist
	}

	seg1Pt = closestOnSegmentCached(a1, a2, d21, d3121, r1pow2)
	seg2Pt = b1
	dist = seg2Pt.Sub(seg1Pt).Norm2()
	if dist < defaultCollisionBufferMM {
		return seg1Pt, seg2Pt
	}
	if dist < bestDist {
		bestSeg1Pt, bestSeg2Pt = seg1Pt, seg2Pt
		bestDist = dist
	}

	seg1Pt = closestOnSegmentCached(a1, a2, d21, d3121+d4321, r1pow2)
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
	props := computeTriCache(t)
	return closestPointsSegmentTriangleCached(ap1, ap2, t, &props)
}

// triCache holds per-triangle quantities that ClosestTriangleInsidePoint and
// ClosestPointsSegmentTriangle need. Computed once per outer triangle then
// reused across the 3 segment-vs-triangle checks done by collidesWithTriangle.
type triCache struct {
	e0, e1  r3.Vector // edge vectors p1-p0, p2-p0
	a, b, c float64   // |e0|², e0·e1, |e1|²
	invDet  float64   // 1 / (a*c - b²)
}

func computeTriCache(t *Triangle) triCache {
	e0 := t.p1.Sub(t.p0)
	e1 := t.p2.Sub(t.p0)
	a := e0.Norm2()
	b := e0.Dot(e1)
	c := e1.Norm2()
	return triCache{
		e0:     e0,
		e1:     e1,
		a:      a,
		b:      b,
		c:      c,
		invDet: 1.0 / (a*c - b*b),
	}
}

// closestPointsSegmentTriangleCached is the workhorse of ClosestPointsSegmentTriangle.
// Reuses the caller's triCache across the inside-point test and shares the
// segment edge vector across the three segment-segment edge checks (saves a Sub
// and a Norm2 per CPSS call relative to going through the uncached entry point).
func closestPointsSegmentTriangleCached(ap1, ap2 r3.Vector, t *Triangle, tc *triCache) (bestSegPt, bestTriPt r3.Vector) {
	segPt := closestSegPointToPlane(ap1, ap2, t.p0, t.normal)
	triPt, inside := closestTriInsidePointCached(t, tc, segPt)
	if inside {
		return segPt, triPt
	}

	// Cache the segment edge vector + squared length: shared by all three
	// triangle-edge checks below.
	d21 := ap2.Sub(ap1)
	r1pow2 := d21.Norm2()

	bestSegPt, bestTriPt = closestPointsSegmentSegmentCached(ap1, ap2, d21, r1pow2, t.p0, t.p1)
	bestDist := bestSegPt.Sub(bestTriPt).Norm2()
	if bestDist < defaultCollisionBufferMM {
		return bestSegPt, bestTriPt
	}
	segPt2, triPt2 := closestPointsSegmentSegmentCached(ap1, ap2, d21, r1pow2, t.p1, t.p2)
	d2 := segPt2.Sub(triPt2).Norm2()
	if d2 < defaultCollisionBufferMM {
		return segPt2, triPt2
	}
	if d2 < bestDist {
		bestSegPt, bestTriPt = segPt2, triPt2
		bestDist = d2
	}
	segPt3, triPt3 := closestPointsSegmentSegmentCached(ap1, ap2, d21, r1pow2, t.p0, t.p2)
	d3 := segPt3.Sub(triPt3).Norm2()
	if d3 < bestDist {
		return segPt3, triPt3
	}
	return bestSegPt, bestTriPt
}

// closestSegPointToPlane returns just the segment-side closest point (no
// coplanar pt, since CPST discards it). Equivalent to ClosestPointsSegmentPlane's
// first return value, but skips the coplanarPt Sub/Mul/Add when t is out of [0,1].
func closestSegPointToPlane(ap1, ap2, planePt, planeNormal r3.Vector) r3.Vector {
	segVec := ap2.Sub(ap1)
	denom := planeNormal.Dot(segVec)
	if math.Abs(denom) < floatEpsilon {
		return ap1
	}
	d := planePt.Dot(planeNormal)
	t := (d - planeNormal.Dot(ap1)) / denom
	if t <= 0 {
		return ap1
	}
	if t >= 1 {
		return ap2
	}
	return segVec.Mul(t).Add(ap1)
}

// closestTriInsidePointCached is the cached variant of ClosestTriangleInsidePoint —
// the triangle's e0, e1, a, b, c, det are taken from the precomputed cache.
func closestTriInsidePointCached(t *Triangle, tc *triCache, point r3.Vector) (r3.Vector, bool) {
	const eps = 1e-6
	d := point.Sub(t.p0)
	e0DotD := tc.e0.Dot(d)
	e1DotD := tc.e1.Dot(d)
	u := (tc.c*e0DotD - tc.b*e1DotD) * tc.invDet
	v := (-tc.b*e0DotD + tc.a*e1DotD) * tc.invDet
	inside := (0 <= u+eps) && (u <= 1+eps) && (0 <= v+eps) && (v <= 1+eps) && (u+v <= 1+eps)
	return t.p0.Add(tc.e0.Mul(u)).Add(tc.e1.Mul(v)), inside
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
