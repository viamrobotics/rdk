// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"math"
	"sync"
	"time"

	"go.viam.com/rdk/utils"
)

// Channel/op labels recorded in each sample. The label values are pinned to what
// plot_pipeline_trace.py expects, independent of the code's current channel names.
const (
	pipeChanPlanQ        = "jointPositionsCh" // stream_push producer -> trajex session (jpCh)
	pipeChanArmPending   = "armQ"             // currentEstimatedRunwayInArm (ms) vs targetRunway (ms)
	pipeChanArmSent      = "armSent"          // cumulative PVATs delivered to the arm RPC (len; cap unused)
	pipeChanTrajexGen    = "trajex-gen"       // generation_count after each Extend (len); cap unused
	pipeChanTrajexRunway = "trajex-runway"    // trajexRunway() estimate in ms (len) after each Extend/sampleAtLeast; cap unused
	pipeOpEnqueue        = "enq"
	pipeOpDequeue        = "deq"

	// pipeChanExtendBranch records one sample per Extend: op is the extend's disposition
	// ("first", "pivot", "stage-behind", "stage-nomaterial", "stage-locked", "noop"), len
	// the signed branch margin in ms (divergence point minus sampling watermark), and cap
	// is 1 when that margin exists — a locked-out stage builds no candidate, so it has
	// no branch point and cap is 0 with len 0.
	pipeChanExtendBranch = "trajex-extend"
)

// Event kinds — point-in-time lifecycle markers overlaid on the occupancy trace.
const (
	pipeEventSessionOpen  = "trajex-session-open"  // the trajex (totg) session opened
	pipeEventSessionClose = "trajex-session-close" // the trajex session closed (shutdown)
	pipeEventStreamOpen   = "stream-open"          // the arm stream RPC opened
	pipeEventStreamClose  = "stream-close"         // the arm stream RPC closed
	pipeEventStreamDied   = "stream-died"          // the arm stream ended unexpectedly (send failed)
)

// Timing kinds — per-call durations recorded alongside the occupancy trace.
const (
	pipeTimingExtend    = "trajex-extend" // one trajexSession.addJointPositionsToSession (Extend) call
	pipeTimingSendPoint = "send-point"    // one armStream.send call (one sampled batch to the arm RPC)
	pipeTimingTrajSent  = "traj-sent"     // trajectory duration (ms) delivered in one sendPVATs call
)

// PipeSample is one occupancy reading captured at an enqueue or dequeue of a pipeline channel,
// or at a periodic check of the estimated arm-side buffer (pipeChanArmPending).
type PipeSample struct {
	TMs float64 `json:"t_ms"` // milliseconds since the trace started
	Ch  string  `json:"ch"`   // pipeChanPlanQ, pipeChanArmSent, or pipeChanArmPending
	Op  string  `json:"op"`   // pipeOpEnqueue or pipeOpDequeue
	Len int     `json:"len"`  // channel length (or armQ ms buffered) at the moment of the op
	Cap int     `json:"cap"`  // channel capacity (or armQ target runway in ms)
}

// PipeEvent is a point-in-time lifecycle marker.
type PipeEvent struct {
	TMs   float64 `json:"t_ms"`  // milliseconds since the trace started
	Kind  string  `json:"kind"`  // one of the pipeEvent* constants
	Label string  `json:"label"` // short display text, may be empty
}

// PipeTiming is one measured call duration.
type PipeTiming struct {
	TMs  float64 `json:"t_ms"` // milliseconds since the trace started
	Kind string  `json:"kind"` // pipeTimingExtend, pipeTimingSendPoint, or pipeTimingTrajSent
	Ms   float64 `json:"ms"`   // the measured duration in milliseconds
}

// PipeVelocity is the arm's speed and configuration at one PVAT, taken from the trajex output.
// DegPerSec collapses JointDegPerSec to a single number for the existing aggregate chart;
// JointDegPerSec/JointPositionsDeg carry the full per-joint state so a fault right before a
// trajectory rejection can be attributed to a specific joint instead of just "some joint, somewhere".
type PipeVelocity struct {
	TMs               float64   `json:"t_ms"`                // milliseconds since the trace started
	DegPerSec         float64   `json:"deg_per_sec"`         // max |joint velocity| across all joints for this PVAT
	JointDegPerSec    []float64 `json:"joint_deg_per_sec"`   // per-joint velocity, deg/s, arm DoF order
	JointPositionsDeg []float64 `json:"joint_positions_deg"` // per-joint position, deg, arm DoF order
}

