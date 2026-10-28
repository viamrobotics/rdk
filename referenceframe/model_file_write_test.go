package referenceframe

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/spatialmath"
)

// conversionURDF references its meshes the ROS way, so the package root resolution is exercised,
// and carries one visual nothing reads.
const conversionURDF = `<robot name="tester">
  <link name="base">
    <collision>
      <origin xyz="0 0 0.05" rpy="0 0 0"/>
      <geometry><mesh filename="package://tester/meshes/base.ply"/></geometry>
    </collision>
    <visual>
      <origin xyz="0 0 0.01" rpy="0 0 0"/>
      <geometry><mesh filename="package://tester/meshes/base.glb"/></geometry>
    </visual>
    <visual>
      <geometry><mesh filename="package://tester/meshes/base.dae"/></geometry>
    </visual>
  </link>
  <link name="tip"/>
  <joint name="j1" type="revolute">
    <origin xyz="0 0 0.1" rpy="0 0 0"/>
    <parent link="base"/>
    <child link="tip"/>
    <axis xyz="0 0 1"/>
    <limit lower="-1.57" upper="1.57" velocity="1.0"/>
  </joint>
</robot>`

// writeConversionFixture lays out a package root with the URDF either beside its meshes or one
// directory down, the way a ROS package keeps it, and returns the URDF's path.
func writeConversionFixture(t *testing.T, root string, urdfInSubdir bool) string {
	t.Helper()
	ply, err := os.ReadFile("testfiles/test_simple.ply")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, os.MkdirAll(filepath.Join(root, "meshes"), 0o755), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(root, "meshes", "base.ply"), ply, 0o600), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(root, "meshes", "base.glb"), []byte("glTF bytes"), 0o600), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(root, "meshes", "base.dae"), []byte("<COLLADA/>"), 0o600), test.ShouldBeNil)
	urdfPath := filepath.Join(root, "robot.urdf")
	if urdfInSubdir {
		test.That(t, os.MkdirAll(filepath.Join(root, "urdf"), 0o755), test.ShouldBeNil)
		urdfPath = filepath.Join(root, "urdf", "robot.urdf")
	}
	test.That(t, os.WriteFile(urdfPath, []byte(conversionURDF), 0o600), test.ShouldBeNil)
	return urdfPath
}

func TestReadURDFModel(t *testing.T) {
	check := func(t *testing.T, m Model, skipped []string) {
		t.Helper()
		test.That(t, m.Name(), test.ShouldEqual, "tester")
		test.That(t, m.DoF(), test.ShouldHaveLength, 1)
		test.That(t, skipped, test.ShouldHaveLength, 1)
		test.That(t, skipped[0], test.ShouldContainSubstring, "base.dae")
		pb, err := ModelToProto(m)
		test.That(t, err, test.ShouldBeNil)
		base := pb.GetLinks()[0]
		test.That(t, base.GetId(), test.ShouldEqual, "base")
		test.That(t, base.GetCollision(), test.ShouldHaveLength, 1)
		test.That(t, len(base.GetCollision()[0].GetMesh().GetMesh()), test.ShouldBeGreaterThan, 0)
		test.That(t, base.GetVisual(), test.ShouldHaveLength, 1)
		visual := base.GetVisual()[0].GetMesh()
		test.That(t, string(visual.GetMesh()), test.ShouldEqual, "glTF bytes")
		test.That(t, visual.GetContentType(), test.ShouldEqual, "glb")
		test.That(t, visual.GetOriginalFileSourcePathDiagnostic(), test.ShouldEqual, "meshes/base.glb")
		test.That(t, base.GetVisual()[0].GetCenter().GetZ(), test.ShouldAlmostEqual, 10)
	}

	t.Run("beside its meshes", func(t *testing.T) {
		m, skipped, err := ReadURDFModel(writeConversionFixture(t, t.TempDir(), false), "")
		test.That(t, err, test.ShouldBeNil)
		check(t, m, skipped)
	})

	t.Run("one directory down with a package root", func(t *testing.T) {
		root := t.TempDir()
		urdfPath := writeConversionFixture(t, root, true)
		_, _, err := ReadURDFModel(urdfPath, "")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "base.ply")
		m, skipped, err := ReadURDFModel(urdfPath, root)
		test.That(t, err, test.ShouldBeNil)
		check(t, m, skipped)
	})
}

