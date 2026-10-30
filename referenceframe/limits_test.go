package referenceframe

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// twoJointModelJSON is a small SVA document with two revolute joints, so tests can check that
// only the named joint moves.
const twoJointModelJSON = `{
  "name": "arm",
  "kinematic_param_type": "SVA",
  "links": [
    {"id": "base", "parent": "world"},
    {"id": "mid", "parent": "j1"},
    {"id": "tcp", "parent": "j2"}
  ],
  "joints": [
    {"id": "j1", "type": "revolute", "parent": "base", "axis": {"x": 0, "y": 0, "z": 1},
     "min": -360, "max": 360},
    {"id": "j2", "type": "revolute", "parent": "mid", "axis": {"x": 0, "y": 0, "z": 1},
     "min": -180, "max": 180, "max_velocity": 90}
  ]
}`

func parseTwoJointConfig(t *testing.T) *ModelConfigJSON {
	t.Helper()
	m, err := UnmarshalModelJSON([]byte(twoJointModelJSON), "")
	test.That(t, err, test.ShouldBeNil)
	return m.ModelConfig()
}

// The property the whole module boundary rests on: the limits have to be in the bytes, because
// that is what KinematicModelToProtobuf sends. Asserting on the returned config's frames would
// pass even if the document were never patched.
func TestSetJointLimitsReachesTheBytes(t *testing.T) {
	cfg := parseTwoJointConfig(t)

	patched, err := SetJointLimits(cfg, map[string]JointLimits{
		"j1": {MaxVelocity: ptr(180.0), MaxAcceleration: ptr(1145.0)},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, patched.OriginalFile, test.ShouldNotBeNil)
	test.That(t, patched.OriginalFile.Extension, test.ShouldEqual, "json")

	// re-parse the emitted document, the way viam-server does after GetKinematics
	reparsed, err := UnmarshalModelJSON(patched.OriginalFile.Bytes, "")
	test.That(t, err, test.ShouldBeNil)

	limits := reparsed.DoF()
	test.That(t, limits, test.ShouldHaveLength, 2)
	test.That(t, *limits[0].MaxVelocity, test.ShouldAlmostEqual, math.Pi, defaultFloatPrecision)
	test.That(t, *limits[0].MaxAcceleration, test.ShouldAlmostEqual, utils.DegToRad(1145), defaultFloatPrecision)

	// and it survives the trip out to a client
	resp := KinematicModelToProtobuf(reparsed)
	rebuilt, err := KinematicModelFromProtobuf("arm", resp)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, limitsAlmostEqual(rebuilt.DoF(), reparsed.DoF(), defaultFloatPrecision), test.ShouldBeTrue)
}

// Marshaling without clearing OriginalFile first would nest the old document inside the new one,
// and parsing that back would hand you the pre-patch bytes.
func TestSetJointLimitsDoesNotNestTheOldDocument(t *testing.T) {
	cfg := parseTwoJointConfig(t)

	patched, err := SetJointLimits(cfg, map[string]JointLimits{"j1": {MaxVelocity: ptr(180.0)}})
	test.That(t, err, test.ShouldBeNil)

	var doc map[string]json.RawMessage
	test.That(t, json.Unmarshal(patched.OriginalFile.Bytes, &doc), test.ShouldBeNil)
	_, nested := doc["original_file"]
	test.That(t, nested, test.ShouldBeFalse)
}

