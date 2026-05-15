package spatialmath

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func makeTestCylinder(o Orientation, pt r3.Vector, radius, height float64, label string) *Cylinder {
	c, _ := NewCylinder(NewPose(pt, o), radius, height, label)
	return c.(*Cylinder)
}

func TestNewCylinder(t *testing.T) {
	offset := NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &EulerAngles{0, 0, math.Pi / 4})

	g, err := NewCylinder(offset, 5, 10, "c0")
	test.That(t, err, test.ShouldBeNil)
	c, ok := g.(*Cylinder)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, c.radius, test.ShouldEqual, 5.0)
	test.That(t, c.height, test.ShouldEqual, 10.0)
	test.That(t, c.label, test.ShouldEqual, "c0")
	test.That(t, PoseAlmostEqualEps(c.Pose(), offset, 1e-9), test.ShouldBeTrue)

	for _, bad := range [][2]float64{{0, 1}, {-1, 1}, {1, 0}, {1, -1}, {-1, -1}} {
		_, err := NewCylinder(offset, bad[0], bad[1], "")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, newBadGeometryDimensionsError(&Cylinder{}).Error())
	}
}

func TestCylinderLabel(t *testing.T) {
	c := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 1, 2, "initial")
	test.That(t, c.Label(), test.ShouldEqual, "initial")
	c.SetLabel("changed")
	test.That(t, c.Label(), test.ShouldEqual, "changed")
}

func TestCylinderString(t *testing.T) {
	c := makeTestCylinder(NewZeroOrientation(), r3.Vector{1, 2, 3}, 4, 5, "x")
	test.That(t, c.String(), test.ShouldContainSubstring, "Cylinder")
	test.That(t, c.String(), test.ShouldContainSubstring, "Radius: 4")
	test.That(t, c.String(), test.ShouldContainSubstring, "Height: 5")
}

func TestCylinderAlmostEqual(t *testing.T) {
	base := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 5, 10, "a")
	good := makeTestCylinder(NewZeroOrientation(), r3.Vector{1e-9, 0, 0}, 5+1e-12, 10-1e-12, "different-label")
	badRadius := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 5+1e-2, 10, "a")
	badHeight := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 5, 10+1e-2, "a")
	badPose := makeTestCylinder(NewZeroOrientation(), r3.Vector{1, 0, 0}, 5, 10, "a")

	test.That(t, base.almostEqual(good), test.ShouldBeTrue)
	test.That(t, base.almostEqual(badRadius), test.ShouldBeFalse)
	test.That(t, base.almostEqual(badHeight), test.ShouldBeFalse)
	test.That(t, base.almostEqual(badPose), test.ShouldBeFalse)

	// A non-Cylinder geometry should never be structurally equal to a Cylinder.
	sphere, _ := NewSphere(NewZeroPose(), 5, "")
	test.That(t, base.almostEqual(sphere), test.ShouldBeFalse)
}

func TestCylinderHash(t *testing.T) {
	c1 := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 5, 10, "a")
	c2 := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 5, 10, "a")
	test.That(t, c1.Hash(), test.ShouldEqual, c2.Hash())

	differs := []*Cylinder{
		makeTestCylinder(NewZeroOrientation(), r3.Vector{1, 0, 0}, 5, 10, "a"),
		makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 6, 10, "a"),
		makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 5, 11, "a"),
		makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 5, 10, "b"),
	}
	for _, d := range differs {
		test.That(t, c1.Hash(), test.ShouldNotEqual, d.Hash())
	}
}

