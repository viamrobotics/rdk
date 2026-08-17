package referenceframe

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func ptr(v float64) *float64 {
	return &v
}

// revoluteModelJSON builds a one joint model, splicing in whatever limit fields the caller
// wants so tests can cover set, unset, and explicitly zero.
func revoluteModelJSON(limitFields string) []byte {
	return []byte(`{
	  "name": "test",
	  "kinematic_param_type": "SVA",
	  "links": [
	    {"id": "base", "parent": "world"},
	    {"id": "tcp", "parent": "j1"}
	  ],
	  "joints": [
	    {"id": "j1", "type": "revolute", "parent": "base", "axis": {"x": 0, "y": 0, "z": 1},
	     "min": -360, "max": 360` + limitFields + `}
	  ]
	}`)
}

func TestJointLimitsParsing(t *testing.T) {
	t.Run("revolute limits convert degrees to radians", func(t *testing.T) {
		m, err := UnmarshalModelJSON(revoluteModelJSON(`, "max_velocity": 180, "max_acceleration": 1145`), "")
		test.That(t, err, test.ShouldBeNil)

		limit := m.DoF()[0]
		test.That(t, limit.MaxVelocity, test.ShouldNotBeNil)
		test.That(t, *limit.MaxVelocity, test.ShouldAlmostEqual, math.Pi, defaultFloatPrecision)
		test.That(t, limit.MaxAcceleration, test.ShouldNotBeNil)
		test.That(t, *limit.MaxAcceleration, test.ShouldAlmostEqual, utils.DegToRad(1145), defaultFloatPrecision)
	})

	t.Run("omitted limits are unbounded", func(t *testing.T) {
		m, err := UnmarshalModelJSON(revoluteModelJSON(""), "")
		test.That(t, err, test.ShouldBeNil)

		limit := m.DoF()[0]
		test.That(t, limit.MaxVelocity, test.ShouldBeNil)
		test.That(t, limit.MaxAcceleration, test.ShouldBeNil)
		test.That(t, limit.MaxVelocityOr(42), test.ShouldEqual, 42)
		test.That(t, limit.MaxAccelerationOr(42), test.ShouldEqual, 42)
	})

	// The reason the fields are pointers: a joint that is pinned in place is a different
	// statement from a joint with no speed limit at all.
	t.Run("explicit zero is not the same as omitted", func(t *testing.T) {
		m, err := UnmarshalModelJSON(revoluteModelJSON(`, "max_velocity": 0`), "")
		test.That(t, err, test.ShouldBeNil)

		limit := m.DoF()[0]
		test.That(t, limit.MaxVelocity, test.ShouldNotBeNil)
		test.That(t, *limit.MaxVelocity, test.ShouldEqual, 0.0)
		test.That(t, limit.MaxVelocityOr(42), test.ShouldEqual, 0.0)
	})

	t.Run("prismatic limits stay in mm", func(t *testing.T) {
		m, err := UnmarshalModelJSON([]byte(`{
		  "name": "test",
		  "kinematic_param_type": "SVA",
		  "links": [
		    {"id": "base", "parent": "world"},
		    {"id": "tcp", "parent": "j1"}
		  ],
		  "joints": [
		    {"id": "j1", "type": "prismatic", "parent": "base", "axis": {"x": 1, "y": 0, "z": 0},
		     "min": -100, "max": 100, "max_velocity": 250, "max_acceleration": 500}
		  ]
		}`), "")
		test.That(t, err, test.ShouldBeNil)

		limit := m.DoF()[0]
		test.That(t, *limit.MaxVelocity, test.ShouldEqual, 250.0)
		test.That(t, *limit.MaxAcceleration, test.ShouldEqual, 500.0)
	})
}

func TestJointLimitsSVARoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name         string
		limitFields  string
		wantVelocity *float64
	}{
		{"set", `, "max_velocity": 180, "max_acceleration": 1145`, ptr(180.0)},
		{"omitted", "", nil},
		{"explicit zero", `, "max_velocity": 0, "max_acceleration": 1145`, ptr(0.0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := UnmarshalModelJSON(revoluteModelJSON(tc.limitFields), "")
			test.That(t, err, test.ShouldBeNil)

			// marshal the joint frame back out and confirm the wire form matches what came in
			frame := m.(*SimpleModel).internalFS.Frame("j1")
			data, err := frame.MarshalJSON()
			test.That(t, err, test.ShouldBeNil)

			var cfg JointConfig
			test.That(t, json.Unmarshal(data, &cfg), test.ShouldBeNil)
			if tc.wantVelocity == nil {
				test.That(t, cfg.MaxVelocity, test.ShouldBeNil)
			} else {
				test.That(t, cfg.MaxVelocity, test.ShouldNotBeNil)
				test.That(t, *cfg.MaxVelocity, test.ShouldAlmostEqual, *tc.wantVelocity, defaultFloatPrecision)
			}

			// and that reading the emitted document back lands on the same internal limits
			reparsed := &rotationalFrame{}
			test.That(t, reparsed.UnmarshalJSON(data), test.ShouldBeNil)
			test.That(t, limitsAlmostEqual(reparsed.DoF(), frame.DoF(), defaultFloatPrecision), test.ShouldBeTrue)
		})
	}
}