func TestSetJointLimitsIsAPartialUpdate(t *testing.T) {
	cfg := parseTwoJointConfig(t)

	patched, err := SetJointLimits(cfg, map[string]JointLimits{"j2": {MaxAcceleration: ptr(500.0)}})
	test.That(t, err, test.ShouldBeNil)

	j2 := patched.Joints[1]
	test.That(t, j2.Min, test.ShouldEqual, -180.0)
	test.That(t, j2.Max, test.ShouldEqual, 180.0)
	test.That(t, *j2.MaxVelocity, test.ShouldEqual, 90.0) // was already set, untouched
	test.That(t, *j2.MaxAcceleration, test.ShouldEqual, 500.0)

	// the joint nobody named is untouched
	test.That(t, patched.Joints[0].MaxVelocity, test.ShouldBeNil)

	t.Run("position can be set through the same call", func(t *testing.T) {
		locked, err := SetJointLimits(cfg, map[string]JointLimits{
			"j1": {Min: ptr(-1.0), Max: ptr(1.0)},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, locked.Joints[0].Min, test.ShouldEqual, -1.0)
		test.That(t, locked.Joints[0].Max, test.ShouldEqual, 1.0)
	})

	t.Run("an explicit zero velocity is applied, not treated as unset", func(t *testing.T) {
		frozen, err := SetJointLimits(cfg, map[string]JointLimits{"j1": {MaxVelocity: ptr(0.0)}})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, frozen.Joints[0].MaxVelocity, test.ShouldNotBeNil)
		test.That(t, *frozen.Joints[0].MaxVelocity, test.ShouldEqual, 0.0)
	})
}

// ModelConfig hands out the live pointer, so a caller passing it straight in must not find its
// own model rewritten.
func TestSetJointLimitsLeavesTheInputAlone(t *testing.T) {
	m, err := UnmarshalModelJSON([]byte(twoJointModelJSON), "")
	test.That(t, err, test.ShouldBeNil)
	cfg := m.ModelConfig()
	originalBytes := append([]byte(nil), cfg.OriginalFile.Bytes...)

	_, err = SetJointLimits(cfg, map[string]JointLimits{
		"j1": {Min: ptr(-1.0), Max: ptr(1.0), MaxVelocity: ptr(180.0)},
	})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, cfg.Joints[0].MaxVelocity, test.ShouldBeNil)
	test.That(t, cfg.Joints[0].Min, test.ShouldEqual, -360.0)
	test.That(t, cfg.OriginalFile.Bytes, test.ShouldResemble, originalBytes)

	// and the model those came from still reports what it always did
	test.That(t, m.DoF()[0].MaxVelocity, test.ShouldBeNil)
	test.That(t, m.DoF()[0].Min, test.ShouldAlmostEqual, utils.DegToRad(-360), defaultFloatPrecision)
}

