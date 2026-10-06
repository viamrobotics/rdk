//go:build !windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt

package streaming

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/multierr"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
)

const stopArmTimeout = time.Minute

// Run executes a streaming session through one trajex session and one arm stream RPC.
// If jpCh is closed, it samples everything out of the trajex session and sends it to the
// arm, then waits for the arm to have finished executing before returning.
//
// --- An important note on backpressure --
//
// The arm provides backpressure to sampling out of trajex in that `Run` maintains an
// estimate of how much runway the arm has buffered on its side, and only samples out of
// trajex enough to keep that runway topped up to the user-configured ArmSideTargetRunwayMs.
//
// Trajex, however, does not provide any backpressure to the client: If the client sends
// joint positions faster than the arm executes them as per the trajectory output by trajex,
// trajectory simply accumulates inside the trajex session. So when acks is non-nil, `Run`
// acknowledges each target once it has been added to the trajectory, reporting how much motion
// is then queued inside trajex. A client that waits for each acknowledgment, and holds its next
// target while the queue is deeper than it wants, is paced by execution.
// Note that if, on the other hand, the client sends joint positions *slower* than the arm
// executes them (as per the trajectory output by trajex), `Run` will run out of pvat points
// to send to the arm, and the arm will (typically, depending on the arm implementation) fault.
func Run(
	ctx context.Context,
	a arm.Arm,
	opts StreamOptions,
	jpCh <-chan []referenceframe.Input,
	seed []referenceframe.Input,
	diagnostics *diagnostics.SingleSessionDiagnostics,
	acks chan<- motion.TempStreamResponse,
) (err error) {
	if err := opts.Validate(); err != nil {
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
			if closeErr := as.close(); closeErr != nil {
				err = multierr.Combine(err, fmt.Errorf("failed to close arm stream: %w", closeErr))
			}

			// Wait for the arm to stop.
			if stopErr := stopArm(ctx, a); stopErr != nil {
				err = multierr.Combine(err, fmt.Errorf("failed to stop arm, arm not guaranteed to be stopped: %w", stopErr))
			} else {
				err = fmt.Errorf("arm stopped after session error: %w", err)
			}
			return
		}
		// On success, close first to signal that the RPC can finish.
		// This blocks until the arm reports that it has completed executing the stream.
		if closeErr := as.close(); closeErr != nil {
			err = fmt.Errorf("failed to close arm stream: %w", closeErr)
			// Wait for the arm to stop.
			if stopErr := stopArm(ctx, a); stopErr != nil {
				err = multierr.Combine(err, fmt.Errorf("failed to stop arm, arm not guaranteed to be stopped: %w", stopErr))
			} else {
				err = fmt.Errorf("arm stopped after session error: %w", err)
			}
		}
		cancel()
	}()

	// Start the trajex session.
	ts := &trajexSession{opts: opts, diagnostics: diagnostics}
	if err := ts.startSession(seed); err != nil {
		return fmt.Errorf("startSession (seed=%v): %w", seed, err)
	}
	defer ts.close()

	targetRunway := time.Duration(opts.ArmSideTargetRunwayMs) * time.Millisecond

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
			if err := ts.addJointPositionsToSession(ctx, jp); err != nil {
				return fmt.Errorf("addJointPositionsToSession (lastJointPositions=%v): %w", ts.lastJointPositions, err)
			}

			diagnostics.RecordArmRunway(as.currentEstimatedRunwayInArm())
			trajexRunway := ts.trajexRunway()
			diagnostics.RecordTrajexRunway(trajexRunway)
			if acks != nil {
				queuedMs := int32(trajexRunway.Milliseconds())
				select {
				case acks <- motion.TempStreamResponse{QueuedMs: &queuedMs}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			// Top up in case we missed the last tick.
			if err := as.topUp(ctx, ts, targetRunway); err != nil {
				return err
			}

		// Time to check whether the arm's runway needs topping up.
		case <-sendToArmTicker.C:
			diagnostics.RecordArmRunway(as.currentEstimatedRunwayInArm())
			diagnostics.RecordTrajexRunway(ts.trajexRunway())
			if err := as.topUp(ctx, ts, targetRunway); err != nil {
				return err
			}
		}
	}
}

// stopArm stops the arm within stopArmTimeout, regardless of whether ctx has been canceled.
func stopArm(ctx context.Context, a arm.Arm) error {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopArmTimeout)
	defer cancel()
	return a.Stop(stopCtx, nil)
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