// PipelineTraceOutput is the snapshot shape returned to callers: occupancy samples, event
// markers, timings, and velocities recorded so far for one streaming session.
type PipelineTraceOutput struct {
	Samples    []PipeSample   `json:"samples"`
	Events     []PipeEvent    `json:"events"`
	Timings    []PipeTiming   `json:"timings"`
	Velocities []PipeVelocity `json:"velocities"`
}

// PipelineTrace accumulates queue-occupancy samples for one streaming session. Rather than
// sampling on a timer, the executor records at each enqueue/dequeue, so the trace captures every
// change point of the pipeline's buffers. Recording happens from the trajex-session and arm-stream
// goroutines, so it is mutex-guarded; len()/cap() on a channel are themselves concurrency-safe.
//
// A nil *PipelineTrace is valid: every method is a nil-safe no-op, so tracing can be disabled by
// simply not providing one.
type PipelineTrace struct {
	mu         sync.Mutex
	start      time.Time
	samples    []PipeSample
	events     []PipeEvent
	timings    []PipeTiming
	velocities []PipeVelocity
}

// NewPipelineTrace returns an empty trace whose clock starts now.
func NewPipelineTrace() *PipelineTrace {
	return &PipelineTrace{start: time.Now()}
}

// record appends one occupancy sample. Safe to call on a nil trace (no-op) so call sites need
// no guard, and safe to call concurrently.
func (t *PipelineTrace) record(ch, op string, length, capacity int) {
	if t == nil {
		return
	}
	tMs := float64(time.Since(t.start).Microseconds()) / 1000.0
	t.mu.Lock()
	t.samples = append(t.samples, PipeSample{TMs: tMs, Ch: ch, Op: op, Len: length, Cap: capacity})
	t.mu.Unlock()
}

// recordEvent appends one lifecycle marker. Safe to call on a nil trace (no-op) and concurrently.
func (t *PipelineTrace) recordEvent(kind, label string) {
	if t == nil {
		return
	}
	tMs := float64(time.Since(t.start).Microseconds()) / 1000.0
	t.mu.Lock()
	t.events = append(t.events, PipeEvent{TMs: tMs, Kind: kind, Label: label})
	t.mu.Unlock()
}

// recordTiming appends one measured call duration. Safe to call on a nil trace (no-op) and concurrently.
func (t *PipelineTrace) recordTiming(kind string, d time.Duration) {
	if t == nil {
		return
	}
	now := time.Since(t.start)
	t.mu.Lock()
	t.timings = append(t.timings, PipeTiming{
		TMs:  float64(now.Microseconds()) / 1000.0,
		Kind: kind,
		Ms:   float64(d.Microseconds()) / 1000.0,
	})
	t.mu.Unlock()
}

// recordVelocity appends one arm-speed/configuration reading, converting the trajex output's
// radians to degrees. Safe to call on a nil trace (no-op) and concurrently.
func (t *PipelineTrace) recordVelocity(positionsRad, velocitiesRadPerSec []float64) {
	if t == nil {
		return
	}
	jointDegPerSec := make([]float64, len(velocitiesRadPerSec))
	var maxAbs float64
	for i, v := range velocitiesRadPerSec {
		jointDegPerSec[i] = utils.RadToDeg(v)
		if a := math.Abs(jointDegPerSec[i]); a > maxAbs {
			maxAbs = a
		}
	}
	jointPositionsDeg := make([]float64, len(positionsRad))
	for i, p := range positionsRad {
		jointPositionsDeg[i] = utils.RadToDeg(p)
	}

	tMs := float64(time.Since(t.start).Microseconds()) / 1000.0
	t.mu.Lock()
	t.velocities = append(t.velocities, PipeVelocity{
		TMs:               tMs,
		DegPerSec:         maxAbs,
		JointDegPerSec:    jointDegPerSec,
		JointPositionsDeg: jointPositionsDeg,
	})
	t.mu.Unlock()
}

// Snapshot returns a copy of the samples, events, timings, and velocities recorded so far.
// Safe to call on a nil trace (returns the zero value).
func (t *PipelineTrace) Snapshot() PipelineTraceOutput {
	if t == nil {
		return PipelineTraceOutput{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	samples := make([]PipeSample, len(t.samples))
	copy(samples, t.samples)
	events := make([]PipeEvent, len(t.events))
	copy(events, t.events)
	timings := make([]PipeTiming, len(t.timings))
	copy(timings, t.timings)
	velocities := make([]PipeVelocity, len(t.velocities))
	copy(velocities, t.velocities)
	return PipelineTraceOutput{Samples: samples, Events: events, Timings: timings, Velocities: velocities}
}
