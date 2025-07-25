package spatialmath

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/utils"
)

// capsule is a collision geometry that represents a capsule, it has a pose and a radius that fully define it.
//
// ....___________________
// .../                   \
// .x|  |-------O-------|  |x
// ...\___________________/
//
// Length is the distance between the x's, or internal segment length + 2*radius.
type capsule struct {
	// this is the pose of one end of the capsule. The full capsule extends `length` mm outwards in the direction of
	// the pose's orientation
	pose   Pose
	radius float64
	length float64 // total length of the capsule, tip to tip
	label  string

	// These values are generated at geometry creation time and should not be altered by hand
	// They are stoed here because they are useful and expensive to calculate
	segA   r3.Vector // Proximal endpoint of capsule line segment. First point from `pose` to be surrounded by `radius` of capsule
	segB   r3.Vector // Distal endpoint of capsule line segment. Most distal point to be surrounded by `radius` of capsule
	center r3.Vector // Centerpoint of capsule as an r3.Vector, cached to prevent recalculation
	capVec r3.Vector // Vector pointing from `center` towards `segB`, cached to prevent recalculation

	rotMatrix *RotationMatrix
	once      sync.Once
}

// NewCapsule instantiates a new capsule Geometry.
func NewCapsule(offset Pose, radius, length float64, label string) (Geometry, error) {
	if radius <= 0 || length <= 0 {
		return nil, newBadGeometryDimensionsError(&capsule{})
	}
	if length < radius*2 {
		return nil, newBadCapsuleLengthError(length, radius)
	}
	if length == radius*2 {
		return NewSphere(offset, radius, label)
	}
	return newCapsuleWithSegPoints(offset, radius, length, label), nil
}

// Will precalculate the linear endpoints for a capsule.
func newCapsuleWithSegPoints(offset Pose, radius, length float64, label string) Geometry {
	segA := Compose(offset, NewPoseFromPoint(r3.Vector{0, 0, -length/2 + radius})).Point()
	segB := Compose(offset, NewPoseFromPoint(r3.Vector{0, 0, length/2 - radius})).Point()
	center := offset.Point()

	return &capsule{
		pose:   offset,
		radius: radius,
		length: length,
		label:  label,
		segA:   segA,
		segB:   segB,
		center: center,
		capVec: segB.Sub(center),
	}
}

func (c *capsule) MarshalJSON() ([]byte, error) {
	config, err := NewGeometryConfig(c)
	if err != nil {
		return nil, err
	}
	config.Type = "capsule"
	config.R = c.radius
	config.L = c.length
	return json.Marshal(config)
}

// String returns a human readable string that represents the capsule.
func (c *capsule) String() string {
	return fmt.Sprintf("Type: Capsule | Position: X:%.1f, Y:%.1f, Z:%.1f | Radius: %.0f | Length: %.0f",
		c.center.X, c.center.Y, c.center.Z, c.radius, c.length)
}

// Label returns the label of this capsule.
func (c *capsule) Label() string {
	return c.label
}

// SetLabel sets the label of this capsule.
func (c *capsule) SetLabel(label string) {
	c.label = label
}

// Pose returns the pose of the capsule.
func (c *capsule) Pose() Pose {
	return c.pose
}

// AlmostEqual compares the capsule with another geometry and checks if they are equivalent.
func (c *capsule) almostEqual(g Geometry) bool {
	other, ok := g.(*capsule)
	if !ok {
		return false
	}
	return PoseAlmostEqualEps(c.pose, other.pose, 1e-6) &&
		utils.Float64AlmostEqual(c.radius, other.radius, 1e-8) &&
		utils.Float64AlmostEqual(c.length, other.length, 1e-8)
}

// Transform premultiplies the capsule pose with a transform, allowing the capsule to be moved in space.
func (c *capsule) Transform(toPremultiply Pose) Geometry {
	newPose := Compose(toPremultiply, c.pose)
	segB := Compose(toPremultiply, NewPoseFromPoint(c.segB)).Point()
	center := newPose.Point()
	return &capsule{
		pose:   newPose,
		radius: c.radius,
		length: c.length,
		label:  c.label,
		segA:   Compose(toPremultiply, NewPoseFromPoint(c.segA)).Point(),
		segB:   segB,
		center: center,
		capVec: segB.Sub(center),
	}
}

