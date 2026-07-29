// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"context"
	"errors"
	"time"

	"github.com/go-viper/mapstructure/v2"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
)

const (
	defaultTargetRunwayInArmMs  = 100
	defaultSendToArmIntervalMs  = 10
	defaultVelLimitDegPerSec    = 10.0
	defaultAccelLimitDegPerSec2 = 10.0
)

// JointPositionsChItem is one joint-space waypoint.
type JointPositionsChItem struct {
	// Positions are the target joint positions for this waypoint.
	Positions []referenceframe.Input
}

// StreamOptions tunes the streaming executor.
type StreamOptions struct {
	// TargetRunwayInArmMs is the duration of pvat points that we aim to keep
	// buffered inside the arm resource.
	TargetRunwayInArmMs int `json:"target_runway_in_arm_ms"`

	// SendToArmIntervalMs is the interval at which batches of pvat points are
	// sent to the arm resource.
	// TODO: Replace this with querying the arm's properties API.
	SendToArmIntervalMs int `json:"send_to_arm_interval_ms"`

	// VelLimitDegPerSec / AccelLimitDegPerSec2 are the per-joint limits the trajex
	// session is built with.
	// TODO: Replace these with querying the arm's properties API.
	VelLimitDegPerSec    float64 `json:"vel_limit_deg_per_sec"`
	AccelLimitDegPerSec2 float64 `json:"accel_limit_deg_per_sec2"`
}

// Validate returns an error if any StreamOptions field is invalid.
func (o *StreamOptions) Validate() error {
	if o.TargetRunwayInArmMs < 0 {
		return errors.New("streaming: target_runway_in_arm_ms must be non-negative")
	}
	if o.SendToArmIntervalMs <= 0 {
		return errors.New("streaming: send_to_arm_interval_ms must be positive")
	}
	if o.VelLimitDegPerSec < 0 {
		return errors.New("streaming: vel_limit_deg_per_sec must be non-negative")
	}
	if o.AccelLimitDegPerSec2 < 0 {
		return errors.New("streaming: accel_limit_deg_per_sec2 must be non-negative")
	}
	return nil
}

// ApplyDefaults fills any zero-valued StreamOptions field with its default.
func (o *StreamOptions) ApplyDefaults() {
	if o.TargetRunwayInArmMs == 0 {
		o.TargetRunwayInArmMs = defaultTargetRunwayInArmMs
	}
	if o.SendToArmIntervalMs == 0 {
		o.SendToArmIntervalMs = defaultSendToArmIntervalMs
	}
	if o.VelLimitDegPerSec == 0 {
		o.VelLimitDegPerSec = defaultVelLimitDegPerSec
	}
	if o.AccelLimitDegPerSec2 == 0 {
		o.AccelLimitDegPerSec2 = defaultAccelLimitDegPerSec2
	}
}

// ParseStreamOptions decodes raw (e.g. a map[string]interface{} parsed from JSON) into opts,
// matching fields by their json tag.
func ParseStreamOptions(raw interface{}, opts *StreamOptions) error {
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:          "json",
		WeaklyTypedInput: true,
		Result:           opts,
	})
	if err != nil {
		return err
	}
	return dec.Decode(raw)
}

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
