package referenceframe

import (
	"encoding/xml"
	"os"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/rdk/spatialmath"
)

func TestGeometrySerialization(t *testing.T) {
	box, err := spatialmath.NewBox(
		spatialmath.NewPose(r3.Vector{X: 10, Y: 2, Z: 3}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 30}),
		r3.Vector{X: 4, Y: 5, Z: 6},
		"",
	)
	test.That(t, err, test.ShouldBeNil)
	sphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 3.3, "")
	test.That(t, err, test.ShouldBeNil)
	capsule, err := spatialmath.NewCapsule(spatialmath.NewZeroPose(), 1, 10, "")
	test.That(t, err, test.ShouldBeNil)

	testCases := []struct {
		name    string
		g       spatialmath.Geometry
		success bool
	}{
		{"box", box, true},
		{"sphere", sphere, true},
		{"capsule", capsule, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			urdf, err := newCollision(tc.g)
			if !tc.success {
				test.That(t, err.Error(), test.ShouldContainSubstring, errGeometryTypeUnsupported.Error())
				return
			}
			test.That(t, err, test.ShouldBeNil)
			bytes, err := xml.MarshalIndent(urdf, "", "  ")
			test.That(t, err, test.ShouldBeNil)
			var urdf2 collision
			xml.Unmarshal(bytes, &urdf2)
			g2, err := urdf2.toGeometry(nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, spatialmath.GeometriesAlmostEqual(tc.g, g2), test.ShouldBeTrue)
		})
	}
}

func TestMeshGeometrySerialization(t *testing.T) {
	// Load test files once
	stlBytes, err := os.ReadFile("testfiles/ur20meshes/base.stl")
	test.That(t, err, test.ShouldBeNil)
	plyBytes, err := os.ReadFile("testfiles/test_simple.ply")
	test.That(t, err, test.ShouldBeNil)

	// Helper for round-trip testing
	testRoundTrip := func(t *testing.T, meshBytes []byte, contentType, filePath string, pose spatialmath.Pose) {
		t.Helper()
		protoMesh := &commonpb.Mesh{Mesh: meshBytes, ContentType: contentType}
		mesh, err := spatialmath.NewMeshFromProto(pose, protoMesh, "")
		test.That(t, err, test.ShouldBeNil)
		mesh.SetOriginalFilePath(filePath)

		// Convert to URDF collision and marshal
		urdfCollision, err := newCollision(mesh)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, urdfCollision.Geometry.Mesh.Filename, test.ShouldEqual, filePath)

		xmlBytes, err := xml.MarshalIndent(urdfCollision, "", "  ")
		test.That(t, err, test.ShouldBeNil)

		// Unmarshal and convert back
		var urdfCollision2 collision
		err = xml.Unmarshal(xmlBytes, &urdfCollision2)
		test.That(t, err, test.ShouldBeNil)

		mesh2, err := urdfCollision2.toGeometry(map[string]*commonpb.Mesh{filePath: protoMesh})
		test.That(t, err, test.ShouldBeNil)

		// Verify round-trip preservation
		mesh2Concrete, ok := mesh2.(*spatialmath.Mesh)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mesh2Concrete.OriginalFilePath(), test.ShouldEqual, filePath)
		test.That(t, spatialmath.PoseAlmostEqualEps(mesh.Pose(), mesh2.Pose(), 1e-6), test.ShouldBeTrue)
	}

	t.Run("stl mesh round-trip", func(t *testing.T) {
		testRoundTrip(t, stlBytes, "stl", "meshes/test.stl",
			spatialmath.NewPose(r3.Vector{X: 10, Y: 20, Z: 30}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 45}))
	})

	t.Run("ply mesh round-trip", func(t *testing.T) {
		testRoundTrip(t, plyBytes, "ply", "models/robot.ply", spatialmath.NewZeroPose())
	})

	t.Run("mesh without file path fails", func(t *testing.T) {
		mesh, err := spatialmath.NewMeshFromProto(spatialmath.NewZeroPose(),
			&commonpb.Mesh{Mesh: stlBytes, ContentType: "stl"}, "")
		test.That(t, err, test.ShouldBeNil)

		_, err = newCollision(mesh)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "mesh geometry does not have")
	})

	t.Run("mesh with nil meshMap fails", func(t *testing.T) {
		urdfCollision := &collision{}
		urdfCollision.Geometry.Mesh = &mesh{Filename: "meshes/test.stl"}
		_, err := urdfCollision.toGeometry(nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "mesh map to be provided")
	})

	t.Run("mesh not in meshMap fails", func(t *testing.T) {
		urdfCollision := &collision{}
		urdfCollision.Geometry.Mesh = &mesh{Filename: "meshes/missing.stl"}
		meshMap := map[string]*commonpb.Mesh{"meshes/other.stl": {Mesh: stlBytes, ContentType: "stl"}}
		_, err := urdfCollision.toGeometry(meshMap)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "mesh file not found in mesh map")
	})

	t.Run("package URI normalization", func(t *testing.T) {
		urdfCollision := &collision{Origin: &pose{XYZ: "0 0 0", RPY: "0 0 0"}}
		urdfCollision.Geometry.Mesh = &mesh{Filename: "package://some_package/meshes/base.stl"}
		meshMap := map[string]*commonpb.Mesh{"meshes/base.stl": {Mesh: stlBytes, ContentType: "stl"}}

		geom, err := urdfCollision.toGeometry(meshMap)
		test.That(t, err, test.ShouldBeNil)
		mesh, _ := geom.(*spatialmath.Mesh)
		test.That(t, mesh.OriginalFilePath(), test.ShouldEqual, "meshes/base.stl")
	})
}
