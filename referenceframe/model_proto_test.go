package referenceframe

import (
	"math/rand"
	"os"
	"strings"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// Every SVA file we ship, plus the URDF and mimic fixtures. dofbot is left out because it is a
// DH model and the message carries links and joints only.
var protoRoundTripFixtures = []struct {
	name string
	load func() (Model, error)
}{
	{"fake", svaFixture("components/arm/kinematics/fake.json")},
	{"lite6", svaFixture("components/arm/kinematics/lite6.json")},
	{"ur20", svaFixture("components/arm/kinematics/ur20.json")},
	{"ur5e", svaFixture("components/arm/kinematics/ur5e.json")},
	{"ur7e", svaFixture("components/arm/kinematics/ur7e.json")},
	{"xarm6", svaFixture("components/arm/kinematics/xarm6.json")},
	{"xarm7", svaFixture("components/arm/kinematics/xarm7.json")},
	{"gantry", svaFixture("referenceframe/testfiles/example_gantry.json")},
	{"gripper", svaFixture("referenceframe/testfiles/test_gripper.json")},
	{"mimic_serial", svaFixture("referenceframe/testfiles/test_mimic_serial.json")},
	{"ur5e_urdf", func() (Model, error) {
		return ParseModelXMLFile(utils.ResolveFile("referenceframe/testfiles/ur5e.urdf"), "ur5e_urdf", nil)
	}},
	{"capsule_urdf", func() (Model, error) {
		return ParseModelXMLFile(utils.ResolveFile("referenceframe/testfiles/capsule.urdf"), "capsule_urdf", nil)
	}},
}

func svaFixture(rel string) func() (Model, error) {
	return func() (Model, error) { return ParseModelJSONFile(utils.ResolveFile(rel), "") }
}

// The property the prototype exists to check: a model that goes through the typed message comes
// back describing the same kinematics. We compare behaviour, forward kinematics and geometry at
// random inputs, rather than bytes, so the test survives field renames.
func TestModelProtoRoundTrip(t *testing.T) {
	for _, fixture := range protoRoundTripFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			original, err := fixture.load()
			test.That(t, err, test.ShouldBeNil)

			pb, err := ModelToProto(original)
			test.That(t, err, test.ShouldBeNil)

			// the wire form has to survive proto JSON, which is also the v2 file format
			jsonBytes, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(pb)
			test.That(t, err, test.ShouldBeNil)
			fromJSON := &commonpb.KinematicModel{}
			test.That(t, protojson.Unmarshal(jsonBytes, fromJSON), test.ShouldBeNil)
			test.That(t, proto.Equal(pb, fromJSON), test.ShouldBeTrue)

			restored, err := ModelFromProto(fromJSON, original.Name())
			test.That(t, err, test.ShouldBeNil)
			assertSameKinematics(t, original, restored)
		})
	}
}

// assertSameKinematics compares two models by behaviour, limits, forward kinematics and geometry
// at random inputs, so a test survives renaming a field.
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
			// labels carry the model name as a prefix, and the two sides may have been given
			// different names, so compare the part that describes the geometry
			test.That(t, strings.TrimPrefix(gotGeom.Label(), got.Name()+":"), test.ShouldEqual,
				strings.TrimPrefix(wantGeom.Label(), want.Name()+":"))
			test.That(t, spatial.PoseAlmostEqualEps(wantGeom.Pose(), gotGeom.Pose(), 1e-6), test.ShouldBeTrue)
		}
	}
}

// The response carries both encodings during the deprecation window. A new client reads the typed
// model, an old client reads the bytes it always read, and either alone is enough.
func TestKinematicsResponseCarriesTypedModel(t *testing.T) {
	original, err := ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)

	resp := KinematicModelToProtobuf(original)
	test.That(t, resp.GetModel(), test.ShouldNotBeNil)
	test.That(t, resp.GetFormat(), test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_SVA)
	test.That(t, len(resp.GetKinematicsData()), test.ShouldBeGreaterThan, 0)

	both, err := KinematicModelFromProtobuf("ur5e", resp)
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, original, both)

	typedOnly, ok := proto.Clone(resp).(*commonpb.GetKinematicsResponse)
	test.That(t, ok, test.ShouldBeTrue)
	typedOnly.KinematicsData = nil
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

