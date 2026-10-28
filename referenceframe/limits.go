package referenceframe

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"

	"go.viam.com/rdk/utils"
)

// ErrURDFLimitsUnsupported is returned when limits are set on a URDF-backed kinematics document.
var ErrURDFLimitsUnsupported = fmt.Errorf(
	"cannot set joint limits on a URDF document; see RSDK-14232",
)

// JointLimits is a partial update to one joint's limits, and the attribute shape a module accepts
// for them in its resource config. A nil field leaves that limit as it is, so a caller who knows
// only how fast a joint may move can say that without restating where it may go.
//
// Values are in the units the kinematics document uses, not the internal ones: degrees and
// degrees per second for revolute joints, millimeters and millimeters per second for prismatic.
// A module reading speeds out of its own config is already holding degrees, which is why this
// takes them.
//
// Every value must be finite, the speeds must not be negative, and a joint's min must not end up
// above its max. Leaving a field out is how you say a joint is unbounded on that axis.
type JointLimits struct {
	Min             *float64 `json:"min,omitempty"`
	Max             *float64 `json:"max,omitempty"`
	MaxVelocity     *float64 `json:"max_velocity,omitempty"`
	MaxAcceleration *float64 `json:"max_acceleration,omitempty"`
}

// validate rejects values that would produce a document nobody can act on. The caller is usually
// passing numbers straight out of a user's resource config, so this is the last place to catch a
// typo before it becomes a limit the motion service plans against.
func (jl JointLimits) validate(id string) error {
	for _, field := range []struct {
		name  string
		value *float64
	}{
		{"min", jl.Min},
		{"max", jl.Max},
		{"max_velocity", jl.MaxVelocity},
		{"max_acceleration", jl.MaxAcceleration},
	} {
		if field.value == nil {
			continue
		}
		// Infinity would mean two different things depending on the field, dropping out of the
		// document for the speeds and failing to marshal at all for the positions, so neither is
		// allowed. Leave a field out to say a joint is unbounded.
		if math.IsNaN(*field.value) || math.IsInf(*field.value, 0) {
			return fmt.Errorf("joint %q: %s must be a finite number, got %v", id, field.name, *field.value)
		}
	}

	if jl.MaxVelocity != nil && *jl.MaxVelocity < 0 {
		return fmt.Errorf("joint %q: max_velocity cannot be negative, got %v", id, *jl.MaxVelocity)
	}
	if jl.MaxAcceleration != nil && *jl.MaxAcceleration < 0 {
		return fmt.Errorf("joint %q: max_acceleration cannot be negative, got %v", id, *jl.MaxAcceleration)
	}

	return nil
}

