package referenceframe

import (
	"math"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/proto"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// Every kinematics file checked in, SVA, DH and URDF alike.
var protoRoundTripFixtures = []struct {
	name string
	path string
}{
	{"fake", "components/arm/kinematics/fake.json"},
	{"lite6", "components/arm/kinematics/lite6.json"},
	{"ur20", "components/arm/kinematics/ur20.json"},
	{"ur5e", "components/arm/kinematics/ur5e.json"},
	{"ur7e", "components/arm/kinematics/ur7e.json"},
	{"xarm6", "components/arm/kinematics/xarm6.json"},
	{"xarm7", "components/arm/kinematics/xarm7.json"},
	{"dofbot", "components/arm/kinematics/dofbot.json"},
	{"ur5e_dh", "referenceframe/testfiles/ur5eDH.json"},
	{"gantry", "referenceframe/testfiles/example_gantry.json"},
	{"gripper", "referenceframe/testfiles/test_gripper.json"},
	{"mimic_gripper", "referenceframe/testfiles/test_mimic_gripper.json"},
	{"mimic_serial", "referenceframe/testfiles/test_mimic_serial.json"},
	{"ur5e_urdf", "referenceframe/testfiles/ur5e.urdf"},
	{"capsule_urdf", "referenceframe/testfiles/capsule.urdf"},
	{"mimic_serial_urdf", "referenceframe/testfiles/test_mimic_serial.urdf"},
}

// A model that goes through the message comes back describing the same kinematics. We compare
// behaviour, limits, forward kinematics and geometry at random inputs, rather than bytes, so the
// test survives a field rename.
func TestModelProtoRoundTrip(t *testing.T) {
	for _, fixture := range protoRoundTripFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			original, err := KinematicModelFromFile(utils.ResolveFile(fixture.path), fixture.name)
			test.That(t, err, test.ShouldBeNil)

			pb, err := ModelToProto(original)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pb.GetName(), test.ShouldEqual, fixture.name)

			wire, err := proto.Marshal(pb)
			test.That(t, err, test.ShouldBeNil)
			received := &commonpb.KinematicModel{}
			test.That(t, proto.Unmarshal(wire, received), test.ShouldBeNil)

			restored, err := ModelFromProto(received, "")
			test.That(t, err, test.ShouldBeNil)
			test.That(t, restored.Name(), test.ShouldEqual, fixture.name)
			assertSameKinematics(t, original, restored)
		})
	}
}

// assertSameKinematics compares two models by behaviour at random inputs.
func assertSameKinematics(t *testing.T, want, got Model) {
	t.Helper()
	test.That(t, limitsAlmostEqual(want.DoF(), got.DoF(), 1e-9), test.ShouldBeTrue)

	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 25; i++ {
		inputs := RandomFrameInputs(want, rng)

		wantPose, err := want.Transform(inputs)
		test.That(t, err, test.ShouldBeNil)
		gotPose, err := got.Transform(inputs)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqualEps(wantPose, gotPose, 1e-6), test.ShouldBeTrue)

		wantGeoms, err := want.Geometries(inputs)
		test.That(t, err, test.ShouldBeNil)
		gotGeoms, err := got.Geometries(inputs)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(gotGeoms.Geometries()), test.ShouldEqual, len(wantGeoms.Geometries()))
		for j, wantGeom := range wantGeoms.Geometries() {
			gotGeom := gotGeoms.Geometries()[j]
			// labels carry the model name as a prefix, and a DH file leaves a geometry unlabelled
			// where the SVA reader would have named it after its link, so only a label the
			// original actually has is compared
			wantLabel := strings.TrimPrefix(wantGeom.Label(), want.Name()+":")
			if wantLabel != "" {
				test.That(t, strings.TrimPrefix(gotGeom.Label(), got.Name()+":"), test.ShouldEqual, wantLabel)
			}
			test.That(t, spatial.PoseAlmostEqualEps(wantGeom.Pose(), gotGeom.Pose(), 1e-6), test.ShouldBeTrue)
		}
	}
}

