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
// arm, then waits for the arm to have finished executing before returning, including for the
// runway estimate to reach zero.
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
//
// trace, if non-nil, accumulates pipeline diagnostics over the session's lifetime; snapshot
// it via PipelineTrace.Snapshot.
func Run(
	ctx context.Context,
	a arm.Arm,
	opts StreamOptions,
	jpCh <-chan JointPositionsChItem,
	seed []referenceframe.Input,
	trace *PipelineTrace,
) (err error) {
	if err := opts.Validate(); err != nil {
		return err
	}

	// Derive a cancelable ctx so error returns can end the arm RPC.
	ctx, cancel := context.WithCancel(ctx)
	// Start the arm RPC stream.
	as := newArmStream(ctx, a)
	trace.recordEvent(pipeEventStreamOpen, "")
	defer func() {
		// Record the stream close on both the error and success paths.
		defer trace.recordEvent(pipeEventStreamClose, "")
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
		err = as.close()
		cancel()
	}()

	// Start the trajex session.
	ts := &trajexSession{opts: opts}
	if err := ts.startSession(seed); err != nil {
		return fmt.Errorf("startSession (seed=%v): %w", seed, err)
	}
	trace.recordEvent(pipeEventSessionOpen, "")
	defer func() {
		ts.close()
		trace.recordEvent(pipeEventSessionClose, "")
	}()

	targetRunway := time.Duration(opts.TargetRunwayInArmMs) * time.Millisecond

	// sendPVATs forwards one sampled batch to the arm, recording per-point velocities, the
	// send latency, and the cumulative count delivered to the RPC.
	totalPVATsSent := 0
	sendPVATs := func(pvats []pvat) error {
		for _, p := range pvats {
			trace.recordVelocity(p.positions, p.velocities)
		}
		sendStart := time.Now()
		if err := as.send(ctx, pvats); err != nil {
			trace.recordEvent(pipeEventStreamDied, "")
			return err
		}
		trace.recordTiming(pipeTimingSendPoint, time.Since(sendStart))
		totalPVATsSent += len(pvats)
		trace.record(pipeChanArmSent, pipeOpEnqueue, totalPVATsSent, 0)
		return nil
	}

	// topUpArm samples the deficit needed to bring the arm's estimated runway up to
	// targetRunway and sends it. It is called both on the ticker and after every extend so
	// that a long extend (longer than the ticker interval) does not cause a tick to be
	// silently dropped, leaving the arm under-filled.
	topUpArm := func() error {
		estimatedRunway := as.currentEstimatedRunwayInArm()
		trace.record(pipeChanArmPending, pipeOpDequeue,
			int(estimatedRunway.Milliseconds()), int(targetRunway.Milliseconds()))
		deficit := targetRunway - estimatedRunway
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
		trace.record(pipeChanTrajexRunway, pipeOpDequeue, int(ts.trajexRunway().Milliseconds()), 0)
		if err := sendPVATs(pvats); err != nil {
			return err
		}
		trace.recordTiming(pipeTimingTrajSent, deficit)
		return nil
	}

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
						return waitOutRunway(ctx, as)
					}
					if err := sendPVATs(pvats); err != nil {
						return err
					}
				}
			}
			trace.record(pipeChanPlanQ, pipeOpDequeue, len(jpCh), cap(jpCh))

			// Add the new joint positions to the trajex session.
			extendStart := time.Now()
			err := ts.addJointPositionsToSession(ctx, jp.Positions)
			trace.recordTiming(pipeTimingExtend, time.Since(extendStart))
			trace.record(pipeChanTrajexGen, pipeOpDequeue, int(ts.generationCount()), 0)
			trace.record(pipeChanTrajexRunway, pipeOpEnqueue, int(ts.trajexRunway().Milliseconds()), 0)
			if err != nil {
				return fmt.Errorf("addJointPositionsToSession (lastJointPositions=%v): %w", ts.lastJointPositions, err)
			}
			if err := topUpArm(); err != nil {
				return err
			}

		// Time to check whether the arm's runway needs topping up.
		case <-sendToArmTicker.C:
			if err := topUpArm(); err != nil {
				return err
			}
		}
	}
}

func waitOutRunway(ctx context.Context, as *armStream) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(as.currentEstimatedRunwayInArm()):
		return nil
	}
}
