package referenceframe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.viam.com/test"
	"google.golang.org/protobuf/proto"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// The ur5e written as a kinematic model file describes the same arm as the SVA file it was
// generated from, whichever entry point reads it.
func TestKinematicModelFileMatchesSVA(t *testing.T) {
	sva, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	want, err := ModelToProto(sva)
	test.That(t, err, test.ShouldBeNil)

	fixture := utils.ResolveFile("referenceframe/testfiles/ur5e_kinematic_model.json")
	data, err := os.ReadFile(fixture)
	test.That(t, err, test.ShouldBeNil)

	readers := map[string]func() (Model, error){
		"KinematicModelFromFile": func() (Model, error) { return KinematicModelFromFile(fixture, "") },
		"ParseModelJSONFile":     func() (Model, error) { return ParseModelJSONFile(fixture, "") },
		// bytes alone are enough for a model with no mesh files, so nothing is read beside the file
		"UnmarshalModelJSON": func() (Model, error) { return UnmarshalModelJSON(data, "") },
	}
	for name, read := range readers {
		t.Run(name, func(t *testing.T) {
			model, err := read()
			test.That(t, err, test.ShouldBeNil)
			test.That(t, model.Name(), test.ShouldEqual, "UR5e")
			assertSameKinematics(t, sva, model)
			got, err := ModelToProto(model)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, proto.Equal(got, want), test.ShouldBeTrue)
		})
	}

	named, err := KinematicModelFromFile(fixture, "arm")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, named.Name(), test.ShouldEqual, "arm")

	pb, err := ReadKinematicModelFile(fixture)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, proto.Equal(pb, want), test.ShouldBeTrue)
}

// writeModelDir lays out a model file under dir/urdf with its meshes under dir/meshes, the usual
// ROS layout, and returns the model file's path.
func writeModelDir(t *testing.T, dir, doc string) string {
	t.Helper()
	test.That(t, os.MkdirAll(filepath.Join(dir, "urdf"), 0o755), test.ShouldBeNil)
	test.That(t, os.MkdirAll(filepath.Join(dir, "meshes"), 0o755), test.ShouldBeNil)
	ply, err := os.ReadFile(utils.ResolveFile("referenceframe/testfiles/test_simple.ply"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dir, "meshes", "part.ply"), ply, 0o644), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dir, "meshes", "part.glb"), []byte("glTF-not-really"), 0o644), test.ShouldBeNil)
	modelPath := filepath.Join(dir, "urdf", "model.json")
	test.That(t, os.WriteFile(modelPath, []byte(doc), 0o644), test.ShouldBeNil)
	return modelPath
}

const meshModelDoc = `{
  "name": "meshy",
  "links": [
    {"id": "base", "parent": "world",
     "collision": [{"mesh": {"original_file_source_path_diagnostic": "../meshes/part.ply"}}],
     "visual": [{"mesh": {"content_type": "model/gltf-binary",
                           "original_file_source_path_diagnostic": "../meshes/part.glb"}}]},
    {"id": "slide", "parent": "slide_joint",
     "collision": [{"mesh": {"original_file_source_path_diagnostic": "../meshes/part.ply"}}]}
  ],
  "joints": [
    {"id": "slide_joint", "parent": "base", "type": "JOINT_TYPE_PRISMATIC", "axis": {"z": 1},
     "limits": {"min": 0, "max": 100},
     "geometry": {"mesh": {"original_file_source_path_diagnostic": "../meshes/part.ply"}}}
  ]
}`