func TestSetJointLimitsErrors(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := SetJointLimits(nil, map[string]JointLimits{"j1": {}})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("unknown joint", func(t *testing.T) {
		cfg := parseTwoJointConfig(t)
		_, err := SetJointLimits(cfg, map[string]JointLimits{"nope": {MaxVelocity: ptr(1.0)}})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `no joint "nope"`)
	})

	// A mimic joint's limits come from whatever drives it, and parsing rejects one that declares
	// its own. Emitting such a document would produce bytes no consumer can read, which is worse
	// than refusing.
	t.Run("mimic joint", func(t *testing.T) {
		m, err := ParseModelJSONFile("testfiles/test_mimic_gripper.json", "")
		test.That(t, err, test.ShouldBeNil)

		cfg := m.ModelConfig()
		var mimicID string
		for _, joint := range cfg.Joints {
			if joint.Mimic != nil {
				mimicID = joint.ID
				break
			}
		}
		test.That(t, mimicID, test.ShouldNotBeBlank)

		_, err = SetJointLimits(cfg, map[string]JointLimits{mimicID: {MaxVelocity: ptr(90.0)}})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, ErrMimicWithLimits.Error())

		// the joints a mimic does not drive are still fair game
		patched, err := SetJointLimits(cfg, map[string]JointLimits{"left_joint": {MaxVelocity: ptr(90.0)}})
		test.That(t, err, test.ShouldBeNil)
		_, err = UnmarshalModelJSON(patched.OriginalFile.Bytes, "")
		test.That(t, err, test.ShouldBeNil)
	})

	// These come straight from a user's resource config in the documented use case, so a typo
	// has to stop here rather than become a limit the motion service plans against.
	t.Run("values that cannot describe a joint", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			limit JointLimits
			want  string
		}{
			{"negative velocity", JointLimits{MaxVelocity: ptr(-50.0)}, "cannot be negative"},
			{"negative acceleration", JointLimits{MaxAcceleration: ptr(-1.0)}, "cannot be negative"},
			{"infinite velocity", JointLimits{MaxVelocity: ptr(math.Inf(1))}, "must be a finite number"},
			{"infinite min", JointLimits{Min: ptr(math.Inf(-1))}, "must be a finite number"},
			{"NaN acceleration", JointLimits{MaxAcceleration: ptr(math.NaN())}, "must be a finite number"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				cfg := parseTwoJointConfig(t)
				_, err := SetJointLimits(cfg, map[string]JointLimits{"j1": tc.limit})
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.want)
			})
		}
	})

	// Min and Max move independently so a joint can be pinned without restating the other side,
	// which means a partial update can leave them crossed.
	t.Run("min above max", func(t *testing.T) {
		cfg := parseTwoJointConfig(t)

		// j1 runs -360 to 360, so raising min alone past 360 empties its range
		_, err := SetJointLimits(cfg, map[string]JointLimits{"j1": {Min: ptr(400.0)}})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "greater than max")

		// setting both is fine
		patched, err := SetJointLimits(cfg, map[string]JointLimits{"j1": {Min: ptr(400.0), Max: ptr(450.0)}})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, patched.Joints[0].Min, test.ShouldEqual, 400.0)
	})

	t.Run("DH config", func(t *testing.T) {
		m, err := ParseModelJSONFile("testfiles/ur5eDH.json", "")
		test.That(t, err, test.ShouldBeNil)

		_, err = SetJointLimits(m.ModelConfig(), map[string]JointLimits{"q_0": {MaxVelocity: ptr(90.0)}})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "DH kinematics")
	})

	// A URDF-backed arm ships a URDF for its meshes. Converting it to SVA to attach limits is
	// something only the Go SDK could do, so the helper refuses everywhere instead.
	t.Run("urdf backed config", func(t *testing.T) {
		cfg := parseTwoJointConfig(t)
		cfg.OriginalFile = &ModelFile{Bytes: []byte("<robot/>"), Extension: "urdf"}

		_, err := SetJointLimits(cfg, map[string]JointLimits{"j1": {MaxVelocity: ptr(1.0)}})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "RSDK-14232")
	})
}

// What a module actually does: take speeds out of its own config, put them on every joint, and
// serve the result. Ends with the check a trajectory generator makes.
func TestSetJointLimitsWholeArm(t *testing.T) {
	cfg := parseTwoJointConfig(t)

	const speedDegsPerSec, accelDegsPerSec2 = 180.0, 1145.0
	limits := map[string]JointLimits{}
	for _, joint := range cfg.Joints {
		limits[joint.ID] = JointLimits{
			MaxVelocity:     ptr(speedDegsPerSec),
			MaxAcceleration: ptr(accelDegsPerSec2),
		}
	}

	patched, err := SetJointLimits(cfg, limits)
	test.That(t, err, test.ShouldBeNil)

	served, err := UnmarshalModelJSON(patched.OriginalFile.Bytes, "")
	test.That(t, err, test.ShouldBeNil)

	vels, accs, ok := TrajectoryLimits(served.DoF())
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, vels, test.ShouldHaveLength, 2)
	for i := range vels {
		test.That(t, vels[i], test.ShouldAlmostEqual, utils.DegToRad(speedDegsPerSec), defaultFloatPrecision)
		test.That(t, accs[i], test.ShouldAlmostEqual, utils.DegToRad(accelDegsPerSec2), defaultFloatPrecision)
	}
}

// prismaticModelJSON has one prismatic joint, so a test can see millimeters pass through untouched.
const prismaticModelJSON = `{
  "name": "slider",
  "kinematic_param_type": "SVA",
  "links": [
    {"id": "base", "parent": "world"},
    {"id": "tip", "parent": "slide"}
  ],
  "joints": [
    {"id": "slide", "type": "prismatic", "parent": "base", "axis": {"x": 1, "y": 0, "z": 0},
     "min": 0, "max": 500, "max_velocity": 100}
  ]
}`

func simpleModelFromJSON(t *testing.T, doc string) *SimpleModel {
	t.Helper()
	m, err := UnmarshalModelJSON([]byte(doc), "")
	test.That(t, err, test.ShouldBeNil)
	sm, ok := m.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	return sm
}

