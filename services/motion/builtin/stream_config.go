package builtin

import (
	"errors"

	"github.com/go-viper/mapstructure/v2"
)

const (
	defaultBufferAheadInArmMs   = 100
	defaultSendToArmIntervalMs  = 10
	defaultVelLimitDegPerSec    = 10.0
	defaultAccelLimitDegPerSec2 = 10.0
)

// streamConfig tunes the streaming executor.
type streamConfig struct {
	// BufferAheadInArmMs is the lookahead window (ms) of points sampled out of
	// trajex that we aim to keep buffered inside the arm resource.
	BufferAheadInArmMs int `json:"buffer_ahead_in_arm_ms"`

	// SendToArmIntervalMs is the interval (ms) at which points are sampled out of
	// trajex and sent to the arm to top the arm's buffer back up to
	// BufferAheadInArmMs.
	// TODO: Replace this with querying the arm's properties API.
	SendToArmIntervalMs int `json:"send_to_arm_interval_ms"`

	// VelLimitDegPerSec / AccelLimitDegPerSec2 are the per-joint limits the trajex
	// session is built with.
	// TODO: Replace these with querying the arm's properties API.
	VelLimitDegPerSec    float64 `json:"vel_limit_deg_per_sec"`
	AccelLimitDegPerSec2 float64 `json:"accel_limit_deg_per_sec2"`
}

// Validate returns an error if any streamConfig field is invalid.
func (c *streamConfig) Validate() error {
	if c.BufferAheadInArmMs < 0 {
		return errors.New("streaming: buffer_ahead_in_arm_ms must be non-negative")
	}
	if c.SendToArmIntervalMs < 0 {
		return errors.New("streaming: send_to_arm_interval_ms must be non-negative")
	}
	if c.SendToArmIntervalMs == 0 {
		return errors.New("streaming: send_to_arm_interval_ms must be positive")
	}
	if c.VelLimitDegPerSec < 0 {
		return errors.New("streaming: vel_limit_deg_per_sec must be non-negative")
	}
	if c.AccelLimitDegPerSec2 < 0 {
		return errors.New("streaming: accel_limit_deg_per_sec2 must be non-negative")
	}
	return nil
}

// ApplyDefaults fills any zero-valued streamConfig field with its default.
func (c *streamConfig) ApplyDefaults() {
	if c.BufferAheadInArmMs == 0 {
		c.BufferAheadInArmMs = defaultBufferAheadInArmMs
	}
	if c.SendToArmIntervalMs == 0 {
		c.SendToArmIntervalMs = defaultSendToArmIntervalMs
	}
	if c.VelLimitDegPerSec == 0 {
		c.VelLimitDegPerSec = defaultVelLimitDegPerSec
	}
	if c.AccelLimitDegPerSec2 == 0 {
		c.AccelLimitDegPerSec2 = defaultAccelLimitDegPerSec2
	}
}

func decodeStreamConfig(raw interface{}, cfg *streamConfig) error {
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:          "json",
		WeaklyTypedInput: true,
		Result:           cfg,
	})
	if err != nil {
		return err
	}
	return dec.Decode(raw)
}