// The response carries both encodings. A new client reads the typed model, an old client reads the
// bytes it always read, and either alone is enough.
func TestKinematicsResponseCarriesBothEncodings(t *testing.T) {
	original, err := ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)

	resp := KinematicModelToProtobuf(original)
	test.That(t, resp.GetModel(), test.ShouldNotBeNil)
	//nolint:staticcheck // deprecated fields are still served
	test.That(t, resp.GetFormat(), test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_SVA)
	test.That(t, len(resp.GetKinematicsData()), test.ShouldBeGreaterThan, 0) //nolint:staticcheck // deprecated fields are still served

	both, err := KinematicModelFromProtobuf("ur5e", resp)
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, original, both)

	typedOnly, ok := proto.Clone(resp).(*commonpb.GetKinematicsResponse)
	test.That(t, ok, test.ShouldBeTrue)
	typedOnly.KinematicsData = nil //nolint:staticcheck // deprecated fields are still served
	//nolint:staticcheck // deprecated fields are still served
	typedOnly.Format = commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_UNSPECIFIED
	fromTyped, err := KinematicModelFromProtobuf("ur5e", typedOnly)
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, original, fromTyped)

	bytesOnly, ok := proto.Clone(resp).(*commonpb.GetKinematicsResponse)
	test.That(t, ok, test.ShouldBeTrue)
	bytesOnly.Model = nil
	fromBytes, err := KinematicModelFromProtobuf("ur5e", bytesOnly)
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, original, fromBytes)
}

// A model built from a configuration in code never had a file, and it still serves both encodings.
func TestKinematicsResponseFromConfigInCode(t *testing.T) {
	cfg := &ModelConfigJSON{
		Name: "in_code",
		Links: []LinkConfig{
			{ID: "base", Parent: World},
			{ID: "tip", Parent: "j", Translation: r3.Vector{Z: 100}, Geometry: &spatial.GeometryConfig{
				Type: spatial.BoxType, X: 10, Y: 20, Z: 30,
			}},
		},
		Joints: []JointConfig{
			{ID: "j", Type: RevoluteJoint, Parent: "base", Axis: spatial.AxisConfig{Z: 1}, Min: -90, Max: 90},
		},
	}
	original, err := cfg.ParseConfig("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, original.ModelConfig().OriginalFile, test.ShouldBeNil)

	resp := KinematicModelToProtobuf(original)
	test.That(t, resp.GetModel(), test.ShouldNotBeNil)
	//nolint:staticcheck // deprecated fields are still served
	test.That(t, resp.GetFormat(), test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_SVA)

	fromBytes, err := UnmarshalModelJSON(resp.GetKinematicsData(), "") //nolint:staticcheck // deprecated fields are still served
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fromBytes.Name(), test.ShouldEqual, "in_code")
	assertSameKinematics(t, original, fromBytes)
}

// A model built from frames alone has nothing to convert, so the response is what it always was.
func TestKinematicsResponseWithoutConfig(t *testing.T) {
	limit := Limit{Min: -1, Max: 1}
	model, err := New2DMobileModelFrame("base", []Limit{limit, limit}, nil)
	test.That(t, err, test.ShouldBeNil)

	resp := KinematicModelToProtobuf(model)
	test.That(t, resp.GetModel(), test.ShouldBeNil)
	//nolint:staticcheck // deprecated fields are still served
	test.That(t, resp.GetFormat(), test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_UNSPECIFIED)
	test.That(t, len(resp.GetKinematicsData()), test.ShouldEqual, 0) //nolint:staticcheck // deprecated fields are still served
}

// The message carries the limits the model enforces, not the ones its file declared. The bytes
// an old client reads still replay the file, as they always have.
func TestModelToProtoCarriesLiveLimits(t *testing.T) {
	model, err := ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	sm, ok := model.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	narrowed, err := NewModelWithLimitOverrides(sm, map[string]Limit{
		"waist": {Min: utils.DegToRad(-10), Max: utils.DegToRad(10)},
	})
	test.That(t, err, test.ShouldBeNil)

	resp := KinematicModelToProtobuf(narrowed)
	waist := jointByID(t, resp.GetModel(), "waist")
	test.That(t, waist.GetLimits().GetMin(), test.ShouldAlmostEqual, -10)
	test.That(t, waist.GetLimits().GetMax(), test.ShouldAlmostEqual, 10)
	test.That(t, jointByID(t, resp.GetModel(), "shoulder").GetLimits().GetMin(), test.ShouldAlmostEqual, -118)
	//nolint:staticcheck // deprecated fields are still served
	test.That(t, resp.GetKinematicsData(), test.ShouldResemble, sm.ModelConfig().OriginalFile.Bytes)

	fromModel, err := KinematicModelFromProtobuf("", resp)
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, narrowed, fromModel)
}

