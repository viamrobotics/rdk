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
	if o.MoveOptions.MaxTCPSpeedMPerSec != nil {
		return errors.New("streaming: move_options.max_tcp_speed is not currently supported for arm streaming")
	}
	if o.DiagnosticsWindowSecs < 0 {
		return errors.New("streaming: diagnostics_window_secs must be non-negative (0 disables window-detail retention)")
	}
	return nil
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