// Cloning a frame goes through MarshalJSON and UnmarshalJSON, so a limit that only one side
// knows about disappears on every clone. This covers bare joint frames; cloning a whole
// SimpleModel is a different path, since it re-parses the model's kinematics document.
func TestJointLimitsSurviveClone(t *testing.T) {
	rot, err := NewRotationalFrame("rot", spatial.R4AA{RX: 0, RY: 0, RZ: 1},
		Limit{Min: -math.Pi, Max: math.Pi, MaxVelocity: ptr(math.Pi), MaxAcceleration: ptr(2 * math.Pi)})
	test.That(t, err, test.ShouldBeNil)

	prism, err := NewTranslationalFrame("prism", r3.Vector{X: 1}, Limit{Min: -100, Max: 100, MaxVelocity: ptr(250.0)})
	test.That(t, err, test.ShouldBeNil)

	fs := NewEmptyFrameSystem("test")
	test.That(t, fs.AddFrame(rot, fs.World()), test.ShouldBeNil)
	test.That(t, fs.AddFrame(prism, rot), test.ShouldBeNil)

	cloned, err := fs.Clone()
	test.That(t, err, test.ShouldBeNil)

	for _, name := range []string{"rot", "prism"} {
		got := cloned.Frame(name)
		test.That(t, got, test.ShouldNotBeNil)
		test.That(t, limitsAlmostEqual(got.DoF(), fs.Frame(name).DoF(), defaultFloatPrecision), test.ShouldBeTrue)
	}

	// the clone must own its floats, not share them with the original
	test.That(t, cloned.Frame("rot").DoF()[0].MaxVelocity, test.ShouldNotEqual, rot.DoF()[0].MaxVelocity)
}

// Limit is a config type as well as an internal one: the motion service's input_range_override
// is a map of them. An operator writing the same field name they use in a kinematics file has to
// get the same result, rather than a silently dropped value.
func TestLimitDecodesSnakeCase(t *testing.T) {
	var limit Limit
	test.That(t, json.Unmarshal([]byte(`{"min":-1,"max":1,"max_velocity":2,"max_acceleration":3}`), &limit),
		test.ShouldBeNil)

	test.That(t, limit.Min, test.ShouldEqual, -1.0)
	test.That(t, limit.Max, test.ShouldEqual, 1.0)
	test.That(t, limit.MaxVelocity, test.ShouldNotBeNil)
	test.That(t, *limit.MaxVelocity, test.ShouldEqual, 2.0)
	test.That(t, *limit.MaxAcceleration, test.ShouldEqual, 3.0)

	// documents written before the fields were tagged still decode, the standard library
	// falls back to a case-insensitive match
	var legacy Limit
	test.That(t, json.Unmarshal([]byte(`{"Min":-1,"Max":1}`), &legacy), test.ShouldBeNil)
	test.That(t, legacy.Min, test.ShouldEqual, -1.0)
	test.That(t, legacy.Max, test.ShouldEqual, 1.0)
}

// Infinity and nil both mean "no bound", and marshaling turns infinity into nil, so a frame
// built with an infinite limit has to stay equal to its own clone.
func TestUnboundedLimitsCompareEqual(t *testing.T) {
	infinite := []Limit{{Min: -1, Max: 1, MaxVelocity: ptr(math.Inf(1))}}
	absent := []Limit{{Min: -1, Max: 1}}
	test.That(t, limitsAlmostEqual(infinite, absent, defaultFloatPrecision), test.ShouldBeTrue)

	bounded := []Limit{{Min: -1, Max: 1, MaxVelocity: ptr(2.0)}}
	test.That(t, limitsAlmostEqual(infinite, bounded, defaultFloatPrecision), test.ShouldBeFalse)

	rot, err := NewRotationalFrame("rot", spatial.R4AA{RZ: 1},
		Limit{Min: -math.Pi, Max: math.Pi, MaxVelocity: ptr(math.Inf(1))})
	test.That(t, err, test.ShouldBeNil)
	cloned, err := clone(rot)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, limitsAlmostEqual(cloned.DoF(), rot.DoF(), defaultFloatPrecision), test.ShouldBeTrue)
}