// A revolute joint with no position bound is what the message calls continuous, and it comes back
// as the unbounded revolute joint the URDF reader has always built.
func TestContinuousJointRoundTrip(t *testing.T) {
	speed := 180.0
	cfg := &ModelConfigJSON{
		Name:  "spinner",
		Links: []LinkConfig{{ID: "base", Parent: World}, {ID: "tip", Parent: "j"}},
		Joints: []JointConfig{{
			ID: "j", Type: RevoluteJoint, Parent: "base", Axis: spatial.AxisConfig{Z: 1},
			Min: math.Inf(-1), Max: math.Inf(1), MaxVelocity: &speed,
		}},
	}
	original, err := cfg.ParseConfig("")
	test.That(t, err, test.ShouldBeNil)

	pb, err := ModelToProto(original)
	test.That(t, err, test.ShouldBeNil)
	joint := jointByID(t, pb, "j")
	test.That(t, joint.GetType(), test.ShouldEqual, commonpb.JointType_JOINT_TYPE_CONTINUOUS)
	test.That(t, joint.GetLimits().Min, test.ShouldBeNil)
	test.That(t, joint.GetLimits().Max, test.ShouldBeNil)
	test.That(t, joint.GetLimits().GetMaxVelocity(), test.ShouldEqual, speed)

	restored, err := ModelFromProto(pb, "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, math.IsInf(restored.DoF()[0].Min, -1), test.ShouldBeTrue)
	test.That(t, math.IsInf(restored.DoF()[0].Max, 1), test.ShouldBeTrue)
	assertSameKinematics(t, original, restored)

	// an unbounded joint cannot be written as JSON, and this model never had a file to replay
	resp := KinematicModelToProtobuf(original)
	test.That(t, resp.GetModel(), test.ShouldNotBeNil)
	//nolint:staticcheck // deprecated fields are still served
	test.That(t, resp.GetFormat(), test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_UNSPECIFIED)
}

// A mesh keeps its bytes, its content type and the path it was read from.
func TestMeshRidesTheMessage(t *testing.T) {
	ply, err := os.ReadFile(utils.ResolveFile("referenceframe/testfiles/test_simple.ply"))
	test.That(t, err, test.ShouldBeNil)
	cfg := &ModelConfigJSON{
		Name: "meshy",
		Links: []LinkConfig{
			{ID: "base", Parent: World},
			{ID: "tip", Parent: "j", Geometry: &spatial.GeometryConfig{
				Type: spatial.MeshType, MeshData: ply, MeshContentType: "ply", MeshFilePath: "meshes/tip.ply",
			}},
		},
		Joints: []JointConfig{
			{ID: "j", Type: RevoluteJoint, Parent: "base", Axis: spatial.AxisConfig{Z: 1}, Min: -90, Max: 90},
		},
	}
	original, err := cfg.ParseConfig("")
	test.That(t, err, test.ShouldBeNil)

	pb, err := ModelToProto(original)
	test.That(t, err, test.ShouldBeNil)
	mesh := linkByID(t, pb, "tip").GetCollision()[0].GetMesh()
	test.That(t, mesh.GetContentType(), test.ShouldEqual, "ply")
	test.That(t, mesh.GetMesh(), test.ShouldResemble, ply)
	test.That(t, mesh.GetOriginalFileSourcePathDiagnostic(), test.ShouldEqual, "meshes/tip.ply")

	restored, err := ModelConfigFromProto(pb)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, restored.Links[1].Geometry.MeshFilePath, test.ShouldEqual, "meshes/tip.ply")
	test.That(t, restored.Links[1].Geometry.MeshData, test.ShouldResemble, ply)
}