// SetJointLimits applies the given limits to the named joints and returns the result, with
// OriginalFile re-marshalled so that the new limits reach whoever asks the machine for its
// kinematics.
//
// Re-marshalling is the point of this function. KinematicModelToProtobuf does not serialize a
// model, it replays cfg.OriginalFile.Bytes, so limits written onto the in-memory frames alone
// would never leave the module process. This is the Go equivalent of what the UR module does by
// hand in to_sva_json.
//
// cfg is not modified by this call. Model.ModelConfig hands out the live pointer rather than a
// copy, so a caller passing it straight in would otherwise find its own model rewritten
// underneath it. The result is not a deep copy though: it shares the Links slice, and each
// joint's Geometry and Mimic, with cfg. Treat those as read-only through either handle.
//
// Re-marshalling also normalizes the document's key casing, because the axis and geometry types
// carry no JSON tags: an axis authored as {"x":0,"y":0,"z":1} comes back as {"X":0,"Y":0,"Z":1}.
// Go reads either, since encoding/json matches field names case-insensitively, but a consumer
// parsing case-sensitively sees a different shape than the module author wrote.
//
// It returns an error for a URDF-backed document. Go could convert one to SVA, but the C++ and
// Python SDKs hold kinematics as opaque bytes and cannot, and a helper that quietly does more in
// one language than the others is worse than one that refuses everywhere. Declaring limits in a
// URDF is RSDK-14232.
//
// Deprecated: narrow the model itself with SimpleModel.NarrowJointLimits. It works on a model
// read from a URDF too, and what it narrows reaches the typed model and the file bytes alike.
func SetJointLimits(cfg *ModelConfigJSON, limits map[string]JointLimits) (*ModelConfigJSON, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cannot set joint limits on a nil model config")
	}
	if cfg.OriginalFile != nil && cfg.OriginalFile.Extension == "urdf" {
		return nil, ErrURDFLimitsUnsupported
	}
	if cfg.KinParamType == "DH" {
		return nil, fmt.Errorf(
			"cannot set joint limits on model %q: DH kinematics describe joints as parameters rather "+
				"than as joints with limits", cfg.Name,
		)
	}

	// Joints is the only field we write, so it is the only one that needs its own copy. Clearing
	// OriginalFile matters more than it looks: marshaling with it still set would nest the old
	// document inside the new one, and parsing that back would restore the pre-patch bytes.
	out := *cfg
	out.Joints = slices.Clone(cfg.Joints)
	out.OriginalFile = nil

	for id, limit := range limits {
		if err := limit.validate(id); err != nil {
			return nil, err
		}

		idx := slices.IndexFunc(out.Joints, func(j JointConfig) bool { return j.ID == id })
		if idx < 0 {
			return nil, fmt.Errorf("no joint %q in kinematics for model %q", id, cfg.Name)
		}

		joint := &out.Joints[idx]

		// A mimic joint takes its limits from whatever drives it, and parsing rejects one that
		// declares its own. Without this check we would emit a document that no consumer can
		// read back, which is a worse failure than refusing here.
		if joint.Mimic != nil {
			return nil, fmt.Errorf("%w: joint %q", ErrMimicWithLimits, id)
		}

		limit.applyTo(joint)

		// Min and Max move independently so that a caller can pin a joint without restating the
		// other side, which means the pair can be left crossed. A joint with an empty range is
		// not something a planner can work with, and randomFrameInputs would hand back positions
		// outside both bounds rather than fail.
		if joint.Min > joint.Max {
			return nil, fmt.Errorf("joint %q: min %v is greater than max %v after applying limits",
				id, joint.Min, joint.Max)
		}
	}

	raw, err := json.Marshal(&out)
	if err != nil {
		return nil, fmt.Errorf("marshaling patched kinematics for model %q: %w", cfg.Name, err)
	}

	// These bytes are the whole product: they are what GetKinematics sends and what every
	// consumer reads. Parsing them here turns any way of emitting a broken document into an
	// error the module author sees, rather than one the far end of the connection discovers.
	if _, err := UnmarshalModelJSON(raw, cfg.Name); err != nil {
		return nil, fmt.Errorf("patched kinematics for model %q no longer parses: %w", cfg.Name, err)
	}
	out.OriginalFile = &ModelFile{Bytes: raw, Extension: "json"}

	return &out, nil
}

// applyTo writes the fields that are set onto a joint configuration, which takes the same units.
func (jl JointLimits) applyTo(joint *JointConfig) {
	if jl.Min != nil {
		joint.Min = *jl.Min
	}
	if jl.Max != nil {
		joint.Max = *jl.Max
	}
	if jl.MaxVelocity != nil {
		joint.MaxVelocity = limitPtr(jl.MaxVelocity, nil)
	}
	if jl.MaxAcceleration != nil {
		joint.MaxAcceleration = limitPtr(jl.MaxAcceleration, nil)
	}
}

