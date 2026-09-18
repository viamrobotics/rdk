package referenceframe

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/encoding/protojson"

	"go.viam.com/rdk/utils"
)

func writeV2(t *testing.T, dir, name string, pb *commonpb.KinematicModel) string {
	t.Helper()
	data, err := protojson.MarshalOptions{UseProtoNames: true, Multiline: true}.Marshal(pb)
	test.That(t, err, test.ShouldBeNil)
	path := filepath.Join(dir, name)
	test.That(t, os.MkdirAll(filepath.Dir(path), 0o755), test.ShouldBeNil)
	test.That(t, os.WriteFile(path, data, 0o600), test.ShouldBeNil)
	return path
}

// A shipped v1 file converted to v2 loads back to the same kinematics, through the v2 entry
// point and through the extension based dispatch that has to tell the two apart.
func TestSVAv2FileRoundTrip(t *testing.T) {
	original, err := ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	pb, err := ModelToProto(original)
	test.That(t, err, test.ShouldBeNil)
	path := writeV2(t, t.TempDir(), "ur5e_v2.json", pb)

	fromV2, err := ParseModelV2File(path, "")
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, original, fromV2)

	dispatched, err := KinematicModelFromFile(path, "")
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, original, dispatched)

	// and the v1 file still goes down the v1 path
	v1, err := KinematicModelFromFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, original, v1)
}

func meshModel(collisionPath, visualPath string) *commonpb.KinematicModel {
	geom := func(path string) *commonpb.Geometry {
		return &commonpb.Geometry{GeometryType: &commonpb.Geometry_Mesh{Mesh: &commonpb.Mesh{SourcePath: path}}}
	}
	tip := &commonpb.KinematicLink{Id: "tip", Parent: "j"}
	if collisionPath != "" {
		tip.Collision = []*commonpb.Geometry{geom(collisionPath)}
	}
	if visualPath != "" {
		tip.Visual = []*commonpb.Geometry{geom(visualPath)}
	}
	minL, maxL := -90.0, 90.0
	return &commonpb.KinematicModel{
		Name:  "meshy",
		Links: []*commonpb.KinematicLink{{Id: "base", Parent: World}, tip},
		Joints: []*commonpb.KinematicJoint{{
			Id: "j", Parent: "base", Type: commonpb.JointType_JOINT_TYPE_REVOLUTE,
			Axis: &commonpb.Vector3{Z: 1}, HardwareLimits: &commonpb.JointLimits{Min: &minL, Max: &maxL},
		}},
	}
}

// Meshes referenced by path load relative to the file, .. included, collision meshes are parsed
// and visual meshes are carried as bytes, and the content type comes from the extension.
func TestSVAv2MeshesByPath(t *testing.T) {
	dir := t.TempDir()
	ply, err := os.ReadFile(utils.ResolveFile("referenceframe/testfiles/test_simple.ply"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, os.MkdirAll(filepath.Join(dir, "meshes"), 0o755), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dir, "meshes", "tip.ply"), ply, 0o600), test.ShouldBeNil)
	test.That(t, os.MkdirAll(filepath.Join(dir, "kinematics", "vis"), 0o755), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dir, "kinematics", "vis", "tip.glb"), []byte("pretend glb"), 0o600), test.ShouldBeNil)

	path := writeV2(t, dir, "kinematics/model.json", meshModel("../meshes/tip.ply", "vis/tip.glb"))

	data, err := os.ReadFile(path)
	test.That(t, err, test.ShouldBeNil)
	pb, err := LoadKinematicModelV2(data, filepath.Dir(path))
	test.That(t, err, test.ShouldBeNil)
	tip := pb.GetLinks()[1]
	test.That(t, tip.GetCollision()[0].GetMesh().GetMesh(), test.ShouldResemble, ply)
	test.That(t, tip.GetCollision()[0].GetMesh().GetContentType(), test.ShouldEqual, "ply")
	test.That(t, tip.GetVisual()[0].GetMesh().GetMesh(), test.ShouldResemble, []byte("pretend glb"))
	test.That(t, tip.GetVisual()[0].GetMesh().GetContentType(), test.ShouldEqual, "glb")

	model, err := ParseModelV2File(path, "")
	test.That(t, err, test.ShouldBeNil)
	geoms, err := model.Geometries(make([]Input, 1))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(geoms.Geometries()), test.ShouldEqual, 1)

	// the same bytes with nowhere to resolve paths from is an error, not a silent empty mesh
	_, err = UnmarshalModelV2(data, "", "")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "did not come from a file")
}

func TestSVAv2Rejects(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name   string
		pb     *commonpb.KinematicModel
		errSub string
	}{
		{"absolute path", meshModel("/etc/passwd", ""), "not absolute"},
		{"uri", meshModel("file:///tmp/x.ply", ""), "not a URI"},
		{"missing file", meshModel("meshes/none.ply", ""), "could not be read"},
		{"collision glb", meshModel("", ""), "must be stl or ply"},
		{"infinite limit", meshModel("", ""), "must be finite"},
	}
	// the collision glb case gets a real file so only the type check can fail
	test.That(t, os.WriteFile(filepath.Join(dir, "thing.glb"), []byte("glb"), 0o600), test.ShouldBeNil)
	cases[3].pb.Links[1].Collision = []*commonpb.Geometry{{GeometryType: &commonpb.Geometry_Mesh{Mesh: &commonpb.Mesh{SourcePath: "thing.glb"}}}}
	inf := math.Inf(1)
	cases[4].pb.Joints[0].HardwareLimits.Max = &inf

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeV2(t, dir, tc.name+".json", tc.pb)
			_, err := ParseModelV2File(path, "")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.errSub)
		})
	}

	t.Run("missing file names both paths", func(t *testing.T) {
		path := writeV2(t, dir, "missing.json", meshModel("meshes/none.ply", ""))
		_, err := ParseModelV2File(path, "")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "meshes/none.ply")
		test.That(t, err.Error(), test.ShouldContainSubstring, filepath.Join(dir, "meshes", "none.ply"))
	})

	t.Run("unknown field", func(t *testing.T) {
		path := writeV2(t, dir, "typo.json", meshModel("", ""))
		data, err := os.ReadFile(path)
		test.That(t, err, test.ShouldBeNil)
		typo := strings.Replace(string(data), `"max"`, `"mx"`, 1)
		test.That(t, os.WriteFile(path, []byte(typo), 0o600), test.ShouldBeNil)
		_, err = ParseModelV2File(path, "")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown field")
		// and a file that is not v2 at all falls through to the v1 parser via the dispatcher
		_, err = KinematicModelFromFile(path, "")
		test.That(t, err, test.ShouldNotBeNil)
	})
}
