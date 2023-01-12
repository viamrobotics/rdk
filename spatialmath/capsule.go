package spatialmath

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/utils"
)

// capsule is a collision geometry that represents a capsule, it has a pose and a radius that fully define it.
//    __________________
//   /                  \
// x|  |--------------|  |x
//   \__________________/
// Length is the distance between the x's, or internal segment length + 2*radius
type capsule struct {
	// this is the pose of one end of the capsule. The full capsule extends `length` mm outwards in the direction of
	// the pose's orientation
	pose   Pose
	radius float64
	length float64 // total length of the capsule, tip to tip
	label  string
	
	// These values are generated at geometry creation time and should not be altered by hand
	// They are stoed here because they are useful and expensive to calculate
	segA r3.Vector // Proximal endpoint of capsule line segment. First point from `pose` to be surrounded by `radius` of capsule
	segB r3.Vector // Distal endpoint of capsule line segment. Most distal point to be surrounded by `radius` of capsule
	normal r3.Vector // Precomputed unit vector colinear with capsule line segment
}

// NewCapsule instantiates a new capsule Geometry.
func NewCapsule(offset Pose, radius float64, length float64, label string) (Geometry, error) {
	if radius < 0 {
		return nil, newBadGeometryDimensionsError(&capsule{})
	}
	if length == radius * 2 {
		return NewSphere(offset, radius, label)
	}
	if length < radius * 2 {
		return nil, newBadGeometryDimensionsError(&capsule{})
	}
	return precalcCapsule(offset, radius, length, label), nil
}

// Will precalculate the linear endpoints for a capsule
func precalcCapsule(offset Pose, radius float64, length float64, label string) Geometry {
	segA := Compose(offset, NewPoseFromPoint(r3.Vector{0,0,radius})).Point()
	segB := Compose(offset, NewPoseFromPoint(r3.Vector{0,0,radius + length})).Point()
	normal := segB.Sub(segA).Normalize()
	
	return &capsule{
		pose: offset,
		radius: radius,
		length: length,
		label: label,
		segA: segA,
		segB: segB,
		normal: normal,
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

// String returns a human readable string that represents the capsuleCreator.
func (c *capsule) String() string {
	return fmt.Sprintf("Type: Capsule, Radius: %.0f, Length: %.0f", c.radius, c.length)
}

// Label returns the label of this capsule.
func (c *capsule) Label() string {
	if c != nil {
		return c.label
	}
	return ""
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
	return PoseAlmostEqual(c.pose, other.pose) && utils.Float64AlmostEqual(c.radius, other.radius, 1e-8)
}

// Transform premultiplies the capsule pose with a transform, allowing the capsule to be moved in space.
func (c *capsule) Transform(toPremultiply Pose) Geometry {
	return precalcCapsule(Compose(toPremultiply, c.pose), c.radius, c.length, c.label)
}

// ToProto converts the capsule to a Geometry proto message.
func (c *capsule) ToProtobuf() *commonpb.Geometry {
	return &commonpb.Geometry{}
		//~ Center: PoseToProtobuf(c.pose),
		//~ GeometryType: &commonpb.Geometry_Capsule{
			//~ Capsule: &commonpb.Capsule{
				//~ RadiusMm: c.radius,
			//~ },
		//~ },
		//~ Label: c.label,
	//~ }
}

// CollidesWith checks if the given capsule collides with the given geometry and returns true if it does.
func (c *capsule) CollidesWith(g Geometry) (bool, error) {
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

// ToPointCloud converts a capsule geometry into []r3.Vector. This method takes one argument which determines
// how many points should like on the capsule's surface. If the argument is set to 0. we automatically
// substitute the value with defaultTotalSpherePoints.
func (c *capsule) ToPoints(resolution float64) []r3.Vector {
	// check for user defined spacing
	area := 4. * math.Pi * c.radius * c.radius
	if resolution == 0. {
		resolution = defaultPointDensity
	}
	iter := area*resolution
	if iter < defaultMinSpherePoints {
		iter = defaultMinSpherePoints
	}
	
	// First points are placed on the hemisphere endcaps
	// code taken from: https://stackoverflow.com/questions/9600801/evenly-distributing-n-points-on-a-sphere
	// we want the number of points on the sphere's surface to grow in proportion with the sphere's radius
	phi := math.Pi * (3.0 - math.Sqrt(5.0)) // golden angle in radians
	var vecList []r3.Vector
	segLen := c.length - (2 * c.radius)
	for i := 0.; i < iter; i++ {
		y := 1 - (i/(iter-1))*2      // y goes from 1 to -1
		radius := math.Sqrt(1 - y*y) // radius at y
		theta := phi * i             // golden angle increment
		x := (math.Cos(theta) * radius) * c.radius
		z := (math.Sin(theta) * radius) * c.radius
		// distal hemisphere
		if z > 0 {
			z += segLen
		}
		vec := r3.Vector{x, y * c.radius, z}
		vecList = append(vecList, vec)
	}
	
	// Now distribute points along the cylindrical shaft
	totalShaftPts := (c.radius * c.length)*resolution
	ptsPerRing := totalShaftPts/(c.length*resolution)
	ringCnt := math.Floor(totalShaftPts/ptsPerRing)
	zInc := c.length/(ringCnt + 1)
	for ring := 1.; ring <= ringCnt; ring++ {
		for ringPt := 0.; ringPt < ptsPerRing; ringPt++ {
			theta := 2. * math.Pi * (ringPt/ptsPerRing)
			vecList = append(vecList, r3.Vector{math.Cos(theta) * c.radius, math.Sin(theta) * c.radius, zInc * ring})
		}
	}
	
	return transformPointsToPose(vecList, c.Pose())
}

func capsuleVsPointDistance(c *capsule, other r3.Vector) float64 {
	return DistToLineSegment(c.segA, c.segB, other) + c.radius
}

func capsuleVsSphereDistance(c *capsule, other *sphere) float64 {
	return DistToLineSegment(c.segA, c.segB, other.pose.Point()) + c.radius + other.radius
}

func capsuleVsCapsuleDistance(c, other *capsule) float64 {
	return SegmentDistanceToSegment(c.segA, c.segB, other.segA, other.segB)
}

func capsuleVsBoxDistance(c *capsule, other *box) float64 {
	// Capsules are not polyhedrons, so separating axis test is not ideal here
	return capsuleVsMeshDistance(c, other.toMesh())
}

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
	return inner.segA.Sub(outer.segA).Norm() + inner.radius <= outer.radius &&
			inner.segB.Sub(outer.segA).Norm() + inner.radius <= outer.radius &&
			inner.segB.Sub(outer.segB).Norm() + inner.radius <= outer.radius &&
			inner.segB.Sub(outer.segB).Norm() + inner.radius <= outer.radius
}

// capsuleInBox returns a bool describing if the given capsule is fully encompassed by the given box.
func capsuleInBox(c *capsule, b *box) bool {
	return -pointVsBoxDistance(c.segA, b) >= c.radius && -pointVsBoxDistance(c.segB, b) >= c.radius
}

// capsuleInSphere returns a bool describing if the given capsule is fully encompassed by the given sphere.
func capsuleInSphere(c *capsule, s *sphere) bool {
	return c.segA.Sub(s.pose.Point()).Norm() + c.radius <= s.radius && c.segB.Sub(s.pose.Point()).Norm() + c.radius <= s.radius
}
