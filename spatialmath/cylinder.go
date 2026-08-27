package spatialmath

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/utils"
)

// cylinderSides is the fixed number of segments around the
// Cylinder's circumference. Total tessellation = 2 * segments (side) +
// 2 * segments (caps, when capped) triangles. 16 segments gives ~1.9% chord
// error at the side wall.
const cylinderSides = 16

// Cylinder is a finite right circular Cylinder collision geometry.
// Its local-frame Z axis is the Cylinder's central axis; the Cylinder's pose
// is at its center. Unlike capsule, Cylinders may be arbitrarily flat
// (no length >= 2*radius constraint).
//
// When capped is false the flat end caps are omitted, producing an open tube:
// a cylindrical *surface* with no interior volume. An open cylinder collides
// only when something crosses its wall — the hollow interior is free space.
// This models open containers (pots, pans, tubes) that a robot must reach into
// without incurring the cost of approximating the wall with many boxes.
type Cylinder struct {
	pose   Pose
	radius float64 // mm, > 0
	height float64 // mm, > 0, total tip-to-tip
	capped bool    // when false, end caps are omitted (open tube; a surface, not a solid)
	label  string
	mesh   *Mesh
}

// NewCylinder instantiates a new solid (capped) Cylinder Geometry. Returns an
// error if radius or height is non-positive.
func NewCylinder(offset Pose, radius, height float64, label string) (Geometry, error) {
	return NewCylinderWithCapped(offset, radius, height, true, label)
}

// NewCylinderWithCapped instantiates a Cylinder with explicit control over
// whether the flat end caps are included. When capped is false the caps are
// omitted, producing an open tube: a cylindrical surface with no interior
// volume, which collides only when something crosses its wall. Returns an error
// if radius or height is non-positive.
func NewCylinderWithCapped(offset Pose, radius, height float64, capped bool, label string) (Geometry, error) {
	if radius <= 0 || height <= 0 {
		return nil, newBadGeometryDimensionsError(&Cylinder{})
	}
	c := &Cylinder{pose: offset, radius: radius, height: height, capped: capped, label: label}
	c.mesh = c.buildMesh()
	return c, nil
}

// Pose returns the pose of the Cylinder's center.
func (c *Cylinder) Pose() Pose {
	return c.pose
}

// Label returns the label of this Cylinder.
func (c *Cylinder) Label() string {
	return c.label
}

// SetLabel sets the label of this Cylinder.
func (c *Cylinder) SetLabel(s string) {
	c.label = s
}

// String returns a human-readable description of the Cylinder.
func (c *Cylinder) String() string {
	p := c.pose.Point()
	kind := "Cylinder"
	if !c.capped {
		kind = "Cylinder(open)"
	}
	return fmt.Sprintf(
		"Type: %s | Position: X:%.1f, Y:%.1f, Z:%.1f | Radius: %.0f | Height: %.0f",
		kind, p.X, p.Y, p.Z, c.radius, c.height,
	)
}

// Hash returns a hash value for this Cylinder. Distinct Cylinders should
// (with high probability) hash to distinct values. Label is part of the hash
// (matches capsule's behavior).
func (c *Cylinder) Hash() int {
	hash := HashPose(c.pose)
	hash += (8 * (int(c.radius*100) + 3000)) * 9
	hash += (9 * (int(c.height*100) + 4000)) * 10
	hash += hashString(c.label) * 11
	if !c.capped {
		hash += 12
	}
	return hash
}

// almostEqual compares the Cylinder with another geometry and returns true
// if the other geometry is a Cylinder with the same pose, radius, and height.
// Label is intentionally NOT part of structural equality (matches capsule's behavior).
func (c *Cylinder) almostEqual(g Geometry) bool {
	other, ok := g.(*Cylinder)
	if !ok {
		return false
	}
	return PoseAlmostEqualEps(c.pose, other.pose, 1e-6) &&
		utils.Float64AlmostEqual(c.radius, other.radius, 1e-8) &&
		utils.Float64AlmostEqual(c.height, other.height, 1e-8) &&
		c.capped == other.capped
}

// Transform premultiplies the Cylinder's pose with the given pose and returns
// a new Cylinder. The mesh's local-frame triangles depend only on radius and
// height, so they are reused as-is; only the wrapping Mesh's pose changes.
func (c *Cylinder) Transform(toPremultiply Pose) Geometry {
	newPose := Compose(toPremultiply, c.pose)
	return &Cylinder{
		pose:   newPose,
		radius: c.radius,
		height: c.height,
		capped: c.capped,
		label:  c.label,
		mesh:   NewMesh(newPose, c.mesh.Triangles(), c.label),
	}
}

// MarshalJSON serializes the Cylinder as a GeometryConfig with type "Cylinder",
// reusing the existing R (radius) and L (length, here = height) fields.
func (c *Cylinder) MarshalJSON() ([]byte, error) {
	config, err := NewGeometryConfig(c)
	if err != nil {
		return nil, err
	}
	config.Type = CylinderType
	config.R = c.radius
	config.L = c.height
	return json.Marshal(config)
}

// ToMesh returns the Cylinder's tessellated triangle mesh, built up front by
// the constructor. The mesh is in the Cylinder's local frame (so its pose
// matches c.pose). An open cylinder omits the two cap fans, leaving only the
// side wall.
//
//	         top cap (Z = +h/2)     <-- omitted when open
//	         +--+--+--...
//	side ----|  |  |       <-- 16 quads, each split into 2 triangles
//	         +--+--+--...
//	         bottom cap (Z = -h/2)  <-- omitted when open
func (c *Cylinder) ToMesh() *Mesh {
	return c.mesh
}

