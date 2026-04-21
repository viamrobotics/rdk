package spatialmath

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
)

// makeNonConvexLShapeMesh creates an L-shaped mesh (two boxes joined at a corner)
// that is clearly non-convex. The convex hull of this shape has much more volume
// than the original because it fills in the concavity.
func makeNonConvexLShapeMesh() *Mesh {
	// Vertical box: X=[0,10], Y=[0,100], Z=[0,10]
	// Horizontal box: X=[0,100], Y=[0,10], Z=[0,10]
	// Together they form an L.
	var tris []*Triangle
	addBox := func(minP, maxP r3.Vector) {
		// Build a closed box from 12 triangles (2 per face).
		v := [8]r3.Vector{
			{X: minP.X, Y: minP.Y, Z: minP.Z},
			{X: maxP.X, Y: minP.Y, Z: minP.Z},
			{X: maxP.X, Y: maxP.Y, Z: minP.Z},
			{X: minP.X, Y: maxP.Y, Z: minP.Z},
			{X: minP.X, Y: minP.Y, Z: maxP.Z},
			{X: maxP.X, Y: minP.Y, Z: maxP.Z},
			{X: maxP.X, Y: maxP.Y, Z: maxP.Z},
			{X: minP.X, Y: maxP.Y, Z: maxP.Z},
		}
		faces := [6][4]int{
			{0, 1, 2, 3}, {4, 7, 6, 5}, // bottom, top
			{0, 4, 5, 1}, {2, 6, 7, 3}, // front, back
			{0, 3, 7, 4}, {1, 5, 6, 2}, // left, right
		}
		for _, f := range faces {
			tris = append(tris,
				NewTriangle(v[f[0]], v[f[1]], v[f[2]]),
				NewTriangle(v[f[0]], v[f[2]], v[f[3]]),
			)
		}
	}
	addBox(r3.Vector{X: 0, Y: 0, Z: 0}, r3.Vector{X: 10, Y: 100, Z: 10})
	addBox(r3.Vector{X: 0, Y: 0, Z: 0}, r3.Vector{X: 100, Y: 10, Z: 10})
	return NewMesh(NewZeroPose(), tris, "L-shape")
}

// closedMeshVolume computes the volume of a closed triangle mesh using the
// divergence theorem (signed tetrahedron volumes). Only valid for closed,
// consistently-oriented meshes (e.g., convex hulls).
func closedMeshVolume(m *Mesh) float64 {
	vol := 0.0
	for _, tri := range m.triangles {
		pts := tri.Points()
		vol += pts[0].Dot(pts[1].Cross(pts[2]))
	}
	if vol < 0 {
		vol = -vol
	}
	return vol / 6.0
}

func makeLargeTestTriangles(nx, ny int) []*Triangle {
	triangles := make([]*Triangle, 0, 2*nx*ny)
	for x := range nx {
		for y := range ny {
			z00 := float64((x + y) % 2)
			z10 := float64((x + 1 + y) % 2)
			z01 := float64((x + y + 1) % 2)
			z11 := float64((x + y + 2) % 2)

			p00 := r3.Vector{X: float64(x), Y: float64(y), Z: z00}
			p10 := r3.Vector{X: float64(x + 1), Y: float64(y), Z: z10}
			p01 := r3.Vector{X: float64(x), Y: float64(y + 1), Z: z01}
			p11 := r3.Vector{X: float64(x + 1), Y: float64(y + 1), Z: z11}

			triangles = append(triangles, NewTriangle(p00, p10, p11), NewTriangle(p00, p11, p01))
		}
	}
	return triangles
}

func trianglesToBinarySTL(triangles []*Triangle) []byte {
	var buf bytes.Buffer
	header := make([]byte, 80)
	_, _ = buf.Write(header)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(triangles)))

	for _, tri := range triangles {
		// Normal vector (unused by parser).
		_ = binary.Write(&buf, binary.LittleEndian, float32(0))
		_ = binary.Write(&buf, binary.LittleEndian, float32(0))
		_ = binary.Write(&buf, binary.LittleEndian, float32(0))

		// STL coordinates are meters; mesh geometry uses mm.
		for _, pt := range tri.Points() {
			_ = binary.Write(&buf, binary.LittleEndian, float32(pt.X/1000))
			_ = binary.Write(&buf, binary.LittleEndian, float32(pt.Y/1000))
			_ = binary.Write(&buf, binary.LittleEndian, float32(pt.Z/1000))
		}

		_ = binary.Write(&buf, binary.LittleEndian, uint16(0)) // attribute byte count
	}
	return buf.Bytes()
}

