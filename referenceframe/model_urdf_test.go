package referenceframe

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestParseURDFFile(t *testing.T) {
	// Test a URDF which has prismatic joints
	u, err := ParseModelXMLFile(utils.ResolveFile("referenceframe/testfiles/example_gantry.xml"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(u.DoF()), test.ShouldEqual, 2)

	// Test a URDF will has collision geometries we can evaluate and a DoF of 6
	u, err = ParseModelXMLFile(utils.ResolveFile("referenceframe/testfiles/ur5e.urdf"), "")
	test.That(t, err, test.ShouldBeNil)
	model, ok := u.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, u.Name(), test.ShouldEqual, "ur5")
	test.That(t, len(u.DoF()), test.ShouldEqual, 6)
	modelGeo, err := model.Geometries(make([]Input, len(model.DoF())))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(modelGeo.Geometries()), test.ShouldEqual, 5) // notably we only have 5 geometries for this model

	// Test naming of a URDF to something other than the robot's name element
	u, err = ParseModelXMLFile(utils.ResolveFile("referenceframe/testfiles/ur5e.urdf"), "foo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, u.Name(), test.ShouldEqual, "foo")
}

func TestWorldStateConversion(t *testing.T) {
	foo, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "foo")
	test.That(t, err, test.ShouldBeNil)
	bar, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 1, Y: 2, Z: 3}, "bar")
	test.That(t, err, test.ShouldBeNil)
	ws, err := NewWorldState(
		[]*GeometriesInFrame{NewGeometriesInFrame(World, []spatialmath.Geometry{foo, bar})},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	cfg, err := NewModelFromWorldState(ws, "test")
	test.That(t, err, test.ShouldBeNil)
	bytes, err := xml.MarshalIndent(cfg, "", "  ")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bytes, test.ShouldNotBeNil)
}

func TestBuildMeshMapFromURDF(t *testing.T) {
	testfilesDir := utils.ResolveFile("referenceframe/testfiles")

	t.Run("fails on missing mesh file", func(t *testing.T) {
		urdfData := []byte(`<?xml version="1.0"?>
<robot name="test">
  <link name="link1">
    <collision>
      <geometry>
        <mesh filename="nonexistent.stl"/>
      </geometry>
    </collision>
  </link>
</robot>`)

		_, err := buildMeshMapFromURDF(urdfData, testfilesDir)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed to load mesh file")
	})

	t.Run("fails on unsupported mesh type", func(t *testing.T) {
		// Create a temporary .obj file to test
		objPath := filepath.Join(testfilesDir, "test.obj")
		err := os.WriteFile(objPath, []byte("# test obj file"), 0o644)
		test.That(t, err, test.ShouldBeNil)
		defer os.Remove(objPath)

		urdfData := []byte(`<?xml version="1.0"?>
<robot name="test">
  <link name="link1">
    <collision>
      <geometry>
        <mesh filename="test.obj"/>
      </geometry>
    </collision>
  </link>
</robot>`)

		_, err = buildMeshMapFromURDF(urdfData, testfilesDir)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unsupported mesh file type")
	})
}

func TestUR20URDFWithMeshes(t *testing.T) {
	urdfPath := artifact.MustPath("urdfs/ur20.urdf")
	testfilesDir := utils.ResolveFile(".artifact/data/urdfs")

	// Load URDF, build mesh map, and parse model once for all tests
	xmlData, err := os.ReadFile(urdfPath)
	test.That(t, err, test.ShouldBeNil)
	meshMap, err := buildMeshMapFromURDF(xmlData, testfilesDir)
	test.That(t, err, test.ShouldBeNil)

	modelConfig, err := UnmarshalModelXML(xmlData, "ur20", meshMap)
	test.That(t, err, test.ShouldBeNil)

	model, err := modelConfig.ParseConfig("ur20")
	test.That(t, err, test.ShouldBeNil)

	expectedMeshes := []string{
		"ur20meshes/base.stl",
		"ur20meshes/shoulder.stl",
		"ur20meshes/upperarm.stl",
		"ur20meshes/forearm.stl",
		"ur20meshes/wrist1.stl",
		"ur20meshes/wrist2.stl",
		"ur20meshes/wrist3.stl",
	}

	t.Run("round-trip through RPC", func(t *testing.T) {
		protoResp := KinematicModelToProtobuf(model)
		test.That(t, protoResp.Format, test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_URDF)
		test.That(t, len(protoResp.MeshesByUrdfFilepath), test.ShouldEqual, 7)

		for _, meshFile := range expectedMeshes {
			mesh := protoResp.MeshesByUrdfFilepath[meshFile]
			test.That(t, mesh.ContentType, test.ShouldEqual, "stl")
			test.That(t, len(mesh.Mesh), test.ShouldBeGreaterThan, 1000)
		}

		model2, err := KinematicModelFromProtobuf("ur20", protoResp)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(model2.DoF()), test.ShouldEqual, 6)

		geometries2, err := model2.(*SimpleModel).Geometries(make([]Input, 6))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(geometries2.Geometries()), test.ShouldEqual, 7)

		filePaths := make(map[string]bool)
		for _, g := range geometries2.Geometries() {
			if mesh, ok := g.(*spatialmath.Mesh); ok {
				filePaths[mesh.OriginalFilePath()] = true
			}
		}
		test.That(t, len(filePaths), test.ShouldEqual, 7)
	})

	t.Run("mesh file paths use package URI normalization", func(t *testing.T) {
		test.That(t, string(xmlData), test.ShouldContainSubstring, urdfPackagePrefix+"urdfs/")

		for meshFile := range meshMap {
			test.That(t, strings.HasPrefix(meshFile, urdfPackagePrefix), test.ShouldBeFalse)
			test.That(t, strings.HasSuffix(meshFile, ".stl"), test.ShouldBeTrue)
		}
	})
}