// Visual geometry and properties have no place in the SVA schema. They ride the message both ways,
// survive a limit override, and are left out of the bytes an old client reads.
func TestSimpleModelVisualAndProperties(t *testing.T) {
	model, err := ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	sm, ok := model.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	glb := &commonpb.Geometry{GeometryType: &commonpb.Geometry_Mesh{Mesh: &commonpb.Mesh{
		ContentType: "glb", Mesh: []byte("not really a glb"), OriginalFileSourcePathDiagnostic: proto.String("visual/base.glb"),
	}}}
	test.That(t, sm.SetVisualGeometries("base", []*commonpb.Geometry{glb}), test.ShouldBeNil)
	test.That(t, sm.SetVisualGeometries("no_such_link", []*commonpb.Geometry{glb}), test.ShouldNotBeNil)
	hz := 100.0
	sm.SetKinematicProperties(&commonpb.KinematicProperties{OperatingFreqHz: &hz})

	// the model hands out copies, so a caller cannot reach in and change what it holds
	sm.VisualGeometries("base")[0].Label = "changed"
	test.That(t, sm.VisualGeometries("base")[0].GetLabel(), test.ShouldEqual, "")

	pb, err := ModelToProto(sm)
	test.That(t, err, test.ShouldBeNil)
	visual := linkByID(t, pb, "base").GetVisual()
	test.That(t, len(visual), test.ShouldEqual, 1)
	test.That(t, visual[0].GetMesh().GetOriginalFileSourcePathDiagnostic(), test.ShouldEqual, "visual/base.glb")
	test.That(t, len(linkByID(t, pb, "upper_arm").GetVisual()), test.ShouldEqual, 0)
	test.That(t, pb.GetProperties().GetOperatingFreqHz(), test.ShouldEqual, hz)

	restored, err := ModelFromProto(pb, "")
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, sm, restored)
	rsm, ok := restored.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, rsm.VisualGeometries("base")[0].GetMesh().GetMesh(), test.ShouldResemble, []byte("not really a glb"))
	test.That(t, rsm.KinematicProperties().GetOperatingFreqHz(), test.ShouldEqual, hz)

	narrowed, err := NewModelWithLimitOverrides(sm, map[string]Limit{"waist": {Min: -1, Max: 1}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(narrowed.VisualGeometries("base")), test.ShouldEqual, 1)
	test.That(t, narrowed.KinematicProperties().GetOperatingFreqHz(), test.ShouldEqual, hz)

	resp := KinematicModelToProtobuf(sm)
	//nolint:staticcheck // deprecated fields are still served
	test.That(t, resp.GetFormat(), test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_SVA)
	//nolint:staticcheck // deprecated fields are still served
	test.That(t, string(resp.GetKinematicsData()), test.ShouldNotContainSubstring, "base.glb")

	_, err = ModelConfigFromProto(pb)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "visual")
}

// The SVA schema holds one collision geometry per link, so more than one is an error on the way
// back rather than a silent drop.
func TestModelConfigFromProtoRefusesSeveralCollisionGeometries(t *testing.T) {
	sphere := func(label string) *commonpb.Geometry {
		return &commonpb.Geometry{Label: label, GeometryType: &commonpb.Geometry_Sphere{Sphere: &commonpb.Sphere{RadiusMm: 1}}}
	}
	_, err := ModelConfigFromProto(&commonpb.KinematicModel{Links: []*commonpb.KinematicLink{
		{Id: "base", Collision: []*commonpb.Geometry{sphere("a"), sphere("b")}},
	}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "one per link")
}

func jointByID(t *testing.T, pb *commonpb.KinematicModel, id string) *commonpb.KinematicJoint {
	t.Helper()
	for _, joint := range pb.GetJoints() {
		if joint.GetId() == id {
			return joint
		}
	}
	t.Fatalf("joint %q missing", id)
	return nil
}

func linkByID(t *testing.T, pb *commonpb.KinematicModel, id string) *commonpb.KinematicLink {
	t.Helper()
	for _, link := range pb.GetLinks() {
		if link.GetId() == id {
			return link
		}
	}
	t.Fatalf("link %q missing", id)
	return nil
}
