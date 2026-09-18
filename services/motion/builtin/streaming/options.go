// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"errors"

	"github.com/go-viper/mapstructure/v2"

	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/utils"
)

const (
	defaultArmSideTargetRunwayMs = 100
	defaultSendToArmIntervalMs   = 10
	defaultVelLimitDegPerSec     = 10.0
	defaultAccelLimitDegPerSec2  = 10.0
	defaultDiagnosticsWindowSecs = 60
)

// StreamOptions tunes the streaming executor.
type StreamOptions struct {
	// ArmSideTargetRunwayMs is the duration of pvat points that we aim to keep
	// buffered inside the arm resource.
	ArmSideTargetRunwayMs int `json:"arm_side_target_runway_ms"`

	// SendToArmIntervalMs is the interval at which batches of pvat points are
	// sent to the arm resource.
	// TODO: Replace this with querying the arm's properties API.
	SendToArmIntervalMs int `json:"send_to_arm_interval_ms"`

	// VelLimitDegPerSec / AccelLimitDegPerSec2 are the per-joint limits the trajex
	// session is built with.
	// TODO: Replace these with querying the arm's properties API.
	VelLimitDegPerSec    float64 `json:"vel_limit_deg_per_sec"`
	AccelLimitDegPerSec2 float64 `json:"accel_limit_deg_per_sec2"`

	// DiagnosticsWindowSecs is how much full-detail diagnostics history the session retains;
	// 0 disables retention of that history, though whole-run diagnostic stats are still
	// collected regardless.
	DiagnosticsWindowSecs int `json:"diagnostics_window_secs"`
}

// Validate returns an error if any StreamOptions field is invalid.
func (o *StreamOptions) Validate() error {
	if o.ArmSideTargetRunwayMs <= 0 {
		return errors.New("streaming: arm_side_target_runway_ms must be positive")
	}
	if o.SendToArmIntervalMs <= 0 {
		return errors.New("streaming: send_to_arm_interval_ms must be positive")
	}
	if o.SendToArmIntervalMs >= o.ArmSideTargetRunwayMs {
		return errors.New("streaming: send_to_arm_interval_ms must be less than arm_side_target_runway_ms")
	}
	if o.VelLimitDegPerSec <= 0 {
		return errors.New("streaming: vel_limit_deg_per_sec must be positive")
	}
	if o.AccelLimitDegPerSec2 <= 0 {
		return errors.New("streaming: accel_limit_deg_per_sec2 must be positive")
	}
	if o.DiagnosticsWindowSecs < 0 {
		return errors.New("streaming: diagnostics_window_secs must be non-negative (0 disables window-detail retention)")
	}
	return nil
}

// From populates o from opts, starting at NewDefaultOptions() and overriding whichever fields
// opts sets; a nil field selects the default instead.
func (o *StreamOptions) From(opts motion.StreamOptions) {
	*o = NewDefaultOptions()
	if opts.ArmSideTargetRunwayMs != nil {
		o.ArmSideTargetRunwayMs = int(*opts.ArmSideTargetRunwayMs)
	}
	if opts.SendToArmIntervalMs != nil {
		o.SendToArmIntervalMs = int(*opts.SendToArmIntervalMs)
	}
	if opts.DiagnosticsWindowSecs != nil {
		o.DiagnosticsWindowSecs = int(*opts.DiagnosticsWindowSecs)
	}
	// Per-joint limits (MoveOptions.MaxVelRadsJoints/MaxAccRadsJoints) aren't supported yet;
	// only the uniform scalar limits are applied.
	if opts.MoveOptions != nil {
		if opts.MoveOptions.MaxVelRads > 0 {
			o.VelLimitDegPerSec = utils.RadToDeg(opts.MoveOptions.MaxVelRads)
		}
		if opts.MoveOptions.MaxAccRads > 0 {
			o.AccelLimitDegPerSec2 = utils.RadToDeg(opts.MoveOptions.MaxAccRads)
		}
	}
}

// NewDefaultOptions returns StreamOptions with every field set to its default.
// Callers overriding individual fields should start from this and then set them.
func NewDefaultOptions() StreamOptions {
	return StreamOptions{
		ArmSideTargetRunwayMs: defaultArmSideTargetRunwayMs,
		SendToArmIntervalMs:   defaultSendToArmIntervalMs,
		VelLimitDegPerSec:     defaultVelLimitDegPerSec,
		AccelLimitDegPerSec2:  defaultAccelLimitDegPerSec2,
		DiagnosticsWindowSecs: defaultDiagnosticsWindowSecs,
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
