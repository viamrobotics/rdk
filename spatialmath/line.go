package spatialmath

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
)

const (
	lineEpsilon     = 1e-10
	minLineSegments = 2
)

type line struct {
	pose     Pose
	label    string
	segments []r3.Vector
}

// NewLine instantiates a new line Geometry.
// A line is defined by a sequence of connected line segments.
// At least 2 segments are required to form a valid line.
func NewLine(pose Pose, segments []r3.Vector, label string) (Geometry, error) {
	if pose == nil {
		return nil, newBadGeometryDimensionsError(&line{})
	}
	if len(segments) < minLineSegments {
		return nil, newBadGeometryDimensionsError(&line{})
	}
	return &line{
		pose:     pose,
		segments: segments,
		label:    label,
	}, nil
}

// String returns a human readable string that represents the line.
func (ln *line) String() string {
	return fmt.Sprintf("Type: Line | Position: X:%.1f, Y:%.1f, Z:%.1f | Segments: %v",
		ln.pose.Point().X, ln.pose.Point().Y, ln.pose.Point().Z, ln.segments)
}

// MarshalJSON marshals the line to JSON format.
func (ln line) MarshalJSON() ([]byte, error) {
	config, err := NewGeometryConfig(&ln)
	if err != nil {
		return nil, err
	}
	return json.Marshal(config)
}

// SetLabel sets the label of this line.
func (ln *line) SetLabel(label string) {
	ln.label = label
}

// Label returns the label of this line.
func (ln *line) Label() string {
	return ln.label
}

// Pose returns the pose of the line.
func (ln *line) Pose() Pose {
	return ln.pose
}

func (ln *line) almostEqual(g Geometry) bool {
	other, ok := g.(*line)
	if !ok {
		return false
	}
	return PoseAlmostEqualEps(ln.pose, other.pose, 1e-6)
}

// Transform premultiplies the line pose with a transform, allowing the line to be moved in space.
func (ln *line) Transform(toPremultiply Pose) Geometry {
	segments := make([]r3.Vector, len(ln.segments))
	for i, seg := range ln.segments {
		segments[i] = Compose(toPremultiply, NewPoseFromPoint(seg)).Point()
	}

	return &line{
		pose:     Compose(toPremultiply, ln.pose),
		segments: segments,
		label:    ln.label,
	}
}

// ToProtobuf converts the line to a Line proto message.
func (ln *line) ToProtobuf() *commonpb.Geometry {
	segments := make([]float32, len(ln.segments)*3)
	for i, p := range ln.segments {
		segments[i*3] = float32(p.X)
		segments[i*3+1] = float32(p.Y)
		segments[i*3+2] = float32(p.Z)
	}

	return &commonpb.Geometry{
		Center: PoseToProtobuf(ln.pose),
		GeometryType: &commonpb.Geometry_Line{
			Line: &commonpb.Line{
				Segments: segments,
			},
		},
		Label: ln.label,
	}
}

// CollidesWith checks if the given line collides with the given geometry and returns true if it does.
func (ln *line) CollidesWith(geometry Geometry, collisionBufferMM float64) (bool, error) {
	switch other := geometry.(type) {
	case *Mesh:
		return other.CollidesWith(ln, collisionBufferMM)
	case *box:
		return lineVsBoxDistance(ln, other) <= collisionBufferMM, nil
	case *sphere:
		return lineVsSphereDistance(ln, other) <= collisionBufferMM, nil
	case *capsule:
		return lineVsCapsuleDistance(ln, other) <= collisionBufferMM, nil
	case *point:
		return lineVsPointDistance(ln, other.position) <= collisionBufferMM, nil
	case *points:
		return lineVsPointsDistance(ln, other) <= collisionBufferMM, nil
	case *line:
		return lineVsLineCollision(ln, other, collisionBufferMM), nil
	default:
		return true, newCollisionTypeUnsupportedError(ln, geometry)
	}
}