// ToProto converts the capsule to a Geometry proto message.
func (c *capsule) ToProtobuf() *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: PoseToProtobuf(c.pose),
		GeometryType: &commonpb.Geometry_Capsule{
			Capsule: &commonpb.Capsule{
				RadiusMm: c.radius,
				LengthMm: c.length,
			},
		},
		Label: c.label,
	}
}

// CollidesWith checks if the given capsule collides with the given geometry and returns true if it does.
func (c *capsule) CollidesWith(g Geometry, collisionBufferMM float64) (bool, error) {
	switch other := g.(type) {
	case *box:
		return capsuleVsBoxCollision(c, other, collisionBufferMM), nil
	default:
		dist, err := c.DistanceFrom(g)
		if err != nil {
			return true, err
		}
		return dist <= collisionBufferMM, nil
	}
}

func (c *capsule) DistanceFrom(g Geometry) (float64, error) {
	switch other := g.(type) {
	case *Mesh:
		return other.DistanceFrom(c)
	case *box:
		return capsuleVsBoxDistance(c, other), nil
	case *capsule:
		return capsuleVsCapsuleDistance(c, other), nil
	case *point:
		return capsuleVsPointDistance(c, other.position), nil
	case *sphere:
		return capsuleVsSphereDistance(c, other), nil
	default:
		return math.Inf(-1), newCollisionTypeUnsupportedError(c, g)
	}
}

func (c *capsule) EncompassedBy(g Geometry) (bool, error) {
	switch other := g.(type) {
	case *Mesh:
		return false, nil // Like points, meshes have no volume and cannot encompass
	case *capsule:
		return capsuleInCapsule(c, other), nil
	case *box:
		return capsuleInBox(c, other), nil
	case *sphere:
		return capsuleInSphere(c, other), nil
	case *point:
		return false, nil
	default:
		return true, newCollisionTypeUnsupportedError(c, g)
	}
}

// ToPoints converts a capsule geometry into []r3.Vector. This method takes one argument which determines
// how many points should like on the capsule's surface. If the argument is set to 0. we automatically
// substitute the value with defaultTotalSpherePoints.
func (c *capsule) ToPoints(resolution float64) []r3.Vector {
	if resolution <= 0 {
		resolution = defaultPointDensity
	}

	s := &sphere{pose: NewZeroPose(), radius: c.radius}
	vecList := s.ToPoints(resolution)
	// move points to be correctly located on capsule endcaps
	adj := c.length/2 - c.radius
	for _, pt := range vecList {
		if pt.Z >= 0 {
			pt.Z += adj
		} else {
			pt.Z -= adj
		}
	}

	// Now distribute points along the cylindrical shaft
	totalShaftPts := (c.radius * c.length) * resolution
	ptsPerRing := totalShaftPts / (c.length * resolution)
	ringCnt := math.Floor(totalShaftPts / ptsPerRing)
	zInc := c.length / (ringCnt + 1)
	for ring := 1.; ring <= ringCnt; ring++ {
		for ringPt := 0.; ringPt < ptsPerRing; ringPt++ {
			theta := 2. * math.Pi * (ringPt / ptsPerRing)
			vecList = append(vecList, r3.Vector{math.Cos(theta) * c.radius, math.Sin(theta) * c.radius, zInc * ring})
		}
	}

	return transformPointsToPose(vecList, c.Pose())
}

// rotationMatrix returns the cached matrix if it exists, and generates it if not.
func (c *capsule) rotationMatrix() *RotationMatrix {
	c.once.Do(func() { c.rotMatrix = c.pose.Orientation().RotationMatrix() })

	return c.rotMatrix
}

func capsuleVsPointDistance(c *capsule, other r3.Vector) float64 {
	return DistToLineSegment(c.segA, c.segB, other) - c.radius
}

func capsuleVsSphereDistance(c *capsule, other *sphere) float64 {
	return DistToLineSegment(c.segA, c.segB, other.pose.Point()) - (c.radius + other.radius)
}

func capsuleVsCapsuleDistance(c, other *capsule) float64 {
	return SegmentDistanceToSegment(c.segA, c.segB, other.segA, other.segB) - (c.radius + other.radius)
}