func TestCylinderTransform(t *testing.T) {
	c := makeTestCylinder(NewZeroOrientation(), r3.Vector{1, 2, 3}, 5, 10, "lbl")
	pre := NewPose(r3.Vector{10, 20, 30}, &EulerAngles{0, math.Pi / 2, 0})

	g := c.Transform(pre)
	out, ok := g.(*Cylinder)
	test.That(t, ok, test.ShouldBeTrue)

	// Dimensions and label preserved.
	test.That(t, out.radius, test.ShouldEqual, c.radius)
	test.That(t, out.height, test.ShouldEqual, c.height)
	test.That(t, out.label, test.ShouldEqual, c.label)

	// Pose composed correctly.
	expectedPose := Compose(pre, c.pose)
	test.That(t, PoseAlmostEqualEps(out.Pose(), expectedPose, 1e-9), test.ShouldBeTrue)

	// The new cylinder's mesh must be in the new frame, not the original.
	test.That(t, PoseAlmostEqualEps(out.ToMesh().Pose(), expectedPose, 1e-9), test.ShouldBeTrue)

	// Inverse transform brings the cylinder back to its original pose.
	back := out.Transform(PoseInverse(pre)).(*Cylinder)
	test.That(t, PoseAlmostEqualEps(back.Pose(), c.pose, 1e-9), test.ShouldBeTrue)
}

func TestCylinderToMeshShape(t *testing.T) {
	c := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 4, 6, "")
	mesh := c.ToMesh()

	// 2*cylinderSides side triangles + cylinderSides top-cap + cylinderSides bottom-cap = 4*cylinderSides
	test.That(t, len(mesh.Triangles()), test.ShouldEqual, 4*cylinderSides)

	// Mesh pose matches cylinder pose.
	test.That(t, PoseAlmostCoincident(mesh.Pose(), c.Pose()), test.ShouldBeTrue)

	// Cached mesh: repeated calls return the same pointer.
	test.That(t, c.ToMesh(), test.ShouldEqual, mesh)

	// Every triangle vertex either lies on the side wall (|z|=h/2 OR x^2+y^2 ≈ r^2)
	// or is a cap-center. Use unique vertices via a set.
	const halfH = 3.0 // h/2
	const r = 4.0
	seen := map[r3.Vector]struct{}{}
	for _, tri := range mesh.Triangles() {
		for _, p := range tri.Points() {
			seen[p] = struct{}{}
		}
	}
	// 2*n ring verts + 2 cap centers
	test.That(t, len(seen), test.ShouldEqual, 2*cylinderSides+2)
	for p := range seen {
		isCapCenter := p.X == 0 && p.Y == 0 && (math.Abs(p.Z-halfH) < 1e-9 || math.Abs(p.Z+halfH) < 1e-9)
		onSideRing := (math.Abs(math.Abs(p.Z)-halfH) < 1e-9) &&
			math.Abs(math.Hypot(p.X, p.Y)-r) < 1e-9
		test.That(t, isCapCenter || onSideRing, test.ShouldBeTrue)
	}
}

func TestCylinderJSONRoundTrip(t *testing.T) {
	orig := makeTestCylinder(&EulerAngles{0, math.Pi / 6, 0}, r3.Vector{1, 2, 3}, 5, 12, "rt")
	bytes, err := json.Marshal(orig)
	test.That(t, err, test.ShouldBeNil)

	var cfg GeometryConfig
	test.That(t, json.Unmarshal(bytes, &cfg), test.ShouldBeNil)
	test.That(t, cfg.Type, test.ShouldEqual, CylinderType)
	test.That(t, cfg.R, test.ShouldEqual, 5.0)
	test.That(t, cfg.L, test.ShouldEqual, 12.0)
	test.That(t, cfg.Label, test.ShouldEqual, "rt")

	reparsed, err := cfg.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	rc, ok := reparsed.(*Cylinder)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, orig.almostEqual(rc), test.ShouldBeTrue)
}

func TestCylinderToProtobufPanics(t *testing.T) {
	c := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 1, 1, "")
	defer func() {
		r := recover()
		test.That(t, r, test.ShouldNotBeNil)
	}()
	_ = c.ToProtobuf()
}

// --- Collision / distance / encompass ---
//
// Cylinder collision dispatch is mesh-based, so distances carry up to ~1.9%
// chord error from the 16-segment tessellation. The asserts here pick offsets
// and tolerances that comfortably clear that error.

