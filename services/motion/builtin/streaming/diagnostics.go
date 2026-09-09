// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"math"
	"sync"
	"time"

	"go.viam.com/rdk/utils"
)

// Channel/op labels recorded in each sample. The label values are a stable contract with
// downstream renderers (offline plotters, on-robot diagnostic cameras), independent of the
// code's current identifier names — do not rename them casually.
const (
	diagChanPlanQ        = "jointPositionsCh" // stream_push producer -> trajex session (jpCh)
	diagChanArmPending   = "armQ"             // currentEstimatedRunwayInArm (ms) vs targetRunway (ms)
	diagChanTrajexRunway = "trajex-runway"    // trajexRunway() estimate in ms (len) after each Extend/sampleAtLeast; cap unused
	diagOpEnqueue        = "enq"
	diagOpDequeue        = "deq"

	// diagChanExtendBranch records one sample per Extend: op is the extend's disposition
	// ("first", "pivot", "stage-behind", "stage-nomaterial", "stage-locked", "noop"), len
	// the signed branch margin in ms (divergence point minus sampling watermark), and cap
	// is 1 when that margin exists — a locked-out stage builds no candidate, so it has
	// no branch point and cap is 0 with len 0.
	diagChanExtendBranch = "trajex-extend"
)

// Event kinds — point-in-time lifecycle markers overlaid on the occupancy samples.
const (
	diagEventTrajexSessionOpen  = "trajex-session-open"  // the trajex (totg) session opened
	diagEventTrajexSessionClose = "trajex-session-close" // the trajex session closed (shutdown)
	diagEventStreamOpen         = "stream-open"          // the arm stream RPC opened
	diagEventStreamClose        = "stream-close"         // the arm stream RPC closed
	diagEventStreamDied         = "stream-died"          // the arm stream RPC returned an unexpected error (the label carries it)
)

// Timing kinds — per-call durations recorded alongside the occupancy samples.
const (
	diagTimingExtend    = "trajex-extend" // one trajexSession.addJointPositionsToSession (Extend) call
	diagTimingSendPoint = "send-point"    // one armStream.send call (one sampled batch to the arm RPC)
	diagTimingTrajSent  = "traj-sent"     // trajectory duration (ms) delivered in one sendPVATs call
)

// DiagnosticsSample is one occupancy reading captured at an enqueue or dequeue of a pipeline channel,
// or at a periodic check of the estimated arm-side buffer (diagChanArmPending).
type DiagnosticsSample struct {
	TMs float64 `json:"t_ms"` // milliseconds since the recording started
	Ch  string  `json:"ch"`   // diagChanPlanQ or diagChanArmPending
	Op  string  `json:"op"`   // diagOpEnqueue or diagOpDequeue
	Len int     `json:"len"`  // channel length (or armQ ms buffered) at the moment of the op
	Cap int     `json:"cap"`  // channel capacity (or armQ target runway in ms)
}

// DiagnosticsEvent is a point-in-time lifecycle marker.
type DiagnosticsEvent struct {
	TMs   float64 `json:"t_ms"`  // milliseconds since the recording started
	Kind  string  `json:"kind"`  // one of the pipeEvent* constants
	Label string  `json:"label"` // short display text, may be empty
}

// DiagnosticsTiming is one measured call duration.
type DiagnosticsTiming struct {
	TMs  float64 `json:"t_ms"` // milliseconds since the recording started
	Kind string  `json:"kind"` // diagTimingExtend, diagTimingSendPoint, or diagTimingTrajSent
	Ms   float64 `json:"ms"`   // the measured duration in milliseconds
}