func capsuleVsBoxDistance(c *capsule, other *box) float64 {
	// Large amounts of capsule collision code were adopted from `brax`
	// https://github.com/google/brax/blob/7eaa16b4bf446b117b538dbe9c9401f97cf4afa2/brax/physics/colliders.py
	// https://github.com/google/brax/blob/7eaa16b4bf446b117b538dbe9c9401f97cf4afa2/brax/physics/geometry.py
	// Brax converts boxes to meshes composed of 12 triangles and does collision detection on those.
	// SAT is faster and easier if we are *NOT* GPU-accelerated. But triangle method is guaranteed accurate at distances.
	dist := capsuleBoxSeparatingAxisDistance(c, other)
	// Separating axis theorum provides accurate penetration depth but is not accurate for separation
	// if we are not in collision, convert box to mesh and determine triangle-capsule separation distance
	if dist > defaultCollisionBufferMM {
		boxAsMesh := other.toMesh()
		return capsuleVsMeshDistance(c, boxAsMesh)
	}
	return dist
}

// IMPORTANT: meshes are not considered solid. A mesh is not guaranteed to represent an enclosed area. This will measure ONLY the distance
// to the closest triangle in the mesh.
func capsuleVsMeshDistance(c *capsule, other *Mesh) float64 {
	lowDist := math.Inf(1)
	for _, t := range other.triangles {
		// Measure distance to each mesh triangle
		// Make sure the triangle is transformed by the pose of the mesh to ensure that it is properly positioned
		properlyPositionedTriangle := t.Transform(other.Pose())
		dist := capsuleVsTriangleDistance(c, properlyPositionedTriangle)
		if dist < lowDist {
			lowDist = dist
		}
	}
	return lowDist
}

func capsuleVsTriangleDistance(c *capsule, other *Triangle) float64 {
	capPt, triPt := ClosestPointsSegmentTriangle(c.segA, c.segB, other)
	return capPt.Sub(triPt).Norm() - c.radius
}

// capsuleInCapsule returns a bool describing if the inner capsule is fully encompassed by the outer capsule.
func capsuleInCapsule(inner, outer *capsule) bool {
	return capsuleVsPointDistance(outer, inner.segA) < -inner.radius &&
		capsuleVsPointDistance(outer, inner.segB) < -inner.radius
}

// capsuleInBox returns a bool describing if the given capsule is fully encompassed by the given box.
func capsuleInBox(c *capsule, b *box) bool {
	return pointVsBoxDistance(c.segA, b) <= -c.radius && pointVsBoxDistance(c.segB, b) <= -c.radius
}

// capsuleInSphere returns a bool describing if the given capsule is fully encompassed by the given sphere.
func capsuleInSphere(c *capsule, s *sphere) bool {
	return c.segA.Sub(s.pose.Point()).Norm()+c.radius <= s.radius && c.segB.Sub(s.pose.Point()).Norm()+c.radius <= s.radius
}

// capsuleVsBoxCollision returns immediately as soon as any result is found indicating that the two objects are not in collision.
func capsuleVsBoxCollision(c *capsule, b *box, collisionBufferMM float64) bool {
	centerDist := b.pose.Point().Sub(c.center)

	// check if there is a distance between bounding spheres to potentially exit early
	if centerDist.Norm()-((c.length/2)+b.boundingSphereR) > collisionBufferMM {
		return false
	}
	rmA := c.rotationMatrix()
	rmB := b.rotationMatrix()

	// Capsule is modeled as a 0x0xN box, where N = (length/2)-radius.
	// This allows us to check separating axes on a reduced set of projections.

	cutoff := collisionBufferMM + c.radius

	for i := 0; i < 3; i++ {
		if separatingAxisTest1D(&centerDist, &c.capVec, rmA.Row(i), b.halfSize, rmB) > cutoff {
			return false
		}
		if separatingAxisTest1D(&centerDist, &c.capVec, rmB.Row(i), b.halfSize, rmB) > cutoff {
			return false
		}
		for j := 0; j < 3; j++ {
			crossProductPlane := rmA.Row(i).Cross(rmB.Row(j))

			// if edges are parallel, this check is already accounted for by one of the face projections, so skip this case
			if !utils.Float64AlmostEqual(crossProductPlane.Norm(), 0, floatEpsilon) {
				if separatingAxisTest1D(&centerDist, &c.capVec, crossProductPlane, b.halfSize, rmB) > cutoff {
					return false
				}
			}
		}
	}
	return true
}

