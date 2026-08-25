package referenceframe

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
)

// ErrURDFLimitsUnsupported is returned when limits are set on a URDF-backed kinematics document.
var ErrURDFLimitsUnsupported = fmt.Errorf(
	"cannot set joint limits on a URDF document; see RSDK-14232")

// JointLimits is a partial update to one joint's limits. A nil field leaves that limit as it is,
// so a caller who knows only how fast a joint may move can say that without restating where it
// may go.
//
// Values are in the units the kinematics document uses, not the internal ones: degrees and
// degrees per second for revolute joints, millimeters and millimeters per second for prismatic.
// A module reading speeds out of its own config is already holding degrees, which is why this
// takes them.
//
// Every value must be finite, the speeds must not be negative, and a joint's min must not end up
// above its max. Leaving a field out is how you say a joint is unbounded on that axis.
type JointLimits struct {
	Min             *float64
	Max             *float64
	MaxVelocity     *float64
	MaxAcceleration *float64
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
				"than as joints with limits", cfg.Name)
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

		if limit.Min != nil {
			joint.Min = *limit.Min
		}
		if limit.Max != nil {
			joint.Max = *limit.Max
		}
		if limit.MaxVelocity != nil {
			joint.MaxVelocity = limitPtr(limit.MaxVelocity, nil)
		}
		if limit.MaxAcceleration != nil {
			joint.MaxAcceleration = limitPtr(limit.MaxAcceleration, nil)
		}

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
