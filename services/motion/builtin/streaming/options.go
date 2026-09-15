// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"errors"

	"github.com/go-viper/mapstructure/v2"

	"go.viam.com/rdk/referenceframe"
)

const (
	defaultTargetRunwayInArmMs = 100
	defaultSendToArmIntervalMs = 10
	defaultDiagnosticsWindowMs = 60_000
)

// JointPositionsChItem is one joint-space waypoint.
type JointPositionsChItem struct {
	// Positions are the target joint positions for this waypoint.
	Positions []referenceframe.Input
}

// StreamOptions tunes the streaming executor.
type StreamOptions struct {
	// TargetRunwayInArmMs is the duration of pvat points that we aim to keep
	// buffered inside the arm resource.
	TargetRunwayInArmMs int `json:"target_runway_in_arm_ms"`

	// SendToArmIntervalMs is the interval at which batches of pvat points are
	// sent to the arm resource.
	// TODO: Replace this with querying the arm's properties API.
	SendToArmIntervalMs int `json:"send_to_arm_interval_ms"`

	// VelLimitDegPerSec / AccelLimitDegPerSec2 are per-joint limits the trajex session is built
	// with, applied uniformly to every joint. 0 (the default) means unset: Run falls back to the
	// arm's own kinematics-declared per-joint limits instead of a fixed value.
	VelLimitDegPerSec    float64 `json:"vel_limit_deg_per_sec"`
	AccelLimitDegPerSec2 float64 `json:"accel_limit_deg_per_sec2"`

	// DiagnosticsWindowMs is how much full-detail diagnostics history the session retains;
	// 0 disables diagnostics for the session.
	DiagnosticsWindowMs int `json:"diagnostics_window_ms"`
}

// Validate returns an error if any StreamOptions field is invalid.
func (o *StreamOptions) Validate() error {
	if o.TargetRunwayInArmMs <= 0 {
		return errors.New("streaming: target_runway_in_arm_ms must be positive")
	}
	if o.SendToArmIntervalMs <= 0 {
		return errors.New("streaming: send_to_arm_interval_ms must be positive")
	}
	if o.SendToArmIntervalMs >= o.TargetRunwayInArmMs {
		return errors.New("streaming: send_to_arm_interval_ms must be less than target_runway_in_arm_ms")
	}
	if o.VelLimitDegPerSec < 0 {
		return errors.New("streaming: vel_limit_deg_per_sec cannot be negative (0 falls back to the arm's kinematics)")
	}
	if o.AccelLimitDegPerSec2 < 0 {
		return errors.New("streaming: accel_limit_deg_per_sec2 cannot be negative (0 falls back to the arm's kinematics)")
	}
	if o.DiagnosticsWindowMs < 0 {
		return errors.New("streaming: diagnostics_window_ms must be non-negative (0 disables diagnostics)")
	}
	return nil
}

// NewDefaultOptions returns StreamOptions with every field set to its default.
// Callers overriding individual fields should start from this and then set them.
func NewDefaultOptions() StreamOptions {
	return StreamOptions{
		TargetRunwayInArmMs: defaultTargetRunwayInArmMs,
		SendToArmIntervalMs: defaultSendToArmIntervalMs,
		DiagnosticsWindowMs: defaultDiagnosticsWindowMs,
	}
}

// ParseStreamOptions decodes raw (e.g. a map[string]interface{} parsed from JSON) into opts,
// matching fields by their json tag.
func ParseStreamOptions(raw interface{}, opts *StreamOptions) error {
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:          "json",
		WeaklyTypedInput: true,
		Result:           opts,
	})
	if err != nil {
		return err
	}
	return dec.Decode(raw)
}