func capsuleBoxSeparatingAxisDistance(c *capsule, b *box) float64 {
	centerDist := b.pose.Point().Sub(c.center)

	// check if there is a distance between bounding spheres to potentially exit early
	if boundingSphereDist := centerDist.Norm() - ((c.length / 2) + b.boundingSphereR); boundingSphereDist > defaultCollisionBufferMM {
		return boundingSphereDist
	}
	rmA := c.rotationMatrix()
	rmB := b.rotationMatrix()

	// Capsule is modeled as a 0x0xN box, where N = (length/2)-radius.
	// This allows us to check separating axes on a reduced set of projections.

	max := math.Inf(-1)
	for i := 0; i < 3; i++ {
		if separation := separatingAxisTest1D(&centerDist, &c.capVec, rmA.Row(i), b.halfSize, rmB); separation > max {
			max = separation
		}
		if separation := separatingAxisTest1D(&centerDist, &c.capVec, rmB.Row(i), b.halfSize, rmB); separation > max {
			max = separation
		}
		for j := 0; j < 3; j++ {
			crossProductPlane := rmA.Row(i).Cross(rmB.Row(j))

			// if edges are parallel, this check is already accounted for by one of the face projections, so skip this case
			if !utils.Float64AlmostEqual(crossProductPlane.Norm(), 0, floatEpsilon) {
				if separation := separatingAxisTest1D(&centerDist, &c.capVec, crossProductPlane, b.halfSize, rmB); separation > max {
					max = separation
				}
			}
		}
	}
	return max - c.radius
}

func separatingAxisTest1D(positionDelta, capVec *r3.Vector, plane r3.Vector, halfSizeB [3]float64, rmB *RotationMatrix) float64 {
	sum := math.Abs(positionDelta.Dot(plane))
	for i := 0; i < 3; i++ {
		sum -= math.Abs(rmB.Row(i).Mul(halfSizeB[i]).Dot(plane))
	}
	sum -= math.Abs(capVec.Dot(plane))
	return sum
}