func TestWriteKinematicModelFileRoundTrip(t *testing.T) {
	source, _, err := ReadURDFModel(writeConversionFixture(t, t.TempDir(), false), "")
	test.That(t, err, test.ShouldBeNil)
	pb, err := ModelToProto(source)
	test.That(t, err, test.ShouldBeNil)

	out := t.TempDir()
	modelPath := filepath.Join(out, "tester.json")
	test.That(t, WriteKinematicModelFile(modelPath, pb), test.ShouldBeNil)

	// the meshes sit beside the file, and the file holds paths rather than bytes
	for _, rel := range []string{"meshes/tester/collision/base.ply", "meshes/tester/visual/base.glb"} {
		_, err := os.Stat(filepath.Join(out, filepath.FromSlash(rel)))
		test.That(t, err, test.ShouldBeNil)
	}
	raw, err := os.ReadFile(modelPath)
	test.That(t, err, test.ShouldBeNil)
	var doc struct {
		Links []struct {
			Collision []struct {
				Mesh map[string]json.RawMessage `json:"mesh"`
			} `json:"collision"`
		} `json:"links"`
	}
	test.That(t, json.Unmarshal(raw, &doc), test.ShouldBeNil)
	_, inline := doc.Links[0].Collision[0].Mesh["mesh"]
	test.That(t, inline, test.ShouldBeFalse)
	test.That(t, string(doc.Links[0].Collision[0].Mesh["original_file_source_path_diagnostic"]),
		test.ShouldEqual, `"meshes/tester/collision/base.ply"`)

	// the caller's message still has its bytes
	test.That(t, len(pb.GetLinks()[0].GetCollision()[0].GetMesh().GetMesh()), test.ShouldBeGreaterThan, 0)

	// and the file reads back as the same model
	back, err := ReadKinematicModelFile(modelPath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, back.GetLinks()[0].GetCollision()[0].GetMesh().GetMesh(),
		test.ShouldResemble, pb.GetLinks()[0].GetCollision()[0].GetMesh().GetMesh())
	test.That(t, string(back.GetLinks()[0].GetVisual()[0].GetMesh().GetMesh()), test.ShouldEqual, "glTF bytes")
	converted, err := ModelFromProto(back, source.Name())
	test.That(t, err, test.ShouldBeNil)
	r := rand.New(rand.NewSource(1))
	for range 5 {
		inputs := RandomFrameInputs(source, r)
		want, err := source.Transform(inputs)
		test.That(t, err, test.ShouldBeNil)
		got, err := converted.Transform(inputs)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostEqualEps(want, got, 1e-6), test.ShouldBeTrue)
	}

	// the same message writes the same bytes every time
	again := filepath.Join(t.TempDir(), "tester.json")
	test.That(t, WriteKinematicModelFile(again, pb), test.ShouldBeNil)
	rawAgain, err := os.ReadFile(again)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(rawAgain), test.ShouldEqual, string(raw))
}

func meshGeometry(source string, data []byte) *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: &commonpb.Pose{OZ: 1},
		GeometryType: &commonpb.Geometry_Mesh{Mesh: &commonpb.Mesh{
			ContentType: "ply", Mesh: data, OriginalFileSourcePathDiagnostic: proto.String(source),
		}},
	}
}

func TestWriteKinematicModelFileNamesMeshes(t *testing.T) {
	pb := &commonpb.KinematicModel{
		Name: "arm",
		Links: []*commonpb.KinematicLink{
			{Id: "base", Parent: "world", Collision: []*commonpb.Geometry{meshGeometry("meshes/a/part.ply", []byte("ply a"))}},
			{Id: "tip", Parent: "j1", Collision: []*commonpb.Geometry{
				meshGeometry("meshes/b/part.ply", []byte("ply b")),
				meshGeometry("meshes/a/part.ply", []byte("ply a")),
				{GeometryType: &commonpb.Geometry_Mesh{Mesh: &commonpb.Mesh{Mesh: []byte("ply c")}}},
			}},
		},
		Joints: []*commonpb.KinematicJoint{{
			Id: "j1", Parent: "base", Type: commonpb.JointType_JOINT_TYPE_REVOLUTE, Axis: &commonpb.Vector3{Z: 1},
			Limits: &commonpb.JointLimits{Min: proto.Float64(-90), Max: proto.Float64(90)},
		}},
	}

	t.Run("a mesh with nothing to name it by is refused", func(t *testing.T) {
		err := WriteKinematicModelFile(filepath.Join(t.TempDir(), "arm.json"), pb)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `link "tip" collision geometry 2`)
	})

	pb.Links[1].Collision = pb.Links[1].Collision[:2]
	out := t.TempDir()
	test.That(t, WriteKinematicModelFile(filepath.Join(out, "arm.json"), pb), test.ShouldBeNil)

	// same base name, different bytes, gets a number, and the same bytes are written once
	entries, err := os.ReadDir(filepath.Join(out, "meshes", "arm", "collision"))
	test.That(t, err, test.ShouldBeNil)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	test.That(t, names, test.ShouldResemble, []string{"part.ply", "part_2.ply"})

	back, err := ReadKinematicModelFile(filepath.Join(out, "arm.json"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(back.GetLinks()[0].GetCollision()[0].GetMesh().GetMesh()), test.ShouldEqual, "ply a")
	test.That(t, string(back.GetLinks()[1].GetCollision()[0].GetMesh().GetMesh()), test.ShouldEqual, "ply b")
	test.That(t, string(back.GetLinks()[1].GetCollision()[1].GetMesh().GetMesh()), test.ShouldEqual, "ply a")
	test.That(t, back.GetLinks()[1].GetCollision()[1].GetMesh().GetOriginalFileSourcePathDiagnostic(),
		test.ShouldEqual, "meshes/arm/collision/part.ply")
}
