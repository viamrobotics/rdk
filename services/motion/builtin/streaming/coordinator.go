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
	c, err := newCoordinator(a, opts)
	if err != nil {
		return err
	}
	return c.run(ctx, jpCh, flushCh, seed).wait()
}

type coordinator struct {
	trajexSession *trajexSession
	armStream     *armStreamWithTargetRunway
}

func newCoordinator(a arm.Arm, opts *StreamOptions) (*coordinator, error) {
	opts.ApplyDefaults()
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	pvatCh := make(chan pvat, max(1, opts.TargetRunwayInArmMs/opts.SendToArmIntervalMs))
	return &coordinator{
		trajexSession: &trajexSession{
			opts:   opts,
			pvatCh: pvatCh,
		},
		armStream: &armStreamWithTargetRunway{
			arm:               a,
			pvatCh:            pvatCh,
			targetRunway:      time.Duration(opts.TargetRunwayInArmMs) * time.Millisecond,
			sendToArmInterval: time.Duration(opts.SendToArmIntervalMs) * time.Millisecond,
		},
	}, nil
}

// run starts the trajex session and arm stream goroutines and returns a handle the caller should wait on.
func (c *coordinator) run(
	ctx context.Context,
	jpCh <-chan JointPositionsChItem,
	flushCh <-chan struct{},
	seed []referenceframe.Input,
) *runHandle {
	ctx, cancel := context.WithCancel(ctx)
	r := &runHandle{
		trajexSession: trajexSessionRunHandle{done: make(chan struct{}), cancel: cancel},
		armStream:     armStreamRunHandle{done: make(chan struct{}), cancel: cancel},
	}
	go c.trajexSession.run(ctx, jpCh, flushCh, seed, &r.trajexSession)
	go c.armStream.run(ctx, &r.armStream)
	return r
}

type trajexSessionRunHandle struct {
	done chan struct{} // closed when the trajex session returns
	err  error
	// cancel stops the arm stream when the trajex session fails
	cancel context.CancelFunc
}

type armStreamRunHandle struct {
	done chan struct{} // closed when the arm stream returns
	err  error
	// cancel stops the trajex session when the arm stream fails
	cancel context.CancelFunc
}

type runHandle struct {
	trajexSession trajexSessionRunHandle
	armStream     armStreamRunHandle
}

func (r *runHandle) wait() error {
	// Wait for both goroutines to finish.
	// Whichever side fails first cancels the shared context.
	<-r.trajexSession.done
	<-r.armStream.done

	// Call cancel, since a context created with context.WithCancel is required to
	// be canceled. It doesn't matter which handle's cancel is called.
	r.armStream.cancel()

	// Report the root cause.
	err := r.trajexSession.err
	if err == nil {
		err = r.armStream.err
	}

	return err
}