func (c *Cylinder) buildMesh() *Mesh {
	halfH := c.height / 2
	top := make([]r3.Vector, cylinderSides)
	bot := make([]r3.Vector, cylinderSides)
	for i := 0; i < cylinderSides; i++ {
		theta := 2 * math.Pi * float64(i) / float64(cylinderSides)
		x := c.radius * math.Cos(theta)
		y := c.radius * math.Sin(theta)
		top[i] = r3.Vector{X: x, Y: y, Z: halfH}
		bot[i] = r3.Vector{X: x, Y: y, Z: -halfH}
	}
	nTris := 2 * cylinderSides
	if c.capped {
		nTris += 2 * cylinderSides
	}
	tris := make([]*Triangle, 0, nTris)
	// Side wall: each quad (top[i], top[j], bot[j], bot[i]) -> 2 triangles.
	// Winding order: outward-facing normals (right-hand rule).
	for i := 0; i < cylinderSides; i++ {
		j := (i + 1) % cylinderSides
		tris = append(tris,
			NewTriangle(bot[i], top[i], top[j]),
			NewTriangle(bot[i], top[j], bot[j]),
		)
	}
	// End caps are omitted for an open cylinder, leaving a hollow tube.
	if c.capped {
		topCenter := r3.Vector{X: 0, Y: 0, Z: halfH}
		botCenter := r3.Vector{X: 0, Y: 0, Z: -halfH}
		// Top cap: fan from topCenter, normal = +Z.
		for i := 0; i < cylinderSides; i++ {
			j := (i + 1) % cylinderSides
			tris = append(tris, NewTriangle(topCenter, top[i], top[j]))
		}
		// Bottom cap: fan from botCenter, normal = -Z (opposite winding).
		for i := 0; i < cylinderSides; i++ {
			j := (i + 1) % cylinderSides
			tris = append(tris, NewTriangle(botCenter, bot[j], bot[i]))
		}
	}
	return NewMesh(c.pose, tris, c.label)
}

// ToProtobuf converts the Cylinder to its protobuf representation.
//
// uncapped is the negation of capped, so the proto3 default (false) is the solid
// cylinder: the overwhelmingly common case, what NewCylinder builds, and what a
// cylinder means in URDF and SDF.
//
// Note this is a defaulting convention, not a safety guarantee. Cylinder
// collision goes through the tessellated mesh, which is a surface: a point fully
// inside a "solid" cylinder does not register a collision, where the same point
// inside a box, sphere or capsule does. Capping therefore adds two zero-thickness
// cap discs rather than filling the interior, and reading an open tube as solid
// is not automatically the conservative direction.
func (c *Cylinder) ToProtobuf() *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: PoseToProtobuf(c.pose),
		GeometryType: &commonpb.Geometry_Cylinder{
			Cylinder: &commonpb.Cylinder{
				RadiusMm: c.radius,
				HeightMm: c.height,
				Uncapped: !c.capped,
			},
		},
		Label: c.label,
	}
}

// asMeshIfCylinder converts g to its mesh form when g is a *Cylinder. Mesh's
// collision switch does not recognize *Cylinder, so we pre-convert at the
// boundary. Returns g unchanged for any other type.
func asMeshIfCylinder(g Geometry) Geometry {
	if other, ok := g.(*Cylinder); ok {
		return other.ToMesh()
	}
	return g
}

// CollidesWith delegates to the Cylinder's tessellated mesh.
func (c *Cylinder) CollidesWith(g Geometry, buffer float64) (bool, float64, error) {
	return c.ToMesh().CollidesWith(asMeshIfCylinder(g), buffer)
}

// DistanceFrom delegates to the Cylinder's tessellated mesh. Note that the
// returned distance is approximate due to ~1.9% chord error from the
// 16-segment tessellation.
func (c *Cylinder) DistanceFrom(g Geometry) (float64, error) {
	return c.ToMesh().DistanceFrom(asMeshIfCylinder(g))
}

// EncompassedBy delegates to the Cylinder's tessellated mesh. Mesh.EncompassedBy
// checks every triangle vertex; since the Cylinder is convex and its mesh
// vertices lie exactly on its surface, "all vertices inside g => Cylinder inside g".
func (c *Cylinder) EncompassedBy(g Geometry) (bool, error) {
	return c.ToMesh().EncompassedBy(asMeshIfCylinder(g))
}

// ToPoints returns surface sample points by delegating to the tessellated mesh.
func (c *Cylinder) ToPoints(resolution float64) []r3.Vector {
	return c.ToMesh().ToPoints(resolution)
}

// containsPoint reports whether the given world-frame point lies within the
// Cylinder's solid volume (inclusive of surface). An open cylinder is a surface
// with no interior volume, so it contains nothing — only wall-surface
// intersections (via the tessellated mesh) count as collisions.
func (c *Cylinder) containsPoint(p r3.Vector) bool {
	if !c.capped {
		return false
	}
	local := TransformPointByPose(PoseInverse(c.pose), p)
	return math.Abs(local.Z) <= c.height/2 &&
		local.X*local.X+local.Y*local.Y <= c.radius*c.radius
}

// containsSphere reports whether a sphere with the given world-frame center
// and radius lies entirely within the Cylinder. Equivalent to: center lies
// within the Cylinder shrunk by radius along both axial and radial directions.
func (c *Cylinder) containsSphere(center r3.Vector, radius float64) bool {
	if !c.capped {
		// An open cylinder has no interior volume; nothing is contained.
		return false
	}
	halfH := c.height/2 - radius
	rShrink := c.radius - radius
	if halfH < 0 || rShrink < 0 {
		return false
	}
	local := TransformPointByPose(PoseInverse(c.pose), center)
	return math.Abs(local.Z) <= halfH &&
		local.X*local.X+local.Y*local.Y <= rShrink*rShrink
}