// NarrowJointLimits narrows the limits the model serves to what a resource's configuration asks
// of each joint, keyed by joint id, in document units: degrees, degrees per second and degrees per
// second squared for a revolute joint, millimeters for a prismatic one. The attribute shape a
// module is expected to accept decodes straight into the argument:
//
//	"joint_limits": {
//	  "joint_2": {"min": -31, "max": -29},
//	  "joint_3": {"max_velocity": 30, "max_acceleration": 380}
//	}
//
// Position, velocity and acceleration are checked on their own, and each value has to sit inside
// what the model already holds. A value that would widen a limit is an error naming the joint,
// since a widened limit is the one that reaches hardware. A joint the model has no velocity or
// acceleration bound for accepts any. A min equal to max locks a joint where it stands. A mimic
// joint follows the joint it mimics and cannot be narrowed, and a DH model keeps its limits as
// parameters and is refused. Nothing changes on an error, so a module can fail its constructor
// with the error as it is, and a request that sets nothing changes nothing.
//
// Call it while building the resource, before the model is shared, since it writes the frames in
// place with no lock. The narrowed limits reach the typed model through the frames. The deprecated
// file bytes carry them too when the configuration can be written back as a file, and otherwise
// the file stays as it was read, which is what happens for a joint with no position bound, since
// JSON has no way to write one.
func (m *SimpleModel) NarrowJointLimits(limits map[string]JointLimits) error {
	if m.modelConfig != nil && m.modelConfig.KinParamType == "DH" {
		return fmt.Errorf("cannot narrow joint limits on model %q: DH kinematics describe joints as parameters "+
			"rather than as joints with limits", m.name)
	}
	type narrowedJoint struct {
		frame Frame
		limit Limit
	}
	narrowed := make(map[string]narrowedJoint, len(limits))
	// ids in order, so the joint an error names does not change from one run to the next
	for _, id := range slices.Sorted(maps.Keys(limits)) {
		jl := limits[id]
		if err := jl.validate(id); err != nil {
			return err
		}
		if _, mimic := m.mimicMappings[id]; mimic {
			return fmt.Errorf("%w: joint %q", ErrMimicWithLimits, id)
		}
		frame := m.internalFS.Frame(id)
		if frame == nil || len(frame.DoF()) == 0 {
			return fmt.Errorf("no joint %q in model %q", id, m.name)
		}
		// only a joint frame holds the one limit this writes back through DoF, a nested model
		// answers DoF from a copy
		var rotational bool
		switch unwrapNamed(frame).(type) {
		case *rotationalFrame:
			rotational = true
		case *translationalFrame:
		default:
			return fmt.Errorf("joint %q in model %q is not a single joint", id, m.name)
		}
		if jl == (JointLimits{}) {
			continue
		}
		limit, err := narrowLimit(id, frame.DoF()[0], jl, rotational)
		if err != nil {
			return err
		}
		narrowed[id] = narrowedJoint{frame: frame, limit: limit}
	}
	if len(narrowed) == 0 {
		return nil
	}

	for _, joint := range narrowed {
		joint.frame.DoF()[0] = joint.limit
	}
	// DoF answers from a copy of the frames' limits taken when the model was built
	if m.inputSchema != nil {
		m.limits = m.inputSchema.GetLimits()
	}
	if m.modelConfig == nil {
		return nil
	}
	for i := range m.modelConfig.Joints {
		if jl, ok := limits[m.modelConfig.Joints[i].ID]; ok {
			jl.applyTo(&m.modelConfig.Joints[i])
		}
	}

	// the file the model was read from no longer says what the model does, so the configuration
	// is written out in its place. Marshaling with the old file still set would nest it inside
	// the new one. Bytes that fail to marshal, which a joint with no position bound does, or that
	// fail to read back leave the file as it was.
	original := m.modelConfig.OriginalFile
	m.modelConfig.OriginalFile = nil
	if raw, err := json.Marshal(m.modelConfig); err == nil {
		if _, err := UnmarshalModelJSON(raw, m.modelConfig.Name); err == nil {
			m.modelConfig.OriginalFile = &ModelFile{Bytes: raw, Extension: "json"}
			return nil
		}
	}
	m.modelConfig.OriginalFile = original
	return nil
}

// narrowLimit applies a request in document units to the limit a frame enforces, in the frame's
// own units, refusing anything that would widen it.
func narrowLimit(id string, current Limit, jl JointLimits, rotational bool) (Limit, error) {
	convert := func(v float64) float64 { return v }
	report := convert
	if rotational {
		convert = utils.DegToRad
		// the model's bound goes back to degrees for the message, rounded so a value that went
		// through radians does not read as 10.999999999999998
		report = func(v float64) float64 { return math.Round(utils.RadToDeg(v)*1e6) / 1e6 }
	}
	// a bound equal to the model's own passes, with a hair of slack for the degree to radian
	// round trip
	const slack = 1e-9

	merged := current
	if jl.Min != nil {
		v := convert(*jl.Min)
		if v < current.Min-slack {
			return Limit{}, fmt.Errorf("joint %q: min %v is below the model's min %v", id, *jl.Min, report(current.Min))
		}
		merged.Min = v
	}
	if jl.Max != nil {
		v := convert(*jl.Max)
		if v > current.Max+slack {
			return Limit{}, fmt.Errorf("joint %q: max %v is above the model's max %v", id, *jl.Max, report(current.Max))
		}
		merged.Max = v
	}
	if merged.Min > merged.Max {
		return Limit{}, fmt.Errorf("joint %q: min %v is above max %v", id, report(merged.Min), report(merged.Max))
	}
	if jl.MaxVelocity != nil {
		v := convert(*jl.MaxVelocity)
		if current.MaxVelocity != nil && v > *current.MaxVelocity+slack {
			return Limit{}, fmt.Errorf("joint %q: max_velocity %v is above the model's %v",
				id, *jl.MaxVelocity, report(*current.MaxVelocity))
		}
		merged.MaxVelocity = &v
	}
	if jl.MaxAcceleration != nil {
		v := convert(*jl.MaxAcceleration)
		if current.MaxAcceleration != nil && v > *current.MaxAcceleration+slack {
			return Limit{}, fmt.Errorf("joint %q: max_acceleration %v is above the model's %v",
				id, *jl.MaxAcceleration, report(*current.MaxAcceleration))
		}
		merged.MaxAcceleration = &v
	}
	return merged, nil
}