func TestLimitOverridesPreserveVelocity(t *testing.T) {
	m, err := UnmarshalModelJSON(revoluteModelJSON(`, "max_velocity": 180, "max_acceleration": 1145`), "")
	test.That(t, err, test.ShouldBeNil)
	base := m.(*SimpleModel)

	t.Run("position only override leaves velocity alone", func(t *testing.T) {
		overridden, err := NewModelWithLimitOverrides(base, map[string]Limit{"j1": {Min: -1, Max: 1}})
		test.That(t, err, test.ShouldBeNil)

		limit := overridden.DoF()[0]
		test.That(t, limit.Min, test.ShouldEqual, -1.0)
		test.That(t, limit.Max, test.ShouldEqual, 1.0)
		test.That(t, limit.MaxVelocity, test.ShouldNotBeNil)
		test.That(t, *limit.MaxVelocity, test.ShouldAlmostEqual, math.Pi, defaultFloatPrecision)
		test.That(t, limit.MaxAcceleration, test.ShouldNotBeNil)
	})

	t.Run("an override that sets velocity still wins", func(t *testing.T) {
		overridden, err := NewModelWithLimitOverrides(base,
			map[string]Limit{"j1": {Min: -1, Max: 1, MaxVelocity: ptr(0.5)}})
		test.That(t, err, test.ShouldBeNil)

		test.That(t, *overridden.DoF()[0].MaxVelocity, test.ShouldEqual, 0.5)
	})

	t.Run("the override map is not aliased into the model", func(t *testing.T) {
		override := Limit{Min: -1, Max: 1, MaxVelocity: ptr(0.5)}
		overridden, err := NewModelWithLimitOverrides(base, map[string]Limit{"j1": override})
		test.That(t, err, test.ShouldBeNil)

		// the caller's config outlives the model and is reused on every plan, so the model
		// must not be holding its float
		test.That(t, overridden.DoF()[0].MaxVelocity, test.ShouldNotPointTo, override.MaxVelocity)
	})

	t.Run("the base model is untouched", func(t *testing.T) {
		test.That(t, *base.DoF()[0].MaxVelocity, test.ShouldAlmostEqual, math.Pi, defaultFloatPrecision)
		test.That(t, base.DoF()[0].Min, test.ShouldAlmostEqual, utils.DegToRad(-360), defaultFloatPrecision)
	})
}

func TestMimicJointRejectsVelocityLimits(t *testing.T) {
	for _, tc := range []struct {
		name  string
		joint JointConfig
	}{
		{"velocity", JointConfig{
			ID: "j2", Type: RevoluteJoint, Parent: "j1", Axis: spatial.AxisConfig{Z: 1},
			MaxVelocity: ptr(180.0), Mimic: &MimicConfig{Joint: "j1"},
		}},
		{"acceleration", JointConfig{
			ID: "j2", Type: RevoluteJoint, Parent: "j1", Axis: spatial.AxisConfig{Z: 1},
			MaxAcceleration: ptr(1145.0), Mimic: &MimicConfig{Joint: "j1"},
		}},
		{"explicit zero velocity", JointConfig{
			ID: "j2", Type: RevoluteJoint, Parent: "j1", Axis: spatial.AxisConfig{Z: 1},
			MaxVelocity: ptr(0.0), Mimic: &MimicConfig{Joint: "j1"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &ModelConfigJSON{
				KinParamType: "SVA",
				OutputFrames: []string{"tcp"},
				Links: []LinkConfig{
					{ID: "base", Parent: "world"},
					{ID: "tcp", Parent: "j2"},
				},
				Joints: []JointConfig{
					{ID: "j1", Type: RevoluteJoint, Parent: "base", Axis: spatial.AxisConfig{Z: 1}, Min: -360, Max: 360},
					tc.joint,
				},
			}
			_, err := cfg.ParseConfig("test")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, ErrMimicWithLimits.Error())
		})
	}
}

// A model whose shipped kinematics file declares the limits carries them across GetKinematics
// today, because the document itself is what crosses the wire. Modules that synthesize limits at
// runtime need the document helper as well, which is a separate change.
func TestJointLimitsCrossProtobuf(t *testing.T) {
	m, err := UnmarshalModelJSON(revoluteModelJSON(`, "max_velocity": 180, "max_acceleration": 1145`), "test")
	test.That(t, err, test.ShouldBeNil)

	resp := KinematicModelToProtobuf(m)
	rebuilt, err := KinematicModelFromProtobuf("test", resp)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, limitsAlmostEqual(rebuilt.DoF(), m.DoF(), defaultFloatPrecision), test.ShouldBeTrue)
	test.That(t, *rebuilt.DoF()[0].MaxVelocity, test.ShouldAlmostEqual, math.Pi, defaultFloatPrecision)
}

// Hash deliberately ignores the velocity fields. The only consumer is the smart seed cache,
// whose contents do not depend on them. This test pins that decision so a future change to
// Limit.Hash is a deliberate one.
func TestJointLimitsAreNotHashed(t *testing.T) {
	withVelocity, err := UnmarshalModelJSON(revoluteModelJSON(`, "max_velocity": 180`), "")
	test.That(t, err, test.ShouldBeNil)
	without, err := UnmarshalModelJSON(revoluteModelJSON(""), "")
	test.That(t, err, test.ShouldBeNil)

	test.That(t, withVelocity.Hash(), test.ShouldEqual, without.Hash())

	// but they are not "equal" for the purposes of comparing two frames
	test.That(t, limitsAlmostEqual(withVelocity.DoF(), without.DoF(), defaultFloatPrecision), test.ShouldBeFalse)
}
