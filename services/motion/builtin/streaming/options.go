package streaming

import (
	"errors"

	"github.com/go-viper/mapstructure/v2"

	"go.viam.com/rdk/referenceframe"
)

const (
	defaultTargetRunwayInArmMs  = 100
	defaultSendToArmIntervalMs  = 10
	defaultVelLimitDegPerSec    = 10.0
	defaultAccelLimitDegPerSec2 = 10.0
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

	// VelLimitDegPerSec / AccelLimitDegPerSec2 are the per-joint limits the trajex
	// session is built with.
	// TODO: Replace these with querying the arm's properties API.
	VelLimitDegPerSec    float64 `json:"vel_limit_deg_per_sec"`
	AccelLimitDegPerSec2 float64 `json:"accel_limit_deg_per_sec2"`
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
	if o.VelLimitDegPerSec <= 0 {
		return errors.New("streaming: vel_limit_deg_per_sec must be positive")
	}
	if o.AccelLimitDegPerSec2 <= 0 {
		return errors.New("streaming: accel_limit_deg_per_sec2 must be positive")
	}
	return nil
}

// NewDefaultOptions returns StreamOptions with every field set to its default.
// Callers overriding individual fields should start from this and then set them.
func NewDefaultOptions() StreamOptions {
	return StreamOptions{
		TargetRunwayInArmMs:  defaultTargetRunwayInArmMs,
		SendToArmIntervalMs:  defaultSendToArmIntervalMs,
		VelLimitDegPerSec:    defaultVelLimitDegPerSec,
		AccelLimitDegPerSec2: defaultAccelLimitDegPerSec2,
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