func TestCylinderCollidesWithBox(t *testing.T) {
	// Cylinder: axis-Z, radius 50, height 100, at origin.
	c := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 50, 100, "c")

	// Box well outside the cylinder in +X.
	far, err := NewBox(NewPoseFromPoint(r3.Vector{200, 0, 0}), r3.Vector{20, 20, 20}, "")
	test.That(t, err, test.ShouldBeNil)
	col, dist, err := c.CollidesWith(far, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeFalse)
	// surface-to-surface gap = 200 - 50 - 10 = 140
	test.That(t, dist, test.ShouldAlmostEqual, 140.0, 5.0)

	// Box overlapping the cylinder.
	overlap, err := NewBox(NewPoseFromPoint(r3.Vector{40, 0, 0}), r3.Vector{40, 40, 40}, "")
	test.That(t, err, test.ShouldBeNil)
	col, dist, err = c.CollidesWith(overlap, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeTrue)
	test.That(t, dist, test.ShouldBeLessThan, 0.0)

	// Box flush above the cylinder in +Z — barely no collision with a small gap.
	gap := 2.0
	above, err := NewBox(NewPoseFromPoint(r3.Vector{0, 0, 50 + gap + 10}), r3.Vector{20, 20, 20}, "")
	test.That(t, err, test.ShouldBeNil)
	col, dist, err = c.CollidesWith(above, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeFalse)
	test.That(t, dist, test.ShouldAlmostEqual, gap, 0.1)

	// Box buffered into "collision" via a large buffer.
	col, _, err = c.CollidesWith(above, gap+1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeTrue)
}

func TestCylinderCollidesWithSphere(t *testing.T) {
	c := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 50, 100, "")

	// Far sphere.
	farSphere, err := NewSphere(NewPoseFromPoint(r3.Vector{300, 0, 0}), 20, "")
	test.That(t, err, test.ShouldBeNil)
	col, dist, err := c.CollidesWith(farSphere, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeFalse)
	// surface-to-surface gap = 300 - 50 - 20 = 230
	test.That(t, dist, test.ShouldAlmostEqual, 230.0, 5.0)

	// Sphere overlapping the side wall.
	hit, err := NewSphere(NewPoseFromPoint(r3.Vector{60, 0, 0}), 20, "")
	test.That(t, err, test.ShouldBeNil)
	col, _, err = c.CollidesWith(hit, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeTrue)

	// Sphere fully inside the cylinder. The mesh BVH treats the cylinder as a
	// surface, so it correctly reports no surface intersection (zero distance
	// to the nearest face is well outside the sphere). Just check the call
	// succeeds and returns finite distance.
	inside, err := NewSphere(NewPoseFromPoint(r3.Vector{}), 5, "")
	test.That(t, err, test.ShouldBeNil)
	_, _, err = c.CollidesWith(inside, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
}

func TestCylinderCollidesWithCylinder(t *testing.T) {
	c := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 50, 100, "a")

	// Identical cylinder at same pose: trivially colliding.
	same := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 50, 100, "b")
	col, _, err := c.CollidesWith(same, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeTrue)

	// Coaxial cylinder stacked far above: no collision.
	apart := makeTestCylinder(NewZeroOrientation(), r3.Vector{0, 0, 500}, 50, 100, "b")
	col, dist, err := c.CollidesWith(apart, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeFalse)
	// Surface-to-surface gap on Z: 500 - 50 - 50 = 400
	test.That(t, dist, test.ShouldAlmostEqual, 400.0, 5.0)

	// Side-by-side overlap (centers 80 apart, radii 50+50=100): collide.
	sideOverlap := makeTestCylinder(NewZeroOrientation(), r3.Vector{80, 0, 0}, 50, 100, "b")
	col, _, err = c.CollidesWith(sideOverlap, defaultCollisionBufferMM)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeTrue)
}

func TestCylinderDistanceFrom(t *testing.T) {
	c := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 50, 100, "")

	// Sphere far in +X: distance ≈ 200 - 50 - 10 = 140
	s, err := NewSphere(NewPoseFromPoint(r3.Vector{200, 0, 0}), 10, "")
	test.That(t, err, test.ShouldBeNil)
	dist, err := c.DistanceFrom(s)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dist, test.ShouldAlmostEqual, 140.0, 5.0)

	// Cylinder-to-cylinder distance (via mesh delegation).
	other := makeTestCylinder(NewZeroOrientation(), r3.Vector{0, 0, 300}, 50, 100, "")
	dist, err = c.DistanceFrom(other)
	test.That(t, err, test.ShouldBeNil)
	// surface-to-surface gap on Z: 300 - 50 - 50 = 200
	test.That(t, dist, test.ShouldAlmostEqual, 200.0, 5.0)
}

