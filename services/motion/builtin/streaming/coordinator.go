//go:build !windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt

package streaming

import (
	"context"
	"time"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
)

type pvat struct {
	positions     []float64
	velocities    []float64
	accelerations []float64
	time          time.Duration
}

// Run executes a streaming session through one trajex session and one arm stream RPC. It returns
// nil once jpCh or flushCh is closed and every sampled PVAT has been drained to the arm and
// acknowledged, or the first non-context error from the trajex session or arm stream otherwise.
// flushCh may be nil, in which case the session ends only via jpCh closing or ctx cancellation.
func Run(
	ctx context.Context,
	a arm.Arm,
	opts *StreamOptions,
	jpCh <-chan JointPositionsChItem,
	flushCh <-chan struct{},
	seed []referenceframe.Input,
) error {
	opts.ApplyDefaults()
	if err := opts.Validate(); err != nil {
		return err
	}

	pvatCh := make(chan pvat, max(1, opts.TargetRunwayInArmMs/opts.SendToArmIntervalMs))
	trajexSession := &trajexSession{
		opts:   opts,
		pvatCh: pvatCh,
	}
	armStream := &armStreamWithTargetRunway{
		arm:               a,
		pvatCh:            pvatCh,
		targetRunway:      time.Duration(opts.TargetRunwayInArmMs) * time.Millisecond,
		sendToArmInterval: time.Duration(opts.SendToArmIntervalMs) * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(ctx)
	// We create a channel buffer of size one for both run handles such that the runner code can
	// safely choose any order of pushing an error and calling done.
	trajexRunHandle := runHandle{
		done:   make(chan error, 1),
		cancel: cancel,
	}

	armRunHandle := runHandle{
		done:   make(chan error, 1),
		cancel: cancel,
	}
	go trajexSession.run(ctx, jpCh, flushCh, seed, &trajexRunHandle)
	go armStream.run(ctx, &armRunHandle)

	// Wait for both goroutines to finish.
	// Whichever side fails first cancels the shared context.
	trajexErr := <-trajexRunHandle.done
	armErr := <-armRunHandle.done

	// Ensure cancel is called to release resources.
	cancel()

	// Report the root cause. Trajex errors take priority over arm errors.
	if trajexErr != nil {
		return trajexErr
	}

	return armErr
}

type runHandle struct {
	done chan error // closed when the caller returns. An error is pushed iff there was an error.

	// cancel is called when the owner errors. This will signal contexts used by codependent
	// processes.
	cancel context.CancelFunc
}