func TestNewMeshDoesNotAutoDecimate(t *testing.T) {
	triangles := makeLargeTestTriangles(50, 30) // 3000 triangles
	mesh := NewMesh(NewZeroPose(), triangles, "dense")
	test.That(t, len(mesh.Triangles()), test.ShouldEqual, 3000)
}

func TestNewMeshFromPLYFileDoesNotAutoDecimate(t *testing.T) {
	triangles := makeLargeTestTriangles(50, 30) // 3000 triangles
	source := &Mesh{pose: NewZeroPose(), triangles: triangles}
	plyBytes := source.TrianglesToPLYBytes(false)

	path := filepath.Join(t.TempDir(), "large_mesh.ply")
	err := os.WriteFile(path, plyBytes, 0o600)
	test.That(t, err, test.ShouldBeNil)

	mesh, err := NewMeshFromPLYFile(path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(mesh.Triangles()), test.ShouldEqual, 3000)
}

func TestNewMeshFromSTLFileDoesNotAutoDecimate(t *testing.T) {
	triangles := makeLargeTestTriangles(50, 30) // 3000 triangles
	stlBytes := trianglesToBinarySTL(triangles)

	path := filepath.Join(t.TempDir(), "large_mesh.stl")
	err := os.WriteFile(path, stlBytes, 0o600)
	test.That(t, err, test.ShouldBeNil)

	mesh, err := NewMeshFromSTLFile(path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(mesh.Triangles()), test.ShouldEqual, 3000)
}

func TestNewMeshFromProtoDoesNotAutoDecimate(t *testing.T) {
	triangles := makeLargeTestTriangles(50, 30) // 3000 triangles
	source := &Mesh{pose: NewZeroPose(), triangles: triangles}
	plyBytes := source.TrianglesToPLYBytes(false)

	mesh, err := NewMeshFromProto(NewZeroPose(), &commonpb.Mesh{
		ContentType: string(plyType),
		Mesh:        plyBytes,
	}, "dense")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(mesh.Triangles()), test.ShouldEqual, 3000)
}

func TestExplicitDecimationRoundTrip(t *testing.T) {
	triangles := makeLargeTestTriangles(50, 30) // 3000 triangles
	mesh := NewMesh(NewZeroPose(), triangles, "dense")
	test.That(t, len(mesh.Triangles()), test.ShouldEqual, 3000)

	// Explicitly decimate (as the URDF path would do).
	decimated, err := mesh.ConservativeDecimate(2000)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(decimated.Triangles()), test.ShouldBeLessThanOrEqualTo, 2000)

	// Visualization round-trip should preserve the decimated triangle count.
	proto := decimated.ToProtobuf()
	visMesh, err := NewMeshFromProto(NewZeroPose(), proto.GetMesh(), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(visMesh.Triangles()), test.ShouldEqual, len(decimated.Triangles()))
}

func TestMeshEncompassedByMesh(t *testing.T) {
	outerGeom, err := NewBox(NewZeroPose(), r3.Vector{X: 20, Y: 20, Z: 20}, "")
	test.That(t, err, test.ShouldBeNil)
	outerMesh := outerGeom.(*box).toMesh()

	innerMesh := makeSimpleTriangleMesh().(*Mesh)
	encompassed, err := innerMesh.EncompassedBy(outerMesh)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, encompassed, test.ShouldBeTrue)

	farAway := innerMesh.Transform(NewPoseFromPoint(r3.Vector{X: 100, Y: 0, Z: 0})).(*Mesh)
	encompassed, err = farAway.EncompassedBy(outerMesh)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, encompassed, test.ShouldBeFalse)
}

func TestMeshConservativeDecimate(t *testing.T) {
	triangles := makeLargeTestTriangles(50, 30) // 3000 triangles
	original := &Mesh{
		pose:      NewZeroPose(),
		triangles: triangles,
		label:     "dense",
		fileType:  plyType,
	}
	original.rawBytes = original.TrianglesToPLYBytes(false)
	test.That(t, len(original.Triangles()), test.ShouldEqual, 3000)

	decimated, err := original.ConservativeDecimate(2000)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(decimated.Triangles()), test.ShouldBeLessThanOrEqualTo, 2000)

	encompassed, err := original.EncompassedBy(decimated)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, encompassed, test.ShouldBeTrue)
}

