package referenceframe

import (
	"encoding/xml"
	"os"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

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

	testCases := []struct {
		name string
		g    spatialmath.Geometry
	}{
		{"box", box},
		{"sphere", sphere},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			urdf, err := newCollision(tc.g)
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

func TestCapsuleSerialization(t *testing.T) {
	// Test capsule -> URDF (cylinder + 2 spheres) -> capsule round-trip
	t.Run("capsule round-trip", func(t *testing.T) {
		// Create a capsule: radius=60mm, length=260mm (tip to tip)
		// This means cylinder length = 260 - 2*60 = 140mm
		capsule, err := spatialmath.NewCapsule(
			spatialmath.NewPose(r3.Vector{X: 100, Y: 200, Z: 300}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 0}),
			60, 260, "",
		)
		test.That(t, err, test.ShouldBeNil)

		// Convert capsule to URDF collisions (should produce 3: cylinder + 2 spheres)
		collisions, err := newCollisions(capsule)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(collisions), test.ShouldEqual, 3)

		// Verify the decomposition
		var cylCount, sphereCount int
		for _, c := range collisions {
			if c.Geometry.Cylinder != nil {
				cylCount++
				// Cylinder length should be 140mm = 0.14m
				test.That(t, c.Geometry.Cylinder.Length, test.ShouldAlmostEqual, 0.14, 1e-6)
				test.That(t, c.Geometry.Cylinder.Radius, test.ShouldAlmostEqual, 0.06, 1e-6)
			}
			if c.Geometry.Sphere != nil {
				sphereCount++
				test.That(t, c.Geometry.Sphere.Radius, test.ShouldAlmostEqual, 0.06, 1e-6)
			}
		}
		test.That(t, cylCount, test.ShouldEqual, 1)
		test.That(t, sphereCount, test.ShouldEqual, 2)

		// Now parse back as capsule
		capsule2, err := tryParseCapsuleFromCollisions(collisions)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capsule2, test.ShouldNotBeNil)

		// Verify the capsule matches the original
		test.That(t, spatialmath.GeometriesAlmostEqual(capsule, capsule2), test.ShouldBeTrue)
	})

	t.Run("capsule at origin", func(t *testing.T) {
		// Simple case: capsule at origin
		capsule, err := spatialmath.NewCapsule(spatialmath.NewZeroPose(), 200, 1400, "")
		test.That(t, err, test.ShouldBeNil)

		collisions, err := newCollisions(capsule)
		test.That(t, err, test.ShouldBeNil)

		capsule2, err := tryParseCapsuleFromCollisions(collisions)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.GeometriesAlmostEqual(capsule, capsule2), test.ShouldBeTrue)
	})

	t.Run("non-capsule pattern returns nil", func(t *testing.T) {
		// Test that non-capsule patterns return nil (not an error)

		// Single collision
		singleColl := []collision{{}}
		singleColl[0].Geometry.Box = &box{Size: "1 1 1"}
		result, err := tryParseCapsuleFromCollisions(singleColl)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldBeNil)

		// Two spheres only (no cylinder)
		twoSpheres := []collision{{}, {}}
		twoSpheres[0].Geometry.Sphere = &sphere{Radius: 0.1}
		twoSpheres[1].Geometry.Sphere = &sphere{Radius: 0.1}
		result, err = tryParseCapsuleFromCollisions(twoSpheres)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldBeNil)

		// Cylinder + 2 spheres with mismatched radii
		mismatchedRadii := []collision{{}, {}, {}}
		mismatchedRadii[0].Geometry.Cylinder = &cylinder{Radius: 0.1, Length: 1.0}
		mismatchedRadii[1].Geometry.Sphere = &sphere{Radius: 0.2} // Different radius
		mismatchedRadii[2].Geometry.Sphere = &sphere{Radius: 0.1}
		result, err = tryParseCapsuleFromCollisions(mismatchedRadii)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldBeNil)
	})
}

func TestMeshGeometrySerialization(t *testing.T) {
	// Load test files once
	baseStlPath := artifact.MustPath("urdfs/ur20meshes/base.stl")
	stlBytes, err := os.ReadFile(baseStlPath)
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
		urdfCollision.Geometry.Mesh = &mesh{Filename: urdfPackagePrefix + "some_package/meshes/base.stl"}
		meshMap := map[string]*commonpb.Mesh{"meshes/base.stl": {Mesh: stlBytes, ContentType: "stl"}}

		geom, err := urdfCollision.toGeometry(meshMap)
		test.That(t, err, test.ShouldBeNil)
		mesh, _ := geom.(*spatialmath.Mesh)
		test.That(t, mesh.OriginalFilePath(), test.ShouldEqual, "meshes/base.stl")
	})
}