// DistanceFrom calculates the distance from the line to another geometry.
func (ln *line) DistanceFrom(geometry Geometry) (float64, error) {
	switch other := geometry.(type) {
	case *Mesh:
		return other.DistanceFrom(ln)
	case *box:
		return lineVsBoxDistance(ln, other), nil
	case *sphere:
		return lineVsSphereDistance(ln, other), nil
	case *capsule:
		return lineVsCapsuleDistance(ln, other), nil
	case *point:
		return lineVsPointDistance(ln, other.position), nil
	case *points:
		return lineVsPointsDistance(ln, other), nil
	case *line:
		return lineVsLineDistance(ln, other), nil
	default:
		return math.Inf(-1), newCollisionTypeUnsupportedError(ln, geometry)
	}
}

// EncompassedBy checks if the line is completely contained within another geometry.
func (ln *line) EncompassedBy(geometry Geometry) (bool, error) {
	switch other := geometry.(type) {
	case *Mesh:
		return false, nil
	case *box:
		// Check if all segments are inside the box
		for _, segment := range ln.segments {
			if !pointVsBoxCollision(segment, other, defaultCollisionBufferMM) {
				return false, nil
			}
		}
		return true, nil
	case *sphere:
		// Check if all segments are within the sphere
		for _, segment := range ln.segments {
			if sphereVsPointDistance(other, segment) > defaultCollisionBufferMM {
				return false, nil
			}
		}
		return true, nil
	case *capsule:
		// Check if all segments are within the capsule
		for _, segment := range ln.segments {
			if capsuleVsPointDistance(other, segment) > defaultCollisionBufferMM {
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
		return false, newCollisionTypeUnsupportedError(ln, geometry)
	}
}

// ToPoints converts the line to a set of points representing its segments.
// The resolution parameter is ignored for lines as they are already discrete segments.
func (ln *line) ToPoints(resolution float64) []r3.Vector {
	points := make([]r3.Vector, len(ln.segments))
	for i, segment := range ln.segments {
		points[i] = segment
	}
	return points
}

// lineVsBoxCollision checks if any line segment endpoint is inside the box, or if any line segment
// intersects the box faces using line-plane intersection tests.
func lineVsBoxCollision(ln *line, box *box, collisionBufferMM float64) bool {
	for _, p := range ln.segments {
		if pointVsBoxCollision(p, box, collisionBufferMM) {
			return true
		}
	}

	// Check if any line segment intersects or touches the box
	for i := range len(ln.segments) - 1 {
		start := ln.segments[i]
		end := ln.segments[i+1]

		// Transform the line segment to the box's local coordinate system
		local := PoseInverse(box.pose)
		p0 := Compose(local, NewPoseFromPoint(start)).Point()
		p1 := Compose(local, NewPoseFromPoint(end)).Point()

		// Line direction vector d = p1 - p0
		d := p1.Sub(p0)
		norm := d.Norm()
		if norm < lineEpsilon {
			// Skip zero-length segments
			continue
		}
		d = d.Mul(1.0 / norm)

		// Check intersection with each axis-aligned face of the box
		for axisIdx := range 3 {
			axis := r3.Axis(axisIdx)
			// Check positive face
			if lineVsPlaneIntersection(p0, d, norm, axis, box.halfSize[axisIdx], box.halfSize) {
				return true
			}
			// Check negative face
			if lineVsPlaneIntersection(p0, d, norm, axis, -box.halfSize[axisIdx], box.halfSize) {
				return true
			}
		}
	}
	return false
}

// lineVsPlaneIntersection checks for line-plane intersection.
func lineVsPlaneIntersection(p0 r3.Vector, d r3.Vector, segmentLength float64, axis r3.Axis, k float64, halfSize [3]float64) bool {
	// n*d (dot product of normal and direction)
	nDotD := pointByAxis(d, axis)

	if math.Abs(nDotD) < lineEpsilon {
		return false
	}

	// n*p0 (dot product of normal and line start point)
	nDotP0 := pointByAxis(p0, axis)

	// Calculate intersection parameter t
	t := (k - nDotP0) / nDotD
	if t < -defaultDistanceEpsilon || t > segmentLength+defaultDistanceEpsilon {
		return false
	}

	// Calculate intersection point p(t) = p0 + t*d
	intersection := p0.Add(d.Mul(t))

	// Check if intersection point is within box bounds on other axes
	for axisIdx := range 3 {
		intersectAxis := r3.Axis(axisIdx)
		if intersectAxis == axis {
			continue
		}
		coord := pointByAxis(intersection, intersectAxis)
		if coord < -halfSize[axisIdx] || coord > halfSize[axisIdx] {
			return false
		}
	}

	return true
}

// lineVsPointDistance calculates the distance from a line to a point.
func lineVsPointDistance(ln *line, p r3.Vector) float64 {
	var minDistance float64 = math.Inf(1)

	// Check distance to each line segment
	for i := range len(ln.segments) - 1 {
		p0 := ln.segments[i]
		p1 := ln.segments[i+1]

		distance := DistToLineSegment(p0, p1, p)
		if distance < minDistance {
			minDistance = distance
		}
	}

	return minDistance
}

// lineVsPointsDistance calculates the distance from a line to the closest point in a set of points.
func lineVsPointsDistance(ln *line, pointsSet *points) float64 {
	var minDistance float64 = math.Inf(1)
	for _, p := range pointsSet.points {
		distance := lineVsPointDistance(ln, p)
		if distance < minDistance {
			minDistance = distance
		}
	}
	return minDistance
}

// lineVsSphereDistance calculates the distance from a line to a sphere.
func lineVsSphereDistance(ln *line, sphere *sphere) float64 {
	distance := lineVsPointDistance(ln, sphere.pose.Point())
	return distance - sphere.radius
}

// lineVsCapsuleDistance calculates the distance from a line to a capsule.
func lineVsCapsuleDistance(ln *line, capsule *capsule) float64 {
	var minDistance float64 = math.Inf(1)
	for segmentIdx := range len(ln.segments) - 1 {
		p0 := ln.segments[segmentIdx]
		p1 := ln.segments[segmentIdx+1]
		distance := lineSegmentVsCapsuleDistance(p0, p1, capsule)
		if distance < minDistance {
			minDistance = distance
		}
	}
	return minDistance
}

// lineSegmentVsCapsuleDistance calculates the distance from a line segment to a capsule.
func lineSegmentVsCapsuleDistance(p0, p1 r3.Vector, capsule *capsule) float64 {
	// Vector from segment start to segment end: u = p1 - p0
	u := p1.Sub(p0)
	norm := u.Norm()
	if norm < lineEpsilon {
		// Segment is essentially a point, use point-to-capsule distance
		return capsuleVsPointDistance(capsule, p0)
	}

	// Normalized segment direction: d = u / |u|
	d := u.Mul(1.0 / norm)
	cylinderDistance := lineSegmentVsCylinderDistance(p0, d, norm, capsule.segA, capsule.segB, capsule.radius)
	capADistance := lineSegmentVsSphereDistance(p0, d, norm, capsule.segA, capsule.radius)
	capBDistance := lineSegmentVsSphereDistance(p0, d, norm, capsule.segB, capsule.radius)

	return math.Min(cylinderDistance, math.Min(capADistance, capBDistance))
}

// lineSegmentVsCylinderDistance calculates the distance from a line segment to a cylinder.
func lineSegmentVsCylinderDistance(p0, d r3.Vector, norm float64, c0, c1 r3.Vector, cylinderRadius float64) float64 {
	// Vector from cylinder start to cylinder end: v = c1 - c0
	v := c1.Sub(c0)
	cylinderLength := v.Norm()

	if cylinderLength < lineEpsilon {
		// Cylinder is essentially a sphere at c0
		return lineSegmentVsSphereDistance(p0, d, norm, c0, cylinderRadius)
	}

	// Normalized cylinder direction: c = v / |v|
	c := v.Mul(1.0 / cylinderLength)

	// Vector from cylinder start to segment start: w = p0 - c0
	w := p0.Sub(c0)

	// Calculate the closest points between the two line segments
	dDotD := d.Dot(d)
	cDotC := c.Dot(c)
	dDotC := d.Dot(c)
	dDotW := d.Dot(w)
	cDotW := c.Dot(w)

	denominator := dDotD*cDotC - dDotC*dDotC

	var s, t float64
	if math.Abs(denominator) > lineEpsilon {
		// Lines are not parallel
		s = (dDotC*cDotW - cDotC*dDotW) / denominator
		t = (dDotC*s + cDotW) / cDotC
	} else {
		// Lines are parallel
		s = 0
		t = cDotW / cDotC
	}

	// Clamp s to segment length [0, norm]
	if s < 0 {
		s = 0
	} else if s > norm {
		s = norm
	}

	// Clamp t to cylinder length [0, cylinderLength]
	if t < 0 {
		t = 0
	} else if t > cylinderLength {
		t = cylinderLength
	}

	// Calculate closest points
	pClosest := p0.Add(d.Mul(s))
	cClosest := c0.Add(c.Mul(t))
	distance := pClosest.Sub(cClosest).Norm()

	return distance - cylinderRadius
}

// lineSegmentVsSphereDistance calculates the distance from a line segment to a sphere.
func lineSegmentVsSphereDistance(p0, d r3.Vector, norm float64, c r3.Vector, sphereRadius float64) float64 {
	p1 := p0.Add(d.Mul(norm))
	pClosest := ClosestPointSegmentPoint(p0, p1, c)
	distance := pClosest.Sub(c).Norm()
	return distance - sphereRadius
}

// lineVsLineCollision checks if two lines collide by checking if any pair of segments is within collisionBufferMM.
func lineVsLineCollision(ln *line, other *line, collisionBufferMM float64) bool {
	for i := range len(ln.segments) - 1 {
		p0 := ln.segments[i]
		p1 := ln.segments[i+1]

		for j := range len(other.segments) - 1 {
			q0 := other.segments[j]
			q1 := other.segments[j+1]
			if SegmentDistanceToSegment(p0, p1, q0, q1) <= collisionBufferMM {
				return true
			}
		}
	}
	return false
}

// lineVsLineDistance calculates the minimum distance between two lines.
func lineVsLineDistance(ln *line, other *line) float64 {
	var minDistance float64 = math.Inf(1)
	for i := range len(ln.segments) - 1 {
		p0 := ln.segments[i]
		p1 := ln.segments[i+1]

		for j := range len(other.segments) - 1 {
			q0 := other.segments[j]
			q1 := other.segments[j+1]
			distance := SegmentDistanceToSegment(p0, p1, q0, q1)
			if distance < minDistance {
				minDistance = distance
			}
		}
	}
	return minDistance
}

// lineVsBoxDistance calculates the distance from a line to a box.
func lineVsBoxDistance(ln *line, box *box) float64 {
	var minDistance float64 = math.Inf(1)
	for i := range len(ln.segments) - 1 {
		p0 := ln.segments[i]
		p1 := ln.segments[i+1]
		distance := lineSegmentVsBoxDistance(p0, p1, box)
		if distance < minDistance {
			minDistance = distance
		}
	}

	return minDistance
}

// lineSegmentVsBoxDistance calculates the distance from a line segment to a box.
func lineSegmentVsBoxDistance(p0, p1 r3.Vector, box *box) float64 {
	startDist := pointVsBoxDistance(p0, box)
	endDist := pointVsBoxDistance(p1, box)
	if startDist <= 0 || endDist <= 0 {
		return 0
	}

	// Transform the line segment to the box's local coordinate system
	local := PoseInverse(box.pose)
	localP0 := Compose(local, NewPoseFromPoint(p0)).Point()
	localP1 := Compose(local, NewPoseFromPoint(p1)).Point()

	// Vector from segment start to segment end: u = p1 - p0
	u := localP1.Sub(localP0)
	norm := u.Norm()
	if norm < lineEpsilon {
		// Segment is essentially a point, return the minimum endpoint distance
		return math.Min(startDist, endDist)
	}

	// Normalized direction vector: d = u / |u|
	d := u.Mul(1.0 / norm)

	// Check if the line segment intersects the box, if it does, distance is 0
	for axisIdx := range 3 {
		axis := r3.Axis(axisIdx)
		// Check positive face
		if lineVsPlaneIntersection(localP0, d, norm, axis, box.halfSize[axisIdx], box.halfSize) {
			return 0
		}
		// Check negative face
		if lineVsPlaneIntersection(localP0, d, norm, axis, -box.halfSize[axisIdx], box.halfSize) {
			return 0
		}
	}

	// If no intersection, the minimum distance is the minimum of endpoint distances
	return math.Min(startDist, endDist)
}