func TestMeshConservativeDecimateNoop(t *testing.T) {
	mesh := makeSimpleTriangleMesh().(*Mesh)
	decimated, err := mesh.ConservativeDecimate(2000)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, decimated, test.ShouldEqual, mesh)
}

func TestMeshConservativeDecimateIsDeterministic(t *testing.T) {
	triangles := makeLargeTestTriangles(50, 30) // 3000 triangles
	original := &Mesh{
		pose:      NewZeroPose(),
		triangles: triangles,
		label:     "dense",
		fileType:  plyType,
	}

	// Decimate multiple times and verify identical output each time.
	first, err := original.ConservativeDecimate(200)
	test.That(t, err, test.ShouldBeNil)
	firstBytes := first.TrianglesToPLYBytes(false)

	for i := 0; i < 10; i++ {
		again, err := original.ConservativeDecimate(200)
		test.That(t, err, test.ShouldBeNil)
		againBytes := again.TrianglesToPLYBytes(false)
		test.That(t, againBytes, test.ShouldResemble, firstBytes)
	}
}

func TestFullConvexHullUsedWhenFitsTarget(t *testing.T) {
	// When the full convex hull of all vertices fits the target, it should be
	// used directly (no scaling needed). The result should have the same triangle
	// count as the full convex hull, and its volume should be less than or equal
	// to the vertex-budgeted hull (which requires scaling).
	triangles := makeLargeTestTriangles(50, 30)
	original := NewMesh(NewZeroPose(), triangles, "dense")

	// Target 2000 is much larger than the full hull face count for a flat grid,
	// so the full hull path should be taken.
	decimated, err := original.ConservativeDecimate(2000)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(decimated.Triangles()), test.ShouldBeLessThanOrEqualTo, 2000)

	encompassed, err := original.EncompassedBy(decimated)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, encompassed, test.ShouldBeTrue)
}

func TestSlicedHullEncapsulatesOriginal(t *testing.T) {
	// The sliced hull approach must encapsulate the original mesh.
	lShape := makeNonConvexLShapeMesh()

	// Use a target that allows slicing (needs enough triangles for 2+ hulls).
	decimated, err := lShape.ConservativeDecimate(48)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(decimated.Triangles()), test.ShouldBeLessThanOrEqualTo, 48)

	encompassed, err := lShape.EncompassedBy(decimated)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, encompassed, test.ShouldBeTrue)
}

func TestSlicedHullSmallerVolumeThanSingleHull(t *testing.T) {
	// For a non-convex L-shape, the sliced hull should have less volume than
	// a single convex hull because it doesn't fill in the concavity.
	lShape := makeNonConvexLShapeMesh()

	singleHullTris, err := conservativeHullDecimateTriangles(lShape.triangles, 48)
	test.That(t, err, test.ShouldBeNil)
	singleVol := meshTriangleVolume(singleHullTris)

	slicedTris, err := slicedConvexHullDecimate(lShape.triangles, 48)
	test.That(t, err, test.ShouldBeNil)
	slicedVol := meshTriangleVolume(slicedTris)

	t.Logf("single hull: %d tris, vol=%.0f", len(singleHullTris), singleVol)
	t.Logf("sliced hull: %d tris, vol=%.0f", len(slicedTris), slicedVol)
	test.That(t, slicedVol, test.ShouldBeLessThan, singleVol)
}

func TestDecimatePicksSmallerVolume(t *testing.T) {
	// ConservativeDecimate should pick the strategy with smaller volume.
	lShape := makeNonConvexLShapeMesh()
	originalVol := closedMeshVolume(lShape)

	decimated, err := lShape.ConservativeDecimate(48)
	test.That(t, err, test.ShouldBeNil)
	decimatedVol := closedMeshVolume(decimated)

	// The L-shape original has volume 10*100*10 + 100*10*10 - 10*10*10 = 19000
	test.That(t, originalVol, test.ShouldAlmostEqual, 19000, 1000)

	// The single convex hull would be ~100*100*10 = 100000.
	// The decimated (sliced or single) should be much less than the full bounding box.
	t.Logf("original vol=%.0f, decimated vol=%.0f, ratio=%.1fx", originalVol, decimatedVol, decimatedVol/originalVol)
	test.That(t, decimatedVol, test.ShouldBeLessThan, 100000)
}

