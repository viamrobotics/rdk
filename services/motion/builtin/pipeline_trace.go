package builtin

import (
	"sync"
	"time"
)

// Channel/op labels recorded in each sample.
const (
	pipeChanPlanQ      = "planQ"   // jointPositionsCh (DoStreamPush producer -> trajex consumer)
	pipeChanStreamQ    = "streamQ" // pvatsCh (trajex producer -> arm-send consumer)
	pipeChanArmPending = "armQ"    // estimatedDurationRemainingInArm (ms) vs BufferAheadInArmMs (ms)
	pipeOpEnqueue      = "enq"
	pipeOpDequeue      = "deq"
)

// Event kinds — point-in-time lifecycle markers overlaid on the occupancy trace.
const (
	pipeEventSessionOpen  = "trajex-session-open"  // producer opened a new trajex (totg) session
	pipeEventSessionClose = "trajex-session-close" // producer closed a trajex session (StopAcceptable or shutdown)
	pipeEventStreamOpen   = "stream-open"          // consumer opened a new arm stream
	pipeEventStreamClose  = "stream-close"         // consumer closed the current arm stream
	pipeEventStreamDied   = "stream-died"          // the arm stream ended unexpectedly (send failed)
)

// Timing kinds — per-call durations recorded alongside the occupancy trace.
const (
	pipeTimingExtend    = "trajex-extend" // one trajexSession.addJointPositions (Extend) call
	pipeTimingSendPoint = "send-point"    // one armStream.send call (batch delivered to the arm)
)

// pipeSample is one occupancy reading captured at an enqueue or dequeue of a pipeline channel,
// or at a periodic check of the estimated arm-side buffer (pipeChanArmPending).
type pipeSample struct {
	TMs float64 `json:"t_ms"` // milliseconds since the trace started
	Ch  string  `json:"ch"`   // pipeChanPlanQ, pipeChanStreamQ, or pipeChanArmPending
	Op  string  `json:"op"`   // pipeOpEnqueue or pipeOpDequeue
	Len int     `json:"len"`  // channel length (or armQ ms buffered) at the moment of the op
	Cap int     `json:"cap"`  // channel capacity (or armQ target BufferAheadInArmMs)
}

// pipeEvent is a point-in-time lifecycle marker.
type pipeEvent struct {
	TMs   float64 `json:"t_ms"`  // milliseconds since the trace started
	Kind  string  `json:"kind"`  // one of the pipeEvent* constants
	Label string  `json:"label"` // short display text, may be empty
}

// pipeTiming is one measured call duration.
type pipeTiming struct {
	TMs  float64 `json:"t_ms"` // milliseconds since the trace started
	Kind string  `json:"kind"` // pipeTimingExtend or pipeTimingSendPoint
	Ms   float64 `json:"ms"`   // the measured duration in milliseconds
}

// pipeVelocity is the arm's speed at one PVAT (max abs joint velocity), taken from the trajex output.
type pipeVelocity struct {
	TMs       float64 `json:"t_ms"`        // milliseconds since the trace started
	DegPerSec float64 `json:"deg_per_sec"` // max |joint velocity| across all joints for this PVAT
}

// pipelineTraceOutput is the snapshot shape returned to callers: occupancy samples, event
// markers, timings, and velocities recorded so far for one streaming session.
type pipelineTraceOutput struct {
	Samples    []pipeSample   `json:"samples"`
	Events     []pipeEvent    `json:"events"`
	Timings    []pipeTiming   `json:"timings"`
	Velocities []pipeVelocity `json:"velocities"`
}

// pipelineTrace accumulates queue-occupancy samples for one streaming session. Rather than
// sampling on a timer, callers record at each enqueue/dequeue, so the trace captures every
// change point of planQ/streamQ/armQ. Recording happens from the producer and consumer
// goroutines, so it is mutex-guarded; len()/cap() on a channel are themselves concurrency-safe.
type pipelineTrace struct {
	mu         sync.Mutex
	start      time.Time
	samples    []pipeSample
	events     []pipeEvent
	timings    []pipeTiming
	velocities []pipeVelocity
}

func newPipelineTrace() *pipelineTrace {
	return &pipelineTrace{start: time.Now()}
}

// record appends one occupancy sample. Safe to call on a nil trace (no-op) so call sites need
// no guard, and safe to call concurrently.
func (t *pipelineTrace) record(ch, op string, length, capacity int) {
	if t == nil {
		return
	}
	tMs := float64(time.Since(t.start).Microseconds()) / 1000.0
	t.mu.Lock()
	t.samples = append(t.samples, pipeSample{TMs: tMs, Ch: ch, Op: op, Len: length, Cap: capacity})
	t.mu.Unlock()
}

// recordEvent appends one lifecycle marker. Safe to call on a nil trace (no-op) and concurrently.
func (t *pipelineTrace) recordEvent(kind, label string) {
	if t == nil {
		return
	}
	tMs := float64(time.Since(t.start).Microseconds()) / 1000.0
	t.mu.Lock()
	t.events = append(t.events, pipeEvent{TMs: tMs, Kind: kind, Label: label})
	t.mu.Unlock()
}

// recordTiming appends one measured call duration. Safe to call on a nil trace (no-op) and concurrently.
func (t *pipelineTrace) recordTiming(kind string, d time.Duration) {
	if t == nil {
		return
	}
	now := time.Since(t.start)
	t.mu.Lock()
	t.timings = append(t.timings, pipeTiming{
		TMs:  float64(now.Microseconds()) / 1000.0,
		Kind: kind,
		Ms:   float64(d.Microseconds()) / 1000.0,
	})
	t.mu.Unlock()
}

// recordVelocity appends one arm-speed reading. Safe to call on a nil trace (no-op) and concurrently.
func (t *pipelineTrace) recordVelocity(degPerSec float64) {
	if t == nil {
		return
	}
	tMs := float64(time.Since(t.start).Microseconds()) / 1000.0
	t.mu.Lock()
	t.velocities = append(t.velocities, pipeVelocity{TMs: tMs, DegPerSec: degPerSec})
	t.mu.Unlock()
}

// snapshot returns a copy of the samples, events, timings, and velocities recorded so far.
// Safe to call on a nil trace (returns the zero value).
func (t *pipelineTrace) snapshot() pipelineTraceOutput {
	if t == nil {
		return pipelineTraceOutput{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	samples := make([]pipeSample, len(t.samples))
	copy(samples, t.samples)
	events := make([]pipeEvent, len(t.events))
	copy(events, t.events)
	timings := make([]pipeTiming, len(t.timings))
	copy(timings, t.timings)
	velocities := make([]pipeVelocity, len(t.velocities))
	copy(velocities, t.velocities)
	return pipelineTraceOutput{Samples: samples, Events: events, Timings: timings, Velocities: velocities}
}