// Meshes come from files beside the model, the content type follows the extension and the path
// stays on the message so a client can tell which mesh is which.
func TestKinematicModelFileLoadsMeshes(t *testing.T) {
	modelPath := writeModelDir(t, t.TempDir(), meshModelDoc)
	ply, err := os.ReadFile(utils.ResolveFile("referenceframe/testfiles/test_simple.ply"))
	test.That(t, err, test.ShouldBeNil)

	pb, err := ReadKinematicModelFile(modelPath)
	test.That(t, err, test.ShouldBeNil)
	base := pb.GetLinks()[0]
	collision := base.GetCollision()[0].GetMesh()
	test.That(t, collision.GetContentType(), test.ShouldEqual, "ply")
	test.That(t, collision.GetMesh(), test.ShouldResemble, ply)
	test.That(t, collision.GetOriginalFileSourcePathDiagnostic(), test.ShouldEqual, "../meshes/part.ply")
	visual := base.GetVisual()[0].GetMesh()
	test.That(t, visual.GetContentType(), test.ShouldEqual, "glb")
	test.That(t, string(visual.GetMesh()), test.ShouldEqual, "glTF-not-really")
	test.That(t, visual.GetOriginalFileSourcePathDiagnostic(), test.ShouldEqual, "../meshes/part.glb")
	test.That(t, pb.GetLinks()[1].GetCollision()[0].GetMesh().GetMesh(), test.ShouldResemble, ply)
	test.That(t, pb.GetJoints()[0].GetGeometry().GetMesh().GetMesh(), test.ShouldResemble, ply)

	model, err := KinematicModelFromFile(modelPath, "")
	test.That(t, err, test.ShouldBeNil)
	geometries, err := model.Geometries([]Input{50})
	test.That(t, err, test.ShouldBeNil)
	meshes := 0
	for _, g := range geometries.Geometries() {
		mesh, ok := g.(*spatial.Mesh)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mesh.OriginalFilePath(), test.ShouldEqual, "../meshes/part.ply")
		meshes++
	}
	test.That(t, meshes, test.ShouldEqual, 3)
	sm, ok := model.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sm.VisualGeometries("base"), test.ShouldHaveLength, 1)
	test.That(t, sm.VisualGeometries("base")[0].MeshContentType, test.ShouldEqual, "glb")

	served, err := ModelToProto(model)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, served.GetLinks()[0].GetCollision()[0].GetMesh().GetOriginalFileSourcePathDiagnostic(),
		test.ShouldEqual, "../meshes/part.ply")
	test.That(t, served.GetLinks()[0].GetVisual()[0].GetMesh().GetMesh(), test.ShouldResemble, visual.GetMesh())

	// bytes alone give no directory to find the meshes in
	data, err := os.ReadFile(modelPath)
	test.That(t, err, test.ShouldBeNil)
	_, err = UnmarshalModelJSON(data, "")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "without the directory of the model file")
}

// A mesh that already carries bytes, the way a model dumped from the wire does, is not read
// again, whatever its path looks like, and its content type is matched without regard to case
// or taken from the path when it was left out.
func TestKinematicModelFileKeepsInlineMesh(t *testing.T) {
	doc := `{"name": "inline", "links": [{"id": "base", "parent": "world",
		"collision": [{"mesh": {"content_type": "PLY", "mesh": "cGx5", "original_file_source_path_diagnostic": "/opt/meshes/part.ply"}}],
		"visual": [{"mesh": {"mesh": "Z2xi", "original_file_source_path_diagnostic": "nowhere/part.glb"}}]}]}`
	pb, err := unmarshalKinematicModel([]byte(doc), "")
	test.That(t, err, test.ShouldBeNil)
	mesh := pb.GetLinks()[0].GetCollision()[0].GetMesh()
	test.That(t, string(mesh.GetMesh()), test.ShouldEqual, "ply")
	test.That(t, mesh.GetContentType(), test.ShouldEqual, "ply")
	test.That(t, mesh.GetOriginalFileSourcePathDiagnostic(), test.ShouldEqual, "/opt/meshes/part.ply")
	visual := pb.GetLinks()[0].GetVisual()[0].GetMesh()
	test.That(t, string(visual.GetMesh()), test.ShouldEqual, "glb")
	test.That(t, visual.GetContentType(), test.ShouldEqual, "glb")
}