// DiagnosticsKinematics is the arm's kinematic state at one PVAT, taken from the trajex output.
// DegPerSec collapses JointDegPerSec to a single number for the existing aggregate chart;
// JointDegPerSec/JointPositionsDeg/JointAccelDegPerSec2 carry the full per-joint state so a fault
// right before a trajectory rejection can be attributed to a specific joint instead of just
// "some joint, somewhere".
type DiagnosticsKinematics struct {
	TMs                  float64   `json:"t_ms"`                     // milliseconds since the recording started
	DegPerSec            float64   `json:"deg_per_sec"`              // max |joint velocity| across all joints for this PVAT
	JointDegPerSec       []float64 `json:"joint_deg_per_sec"`        // per-joint velocity, deg/s, arm DoF order
	JointPositionsDeg    []float64 `json:"joint_positions_deg"`      // per-joint position, deg, arm DoF order
	JointAccelDegPerSec2 []float64 `json:"joint_accel_deg_per_sec2"` // per-joint acceleration, deg/s^2, arm DoF order
}

// StreamDiagnosticsOutput is the snapshot shape returned to callers: occupancy samples, event
// markers, timings, and kinematics recorded so far for one streaming session. StartUnixMs
// is the wall-clock time the recorder's clock started, so consumers can render the relative
// t_ms values as timestamps.
type StreamDiagnosticsOutput struct {
	StartUnixMs float64                 `json:"start_unix_ms"`
	Samples     []DiagnosticsSample     `json:"samples"`
	Events      []DiagnosticsEvent      `json:"events"`
	Timings     []DiagnosticsTiming     `json:"timings"`
	Kinematics  []DiagnosticsKinematics `json:"kinematics"`
}

// diagnosticsWindowMs is how much history a StreamDiagnostics retains: entries older than this
// (relative to the newest activity) are dropped, so a long-running session's recording stays
// bounded instead of growing without limit.
const diagnosticsWindowMs = 60_000

// StreamDiagnostics is a flight recorder for one arm-streaming session: queue-occupancy
// samples, call timings, per-extend outcomes, per-PVAT kinematics, and lifecycle events. Rather
// than sampling on a timer, the executor records at each enqueue/dequeue, so the recording
// captures every change point of the pipeline's buffers. Recording happens from the
// trajex-session and arm-stream goroutines, so it is mutex-guarded; len()/cap() on a channel
// are themselves concurrency-safe.
//
// Only the most recent windowMs of history is retained (and returned by Snapshot); each record
// prunes entries that have aged out, so memory stays bounded for arbitrarily long sessions.
//
// A nil *StreamDiagnostics is valid: every method is a nil-safe no-op, so tracing can be disabled by
// simply not providing one.
type StreamDiagnostics struct {
	mu         sync.Mutex
	start      time.Time
	windowMs   float64
	samples    []DiagnosticsSample
	events     []DiagnosticsEvent
	timings    []DiagnosticsTiming
	kinematics []DiagnosticsKinematics
}

// NewDiagnostics returns an empty recorder whose clock starts now.
func NewDiagnostics() *StreamDiagnostics {
	return &StreamDiagnostics{start: time.Now(), windowMs: diagnosticsWindowMs}
}

// pruneBefore drops the aged prefix of a time-ordered slice by reslicing, which costs only
// the dropped count; the dead prefix's backing memory is reclaimed the next time an append
// outgrows the array and reallocates around the live elements.
func pruneBefore[T any](items []T, tMs func(T) float64, cutoffMs float64) []T {
	i := 0
	for i < len(items) && tMs(items[i]) < cutoffMs {
		i++
	}
	return items[i:]
}

// pruneLocked drops every entry older than the window behind nowMs. Callers hold t.mu.
func (t *StreamDiagnostics) pruneLocked(nowMs float64) {
	cutoff := nowMs - t.windowMs
	t.samples = pruneBefore(t.samples, func(s DiagnosticsSample) float64 { return s.TMs }, cutoff)
	t.events = pruneBefore(t.events, func(e DiagnosticsEvent) float64 { return e.TMs }, cutoff)
	t.timings = pruneBefore(t.timings, func(x DiagnosticsTiming) float64 { return x.TMs }, cutoff)
	t.kinematics = pruneBefore(t.kinematics, func(v DiagnosticsKinematics) float64 { return v.TMs }, cutoff)
}