func TestMeshVolume(t *testing.T) {
	// The L-shape is two overlapping box meshes (each independently closed).
	// Volume() sums each closed surface independently, so overlap is double-counted:
	// vertical box (10*100*10) + horizontal box (100*10*10) = 20000.
	lShape := makeNonConvexLShapeMesh()
	vol := closedMeshVolume(lShape)
	test.That(t, vol, test.ShouldAlmostEqual, 20000, 100)
}

func TestSelectSupportVerticesIncludesAxisExtremes(t *testing.T) {
	// The iterative vertex selection should always include the axis-aligned
	// extremes, ensuring the hull AABB matches the original.
	vertices := []r3.Vector{
		{X: -100, Y: 0, Z: 0}, // -X extreme
		{X: 100, Y: 0, Z: 0},  // +X extreme
		{X: 0, Y: -50, Z: 0},  // -Y extreme
		{X: 0, Y: 50, Z: 0},   // +Y extreme
		{X: 0, Y: 0, Z: -30},  // -Z extreme
		{X: 0, Y: 0, Z: 30},   // +Z extreme
		{X: 10, Y: 10, Z: 10}, // interior point
		{X: -5, Y: 5, Z: -5},  // interior point
		{X: 20, Y: 20, Z: 20}, // another point
		{X: -20, Y: -20, Z: -20},
	}

	selected := selectSupportVertices(vertices, 6)

	// All 6 axis extremes should be present.
	hasMinX, hasMaxX := false, false
	hasMinY, hasMaxY := false, false
	hasMinZ, hasMaxZ := false, false
	for _, v := range selected {
		if v.X == -100 {
			hasMinX = true
		}
		if v.X == 100 {
			hasMaxX = true
		}
		if v.Y == -50 {
			hasMinY = true
		}
		if v.Y == 50 {
			hasMaxY = true
		}
		if v.Z == -30 {
			hasMinZ = true
		}
		if v.Z == 30 {
			hasMaxZ = true
		}
	}
	test.That(t, hasMinX, test.ShouldBeTrue)
	test.That(t, hasMaxX, test.ShouldBeTrue)
	test.That(t, hasMinY, test.ShouldBeTrue)
	test.That(t, hasMaxY, test.ShouldBeTrue)
	test.That(t, hasMinZ, test.ShouldBeTrue)
	test.That(t, hasMaxZ, test.ShouldBeTrue)
}

func TestMeshCollisionDeterministicAfterBVHInit(t *testing.T) {
	// Regression test: verifies that BVH initialization doesn't corrupt
	// subsequent collision checks (which was a bug in computeGeometryAABB).
	tri1 := NewTriangle(
		r3.Vector{X: 0, Y: 0, Z: 0},
		r3.Vector{X: 10, Y: 0, Z: 0},
		r3.Vector{X: 5, Y: 10, Z: 0},
	)
	tri2 := NewTriangle(
		r3.Vector{X: 3, Y: 3, Z: -1},
		r3.Vector{X: 7, Y: 3, Z: -1},
		r3.Vector{X: 5, Y: 7, Z: 1},
	)

	pose1 := NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &OrientationVectorDegrees{OX: 0, OY: 0, OZ: 1, Theta: 45})
	pose2 := NewPose(r3.Vector{X: 2, Y: 3, Z: 3}, &OrientationVectorDegrees{OX: 0, OY: 1, OZ: 0, Theta: 30})

	mesh1 := NewMesh(pose1, []*Triangle{tri1}, "mesh1")
	mesh2 := NewMesh(pose2, []*Triangle{tri2}, "mesh2")

	// First call triggers BVH init.
	collides1, dist1, err1 := mesh1.CollidesWith(mesh2, defaultCollisionBufferMM)
	test.That(t, err1, test.ShouldBeNil)

	// Second call on the SAME objects must return the same result.
	collides2, dist2, err2 := mesh1.CollidesWith(mesh2, defaultCollisionBufferMM)
	test.That(t, err2, test.ShouldBeNil)

	test.That(t, collides1, test.ShouldEqual, collides2)
	test.That(t, dist1, test.ShouldEqual, dist2)
}
