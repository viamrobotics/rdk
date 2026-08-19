package referenceframe

import (
	"encoding/json"
	"fmt"
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
type JointLimits struct {
	Min             *float64
	Max             *float64
	MaxVelocity     *float64
	MaxAcceleration *float64
}

// SetJointLimits returns a copy of cfg with the given limits applied to the named joints, and
// with OriginalFile re-marshalled so that the new limits reach whoever asks the machine for its
// kinematics.
//
// Re-marshalling is the point of this function. KinematicModelToProtobuf does not serialize a
// model, it replays cfg.OriginalFile.Bytes, so limits written onto the in-memory frames alone
// would never leave the module process. This is the Go equivalent of what the UR module does by
// hand in to_sva_json.
//
// cfg is not modified. Model.ModelConfig hands out the live pointer rather than a copy, so a
// caller passing it straight in would otherwise find its own model rewritten underneath it.
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

	// Joints is the only field we write, so it is the only one that needs its own copy. Clearing
	// OriginalFile matters more than it looks: marshaling with it still set would nest the old
	// document inside the new one, and parsing that back would restore the pre-patch bytes.
	out := *cfg
	out.Joints = slices.Clone(cfg.Joints)
	out.OriginalFile = nil

	for id, limit := range limits {
		idx := slices.IndexFunc(out.Joints, func(j JointConfig) bool { return j.ID == id })
		if idx < 0 {
			return nil, fmt.Errorf("no joint %q in kinematics for model %q", id, cfg.Name)
		}

		joint := &out.Joints[idx]
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
	}

	raw, err := json.Marshal(&out)
	if err != nil {
		return nil, fmt.Errorf("marshaling patched kinematics for model %q: %w", cfg.Name, err)
	}
	out.OriginalFile = &ModelFile{Bytes: raw, Extension: "json"}

	return &out, nil
}
