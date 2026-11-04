package armplanning

import (
	"encoding/json"
	"os"

	"github.com/pkg/errors"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
)

// FrameSystemConfigEnvVar opts a viam-server process into writing plan requests as an
// frame system config instead of the legacy FrameSystem-dumped form. Readers accept both
// regardless of this flag.
//
// TODO: remove this flag once the frame system config form is verified against the live
// consumer graph and made the default.
const FrameSystemConfigEnvVar = "VIAM_PLAN_REQUEST_FRAME_SYSTEM_CONFIG"

func frameSystemConfigEnabled() bool {
	v := os.Getenv(FrameSystemConfigEnvVar)
	return v == "1" || v == "true"
}

type planRequestJSON struct {
	FrameSystem           *frameSystemJSON                  `json:"frame_system"`
	Goals                 []*PlanState                      `json:"goals,omitempty"`
	StartState            *PlanState                        `json:"start_state"`
	ObstaclesInWorldFrame *referenceframe.GeometriesInFrame `json:"obstacles_in_world_frame"`
	Constraints           *motionplan.Constraints           `json:"constraints"`
	PlannerOptions        *PlannerOptions                   `json:"planner_options"`
}

type frameSystemJSON struct {
	Name  string      `json:"name"`
	Parts []*partJSON `json:"parts"`
}

// partJSON keeps Model as a raw blob so unmarshal can dispatch through
// SimpleModel.UnmarshalJSON.
//
// TODO: moving JSON methods onto FrameSystemPart in referenceframe would let
// us drop partJSON and the encode/decode helpers here.
type partJSON struct {
	FrameConfig *referenceframe.LinkConfig `json:"frame_config"`
	Model       json.RawMessage            `json:"model,omitempty"`
}

// MarshalJSON emits the frame system config form when FrameSystemConfigEnvVar is set, otherwise
// the legacy form.
func (req *PlanRequest) MarshalJSON() ([]byte, error) {
	if !frameSystemConfigEnabled() {
		return req.marshalLegacy()
	}
	return req.marshalFrameSystemConfig()
}

// marshalLegacy emits the pre-change JSON shape: just PlanRequest's fields
// with their json tags. We can't call json.Marshal(req) directly here because
// that would call MarshalJSON again and recurse forever. The trick: define a
// new type Alias with PlanRequest's fields but none of its methods, cast req
// to *Alias, and marshal that. json.Marshal sees no MarshalJSON method on
// Alias and falls back to default struct-tag serialization.
//
// Preferred over the parallel-struct pattern in config/config.go's configData
// because Alias inherits PlanRequest's fields, so a new field can't silently
// drift out of the legacy wire form.
func (req *PlanRequest) marshalLegacy() ([]byte, error) {
	type Alias PlanRequest
	return json.Marshal((*Alias)(req))
}

func (req *PlanRequest) marshalFrameSystemConfig() ([]byte, error) {
	parts, err := req.FrameSystem.Parts()
	if err != nil {
		return nil, errors.Wrap(err, "extracting parts from FrameSystem")
	}
	encoded := make([]*partJSON, 0, len(parts))
	for _, p := range parts {
		enc, err := encodePart(p)
		if err != nil {
			return nil, errors.Wrapf(err, "encoding part %q", p.FrameConfig.Name())
		}
		encoded = append(encoded, enc)
	}
	return json.Marshal(planRequestJSON{
		FrameSystem: &frameSystemJSON{
			Name:  req.FrameSystem.Name(),
			Parts: encoded,
		},
		Goals:                 req.Goals,
		StartState:            req.StartState,
		ObstaclesInWorldFrame: req.ObstaclesInWorldFrame,
		Constraints:           req.Constraints,
		PlannerOptions:        req.PlannerOptions,
	})
}

// UnmarshalJSON reads either the frame system config form or the legacy form.
func (req *PlanRequest) UnmarshalJSON(data []byte) error {
	isConfig, err := isFrameSystemConfig(data)
	if err != nil {
		return err
	}
	if isConfig {
		return req.unmarshalFrameSystemConfig(data)
	}
	return req.unmarshalLegacy(data)
}

func isFrameSystemConfig(data []byte) (bool, error) {
	var probe struct {
		FrameSystem struct {
			Parts json.RawMessage `json:"parts"`
		} `json:"frame_system"`
	}
	// If this errors, the payload is malformed -- any later unmarshal would fail anyway.
	if err := json.Unmarshal(data, &probe); err != nil {
		return false, err
	}
	// nil means the "parts" key was absent. An empty array still counts: the
	// writer emits `[]` for a FrameSystem with no parts.
	return probe.FrameSystem.Parts != nil, nil
}

