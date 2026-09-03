//go:build !windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt

package streaming

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/multierr"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
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
// Trajex itself does not provide any backpressure to the client: If the client sends
// joint positions faster than the arm executes them as per the trajectory output by trajex,
// trajectory simply accumulates inside the trajex session. When opts.MaxTrajexRunwayMs is
// positive, `Run` provides that backpressure instead: it stops receiving from jpCh while
// the trajectory buffered inside trajex exceeds that cap, so the pusher stays blocked in
// its channel send until sampling drains the runway back under the cap.
// Note that if, on the other hand, the client sends joint positions *slower* than the arm
// executes them (as per the trajectory output by trajex), `Run` will run out of pvat points
// to send to the arm, and the arm will (typically, depending on the arm implementation) fault.
func Run(
	ctx context.Context,
	a arm.Arm,
	opts StreamOptions,
	jpCh <-chan JointPositionsChItem,
	seed []referenceframe.Input,
) (err error) {
	if err := opts.Validate(); err != nil {
		return err
	}

	// Derive a cancelable ctx so error returns can end the arm RPC.
	ctx, cancel := context.WithCancel(ctx)
	// Start the arm RPC stream.
	as := newArmStream(ctx, a)
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
	ts := &trajexSession{opts: opts}
	if err := ts.startSession(seed); err != nil {
		return fmt.Errorf("startSession (seed=%v): %w", seed, err)
	}
	defer ts.close()

	targetRunway := time.Duration(opts.TargetRunwayInArmMs) * time.Millisecond
	maxTrajexRunway := time.Duration(opts.MaxTrajexRunwayMs) * time.Millisecond

	sendToArmTicker := time.NewTicker(time.Duration(opts.SendToArmIntervalMs) * time.Millisecond)
	defer sendToArmTicker.Stop()

	for {
		// While the trajectory buffered inside trajex is over the cap, receive from a nil
		// channel instead of jpCh so no new target can be accepted; the ticker case still
		// samples trajectory out toward the arm, and the gate reopens once that drains the
		// runway under the cap. Ending the session (ctx cancellation here, flush or abort
		// on the pusher's side) unblocks a gated push through the existing paths.
		gatedJpCh := jpCh
		if maxTrajexRunway > 0 && ts.trajexRunway() >= maxTrajexRunway {
			gatedJpCh = nil
		}
		select {
		// Cancel was called.
		case <-ctx.Done():
			return ctx.Err()

		// A new set of joint positions is available.
		case jp, ok := <-gatedJpCh:
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

			// Add the new joint positions to the trajex session.
			if err := ts.addJointPositionsToSession(ctx, jp.Positions); err != nil {
				return fmt.Errorf("addJointPositionsToSession (lastJointPositions=%v): %w", ts.lastJointPositions, err)
			}

			// Top up in case we missed the last tick.
			if err := as.topUp(ctx, ts, targetRunway); err != nil {
				return err
			}

		// Time to check whether the arm's runway needs topping up.
		case <-sendToArmTicker.C:
			if err := as.topUp(ctx, ts, targetRunway); err != nil {
				return err
			}
		}
	}
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