// The exclude flags blank the bytes of one role and leave source_path in place. The deprecated bytes
// fields are untouched, since a client old enough to read them cannot have set a flag.
func TestKinematicsRequestFlagsStripMeshBytes(t *testing.T) {
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
	model, err := cfg.ParseConfig("meshy")
	test.That(t, err, test.ShouldBeNil)

	tipMesh := func(resp *commonpb.GetKinematicsResponse) *commonpb.Mesh {
		for _, link := range resp.GetModel().GetLinks() {
			if link.GetId() == "tip" {
				return link.GetCollision()[0].GetMesh()
			}
		}
		t.Fatal("tip link missing")
		return nil
	}

	full := KinematicModelToProtobufForRequest(model, &commonpb.GetKinematicsRequest{})
	test.That(t, len(tipMesh(full).GetMesh()), test.ShouldBeGreaterThan, 0)

	stripped := KinematicModelToProtobufForRequest(model, &commonpb.GetKinematicsRequest{ExcludeCollisionMeshes: true})
	test.That(t, len(tipMesh(stripped).GetMesh()), test.ShouldEqual, 0)
	test.That(t, tipMesh(stripped).GetSourcePath(), test.ShouldEqual, "meshes/tip.ply")
	test.That(t, tipMesh(stripped).GetContentType(), test.ShouldEqual, "ply")
	// a client that set a flag knows about the typed model, so it is spared the legacy bytes, which
	// would otherwise carry the excluded mesh right back inline
	test.That(t, len(stripped.GetKinematicsData()), test.ShouldEqual, 0)
	test.That(t, len(full.GetKinematicsData()), test.ShouldBeGreaterThan, 0)

	// visual is never populated from a v1 config, so exercise that role on the message directly
	pb := &commonpb.KinematicModel{Links: []*commonpb.KinematicLink{{
		Id:        "l",
		Collision: []*commonpb.Geometry{{GeometryType: &commonpb.Geometry_Mesh{Mesh: &commonpb.Mesh{Mesh: []byte("c"), SourcePath: "c.stl"}}}},
		Visual:    []*commonpb.Geometry{{GeometryType: &commonpb.Geometry_Mesh{Mesh: &commonpb.Mesh{Mesh: []byte("v"), SourcePath: "v.glb"}}}},
	}}}
	stripMeshBytes(pb, false, true)
	test.That(t, pb.Links[0].Collision[0].GetMesh().GetMesh(), test.ShouldResemble, []byte("c"))
	test.That(t, len(pb.Links[0].Visual[0].GetMesh().GetMesh()), test.ShouldEqual, 0)
	test.That(t, pb.Links[0].Visual[0].GetMesh().GetSourcePath(), test.ShouldEqual, "v.glb")
}

// FrameSystemConfig carries the typed model beside the deprecated Struct, and a reader prefers the
// model but still understands a Struct from an older remote.
func TestFrameSystemPartCarriesTypedModel(t *testing.T) {
	arm, err := ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	part := &FrameSystemPart{
		FrameConfig: NewLinkInFrame(World, spatial.NewZeroPose(), "arm", nil),
		ModelFrame:  arm,
	}

	fsc, err := part.ToProtobuf()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fsc.GetModel(), test.ShouldNotBeNil)
	test.That(t, len(fsc.GetKinematics().AsMap()), test.ShouldBeGreaterThan, 0)

	fromTyped, err := ProtobufToFrameSystemPart(fsc)
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, arm, fromTyped.ModelFrame)

	fsc.Model = nil
	fromStruct, err := ProtobufToFrameSystemPart(fsc)
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, arm, fromStruct.ModelFrame)
}