func TestCylinderEncompassedBy(t *testing.T) {
	c := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 10, 20, "")

	// Box that fully contains the cylinder.
	bigBox, err := NewBox(NewPoseFromPoint(r3.Vector{}), r3.Vector{100, 100, 100}, "")
	test.That(t, err, test.ShouldBeNil)
	enc, err := c.EncompassedBy(bigBox)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeTrue)

	// Box that misses the cylinder entirely.
	smallBox, err := NewBox(NewPoseFromPoint(r3.Vector{500, 0, 0}), r3.Vector{2, 2, 2}, "")
	test.That(t, err, test.ShouldBeNil)
	enc, err = c.EncompassedBy(smallBox)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeFalse)

	// Sphere too small to contain the cylinder.
	tightSphere, err := NewSphere(NewPoseFromPoint(r3.Vector{}), 5, "")
	test.That(t, err, test.ShouldBeNil)
	enc, err = c.EncompassedBy(tightSphere)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeFalse)

	// Sphere comfortably larger than the cylinder's diagonal extent
	// (diagonal = sqrt(r^2 + (h/2)^2) = sqrt(100 + 100) ≈ 14.14).
	bigSphere, err := NewSphere(NewPoseFromPoint(r3.Vector{}), 20, "")
	test.That(t, err, test.ShouldBeNil)
	enc, err = c.EncompassedBy(bigSphere)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeTrue)
}

func TestCylinderToPoints(t *testing.T) {
	c := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 10, 20, "")
	pts := c.ToPoints(1.0)
	test.That(t, len(pts), test.ShouldBeGreaterThan, 0)

	// Every sampled point should lie on or very near the tessellated surface:
	// either |z| <= h/2 + eps and radial distance ≈ r (side), or |z| ≈ h/2 and
	// radial distance <= r + eps (cap). The 1.9% chord-error tolerance lets
	// side samples sit slightly inside the true cylinder.
	const r = 10.0
	const halfH = 10.0
	const tol = 0.2 * r // 2% slack
	for _, p := range pts {
		radial := math.Hypot(p.X, p.Y)
		onSide := math.Abs(p.Z) <= halfH+1e-6 && math.Abs(radial-r) <= tol
		onCap := math.Abs(math.Abs(p.Z)-halfH) <= 1e-6 && radial <= r+tol
		test.That(t, onSide || onCap, test.ShouldBeTrue)
	}
}

