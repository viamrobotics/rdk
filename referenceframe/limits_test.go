package referenceframe

import (
	"encoding/json"
	"math"
	"testing"

	"go.viam.com/test"

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