func (req *PlanRequest) unmarshalFrameSystemConfig(data []byte) error {
	var decoded planRequestJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded.FrameSystem == nil {
		return errors.New("PlanRequest has no frame_system")
	}
	parts := make([]*referenceframe.FrameSystemPart, 0, len(decoded.FrameSystem.Parts))
	for _, encodedPart := range decoded.FrameSystem.Parts {
		part, err := decodePart(encodedPart)
		if err != nil {
			return errors.Wrapf(err, "decoding part %q", encodedPart.FrameConfig.ID)
		}
		parts = append(parts, part)
	}
	fs, err := referenceframe.NewFrameSystem(decoded.FrameSystem.Name, parts, nil)
	if err != nil {
		return errors.Wrap(err, "rebuilding FrameSystem from parts")
	}
	req.FrameSystem = fs
	req.Goals = decoded.Goals
	req.StartState = decoded.StartState
	req.ObstaclesInWorldFrame = decoded.ObstaclesInWorldFrame
	req.Constraints = decoded.Constraints
	req.PlannerOptions = decoded.PlannerOptions
	return nil
}

// unmarshalLegacy uses the same Alias-type trick as marshalLegacy.
func (req *PlanRequest) unmarshalLegacy(data []byte) error {
	type Alias PlanRequest
	return json.Unmarshal(data, (*Alias)(req))
}

func encodePart(p *referenceframe.FrameSystemPart) (*partJSON, error) {
	// Part mount info is pose-only; ToStaticFrame wraps it as a Frame so
	// NewLinkConfig has something to extract pose and geometry from.
	frame, err := p.FrameConfig.ToStaticFrame("")
	if err != nil {
		return nil, err
	}
	cfg, err := referenceframe.NewLinkConfig(frame)
	if err != nil {
		return nil, err
	}
	// NewLinkConfig can't set Parent -- a Frame has no parent concept, that
	// lives on the FrameSystem. LinkInFrame does carry it, so copy it over.
	cfg.Parent = p.FrameConfig.Parent()
	encoded := &partJSON{FrameConfig: cfg}
	if p.ModelFrame != nil {
		raw, err := marshalModelCompact(p.ModelFrame)
		if err != nil {
			return nil, err
		}
		encoded.Model = raw
	}
	return encoded, nil
}

// marshalModelCompact emits a Model as just its ModelConfigJSON, dropping
// internal_fs (reconstructed by ParseConfig on the reader) and (for Viam JSON
// sources) original_file.bytes. A few code paths build a Model directly from
// a FrameSystem without ever parsing a kinematics file (NewModelWithLimitOverrides,
// NewModel) -- those Models have no ModelConfigJSON, so internal_fs is their
// only representation and we fall back to the full SimpleModel.MarshalJSON output.
func marshalModelCompact(m referenceframe.Model) ([]byte, error) {
	cfg := m.ModelConfig()
	if cfg == nil {
		return json.Marshal(m)
	}
	// Viam JSON round-trips losslessly, so dropping original_file.bytes is safe.
	// Other formats (URDF, XML) keep it because their parsers drop information.
	if cfg.OriginalFile != nil && cfg.OriginalFile.Extension == "json" {
		clone := *cfg
		clone.OriginalFile = nil
		cfg = &clone
	}
	return json.Marshal(struct {
		Name   string                          `json:"name"`
		Model  *referenceframe.ModelConfigJSON `json:"model"`
		Limits []referenceframe.Limit          `json:"limits"`
	}{
		Name:   m.Name(),
		Model:  cfg,
		Limits: m.DoF(),
	})
}

func decodePart(encoded *partJSON) (*referenceframe.FrameSystemPart, error) {
	link, err := encoded.FrameConfig.ParseConfig()
	if err != nil {
		return nil, err
	}
	part, err := referenceframe.LinkInFrameToFrameSystemPart(link)
	if err != nil {
		return nil, err
	}
	// Static Parts (floor, mounted sensor, ...) have no Model block.
	if len(encoded.Model) > 0 {
		m := referenceframe.NewSimpleModel(encoded.FrameConfig.ID)
		if err := json.Unmarshal(encoded.Model, m); err != nil {
			return nil, errors.Wrapf(err, "unmarshaling Model for %q", encoded.FrameConfig.ID)
		}
		part.ModelFrame = m
	}
	return part, nil
}