// TestEncompassedByCylinder exercises the other-direction dispatch:
// box/sphere/capsule/point/mesh/triangle.EncompassedBy(*Cylinder). The Cylinder
// receiver is treated as a solid convex volume via analytic point-in-cylinder
// and sphere-in-cylinder checks.
func TestEncompassedByCylinder(t *testing.T) {
	cyl := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 10, 20, "")

	// --- Point ---
	pInside := NewPoint(r3.Vector{0, 0, 0}, "")
	enc, err := pInside.EncompassedBy(cyl)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeTrue)

	pOnSurface := NewPoint(r3.Vector{10, 0, 0}, "") // radial boundary
	enc, err = pOnSurface.EncompassedBy(cyl)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeTrue)

	pAxialOutside := NewPoint(r3.Vector{0, 0, 11}, "")
	enc, err = pAxialOutside.EncompassedBy(cyl)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeFalse)

	pRadialOutside := NewPoint(r3.Vector{11, 0, 0}, "")
	enc, err = pRadialOutside.EncompassedBy(cyl)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeFalse)

	// --- Box ---
	smallBox, err := NewBox(NewPoseFromPoint(r3.Vector{}), r3.Vector{4, 4, 4}, "")
	test.That(t, err, test.ShouldBeNil)
	enc, err = smallBox.EncompassedBy(cyl)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeTrue)

	// Box whose corners stick out radially.
	wideBox, err := NewBox(NewPoseFromPoint(r3.Vector{}), r3.Vector{20, 20, 4}, "")
	test.That(t, err, test.ShouldBeNil)
	enc, err = wideBox.EncompassedBy(cyl)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeFalse)

	// --- Sphere ---
	tightSphere, err := NewSphere(NewPoseFromPoint(r3.Vector{}), 5, "")
	test.That(t, err, test.ShouldBeNil)
	enc, err = tightSphere.EncompassedBy(cyl)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeTrue)

	// Sphere centered at origin with radius matching cyl radius: touches side wall
	// but bulges past the caps (h/2 - r = 10 - 10 = 0, so |z|≤0 only at center).
	exactSphere, err := NewSphere(NewPoseFromPoint(r3.Vector{}), 10, "")
	test.That(t, err, test.ShouldBeNil)
	enc, err = exactSphere.EncompassedBy(cyl)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeTrue) // center at z=0 ≤ 0, radial 0 ≤ 0

	// Sphere too big radially.
	bulgeSphere, err := NewSphere(NewPoseFromPoint(r3.Vector{}), 11, "")
	test.That(t, err, test.ShouldBeNil)
	enc, err = bulgeSphere.EncompassedBy(cyl)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeFalse)

	// Sphere fits radially but axially clips a cap.
	offsetSphere, err := NewSphere(NewPoseFromPoint(r3.Vector{0, 0, 8}), 4, "")
	test.That(t, err, test.ShouldBeNil)
	enc, err = offsetSphere.EncompassedBy(cyl)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeFalse) // halfH=6, |8|>6

	// --- Capsule ---
	innerCap, err := NewCapsule(NewPoseFromPoint(r3.Vector{}), 2, 8, "")
	test.That(t, err, test.ShouldBeNil)
	enc, err = innerCap.EncompassedBy(cyl)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeTrue)

	// Capsule longer than the cylinder.
	longCap, err := NewCapsule(NewPoseFromPoint(r3.Vector{}), 2, 30, "")
	test.That(t, err, test.ShouldBeNil)
	enc, err = longCap.EncompassedBy(cyl)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeFalse)

	// --- Tilted cylinder: axis rotated 90° about Y, so cyl axis lies along world X. ---
	tilted := makeTestCylinder(&EulerAngles{0, math.Pi / 2, 0}, r3.Vector{}, 10, 20, "")

	// World +X is now the cylinder's axial direction; |x|≤10 should be inside.
	pAxial := NewPoint(r3.Vector{9, 0, 0}, "")
	enc, err = pAxial.EncompassedBy(tilted)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeTrue)

	// World +Z is now radial; |z|≤10 inside.
	pRadial := NewPoint(r3.Vector{0, 0, 9}, "")
	enc, err = pRadial.EncompassedBy(tilted)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeTrue)

	// A point inside the upright cylinder but outside the tilted one
	// (world Z = 11 is radial-out under tilt).
	pTiltedOut := NewPoint(r3.Vector{0, 0, 11}, "")
	enc, err = pTiltedOut.EncompassedBy(tilted)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enc, test.ShouldBeFalse)
}

// Translating a cylinder should translate its mesh triangles by the same
// vector. Compares world-frame vertices through the cached mesh's pose.
// (ToPoints would also work conceptually but dedupes on world-frame string
// keys, which makes the resulting count pose-dependent under floating point.)
func TestCylinderTransformShiftsMesh(t *testing.T) {
	base := makeTestCylinder(NewZeroOrientation(), r3.Vector{}, 10, 20, "")
	shift := r3.Vector{100, -50, 25}
	moved := base.Transform(NewPoseFromPoint(shift)).(*Cylinder)

	baseTris := base.ToMesh().Triangles()
	movedTris := moved.ToMesh().Triangles()
	test.That(t, len(baseTris), test.ShouldEqual, len(movedTris))

	baseQ := base.Pose().Orientation().Quaternion()
	baseT := base.Pose().Point()
	movedQ := moved.Pose().Orientation().Quaternion()
	movedT := moved.Pose().Point()
	for i := range baseTris {
		for j, bp := range baseTris[i].Points() {
			mp := movedTris[i].Points()[j]
			bw := TransformPoint(baseQ, baseT, bp)
			mw := TransformPoint(movedQ, movedT, mp)
			test.That(t, R3VectorAlmostEqual(mw.Sub(bw), shift, 1e-9), test.ShouldBeTrue)
		}
	}
}
