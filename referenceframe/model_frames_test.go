package referenceframe

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
)

// A model built in code with no file behind it converts like one read from a file, and a client
// that predates the typed model still gets SVA bytes it can parse.
func TestFrameBuiltModelRoundTrips(t *testing.T) {
	box, err := spatial.NewBox(spatial.NewPoseFromPoint(r3.Vector{Z: 50}), r3.Vector{X: 100, Y: 100, Z: 100}, "")
	test.That(t, err, test.ShouldBeNil)
	limit := Limit{Min: -1000, Max: 1000}
	model, err := New2DMobileModelFrame("base", []Limit{limit, limit, {Min: -math.Pi, Max: math.Pi}}, box)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, model.ModelConfig(), test.ShouldBeNil)

	pb, err := ModelToProto(model)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pb.GetOutputFrames(), test.ShouldResemble, []string{"geometry"})
	test.That(t, jointByID(t, pb, "x").GetType(), test.ShouldEqual, commonpb.JointType_JOINT_TYPE_PRISMATIC)
	test.That(t, jointByID(t, pb, "x").GetParent(), test.ShouldEqual, World)
	test.That(t, jointByID(t, pb, "y").GetParent(), test.ShouldEqual, "x")
	test.That(t, jointByID(t, pb, "theta").GetType(), test.ShouldEqual, commonpb.JointType_JOINT_TYPE_REVOLUTE)
	test.That(t, jointByID(t, pb, "theta").GetLimits().GetMax(), test.ShouldAlmostEqual, 180)
	link := linkByID(t, pb, "geometry")
	test.That(t, link.GetParent(), test.ShouldEqual, "theta")
	test.That(t, len(link.GetCollision()), test.ShouldEqual, 1)
	test.That(t, link.GetCollision()[0].GetLabel(), test.ShouldEqual, "geometry")

	restored, err := ModelFromProto(pb, "")
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, model, restored)

	resp := KinematicModelToProtobuf(model)
	test.That(t, resp.GetModel(), test.ShouldNotBeNil)
	//nolint:staticcheck // deprecated fields are still served
	test.That(t, resp.GetFormat(), test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_SVA)
	old, err := UnmarshalModelJSON(resp.GetKinematicsData(), "base") //nolint:staticcheck // deprecated fields are still served
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, model, old)
}

// Frames assembled by hand, with a rotated link, several geometries on one link, a limit with a
// velocity bound, and a joint that mimics another, all reach the message and come back.
func TestFrameBuiltModelWithMimic(t *testing.T) {
	sphere := func(label string, x float64) spatial.Geometry {
		s, err := spatial.NewSphere(spatial.NewPoseFromPoint(r3.Vector{X: x}), 10, label)
		test.That(t, err, test.ShouldBeNil)
		return s
	}
	fs := NewEmptyFrameSystem("internal")
	basePose := spatial.NewPose(r3.Vector{Z: 100}, &spatial.R4AA{Theta: math.Pi / 2, RX: 0, RY: 0, RZ: 1})
	base, err := NewStaticFrameWithGeometry("base", basePose, sphere("", 0))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(base, fs.World()), test.ShouldBeNil)
	vel := 2.0
	j1, err := NewRotationalFrame("j1", spatial.R4AA{RZ: 1}, Limit{Min: -math.Pi, Max: math.Pi, MaxVelocity: &vel})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(j1, base), test.ShouldBeNil)
	link1, err := NewStaticFrameWithGeometries(
		"link1", spatial.NewPoseFromPoint(r3.Vector{X: 200}), []spatial.Geometry{sphere("", 20), sphere("named", 40)},
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(link1, j1), test.ShouldBeNil)
	j2, err := NewRotationalFrame("j2", spatial.R4AA{RZ: 1}, Limit{Min: math.Inf(-1), Max: math.Inf(1)})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(j2, link1), test.ShouldBeNil)
	tip, err := NewStaticFrame("tip", spatial.NewPoseFromPoint(r3.Vector{X: 50}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(tip, j2), test.ShouldBeNil)
	model, err := NewModelWithMimics("arm", fs, "tip", map[string]*mimicMapping{
		"j2": {sourceFrameName: "j1", valueMultiplier: -1, valueOffset: 0.5},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(model.DoF()), test.ShouldEqual, 1)

	pb, err := ModelToProto(model)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, jointByID(t, pb, "j1").GetLimits().GetMaxVelocity(), test.ShouldAlmostEqual, 2*180/math.Pi)
	mimic := jointByID(t, pb, "j2").GetMimic()
	test.That(t, mimic.GetJoint(), test.ShouldEqual, "j1")
	test.That(t, mimic.GetMultiplier(), test.ShouldEqual, -1)
	test.That(t, mimic.GetOffset(), test.ShouldEqual, 0.5)
	collisions := linkByID(t, pb, "link1").GetCollision()
	test.That(t, len(collisions), test.ShouldEqual, 2)
	test.That(t, collisions[0].GetLabel(), test.ShouldEqual, "link1_collision_0")
	test.That(t, collisions[1].GetLabel(), test.ShouldEqual, "named")

	restored, err := ModelFromProto(pb, "")
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, model, restored)
}

// A joint with no position bound has no JSON form, so the bytes stay empty and the message alone
// carries the model, the same as for a file backed model with such a joint.
func TestFrameBuiltModelWithContinuousJoint(t *testing.T) {
	spin, err := NewRotationalFrame("spin", spatial.R4AA{RZ: 1}, Limit{Min: math.Inf(-1), Max: math.Inf(1)})
	test.That(t, err, test.ShouldBeNil)
	tip, err := NewStaticFrame("tip", spatial.NewPoseFromPoint(r3.Vector{X: 50}))
	test.That(t, err, test.ShouldBeNil)
	model, err := NewSerialModel("spinner", []Frame{spin, tip})
	test.That(t, err, test.ShouldBeNil)

	resp := KinematicModelToProtobuf(model)
	test.That(t, jointByID(t, resp.GetModel(), "spin").GetType(), test.ShouldEqual, commonpb.JointType_JOINT_TYPE_CONTINUOUS)
	test.That(t, len(resp.GetKinematicsData()), test.ShouldEqual, 0) //nolint:staticcheck // deprecated fields are still served
	restored, err := ModelFromProto(resp.GetModel(), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, math.IsInf(restored.DoF()[0].Max, 1), test.ShouldBeTrue)
}

// A pose frame has no file form, so a model holding one still sends an empty response the way
// every model built in code did before.
func TestKinematicsResponseWithPoseFrame(t *testing.T) {
	pose, err := NewPoseFrame("free", nil)
	test.That(t, err, test.ShouldBeNil)
	model, err := NewSerialModel("floating", []Frame{pose})
	test.That(t, err, test.ShouldBeNil)

	_, err = ModelToProto(model)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no kinematics file form")
	resp := KinematicModelToProtobuf(model)
	test.That(t, resp.GetModel(), test.ShouldBeNil)
	test.That(t, len(resp.GetKinematicsData()), test.ShouldEqual, 0) //nolint:staticcheck // deprecated fields are still served
}
