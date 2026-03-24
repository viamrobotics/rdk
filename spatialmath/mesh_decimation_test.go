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