// A typo in a v2 file must fail at parse. This is most of what proto JSON buys over the v1 parser,
// where an unknown key was silently ignored.
func TestModelProtoRejectsUnknownFields(t *testing.T) {
	original, err := ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	pb, err := ModelToProto(original)
	test.That(t, err, test.ShouldBeNil)
	jsonBytes, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(pb)
	test.That(t, err, test.ShouldBeNil)

	typo := strings.Replace(string(jsonBytes), `"max_velocity"`, `"max_velocty"`, 1)
	if typo == string(jsonBytes) {
		typo = strings.Replace(string(jsonBytes), `"min"`, `"mn"`, 1)
	}
	test.That(t, typo, test.ShouldNotEqual, string(jsonBytes))
	err = protojson.Unmarshal([]byte(typo), &commonpb.KinematicModel{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unknown field")
}

// Prints the ur5e shoulder as v2 so a reviewer can look at the file format without running
// anything. The assertions are about shape, the log is the point.
func TestModelProtoV2Sample(t *testing.T) {
	original, err := ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	pb, err := ModelToProto(original)
	test.That(t, err, test.ShouldBeNil)

	sample := &commonpb.KinematicModel{Name: pb.Name, Links: pb.Links[:2], Joints: pb.Joints[:1]}
	jsonBytes, err := protojson.MarshalOptions{UseProtoNames: true, Multiline: true, Indent: "  "}.Marshal(sample)
	test.That(t, err, test.ShouldBeNil)
	t.Logf("ur5e as SVA v2, first two links and one joint:\n%s", jsonBytes)

	test.That(t, string(jsonBytes), test.ShouldContainSubstring, `"hardware_limits"`)
	test.That(t, string(jsonBytes), test.ShouldContainSubstring, `"JOINT_TYPE_REVOLUTE"`)
	test.That(t, string(jsonBytes), test.ShouldNotContainSubstring, `"kinematic_param_type"`)
}

// The v1 config is narrower than the message. Anything the message can say that v1 cannot must
// be an error on the way back, never a silent drop.
func TestModelConfigFromProtoRefusesWhatV1CannotCarry(t *testing.T) {
	joint := func() *commonpb.KinematicJoint {
		return &commonpb.KinematicJoint{
			Id: "j", Parent: "base", Type: commonpb.JointType_JOINT_TYPE_REVOLUTE,
			Axis:           &commonpb.Vector3{Z: 1},
			HardwareLimits: &commonpb.JointLimits{Min: proto.Float64(-90), Max: proto.Float64(90)},
		}
	}
	base := func() *commonpb.KinematicLink { return &commonpb.KinematicLink{Id: "base"} }
	sphere := func(label string) *commonpb.Geometry {
		return &commonpb.Geometry{Label: label, GeometryType: &commonpb.Geometry_Sphere{Sphere: &commonpb.Sphere{RadiusMm: 1}}}
	}

	cases := []struct {
		name   string
		pb     *commonpb.KinematicModel
		errSub string
	}{
		{"visual geometry", &commonpb.KinematicModel{Links: []*commonpb.KinematicLink{
			{Id: "base", Visual: []*commonpb.Geometry{sphere("v")}},
		}}, "visual"},
		{"two collision geometries", &commonpb.KinematicModel{Links: []*commonpb.KinematicLink{
			{Id: "base", Collision: []*commonpb.Geometry{sphere("a"), sphere("b")}},
		}}, "one per link"},
		{"user limits", func() *commonpb.KinematicModel {
			j := joint()
			j.UserLimits = &commonpb.JointLimits{Max: proto.Float64(45)}
			return &commonpb.KinematicModel{Links: []*commonpb.KinematicLink{base()}, Joints: []*commonpb.KinematicJoint{j}}
		}(), "user limits"},
		{"unbounded position", func() *commonpb.KinematicModel {
			j := joint()
			j.HardwareLimits = &commonpb.JointLimits{MaxVelocity: proto.Float64(10)}
			return &commonpb.KinematicModel{Links: []*commonpb.KinematicLink{base()}, Joints: []*commonpb.KinematicJoint{j}}
		}(), "unbounded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ModelConfigFromProto(tc.pb)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.errSub)
		})
	}
}

// User limits narrow the effective DoF, refuse to widen, and ride the message both ways alongside
// visual geometry, properties and the generation counter.
func TestSimpleModelTypedOverlay(t *testing.T) {
	model, err := ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	sm, ok := model.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	hardware := sm.DoF()

	// waist is revolute with hardware -359..359 degrees and no velocity limit in the file
	speed, accel := 90.0, 500.0
	lock := 10.0
	err = sm.SetUserLimits(map[string]JointLimits{
		"waist":    {MaxVelocity: &speed, MaxAcceleration: &accel},
		"shoulder": {Min: &lock, Max: &lock},
	})
	test.That(t, err, test.ShouldBeNil)

	effective := sm.DoF()
	test.That(t, effective[0].Min, test.ShouldAlmostEqual, hardware[0].Min)
	test.That(t, *effective[0].MaxVelocity, test.ShouldAlmostEqual, utils.DegToRad(speed))
	test.That(t, *effective[0].MaxAcceleration, test.ShouldAlmostEqual, utils.DegToRad(accel))
	test.That(t, effective[1].Min, test.ShouldAlmostEqual, utils.DegToRad(lock))
	test.That(t, effective[1].Max, test.ShouldAlmostEqual, utils.DegToRad(lock))
	test.That(t, effective[2], test.ShouldResemble, hardware[2])

	// a second call replaces rather than narrows further
	test.That(t, sm.SetUserLimits(map[string]JointLimits{"waist": {MaxVelocity: &speed}}), test.ShouldBeNil)
	test.That(t, sm.DoF()[1], test.ShouldResemble, hardware[1])

	// widening the hardware range is an error, as is an unknown joint
	wide := 400.0
	err = sm.SetUserLimits(map[string]JointLimits{"waist": {Max: &wide}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "above the hardware max")
	err = sm.SetUserLimits(map[string]JointLimits{"nope": {}})
	test.That(t, err, test.ShouldNotBeNil)

	// the rest of the overlay
	glb := &commonpb.Geometry{GeometryType: &commonpb.Geometry_Mesh{Mesh: &commonpb.Mesh{
		ContentType: "glb", Mesh: []byte("not really a glb"), SourcePath: "3d_models/xArm6/base.glb",
	}}}
	test.That(t, sm.SetVisualGeometries("base", []*commonpb.Geometry{glb}), test.ShouldBeNil)
	test.That(t, sm.SetVisualGeometries("no_such_link", []*commonpb.Geometry{glb}), test.ShouldNotBeNil)
	hz := 100.0
	sm.SetKinematicProperties(&commonpb.KinematicProperties{TrajectorySamplingFreqHz: &hz})
	sm.SetGeneration(7)

	// everything survives the message and comes back onto a fresh model
	pb, err := ModelToProto(sm)
	test.That(t, err, test.ShouldBeNil)
	var waist *commonpb.KinematicJoint
	for _, j := range pb.GetJoints() {
		if j.GetId() == "waist" {
			waist = j
		}
	}
	test.That(t, waist.GetUserLimits().GetMaxVelocity(), test.ShouldEqual, speed)
	test.That(t, waist.GetUserLimits().Min, test.ShouldBeNil)
	test.That(t, waist.GetHardwareLimits().GetMin(), test.ShouldEqual, -359)
	test.That(t, pb.GetLinks()[0].GetVisual()[0].GetMesh().GetSourcePath(), test.ShouldEqual, "3d_models/xArm6/base.glb")
	test.That(t, pb.GetProperties().GetTrajectorySamplingFreqHz(), test.ShouldEqual, hz)
	test.That(t, pb.GetGeneration(), test.ShouldEqual, 7)

	restored, err := ModelFromProto(pb, "")
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, sm, restored)
	rsm, ok := restored.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, *rsm.UserLimits()["waist"].MaxVelocity, test.ShouldEqual, speed)
	test.That(t, rsm.Generation(), test.ShouldEqual, 7)
	test.That(t, rsm.KinematicProperties().GetTrajectorySamplingFreqHz(), test.ShouldEqual, hz)

	// and the visual role can be left out of a response while collision stays
	resp := KinematicModelToProtobufForRequest(sm, &commonpb.GetKinematicsRequest{ExcludeVisualMeshes: true})
	test.That(t, len(resp.GetModel().GetLinks()[0].GetVisual()[0].GetMesh().GetMesh()), test.ShouldEqual, 0)
	test.That(t, resp.GetModel().GetLinks()[0].GetVisual()[0].GetMesh().GetSourcePath(), test.ShouldEqual, "3d_models/xArm6/base.glb")

	// the v1 config path still refuses what it cannot carry
	_, err = ModelConfigFromProto(pb)
	test.That(t, err, test.ShouldNotBeNil)
}
