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
// arm. It waits for the arm to have finished executing before returning.
func Run(
	ctx context.Context,
	a arm.Arm,
	opts *StreamOptions,
	jpCh <-chan JointPositionsChItem,
	seed []referenceframe.Input,
) (err error) {
	opts.ApplyDefaults()
	if err := opts.Validate(); err != nil {
		return err
	}

	// Start the trajex session.
	ts := &trajexSession{opts: opts}
	if err := ts.startSession(seed); err != nil {
		return fmt.Errorf("startSession (seed=%v): %w", seed, err)
	}
	defer ts.close()

	// Start the arm RPC stream.
	as := &armStream{arm: a}
	// Derive a cancelable ctx so error returns can end the arm RPC.
	ctx, cancel := context.WithCancel(ctx)
	as.startStream(ctx)
	defer func() {
		if err != nil {
			// On error, cancel first so that the RPC gets interrupted.
			cancel()
			err = multierr.Combine(err, as.close())
			return
		}
		// On success, close first so that the RPC can finish gracefully.
		err = as.close()
		cancel()
	}()

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
					pvats, err := ts.sample(ctx, targetRunway)
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

		// Time to check whether the arm's runway needs topping up.
		case <-sendToArmTicker.C:
			// Sample out of trajex only the deficit needed to top up the arm's runway.
			deficit := targetRunway - as.currentEstimatedRunwayInArm()
			if deficit <= 0 {
				continue
			}
			pvats, err := ts.sample(ctx, deficit)
			if err != nil {
				return fmt.Errorf("sample (lastJointPositions=%v): %w", ts.lastJointPositions, err)
			}
			if len(pvats) == 0 {
				// No trajectory yet, or what we have received so far has been sampled through.
				continue
			}
			if err := as.send(ctx, pvats); err != nil {
				return err
			}
		}
	}
}
