// Package streaming executes a live, continuously-updated stream of joint-space
// targets on an arm. It smooths the targets through the trajex online trajectory
// generator (OTG) and flow-controls a bidirectional gRPC stream to the arm via
// arm.MoveThroughJointPositionsStreamed.
//
// The pipeline is a producer/consumer pair over a shared channel of sampled
// PVATs (position/velocity/acceleration/time):
//   - the producer feeds each incoming target into a trajex session (Extend) and
//     samples PVATs out of it one at a time;
//   - the consumer batches PVATs into arm.TrajectoryPoint slices and sends them,
//     keeping the arm buffered a configurable window ahead of real time.
//
// A target marked Flush ends the current trajex session and arm stream and starts
// a fresh one, for segment boundaries where the arm may briefly come to rest.
package streaming

import (
	"context"
	"errors"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
)

const (
	defaultBufferAheadInArmMs   = 100
	defaultSendToArmIntervalMs  = 10
	defaultVelLimitDegPerSec    = 10.0
	defaultAccelLimitDegPerSec2 = 10.0
)

// Target is one live joint-space waypoint fed to the streaming executor.
type Target struct {
	// Positions are the target joint positions for this waypoint.
	Positions []referenceframe.Input

	// Flush ends the current trajex session and arm stream at this point and
	// starts a fresh one. Use it at segment boundaries where the arm may briefly
	// stop (e.g. between sanding strokes).
	Flush bool
}

// Config tunes the streaming executor.
type Config struct {
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

// Validate returns an error if any Config field is invalid.
func (c *Config) Validate() error {
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

// ApplyDefaults fills any zero-valued Config field with its default.
func (c *Config) ApplyDefaults() {
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

// StreamJointTargets executes a live stream of joint-space targets on arm,
// smoothing them through a trajex session and flow-controlling a bidirectional
// stream. It seeds the first trajex session at seed (typically the arm's current
// joint positions).
//
// Return contract:
//   - nil once targets is closed and every sampled PVAT has been drained to the
//     arm and acknowledged;
//   - the first non-context error from either the producer or consumer otherwise.
//
// StreamJointTargets applies defaults to and validates a copy of cfg; the caller's
// cfg is not modified.
func StreamJointTargets(
	ctx context.Context,
	a arm.Arm,
	seed []referenceframe.Input,
	targets <-chan Target,
	cfg Config,
) error {
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}
	return newCoordinator(a, &cfg).run(ctx, targets, seed).wait()
}