func TestKinematicModelFileRefuses(t *testing.T) {
	link := func(extra string) string {
		return `{"name": "m", "links": [{"id": "base", "parent": "world"` + extra + `}]}`
	}
	joint := func(extra string) string {
		return `{"name": "m", "links": [{"id": "base", "parent": "world"}],
			"joints": [{"id": "j", "parent": "base", "type": "JOINT_TYPE_REVOLUTE", "axis": {"z": 1}` + extra + `}]}`
	}
	cases := []struct {
		name, doc, want string
	}{
		{"unknown field", link(`, "colision": []`), `unknown field "colision"`},
		{"infinity", joint(`, "limits": {"max": "Infinity"}`), "limits.max is +Inf"},
		{"nan", joint(`, "limits": {"min": "NaN"}`), "limits.min is NaN"},
		{"empty", `{}`, "no links or joints"},
		{"no id", `{"links": [{"parent": "world"}]}`, "a link has no id"},
		{"world id", `{"links": [{"id": "world"}]}`, "reserved word"},
		{
			"duplicate id", `{"links": [{"id": "a"}], "joints": [{"id": "a", "type": "JOINT_TYPE_REVOLUTE"}]}`,
			`"a" names both a link and a joint`,
		},
		{"duplicate link", `{"links": [{"id": "a"}, {"id": "a"}]}`, `link "a" is defined twice`},
		{
			"mimic with limits", `{"links": [{"id": "base", "parent": "world"}], "joints": [
				{"id": "j", "parent": "base", "type": "JOINT_TYPE_REVOLUTE", "axis": {"z": 1}},
				{"id": "j2", "parent": "base", "type": "JOINT_TYPE_REVOLUTE", "axis": {"z": 1}, "mimic": {"joint": "j"}, "limits": {"max": 10}}]}`,
			ErrMimicWithLimits.Error(),
		},
		{"missing parent", link(`}, {"id": "tool", "parent": "bsae"`), `parent "bsae" is not a link or joint`},
		{"cycle", `{"links": [{"id": "a", "parent": "b"}, {"id": "b", "parent": "a"}]}`, "its own ancestor"},
		{"mimics itself", joint(`, "mimic": {"joint": "j"}`), `mimics itself`},
		{"mimics a link", joint(`, "mimic": {"joint": "base"}`), `mimics "base", which is not a joint`},
		{
			"missing mesh", link(`, "collision": [{"mesh": {"original_file_source_path_diagnostic": "../meshes/gone.ply"}}]`),
			`failed to read mesh "../meshes/gone.ply" at `,
		},
		{
			"absolute mesh path", link(`, "collision": [{"mesh": {"original_file_source_path_diagnostic": "/meshes/part.ply"}}]`),
			"is absolute",
		},
		{
			"uri mesh path", link(`, "collision": [{"mesh": {"original_file_source_path_diagnostic": "package://m/part.ply"}}]`),
			"is a URI",
		},
		{
			"backslash mesh path", link(`, "collision": [{"mesh": {"original_file_source_path_diagnostic": "..\\meshes\\part.ply"}}]`),
			"uses backslashes",
		},
		{"content type disagrees", link(`, "collision": [{"mesh": {"content_type": "stl",
			"original_file_source_path_diagnostic": "../meshes/part.ply"}}]`), `content type "stl" disagrees`},
		{
			"collision glb", link(`, "collision": [{"mesh": {"original_file_source_path_diagnostic": "../meshes/part.glb"}}]`),
			`collision mesh content type "glb" is not stl or ply`,
		},
		{
			"no extension", link(`, "visual": [{"mesh": {"original_file_source_path_diagnostic": "../meshes/part"}}]`),
			"has no extension",
		},
		{"inline bytes without type", link(`, "collision": [{"mesh": {"mesh": "cGx5"}}]`), "need a content type"},
		{"empty mesh", link(`, "collision": [{"mesh": {}}]`), "neither bytes nor a source path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			modelPath := writeModelDir(t, t.TempDir(), tc.doc)
			_, err := KinematicModelFromFile(modelPath, "")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.want)
			test.That(t, err.Error(), test.ShouldContainSubstring, modelPath)
			if tc.name == "missing mesh" {
				test.That(t, err.Error(), test.ShouldContainSubstring, filepath.Join("meshes", "gone.ply"))
			}
		})
	}
}

// A document is read as the message whenever it has a field only the message has, or nothing
// that tells the two apart, so a typo in a kinematic model file is reported by the strict parser
// instead of being read as an SVA document missing most of its fields.
func TestKinematicModelDocumentDetection(t *testing.T) {
	test.That(t, isKinematicModelDocument([]byte(meshModelDoc)), test.ShouldBeTrue)
	test.That(t, isKinematicModelDocument([]byte(`{"links": [{"id": "a", "parent": "world"}]}`)), test.ShouldBeTrue)
	test.That(t, isKinematicModelDocument([]byte(`{"links": [{"id": "a", "parent": "world", "colision": []}]}`)), test.ShouldBeTrue)
	test.That(t, isKinematicModelDocument([]byte(`{"joints": [{"id": "j", "type": "JOINT_TYPE_REVOLUTE"}]}`)), test.ShouldBeTrue)
	test.That(t, isKinematicModelDocument([]byte(`not json`)), test.ShouldBeTrue)

	test.That(t, isKinematicModelDocument([]byte(`{"kinematic_param_type": "SVA"}`)), test.ShouldBeFalse)
	test.That(t, isKinematicModelDocument([]byte(`{"dhParams": []}`)), test.ShouldBeFalse)
	test.That(t, isKinematicModelDocument([]byte(`{"links": [{"id": "a", "translation": {}}]}`)), test.ShouldBeFalse)
	test.That(t, isKinematicModelDocument([]byte(`{"links": [{"id": "a", "Translation": {}}]}`)), test.ShouldBeFalse)
	test.That(t, isKinematicModelDocument([]byte(`{"joints": [{"id": "j", "Type": "revolute"}]}`)), test.ShouldBeFalse)
	test.That(t, isKinematicModelDocument([]byte(`{"joints": [{"id": "j", "type": "revolute"}]}`)), test.ShouldBeFalse)
	test.That(t, isKinematicModelDocument([]byte(`{"joints": [{"id": "j", "min": -90, "max": 90}]}`)), test.ShouldBeFalse)
	for _, fixture := range protoRoundTripFixtures {
		if !strings.HasSuffix(fixture.path, ".json") {
			continue
		}
		data, err := os.ReadFile(utils.ResolveFile(fixture.path))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isKinematicModelDocument(data), test.ShouldBeFalse)
	}

	_, err := UnmarshalModelJSON([]byte(`{"links": [{"id": "a", "parent": "world", "colision": []}]}`), "")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `unknown field "colision"`)
}