// record appends one occupancy sample. Safe to call on a nil recorder (no-op) so call sites need
// no guard, and safe to call concurrently.
func (t *StreamDiagnostics) record(ch, op string, length, capacity int) {
	if t == nil {
		return
	}
	tMs := float64(time.Since(t.start).Microseconds()) / 1000.0
	t.mu.Lock()
	t.samples = append(t.samples, DiagnosticsSample{TMs: tMs, Ch: ch, Op: op, Len: length, Cap: capacity})
	t.pruneLocked(tMs)
	t.mu.Unlock()
}

// recordEvent appends one lifecycle marker. Safe to call on a nil recorder (no-op) and concurrently.
func (t *StreamDiagnostics) recordEvent(kind, label string) {
	if t == nil {
		return
	}
	tMs := float64(time.Since(t.start).Microseconds()) / 1000.0
	t.mu.Lock()
	t.events = append(t.events, DiagnosticsEvent{TMs: tMs, Kind: kind, Label: label})
	t.pruneLocked(tMs)
	t.mu.Unlock()
}

// recordTiming appends one measured call duration. Safe to call on a nil recorder (no-op) and concurrently.
func (t *StreamDiagnostics) recordTiming(kind string, d time.Duration) {
	if t == nil {
		return
	}
	now := time.Since(t.start)
	tMs := float64(now.Microseconds()) / 1000.0
	t.mu.Lock()
	t.timings = append(t.timings, DiagnosticsTiming{
		TMs:  tMs,
		Kind: kind,
		Ms:   float64(d.Microseconds()) / 1000.0,
	})
	t.pruneLocked(tMs)
	t.mu.Unlock()
}

// recordKinematics appends one PVAT's full kinematic state (positions, velocities,
// accelerations), converting the trajex output's radians to degrees. Safe to call on a nil
// recorder (no-op) and concurrently.
func (t *StreamDiagnostics) recordKinematics(positionsRad, velocitiesRadPerSec, accelerationsRadPerSec2 []float64) {
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
	jointAccelDegPerSec2 := make([]float64, len(accelerationsRadPerSec2))
	for i, a := range accelerationsRadPerSec2 {
		jointAccelDegPerSec2[i] = utils.RadToDeg(a)
	}

	tMs := float64(time.Since(t.start).Microseconds()) / 1000.0
	t.mu.Lock()
	t.kinematics = append(t.kinematics, DiagnosticsKinematics{
		TMs:                  tMs,
		DegPerSec:            maxAbs,
		JointDegPerSec:       jointDegPerSec,
		JointPositionsDeg:    jointPositionsDeg,
		JointAccelDegPerSec2: jointAccelDegPerSec2,
	})
	t.pruneLocked(tMs)
	t.mu.Unlock()
}

// Snapshot returns a copy of the samples, events, timings, and kinematics recorded so far.
// Safe to call on a nil recorder (returns the zero value).
func (t *StreamDiagnostics) Snapshot() StreamDiagnosticsOutput {
	if t == nil {
		return StreamDiagnosticsOutput{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	// Prune against wall time too, so a snapshot taken after the session went quiet
	// still returns only the last window of history.
	t.pruneLocked(float64(time.Since(t.start).Microseconds()) / 1000.0)
	samples := make([]DiagnosticsSample, len(t.samples))
	copy(samples, t.samples)
	events := make([]DiagnosticsEvent, len(t.events))
	copy(events, t.events)
	timings := make([]DiagnosticsTiming, len(t.timings))
	copy(timings, t.timings)
	kinematics := make([]DiagnosticsKinematics, len(t.kinematics))
	copy(kinematics, t.kinematics)
	return StreamDiagnosticsOutput{
		StartUnixMs: float64(t.start.UnixMilli()),
		Samples:     samples,
		Events:      events,
		Timings:     timings,
		Kinematics:  kinematics,
	}
}
