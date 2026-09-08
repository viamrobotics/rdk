package referenceframe

import (
	"math/rand"
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

			test.That(t, limitsAlmostEqual(original.DoF(), restored.DoF(), 1e-9), test.ShouldBeTrue)

			rng := rand.New(rand.NewSource(1))
			for i := 0; i < 25; i++ {
				inputs := RandomFrameInputs(original, rng)

				wantPose, err := original.Transform(inputs)
				test.That(t, err, test.ShouldBeNil)
				gotPose, err := restored.Transform(inputs)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, spatial.PoseAlmostEqualEps(wantPose, gotPose, 1e-6), test.ShouldBeTrue)

				wantGeoms, err := original.Geometries(inputs)
				test.That(t, err, test.ShouldBeNil)
				gotGeoms, err := restored.Geometries(inputs)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, len(gotGeoms.Geometries()), test.ShouldEqual, len(wantGeoms.Geometries()))
				for j, want := range wantGeoms.Geometries() {
					got := gotGeoms.Geometries()[j]
					test.That(t, got.Label(), test.ShouldEqual, want.Label())
					test.That(t, spatial.PoseAlmostEqualEps(want.Pose(), got.Pose(), 1e-6), test.ShouldBeTrue)
				}
			}
		})
	}
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