// jointLimit returns the limit the named joint's frame enforces.
func jointLimit(t *testing.T, m Model, id string) Limit {
	t.Helper()
	sm, ok := m.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	frame := sm.internalFS.Frame(id)
	test.That(t, frame, test.ShouldNotBeNil)
	test.That(t, frame.DoF(), test.ShouldHaveLength, 1)
	return frame.DoF()[0]
}

func protoJoint(t *testing.T, pb *commonpb.KinematicModel, id string) *commonpb.KinematicJoint {
	t.Helper()
	idx := slices.IndexFunc(pb.GetJoints(), func(j *commonpb.KinematicJoint) bool { return j.GetId() == id })
	test.That(t, idx, test.ShouldBeGreaterThanOrEqualTo, 0)
	return pb.GetJoints()[idx]
}

// A narrowed limit has to show up everywhere a consumer can look: the frame a planner runs
// against, DoF, the typed model, the configuration, and the deprecated bytes a client that
// predates the typed model still reads.
func TestNarrowJointLimitsReachesEveryEncoding(t *testing.T) {
	sm := simpleModelFromJSON(t, twoJointModelJSON)
	err := sm.NarrowJointLimits(map[string]JointLimits{
		"j2": {Min: ptr(-90.0), Max: ptr(45.0), MaxVelocity: ptr(45.0), MaxAcceleration: ptr(100.0)},
	})
	test.That(t, err, test.ShouldBeNil)

	got := jointLimit(t, sm, "j2")
	test.That(t, got.Min, test.ShouldAlmostEqual, utils.DegToRad(-90))
	test.That(t, got.Max, test.ShouldAlmostEqual, utils.DegToRad(45))
	test.That(t, *got.MaxVelocity, test.ShouldAlmostEqual, utils.DegToRad(45))
	test.That(t, *got.MaxAcceleration, test.ShouldAlmostEqual, utils.DegToRad(100))
	test.That(t, sm.DoF()[1].Max, test.ShouldAlmostEqual, utils.DegToRad(45))
	test.That(t, jointLimit(t, sm, "j1").Max, test.ShouldAlmostEqual, utils.DegToRad(360))

	pb, err := ModelToProto(sm)
	test.That(t, err, test.ShouldBeNil)
	limits := protoJoint(t, pb, "j2").GetLimits()
	test.That(t, limits.GetMin(), test.ShouldAlmostEqual, -90)
	test.That(t, limits.GetMax(), test.ShouldAlmostEqual, 45)
	test.That(t, limits.GetMaxVelocity(), test.ShouldAlmostEqual, 45)
	test.That(t, limits.GetMaxAcceleration(), test.ShouldAlmostEqual, 100)

	cfg := sm.ModelConfig()
	idx := slices.IndexFunc(cfg.Joints, func(j JointConfig) bool { return j.ID == "j2" })
	test.That(t, cfg.Joints[idx].Min, test.ShouldEqual, -90.0)
	test.That(t, cfg.Joints[idx].Max, test.ShouldEqual, 45.0)
	test.That(t, *cfg.Joints[idx].MaxVelocity, test.ShouldEqual, 45.0)
	test.That(t, *cfg.Joints[idx].MaxAcceleration, test.ShouldEqual, 100.0)
	test.That(t, cfg.OriginalFile.Extension, test.ShouldEqual, "json")

	resp := KinematicModelToProtobuf(sm)
	format := resp.GetFormat() //nolint:staticcheck // deprecated fields are still served
	test.That(t, format, test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_SVA)
	legacy, err := UnmarshalModelJSON(resp.GetKinematicsData(), "") //nolint:staticcheck // deprecated fields are still served
	test.That(t, err, test.ShouldBeNil)
	old := jointLimit(t, legacy, "j2")
	test.That(t, old.Max, test.ShouldAlmostEqual, utils.DegToRad(45))
	test.That(t, *old.MaxVelocity, test.ShouldAlmostEqual, utils.DegToRad(45))
}

