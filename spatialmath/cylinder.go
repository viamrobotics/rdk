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

// Cylinder is a finite right circular Cylinder collision geometry.
// Its local-frame Z axis is the Cylinder's central axis; the Cylinder's pose
// is at its center. Unlike capsule, Cylinders may be arbitrarily flat
// (no length >= 2*radius constraint).
//
// Collision dispatch is asymmetric: Cylinder.CollidesWith(other) handles every
// other geometry type via mesh delegation, but other.CollidesWith(Cylinder)
// will hit the other type's default switch branch and return an unsupported-
// pair error. Callers should put the Cylinder on the receiver side, or use a
// swap-and-retry pattern (motionplan/collision.go does this).
type Cylinder struct {
	pose   Pose
	radius float64 // mm, > 0
	height float64 // mm, > 0, total tip-to-tip
	label  string
	// lazily computed and cached
	meshOnce   sync.Once
	cachedMesh *Mesh
}

// NewCylinder instantiates a new Cylinder Geometry. Returns an error if
// radius or height is non-positive.
func NewCylinder(offset Pose, radius, height float64, label string) (Geometry, error) {
	if radius <= 0 || height <= 0 {
		return nil, newBadGeometryDimensionsError(&Cylinder{})
	}
	return &Cylinder{pose: offset, radius: radius, height: height, label: label}, nil
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
	return fmt.Sprintf(
		"Type: Cylinder | Position: X:%.1f, Y:%.1f, Z:%.1f | Radius: %.0f | Height: %.0f",
		p.X, p.Y, p.Z, c.radius, c.height,
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
		utils.Float64AlmostEqual(c.height, other.height, 1e-8)
}

// Transform premultiplies the Cylinder's pose with the given pose and returns a
// new Cylinder. Caches are intentionally zero-valued so that ToMesh is
// recomputed in the new frame on first use.
func (c *Cylinder) Transform(toPremultiply Pose) Geometry {
	return &Cylinder{
		pose:   Compose(toPremultiply, c.pose),
		radius: c.radius,
		height: c.height,
		label:  c.label,
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

// CylinderTessellationSegments is the fixed number of segments around the
// Cylinder's circumference. Total tessellation = 2 * segments (side) +
// 2 * segments (caps) triangles. 16 segments gives ~1.9% chord error at the side wall.
const CylinderTessellationSegments = 16

// ToMesh tessellates the Cylinder into a triangle mesh, computed lazily and cached.
// The mesh is in the Cylinder's local frame (so its pose matches c.pose).
//
//	         top cap (Z = +h/2)
//	         ┌──┬──┬──...
//	side ────┤  │  │       <-- 16 quads, each split into 2 triangles
//	         └──┴──┴──...
//	         bottom cap (Z = -h/2)
func (c *Cylinder) ToMesh() *Mesh {
	c.meshOnce.Do(func() {
		const n = CylinderTessellationSegments
		halfH := c.height / 2
		// Precompute ring vertices on top and bottom.
		top := make([]r3.Vector, n)
		bot := make([]r3.Vector, n)
		for i := 0; i < n; i++ {
			theta := 2 * math.Pi * float64(i) / float64(n)
			x := c.radius * math.Cos(theta)
			y := c.radius * math.Sin(theta)
			top[i] = r3.Vector{X: x, Y: y, Z: halfH}
			bot[i] = r3.Vector{X: x, Y: y, Z: -halfH}
		}
		topCenter := r3.Vector{X: 0, Y: 0, Z: halfH}
		botCenter := r3.Vector{X: 0, Y: 0, Z: -halfH}
		tris := make([]*Triangle, 0, 2*n+2*n)
		// Side wall: each quad (top[i], top[j], bot[j], bot[i]) -> 2 triangles.
		// Winding order: outward-facing normals (right-hand rule).
		for i := 0; i < n; i++ {
			j := (i + 1) % n
			tris = append(tris,
				NewTriangle(bot[i], top[i], top[j]),
				NewTriangle(bot[i], top[j], bot[j]),
			)
		}
		// Top cap: fan from topCenter, normal = +Z.
		for i := 0; i < n; i++ {
			j := (i + 1) % n
			tris = append(tris, NewTriangle(topCenter, top[i], top[j]))
		}
		// Bottom cap: fan from botCenter, normal = -Z (opposite winding).
		for i := 0; i < n; i++ {
			j := (i + 1) % n
			tris = append(tris, NewTriangle(botCenter, bot[j], bot[i]))
		}
		c.cachedMesh = NewMesh(c.pose, tris, c.label)
	})
	return c.cachedMesh
}

// ToProtobuf is not implemented for Cylinder: there is no Cylinder message in
// commonpb. Any attempt to serialize a Cylinder over gRPC must be intercepted
// upstream. This panic is intentional and load-bearing.
func (c *Cylinder) ToProtobuf() *commonpb.Geometry {
	panic("Cylinder.ToProtobuf: unimplemented — no Cylinder message in commonpb")
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
// vertices lie exactly on its surface, "all vertices inside g ⇒ Cylinder inside g".
func (c *Cylinder) EncompassedBy(g Geometry) (bool, error) {
	return c.ToMesh().EncompassedBy(asMeshIfCylinder(g))
}

// ToPoints returns surface sample points by delegating to the tessellated mesh.
func (c *Cylinder) ToPoints(resolution float64) []r3.Vector {
	return c.ToMesh().ToPoints(resolution)
}
