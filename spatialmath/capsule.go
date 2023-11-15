package spatialmath

import (
	"encoding/json"
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
	return fmt.Sprintf("Type: Capsule, Radius: %.0f, Length: %.0f", c.radius, c.length)
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
func (c *capsule) AlmostEqual(g Geometry) bool {
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
func (c *capsule) CollidesWith(g Geometry) (bool, error) {
	if other, ok := g.(*box); ok {
		return capsuleVsBoxCollision(c, other), nil
	}
	dist, err := c.DistanceFrom(g)
	if err != nil {
		return true, err
	}
	return dist <= CollisionBuffer, nil
}

// CollidesWith checks if the given capsule collides with the given geometry and returns true if it does.
func (c *capsule) DistanceFrom(g Geometry) (float64, error) {
	if other, ok := g.(*box); ok {
		return capsuleVsBoxDistance(c, other), nil
	}
	if other, ok := g.(*capsule); ok {
		return capsuleVsCapsuleDistance(c, other), nil
	}
	if other, ok := g.(*point); ok {
		return capsuleVsPointDistance(c, other.position), nil
	}
	if other, ok := g.(*sphere); ok {
		return capsuleVsSphereDistance(c, other), nil
	}
	return math.Inf(-1), newCollisionTypeUnsupportedError(c, g)
}

func (c *capsule) EncompassedBy(g Geometry) (bool, error) {
	if other, ok := g.(*capsule); ok {
		return capsuleInCapsule(c, other), nil
	}
	if other, ok := g.(*box); ok {
		return capsuleInBox(c, other), nil
	}
	if other, ok := g.(*sphere); ok {
		return capsuleInSphere(c, other), nil
	}
	if _, ok := g.(*point); ok {
		return false, nil
	}
	return true, newCollisionTypeUnsupportedError(c, g)
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
	if dist > CollisionBuffer {
		return capsuleVsMeshDistance(c, other.toMesh())
	}
	return dist
}

// IMPORTANT: meshes are not considered solid. A mesh is not guaranteed to represent an enclosed area. This will measure ONLY the distance
// to the closest triangle in the mesh.
func capsuleVsMeshDistance(c *capsule, other *mesh) float64 {
	lowDist := math.Inf(1)
	for _, t := range other.triangles {
		// Measure distance to each mesh triangle
		dist := capsuleVsTriangleDistance(c, t)
		if dist < lowDist {
			lowDist = dist
		}
	}
	return lowDist
}

func capsuleVsTriangleDistance(c *capsule, other *triangle) float64 {
	capPt, triPt := closestPointsSegmentTriangle(c.segA, c.segB, other)
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
func capsuleVsBoxCollision(c *capsule, b *box) bool {
	centerDist := b.pose.Point().Sub(c.center)

	// check if there is a distance between bounding spheres to potentially exit early
	if centerDist.Norm()-((c.length/2)+b.boundingSphereR) > CollisionBuffer {
		return false
	}
	rmA := c.rotationMatrix()
	rmB := b.rotationMatrix()

	// Capsule is modeled as a 0x0xN box, where N = (length/2)-radius.
	// This allows us to check separating axes on a reduced set of projections.

	cutoff := CollisionBuffer + c.radius

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
	if boundingSphereDist := centerDist.Norm() - ((c.length / 2) + b.boundingSphereR); boundingSphereDist > CollisionBuffer {
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