func TestNarrowJointLimitsRefusesWidening(t *testing.T) {
	for _, tc := range []struct {
		name    string
		request JointLimits
		want    string
	}{
		{"min below the model", JointLimits{Min: ptr(-200.0)}, "min -200 is below the model's min -180"},
		{"max above the model", JointLimits{Max: ptr(190.0)}, "max 190 is above the model's max 180"},
		{"faster than the model", JointLimits{MaxVelocity: ptr(100.0)}, "max_velocity 100 is above the model's 90"},
		{"min above max", JointLimits{Min: ptr(30.0), Max: ptr(10.0)}, "min 30 is above max 10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sm := simpleModelFromJSON(t, twoJointModelJSON)
			// j1 is fine on its own, and has to stay as it was when j2 fails beside it
			err := sm.NarrowJointLimits(map[string]JointLimits{"j1": {Max: ptr(90.0)}, "j2": tc.request})
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, `joint "j2"`)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.want)
			test.That(t, jointLimit(t, sm, "j1").Max, test.ShouldAlmostEqual, utils.DegToRad(360))
			test.That(t, jointLimit(t, sm, "j2").Max, test.ShouldAlmostEqual, utils.DegToRad(180))
			test.That(t, sm.ModelConfig().OriginalFile, test.ShouldNotBeNil)
		})
	}

	t.Run("a bound equal to the model's own passes", func(t *testing.T) {
		sm := simpleModelFromJSON(t, twoJointModelJSON)
		err := sm.NarrowJointLimits(map[string]JointLimits{"j2": {Min: ptr(-180.0), Max: ptr(180.0), MaxVelocity: ptr(90.0)}})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("a joint the model does not bound accepts any speed", func(t *testing.T) {
		sm := simpleModelFromJSON(t, twoJointModelJSON)
		err := sm.NarrowJointLimits(map[string]JointLimits{"j1": {MaxVelocity: ptr(720.0), MaxAcceleration: ptr(9000.0)}})
		test.That(t, err, test.ShouldBeNil)
		got := jointLimit(t, sm, "j1")
		test.That(t, *got.MaxVelocity, test.ShouldAlmostEqual, utils.DegToRad(720))
		test.That(t, *got.MaxAcceleration, test.ShouldAlmostEqual, utils.DegToRad(9000))
	})

	t.Run("values that cannot describe a joint", func(t *testing.T) {
		sm := simpleModelFromJSON(t, twoJointModelJSON)
		err := sm.NarrowJointLimits(map[string]JointLimits{"j1": {Max: ptr(math.NaN())}})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "finite")
		err = sm.NarrowJointLimits(map[string]JointLimits{"j1": {MaxVelocity: ptr(-1.0)}})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "negative")
	})
}

func TestNarrowJointLimitsLocksAndNarrowsAgain(t *testing.T) {
	sm := simpleModelFromJSON(t, twoJointModelJSON)
	err := sm.NarrowJointLimits(map[string]JointLimits{"j1": {Min: ptr(10.0), Max: ptr(10.0)}})
	test.That(t, err, test.ShouldBeNil)
	locked := jointLimit(t, sm, "j1")
	test.That(t, locked.Min, test.ShouldAlmostEqual, utils.DegToRad(10))
	test.That(t, locked.Max, test.ShouldAlmostEqual, utils.DegToRad(10))

	// each call narrows what the one before left, so the second call's bound is the one that counts
	test.That(t, sm.NarrowJointLimits(map[string]JointLimits{"j2": {Max: ptr(100.0)}}), test.ShouldBeNil)
	test.That(t, sm.NarrowJointLimits(map[string]JointLimits{"j2": {Max: ptr(50.0)}}), test.ShouldBeNil)
	err = sm.NarrowJointLimits(map[string]JointLimits{"j2": {Max: ptr(80.0)}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "max 80 is above the model's max 50")
}

func TestNarrowJointLimitsRefusesWhatIsNotAJoint(t *testing.T) {
	t.Run("unknown joint", func(t *testing.T) {
		sm := simpleModelFromJSON(t, twoJointModelJSON)
		err := sm.NarrowJointLimits(map[string]JointLimits{"j9": {Max: ptr(1.0)}})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `no joint "j9" in model "arm"`)
	})

	t.Run("a link is not a joint", func(t *testing.T) {
		sm := simpleModelFromJSON(t, twoJointModelJSON)
		err := sm.NarrowJointLimits(map[string]JointLimits{"base": {Max: ptr(1.0)}})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `no joint "base"`)
	})

	t.Run("mimic joint", func(t *testing.T) {
		m, err := ParseModelJSONFile("testfiles/test_mimic_gripper.json", "")
		test.That(t, err, test.ShouldBeNil)
		sm, ok := m.(*SimpleModel)
		test.That(t, ok, test.ShouldBeTrue)
		var mimicID string
		for _, joint := range sm.ModelConfig().Joints {
			if joint.Mimic != nil {
				mimicID = joint.ID
				break
			}
		}
		test.That(t, mimicID, test.ShouldNotBeBlank)

		err = sm.NarrowJointLimits(map[string]JointLimits{mimicID: {MaxVelocity: ptr(1.0)}})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, ErrMimicWithLimits.Error())

		// the joint a mimic follows is still fair game
		test.That(t, sm.NarrowJointLimits(map[string]JointLimits{"left_joint": {MaxVelocity: ptr(1.0)}}), test.ShouldBeNil)
	})
}

