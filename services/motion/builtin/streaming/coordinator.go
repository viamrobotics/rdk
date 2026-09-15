//go:build !windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt

package streaming

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/multierr"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
	"go.viam.com/rdk/utils"
)

// Run executes a streaming session through one trajex session and one arm stream RPC.
// If jpCh is closed, it samples everything out of the trajex session and sends it to the
// arm, then waits for the arm to have finished executing before returning.
//
// --- An important note on backpressure --
//
// The arm provides backpressure to sampling out of trajex in that `Run` maintains an
// estimate of how much runway the arm has buffered on its side, and only samples out of
// trajex enough to keep that runway topped up to the user-configured TargetRunwayInArmMs.
//
// Trajex, however, does not provide any backpressure to the client: If the client sends
// joint positions faster than the arm executes them as per the trajectory output by trajex,
// trajectory simply accumulates inside the trajex session.
// Note that if, on the other hand, the client sends joint positions *slower* than the arm
// executes them (as per the trajectory output by trajex), `Run` will run out of pvat points
// to send to the arm, and the arm will (typically, depending on the arm implementation) fault.
func Run(
	ctx context.Context,
	a arm.Arm,
	opts StreamOptions,
	jpCh <-chan JointPositionsChItem,
	seed []referenceframe.Input,
	diagnostics *diagnostics.SingleSessionDiagnostics,
) (err error) {
	if err := opts.Validate(); err != nil {
		return err
	}

	velLimits, accelLimits, err := resolveTrajectoryLimits(ctx, a, opts, len(seed))
	if err != nil {
		return err
	}

	// Derive a cancelable ctx so error returns can end the arm RPC.
	ctx, cancel := context.WithCancel(ctx)
	// Start the arm RPC stream.
	as := newArmStream(ctx, a, diagnostics)
	defer func() {
		if err != nil {
			// On error, cancel first so that the RPC gets interrupted.
			// as.close() will typically return a cancellation error (due to the
			// cancel()), but if the arm independently errored just before
			// cancel(), as.close() will return that error instead.
			cancel()
			err = multierr.Combine(err, as.close())
			return
		}
		// On success, close first to signal that the RPC can finish.
		// This blocks until the arm reports that it has completed executing the stream.
		err = as.close()
		cancel()
	}()

	// Start the trajex session.
	ts := &trajexSession{opts: opts, diagnostics: diagnostics}
	if err := ts.startSession(seed, velLimits, accelLimits); err != nil {
		return fmt.Errorf("startSession (seed=%v): %w", seed, err)
	}
	defer ts.close()

	targetRunway := time.Duration(opts.TargetRunwayInArmMs) * time.Millisecond

	sendToArmTicker := time.NewTicker(time.Duration(opts.SendToArmIntervalMs) * time.Millisecond)
	defer sendToArmTicker.Stop()

	for {
		select {
		// Cancel was called.
		case <-ctx.Done():
			return ctx.Err()

		// A new set of joint positions is available.
		case jp, ok := <-jpCh:
			if !ok {
				// jpCh closed: no more targets can arrive, so nothing can pivot the remaining
				// trajectory. Send all of it to the arm now, in targetRunway-sized batches.
				for {
					pvats, err := ts.sampleAtLeast(ctx, targetRunway)
					if err != nil {
						return fmt.Errorf("sample (lastJointPositions=%v): %w", ts.lastJointPositions, err)
					}
					if len(pvats) == 0 {
						return nil
					}
					if err := as.send(ctx, pvats); err != nil {
						return err
					}
				}
			}
			diagnostics.RecordReceivedJointPositionTargetEvent()

			// Add the new joint positions to the trajex session.
			if err := ts.addJointPositionsToSession(ctx, jp.Positions); err != nil {
				return fmt.Errorf("addJointPositionsToSession (lastJointPositions=%v): %w", ts.lastJointPositions, err)
			}

			diagnostics.RecordArmRunway(as.currentEstimatedRunwayInArm())
			// Top up in case we missed the last tick.
			if err := as.topUp(ctx, ts, targetRunway); err != nil {
				return err
			}

		// Time to check whether the arm's runway needs topping up.
		case <-sendToArmTicker.C:
			diagnostics.RecordArmRunway(as.currentEstimatedRunwayInArm())
			if err := as.topUp(ctx, ts, targetRunway); err != nil {
				return err
			}
		}
	}
}

// resolveTrajectoryLimits returns the per-joint velocity/acceleration limits Run paces the
// derived trajectory against: opts.VelLimitDegPerSec/AccelLimitDegPerSec2, applied uniformly to
// every joint, when the caller set them; falling back independently, for whichever of the two
// was left unset (0), to the arm's own kinematics-declared per-joint limits.
func resolveTrajectoryLimits(ctx context.Context, a arm.Arm, opts StreamOptions, dof int) (vel, accel []float64, err error) {
	if opts.VelLimitDegPerSec > 0 && opts.AccelLimitDegPerSec2 > 0 {
		return uniformLimits(dof, opts.VelLimitDegPerSec), uniformLimits(dof, opts.AccelLimitDegPerSec2), nil
	}

	kinematics, err := a.Kinematics(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get kinematics for arm streaming: %w", err)
	}
	kinVel, kinAccel, ok := referenceframe.TrajectoryLimits(kinematics.DoF())
	if !ok {
		return nil, nil, errors.New("arm streaming requires the arm's kinematics to declare " +
			"max_velocity and max_acceleration for every joint, unless vel_limit_deg_per_sec " +
			"and accel_limit_deg_per_sec2 are both set")
	}

	vel, accel = kinVel, kinAccel
	if opts.VelLimitDegPerSec > 0 {
		vel = uniformLimits(dof, opts.VelLimitDegPerSec)
	}
	if opts.AccelLimitDegPerSec2 > 0 {
		accel = uniformLimits(dof, opts.AccelLimitDegPerSec2)
	}
	return vel, accel, nil
}

func uniformLimits(dof int, limitDegPerSec float64) []float64 {
	limit := utils.DegToRad(limitDegPerSec)
	out := make([]float64, dof)
	for i := range out {
		out[i] = limit
	}
	return out
}

func (s *armStream) topUp(ctx context.Context, ts *trajexSession, targetRunway time.Duration) error {
	deficit := targetRunway - s.currentEstimatedRunwayInArm()
	if deficit <= 0 {
		return nil
	}
	pvats, err := ts.sampleAtLeast(ctx, deficit)
	if err != nil {
		return fmt.Errorf("sample (lastJointPositions=%v): %w", ts.lastJointPositions, err)
	}
	if len(pvats) == 0 {
		return nil
	}
	return s.send(ctx, pvats)
}