// CapsuleIntersectionWithPlane calculates the intersection of a geometry with a plane and returns
// a list of points along the surface of the geometry at the points of intersection.
// It returns an error if the geometry type is unsupported or if points cannot be computed.
// The points returned are in order, in frame of the capsule's parent, and follow the right hand rule around the plane normal.
func CapsuleIntersectionWithPlane(g Geometry, planeNormal, planePoint r3.Vector, numPoints int) ([]r3.Vector, error) {
	c, ok := g.(*capsule)
	if !ok {
		return nil, fmt.Errorf("unsupported geometry type: %T", g)
	}

	// Normalize the plane normal
	planeNormal = planeNormal.Normalize()

	// Calculate the distance from the plane to the capsule's center
	centerToPlane := c.center.Sub(planePoint).Dot(planeNormal) * -1
	// If the distance is greater than the capsule's half-length plus radius, there's no intersection
	if math.Abs(centerToPlane) > c.length/2+c.radius {
		return nil, errors.New("no intersection: plane is too far from capsule")
	}

	capVecNormalized := c.capVec.Normalize()
	capVecDotNormalAbs := math.Abs(capVecNormalized.Dot(planeNormal))

	// Check if the plane is perpendicular to the capsule axis
	if capVecDotNormalAbs < 1e-6 {
		// The plane is perpendicular (or very close to perpendicular) to the capsule axis
		// We'll generate points for two parallel lines and two semicircles

		// Vector perpendicular to both capsule axis and plane normal
		perpVector := planeNormal.Cross(capVecNormalized).Normalize()

		numLinePoints := numPoints / 4   // Number of points for each parallel line
		numCirclePoints := numPoints / 4 // Number of points for each semicircle (excluding endpoints)

		intersectionPoints := make([]r3.Vector, 0, numPoints)

		// Generate points for the first parallel line
		for i := 0; i < numLinePoints; i++ {
			t := float64(i) / float64(numLinePoints-1)
			pt := c.center.Add(capVecNormalized.Mul((t - 0.5) * c.length))
			leftPoint := pt.Add(perpVector.Mul(c.radius))
			intersectionPoints = append(intersectionPoints, leftPoint)
		}

		// Generate points for the first semicircle
		center := c.center.Add(capVecNormalized.Mul(0.5 * c.length))
		for i := 0; i <= numCirclePoints; i++ {
			angle := math.Pi * float64(i) / float64(numCirclePoints+1)
			cosComponent := perpVector.Mul(c.radius * math.Cos(angle))
			sinComponent := capVecNormalized.Mul(c.radius * math.Sin(angle))
			pt := center.Add(cosComponent).Sub(sinComponent)
			intersectionPoints = append(intersectionPoints, pt)
		}

		// Generate points for the second parallel line (in reverse order)
		for i := numLinePoints - 1; i >= 0; i-- {
			t := float64(i) / float64(numLinePoints-1)
			pt := c.center.Add(capVecNormalized.Mul((t - 0.5) * c.length))
			rightPoint := pt.Add(perpVector.Mul(-c.radius))
			intersectionPoints = append(intersectionPoints, rightPoint)
		}

		// Generate points for the second semicircle
		center = c.center.Add(capVecNormalized.Mul(-0.5 * c.length))
		for i := 0; i <= numCirclePoints; i++ {
			angle := math.Pi * float64(i) / float64(numCirclePoints+1)
			cosComponent := perpVector.Mul(c.radius * math.Cos(angle))
			sinComponent := capVecNormalized.Mul(c.radius * math.Sin(angle))
			pt := center.Sub(cosComponent).Sub(sinComponent)
			intersectionPoints = append(intersectionPoints, pt)
		}

		// At the end of the function, before returning the points:
		if len(intersectionPoints) == 0 {
			return nil, errors.New("no intersection points found")
		}

		return intersectionPoints, nil
	}

	// Calculate the semi-major and semi-minor axes of the ellipse
	axisPlaneAngleCos := capVecDotNormalAbs // cosine of angle between capsule axis and plane normal
	a := c.radius / axisPlaneAngleCos
	b := c.radius

	// Calculate the axis intersection
	// The capsule's axis is not perpendicular to the plane normal
	axisIntersection := c.center.Add(capVecNormalized.Mul(centerToPlane / capVecNormalized.Dot(planeNormal)))

	// Create two perpendicular vectors in the plane
	u := planeNormal.Cross(capVecNormalized)
	if u.Norm() < 1e-6 {
		// The capsule axis is parallel or nearly parallel to the plane normal
		// Use Gram-Schmidt process to find a vector perpendicular to the plane normal
		u = r3.Vector{1, 0, 0}
		u = u.Sub(planeNormal.Mul(u.Dot(planeNormal)))
		if u.Norm() < 1e-6 {
			// If u is still too small, this will definitely work
			u = r3.Vector{0, 1, 0}
			u = u.Sub(planeNormal.Mul(u.Dot(planeNormal)))
		}
	}
	u = u.Normalize()
	v := planeNormal.Cross(u)

	// Ensure u is aligned with the capsule's axis projection onto the plane
	uDotCap := u.Dot(capVecNormalized)
	if math.Abs(uDotCap) < math.Abs(v.Dot(capVecNormalized)) {
		u, v = v, u.Mul(-1)
	}
	// Generate points along the intersection ellipse
	intersectionPoints := make([]r3.Vector, 0, numPoints)

	for i := 0; i < numPoints; i++ {
		angle := 2 * math.Pi * float64(i) / float64(numPoints)
		pt := axisIntersection.Add(u.Mul(a * math.Cos(angle))).Add(v.Mul(b * math.Sin(angle)))

		// Check if the point is within the capsule's cylindrical length
		projectedDist := pt.Sub(c.center).Dot(capVecNormalized)
		if math.Abs(projectedDist) <= c.length/2 {
			intersectionPoints = append(intersectionPoints, pt)
		} else if math.Abs(projectedDist) <= c.length/2+c.radius {
			// Project the point onto the hemisphere
			closestEndpoint := c.center.Add(capVecNormalized.Mul(math.Copysign(c.length/2, projectedDist)))
			sphereCenter := closestEndpoint
			sphereIntersection := sphereCenter.Add(pt.Sub(sphereCenter).Normalize().Mul(c.radius))
			intersectionPoints = append(intersectionPoints, sphereIntersection)
		}
	}
	// At the end of the function, before returning the points:
	if len(intersectionPoints) == 0 {
		return nil, errors.New("no intersection points found")
	}

	return intersectionPoints, nil
}