func TestNarrowJointLimitsPrismaticStaysInMillimeters(t *testing.T) {
	sm := simpleModelFromJSON(t, prismaticModelJSON)
	err := sm.NarrowJointLimits(map[string]JointLimits{"slide": {Max: ptr(250.0), MaxVelocity: ptr(50.0)}})
	test.That(t, err, test.ShouldBeNil)
	got := jointLimit(t, sm, "slide")
	test.That(t, got.Max, test.ShouldEqual, 250.0)
	test.That(t, *got.MaxVelocity, test.ShouldEqual, 50.0)

	err = sm.NarrowJointLimits(map[string]JointLimits{"slide": {MaxVelocity: ptr(150.0)}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "max_velocity 150 is above the model's 50")
}

// A model read from a URDF could not carry a narrowed limit to an old client at all, since the
// file was replayed as it was. Now the bytes are written from the model, as an SVA document.
func TestNarrowJointLimitsOnURDFModel(t *testing.T) {
	m, err := ParseModelXMLFile("testfiles/ur5e.urdf", "", nil)
	test.That(t, err, test.ShouldBeNil)
	sm, ok := m.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	format := KinematicModelToProtobuf(sm).GetFormat() //nolint:staticcheck // deprecated fields are still served
	test.That(t, format, test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_URDF)

	err = sm.NarrowJointLimits(map[string]JointLimits{"elbow_joint": {Max: ptr(90.0), MaxVelocity: ptr(60.0)}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, jointLimit(t, sm, "elbow_joint").Max, test.ShouldAlmostEqual, utils.DegToRad(90))

	resp := KinematicModelToProtobuf(sm)
	format = resp.GetFormat() //nolint:staticcheck // deprecated fields are still served
	test.That(t, format, test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_SVA)
	legacy, err := UnmarshalModelJSON(resp.GetKinematicsData(), "") //nolint:staticcheck // deprecated fields are still served
	test.That(t, err, test.ShouldBeNil)
	old := jointLimit(t, legacy, "elbow_joint")
	test.That(t, old.Max, test.ShouldAlmostEqual, utils.DegToRad(90))
	test.That(t, *old.MaxVelocity, test.ShouldAlmostEqual, utils.DegToRad(60))
	limits := protoJoint(t, resp.GetModel(), "elbow_joint").GetLimits()
	test.That(t, limits.GetMax(), test.ShouldAlmostEqual, 90)
	test.That(t, limits.GetMaxVelocity(), test.ShouldAlmostEqual, 60)

	err = sm.NarrowJointLimits(map[string]JointLimits{"elbow_joint": {MaxVelocity: ptr(181.0)}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "max_velocity 181 is above the model's 60")
}

// A model built from frames in code has no configuration, and narrowing it works on the frames
// alone.
func TestNarrowJointLimitsOnFrameBuiltModel(t *testing.T) {
	fs := NewEmptyFrameSystem("internal")
	vel := 1.0
	j1, err := NewRotationalFrame("j1", spatial.R4AA{RZ: 1}, Limit{Min: -math.Pi, Max: math.Pi, MaxVelocity: &vel})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(j1, fs.World()), test.ShouldBeNil)
	sm, err := NewModel("built", fs, "j1")
	test.That(t, err, test.ShouldBeNil)

	err = sm.NarrowJointLimits(map[string]JointLimits{"j1": {Max: ptr(90.0), MaxVelocity: ptr(30.0)}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sm.ModelConfig(), test.ShouldBeNil)
	got := jointLimit(t, sm, "j1")
	test.That(t, got.Max, test.ShouldAlmostEqual, utils.DegToRad(90))
	test.That(t, *got.MaxVelocity, test.ShouldAlmostEqual, utils.DegToRad(30))

	pb, err := ModelToProto(sm)
	test.That(t, err, test.ShouldBeNil)
	limits := protoJoint(t, pb, "j1").GetLimits()
	test.That(t, limits.GetMax(), test.ShouldAlmostEqual, 90)
	test.That(t, limits.GetMaxVelocity(), test.ShouldAlmostEqual, 30)

	err = sm.NarrowJointLimits(map[string]JointLimits{"j1": {MaxVelocity: ptr(60.0)}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "max_velocity 60 is above the model's 30")
}

// The attribute shape decodes straight into the helper's argument.
func TestJointLimitsAttributeShape(t *testing.T) {
	var attrs struct {
		JointLimits map[string]JointLimits `json:"joint_limits"`
	}
	err := json.Unmarshal([]byte(`{"joint_limits": {
		"j1": {"min": -31, "max": -29},
		"j2": {"max_velocity": 30, "max_acceleration": 380}
	}}`), &attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, attrs.JointLimits["j1"].Min, test.ShouldNotBeNil)
	test.That(t, attrs.JointLimits["j1"].MaxVelocity, test.ShouldBeNil)
	test.That(t, *attrs.JointLimits["j2"].MaxAcceleration, test.ShouldEqual, 380.0)

	sm := simpleModelFromJSON(t, twoJointModelJSON)
	test.That(t, sm.NarrowJointLimits(attrs.JointLimits), test.ShouldBeNil)
	test.That(t, jointLimit(t, sm, "j1").Max, test.ShouldAlmostEqual, utils.DegToRad(-29))
}

func TestNarrowJointLimitsLeavesTheFileAloneWhenNothingChanges(t *testing.T) {
	sm := simpleModelFromJSON(t, twoJointModelJSON)
	before := sm.ModelConfig().OriginalFile
	test.That(t, sm.NarrowJointLimits(nil), test.ShouldBeNil)
	test.That(t, sm.NarrowJointLimits(map[string]JointLimits{"j1": {}}), test.ShouldBeNil)
	test.That(t, sm.ModelConfig().OriginalFile == before, test.ShouldBeTrue)

	// an entry that sets nothing still has to name a joint
	err := sm.NarrowJointLimits(map[string]JointLimits{"j9": {}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `no joint "j9"`)
}

// A joint with no position bound has no JSON form, so the file stays as it was read and the typed
// model alone carries the narrowed limit.
func TestNarrowJointLimitsKeepsAFileItCannotRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spinner.urdf")
	test.That(t, os.WriteFile(path, []byte(`<robot name="spinner">
  <link name="base"/>
  <link name="tip"/>
  <joint name="spin" type="continuous">
    <origin xyz="0 0 0.1" rpy="0 0 0"/>
    <parent link="base"/>
    <child link="tip"/>
    <axis xyz="0 0 1"/>
    <limit velocity="3.14"/>
  </joint>
</robot>`), 0o600), test.ShouldBeNil)
	m, err := ParseModelXMLFile(path, "", nil)
	test.That(t, err, test.ShouldBeNil)
	sm, ok := m.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	before := sm.ModelConfig().OriginalFile
	test.That(t, before, test.ShouldNotBeNil)

	err = sm.NarrowJointLimits(map[string]JointLimits{"spin": {MaxVelocity: ptr(90.0)}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *jointLimit(t, sm, "spin").MaxVelocity, test.ShouldAlmostEqual, utils.DegToRad(90))
	test.That(t, sm.ModelConfig().OriginalFile == before, test.ShouldBeTrue)

	resp := KinematicModelToProtobuf(sm)
	format := resp.GetFormat() //nolint:staticcheck // deprecated fields are still served
	test.That(t, format, test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_URDF)
	test.That(t, protoJoint(t, resp.GetModel(), "spin").GetLimits().GetMaxVelocity(), test.ShouldAlmostEqual, 90)
}

// A cylinder has no shape in the message, so this model never had a typed form and the file is
// all a client gets. It still has to carry the narrowed limit.
func TestNarrowJointLimitsRewritesAFileTheMessageCannotCarry(t *testing.T) {
	sm := simpleModelFromJSON(t, `{
  "name": "arm",
  "kinematic_param_type": "SVA",
  "links": [
    {"id": "base", "parent": "world", "geometry": {"type": "cylinder", "r": 10, "l": 20}},
    {"id": "tip", "parent": "j1"}
  ],
  "joints": [
    {"id": "j1", "type": "revolute", "parent": "base", "axis": {"x": 0, "y": 0, "z": 1}, "min": -90, "max": 90}
  ]
}`)
	test.That(t, sm.NarrowJointLimits(map[string]JointLimits{"j1": {Max: ptr(45.0)}}), test.ShouldBeNil)
	resp := KinematicModelToProtobuf(sm)
	test.That(t, resp.GetModel(), test.ShouldBeNil)
	legacy, err := UnmarshalModelJSON(resp.GetKinematicsData(), "") //nolint:staticcheck // deprecated fields are still served
	test.That(t, err, test.ShouldBeNil)
	test.That(t, jointLimit(t, legacy, "j1").Max, test.ShouldAlmostEqual, utils.DegToRad(45))
}

func TestNarrowJointLimitsRefusesDHModel(t *testing.T) {
	m, err := ParseModelJSONFile("testfiles/ur5eDH.json", "")
	test.That(t, err, test.ShouldBeNil)
	sm, ok := m.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	err = sm.NarrowJointLimits(map[string]JointLimits{"q_0_j": {MaxVelocity: ptr(90.0)}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "DH kinematics")
}

func TestNarrowJointLimitsNamesTheFirstBadJointInOrder(t *testing.T) {
	sm := simpleModelFromJSON(t, twoJointModelJSON)
	for range 5 {
		err := sm.NarrowJointLimits(map[string]JointLimits{"j2": {Min: ptr(-200.0)}, "j1": {Max: ptr(math.NaN())}})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `joint "j1"`)
	}
}

func TestNarrowJointLimitsSeesThroughANamedFrameAndRefusesANestedModel(t *testing.T) {
	fs := NewEmptyFrameSystem("internal")
	rot, err := NewRotationalFrame("inner", spatial.R4AA{RZ: 1}, Limit{Min: -math.Pi, Max: math.Pi})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(NewNamedFrame(rot, "j1"), fs.World()), test.ShouldBeNil)
	arm := simpleModelFromJSON(t, twoJointModelJSON)
	test.That(t, fs.AddFrame(arm, fs.Frame("j1")), test.ShouldBeNil)
	sm, err := NewModel("built", fs, "arm")
	test.That(t, err, test.ShouldBeNil)

	// the renamed revolute joint is still read in degrees
	test.That(t, sm.NarrowJointLimits(map[string]JointLimits{"j1": {Max: ptr(90.0)}}), test.ShouldBeNil)
	test.That(t, jointLimit(t, sm, "j1").Max, test.ShouldAlmostEqual, utils.DegToRad(90))

	// the nested model answers DoF from a copy, so a write through it would reach nothing it plans with
	err = sm.NarrowJointLimits(map[string]JointLimits{"arm": {Max: ptr(1.0)}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `joint "arm" in model "built" is not a single joint`)
}
