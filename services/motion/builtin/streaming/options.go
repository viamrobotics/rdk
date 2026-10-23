// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"errors"
	"fmt"
	"math"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/services/motion"
)

const (
	defaultArmSideTargetRunwayMs = 100
	defaultSendToArmIntervalMs   = 10
	defaultVelLimitRadPerSec     = 10 * math.Pi / 180 // 10 deg/s
	defaultAccelLimitRadPerSec2  = 10 * math.Pi / 180 // 10 deg/s^2
	defaultDiagnosticsWindowSecs = 60

	maxTargetsDroppedInARowDueToTrajexExtendErrorKey = "max_targets_dropped_in_a_row_due_to_trajex_extend_error"
	forceRebaseIfExtendLatencyExceedsMsKey           = "force_rebase_if_extend_latency_exceeds_ms"
)

// StreamOptions tunes the streaming executor.
type StreamOptions struct {
	// ArmSideTargetRunwayMs is the duration of pvat points that we aim to keep
	// buffered inside the arm resource.
	ArmSideTargetRunwayMs int

	// SendToArmIntervalMs is the interval at which batches of pvat points are
	// sent to the arm resource.
	// TODO: Replace this with querying the arm's properties API.
	SendToArmIntervalMs int

	// MoveOptions carries the kinematic limits the trajex session is built with.
	MoveOptions arm.MoveOptions

	// DiagnosticsWindowSecs is how much full-detail diagnostics history the session retains;
	// 0 disables retention of that history, though whole-run diagnostic stats are still
	// collected regardless.
	DiagnosticsWindowSecs int

	// MaxTargetsDroppedInARowDueToTrajexExtendError bounds how many targets in a row trajex may reject before the
	// session gives up. The default, zero, ends the session on the first rejected target.
	//
	// This is a band-aid because trajex still returns errors where it could be made to accept the target.
	// It should eventually be removed.
	MaxTargetsDroppedInARowDueToTrajexExtendError int

	// ForceRebaseIfExtendLatencyExceedsMs is the Extend latency past which the session is forced to
	// stage. The default, zero, never forces a stage.
	//
	// This option can be removed once trajex extend latency no longer grows without bound with the number
	// of targets added to the trajex session.
	ForceRebaseIfExtendLatencyExceedsMs int
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
	validatePositive := func(perJoint []float64, scalar float64, perJointName, scalarName string) error {
		if len(perJoint) == 0 {
			if scalar <= 0 {
				return fmt.Errorf("streaming: move_options.%s must be positive", scalarName)
			}
			return nil
		}
		for _, v := range perJoint {
			if v <= 0 {
				return fmt.Errorf("streaming: move_options.%s entries must all be positive", perJointName)
			}
		}
		return nil
	}
	if err := validatePositive(
		o.MoveOptions.MaxVelRadsJoints, o.MoveOptions.MaxVelRads, "max_vel_degs_per_sec_joints", "max_vel_degs_per_sec",
	); err != nil {
		return err
	}
	if err := validatePositive(
		o.MoveOptions.MaxAccRadsJoints, o.MoveOptions.MaxAccRads, "max_acc_degs_per_sec2_joints", "max_acc_degs_per_sec2",
	); err != nil {
		return err
	}
	if o.MoveOptions.MaxTCPSpeedMPerSec != nil && *o.MoveOptions.MaxTCPSpeedMPerSec <= 0 {
		return errors.New("streaming: move_options.max_tcp_speed must be positive")
	}
	if o.DiagnosticsWindowSecs < 0 {
		return errors.New("streaming: diagnostics_window_secs must be non-negative (0 disables window-detail retention)")
	}
	if o.MaxTargetsDroppedInARowDueToTrajexExtendError < 0 {
		return errors.New("streaming: max_targets_dropped_in_a_row_due_to_trajex_extend_error must be non-negative")
	}
	if o.ForceRebaseIfExtendLatencyExceedsMs < 0 {
		return errors.New("streaming: force_rebase_if_extend_latency_exceeds_ms must be non-negative")
	}
	return nil
}

// MaxTargetsDroppedInARowDueToTrajexExtendError reads maxTargetsDroppedInARowDueToTrajexExtendErrorKey from a
// TempStreamArmJointPositions extra map, returning zero (the default) if it is absent.
func MaxTargetsDroppedInARowDueToTrajexExtendError(extra map[string]interface{}) (int, error) {
	return int32FromExtra(extra, maxTargetsDroppedInARowDueToTrajexExtendErrorKey)
}

// ForceRebaseIfExtendLatencyExceedsMs reads forceRebaseIfExtendLatencyExceedsMsKey from a
// TempStreamArmJointPositions extra map, returning zero (the default) if it is absent.
func ForceRebaseIfExtendLatencyExceedsMs(extra map[string]interface{}) (int, error) {
	return int32FromExtra(extra, forceRebaseIfExtendLatencyExceedsMsKey)
}

func int32FromExtra(extra map[string]interface{}, key string) (int, error) {
	v, ok := extra[key]
	if !ok {
		return 0, nil
	}
	switch n := v.(type) {
	case int32:
		return int(n), nil
	case float64:
		if n != math.Trunc(n) || n < math.MinInt32 || n > math.MaxInt32 {
			return 0, fmt.Errorf("streaming: extra %q must be a whole number that fits in an int32, got %v", key, n)
		}
		return int(n), nil
	default:
		return 0, fmt.Errorf("streaming: extra %q must be an int32 or a float64, got %T", key, v)
	}
}

// NewStreamOptions returns a StreamOptions containing the configuration values set in opts, and
// the defaults for any it leaves unset.
func NewStreamOptions(opts motion.TempStreamOptions) StreamOptions {
	o := StreamOptions{
		ArmSideTargetRunwayMs: defaultArmSideTargetRunwayMs,
		SendToArmIntervalMs:   defaultSendToArmIntervalMs,
		MoveOptions: arm.MoveOptions{
			MaxVelRads: defaultVelLimitRadPerSec,
			MaxAccRads: defaultAccelLimitRadPerSec2,
		},
		DiagnosticsWindowSecs: defaultDiagnosticsWindowSecs,
	}
	if opts.ArmSideTargetRunwayMs != nil {
		o.ArmSideTargetRunwayMs = int(*opts.ArmSideTargetRunwayMs)
	}
	if opts.SendToArmIntervalMs != nil {
		o.SendToArmIntervalMs = int(*opts.SendToArmIntervalMs)
	}
	if opts.DiagnosticsWindowSecs != nil {
		o.DiagnosticsWindowSecs = int(*opts.DiagnosticsWindowSecs)
	}
	if move := opts.MoveOptions; move != nil {
		if move.MaxVelRads > 0 {
			o.MoveOptions.MaxVelRads = move.MaxVelRads
		}
		if move.MaxAccRads > 0 {
			o.MoveOptions.MaxAccRads = move.MaxAccRads
		}
		o.MoveOptions.MaxVelRadsJoints = move.MaxVelRadsJoints
		o.MoveOptions.MaxAccRadsJoints = move.MaxAccRadsJoints
		o.MoveOptions.MaxTCPSpeedMPerSec = move.MaxTCPSpeedMPerSec
	}
	return o
}
