// Package diagnostics implements per-session diagnostics for arm streaming.
package diagnostics

import (
	"maps"
	"slices"
	"sync"
	"time"

	"go.viam.com/rdk/utils"
)

// SingleSessionDiagnostics collects the diagnostics of one arm-streaming session. Despite the
// name, an instance is meant to be reused across consecutive sessions on the same arm resource
// (see ResetStats and SetWindow): the retained detail window survives a session ending, so a
// later session's recordings continue appending to it rather than starting over, and entries
// only ever age out lazily, as a side effect of a later append noticing they're now outside the
// window (see pruneBefore) — never as a wholesale reset tied to a session boundary.
type SingleSessionDiagnostics struct {
	mu    sync.Mutex
	start time.Time

	details singleSessionLastWindowDetails
	stats   singleSessionStats
}

// New returns empty diagnostics retaining the given window of detail.
func New(window time.Duration) *SingleSessionDiagnostics {
	now := time.Now()
	return &SingleSessionDiagnostics{
		start: now,
		details: singleSessionLastWindowDetails{
			windowMs: float64(window.Microseconds()) / 1000.0,
		},
	}
}

// SetWindow changes the retained-detail window. It takes effect lazily: existing entries are
// pruned against the new window on the next recording or LastWindowDetails call, the same way
// the window is always enforced, so shrinking it doesn't retroactively rewrite history and
// growing it doesn't resurrect anything already pruned. Call this when a session reusing
// diagnostics retained from a previous session configures a different window.
func (t *SingleSessionDiagnostics) SetWindow(window time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.details.windowMs = float64(window.Microseconds()) / 1000.0
}

// WindowSecs returns the currently configured retained-detail window, in whole seconds.
func (t *SingleSessionDiagnostics) WindowSecs() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return int(t.details.windowMs / 1000.0)
}

// ResetStats zeroes the whole-run aggregates Stats() reports and restarts its duration clock,
// without touching the retained detail window. Call this when a new session begins reusing
// diagnostics retained from a previous session on the same arm, so Stats() keeps describing only
// the session that's starting even though the detail window now spans across sessions.
func (t *SingleSessionDiagnostics) ResetStats() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.start = time.Now()
	t.stats = singleSessionStats{}
}

// RecordReceivedJointPositionTargetEvent records the arrival of one joint position target.
func (t *SingleSessionDiagnostics) RecordReceivedJointPositionTargetEvent() {
	t.mu.Lock()
	t.details.recordJointPositionTargetReceivedEvent()
	t.stats.recordJointPositionTargetReceived()
	t.mu.Unlock()
}

// RecordArmRunway records one reading of the estimated arm-side runway.
func (t *SingleSessionDiagnostics) RecordArmRunway(runway time.Duration) {
	ms := float64(runway.Microseconds()) / 1000.0
	t.mu.Lock()
	t.details.recordArmRunway(ms)
	t.stats.recordArmRunway(ms)
	t.mu.Unlock()
}

// RecordTrajexRunway records one reading of the trajectory buffered inside the trajex session,
// staged motion included, alongside the cap the backpressure gate compares it to (zero when the
// gate is disabled).
func (t *SingleSessionDiagnostics) RecordTrajexRunway(runway, maxRunway time.Duration) {
	t.mu.Lock()
	t.details.recordTrajexRunway(float64(runway.Microseconds())/1000.0, float64(maxRunway.Microseconds())/1000.0)
	t.mu.Unlock()
}

// RecordTrajexSessionOpenEvent records the trajex session opening.
func (t *SingleSessionDiagnostics) RecordTrajexSessionOpenEvent() {
	t.mu.Lock()
	t.details.recordTrajexSessionOpenEvent()
	t.mu.Unlock()
}

// RecordTrajexSessionCloseEvent records the trajex session closing.
func (t *SingleSessionDiagnostics) RecordTrajexSessionCloseEvent() {
	t.mu.Lock()
	t.details.recordTrajexSessionCloseEvent()
	t.mu.Unlock()
}

// RecordArmStreamOpenEvent records the arm stream RPC opening.
func (t *SingleSessionDiagnostics) RecordArmStreamOpenEvent() {
	t.mu.Lock()
	t.details.recordArmStreamOpenEvent()
	t.mu.Unlock()
}

// RecordArmStreamCloseEvent records the arm stream RPC closing.
func (t *SingleSessionDiagnostics) RecordArmStreamCloseEvent() {
	t.mu.Lock()
	t.details.recordArmStreamCloseEvent()
	t.mu.Unlock()
}

// RecordTrajexExtend records one trajex Extend call that began at start and took d: how the
// session handled the batch (kind, in trajex's spelling; "error" if the call failed) and, where
// trajex computed them, the branch slack and the change in the active trajectory's duration.
func (t *SingleSessionDiagnostics) RecordTrajexExtend(
	start time.Time, d time.Duration, kind string, branchSlack, deltaActiveDuration *time.Duration,
) {
	e := TrajexExtend{
		TimestampMs:           unixMillisFloat(start),
		DurationMs:            float64(d.Microseconds()) / 1000.0,
		Kind:                  kind,
		BranchSlackMs:         optionalMs(branchSlack),
		DeltaActiveDurationMs: optionalMs(deltaActiveDuration),
	}
	t.mu.Lock()
	t.details.recordTrajexExtend(e)
	t.stats.recordTrajexExtend(e)
	t.mu.Unlock()
}

func optionalMs(d *time.Duration) *float64 {
	if d == nil {
		return nil
	}
	ms := float64(d.Microseconds()) / 1000.0
	return &ms
}

// RecordSendToArmLatency records the duration of one batch send to the arm RPC that began at start.
func (t *SingleSessionDiagnostics) RecordSendToArmLatency(start time.Time, d time.Duration) {
	ms := float64(d.Microseconds()) / 1000.0
	t.mu.Lock()
	t.details.recordSendToArmLatency(unixMillisFloat(start), ms)
	t.stats.recordSendToArmLatency(ms)
	t.mu.Unlock()
}

// RecordSampledPVAT records one PVAT sampled out of trajex.
func (t *SingleSessionDiagnostics) RecordSampledPVAT(
	positionsRad, velocitiesRadPerSec, accelerationsRadPerSec2 []float64,
	trajectoryTime time.Duration,
) {
	// Converted from radians to degrees because the vel/accel limits are prescribed in
	// degrees, so the recorded values read directly against them.
	jointDeg := make([]float64, len(positionsRad))
	for i, pos := range positionsRad {
		jointDeg[i] = utils.RadToDeg(pos)
	}
	jointDegPerSec := make([]float64, len(velocitiesRadPerSec))
	for i, v := range velocitiesRadPerSec {
		jointDegPerSec[i] = utils.RadToDeg(v)
	}
	jointDegPerSec2 := make([]float64, len(accelerationsRadPerSec2))
	for i, a := range accelerationsRadPerSec2 {
		jointDegPerSec2[i] = utils.RadToDeg(a)
	}
	sampled := PVAT{
		TrajectoryTimeMs: float64(trajectoryTime.Microseconds()) / 1000.0,
		JointDeg:         jointDeg,
		JointDegPerSec:   jointDegPerSec,
		JointDegPerSec2:  jointDegPerSec2,
	}

	t.mu.Lock()
	t.details.recordSampledPVAT(sampled)
	t.stats.recordSampledPVAT(jointDegPerSec, jointDegPerSec2)
	t.mu.Unlock()
}

// LastWindowDetails returns a copy of the retained window.
func (t *SingleSessionDiagnostics) LastWindowDetails() SingleSessionLastWindowDetails {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.details.pruneBefore(unixMillisFloat(time.Now()) - t.details.windowMs)
	live := &t.details.SingleSessionLastWindowDetails
	return SingleSessionLastWindowDetails{
		JointPositionTargetReceived: slices.Clone(live.JointPositionTargetReceived),
		ArmRunway:                   slices.Clone(live.ArmRunway),
		TrajexRunway:                slices.Clone(live.TrajexRunway),
		TrajexExtends:               slices.Clone(live.TrajexExtends),
		SendToArmLatency:            slices.Clone(live.SendToArmLatency),
		TrajexSessionOpen:           slices.Clone(live.TrajexSessionOpen),
		TrajexSessionClose:          slices.Clone(live.TrajexSessionClose),
		ArmStreamOpen:               slices.Clone(live.ArmStreamOpen),
		ArmStreamClose:              slices.Clone(live.ArmStreamClose),
		SampledPVATs:                slices.Clone(live.SampledPVATs),
	}
}

// Stats returns the session's whole-run aggregates.
func (t *SingleSessionDiagnostics) Stats() SingleSessionStats {
	t.mu.Lock()
	defer t.mu.Unlock()
	return SingleSessionStats{
		DurationMs: float64(time.Since(t.start).Microseconds()) / 1000.0,

		ArmRunwayMinMs: t.stats.armRunway.minMs,
		ArmRunwayMaxMs: t.stats.armRunway.maxMs,

		TrajexExtendLatencyP50Ms: t.stats.extendLatency.quantileMs(0.5),
		TrajexExtendLatencyP99Ms: t.stats.extendLatency.quantileMs(0.99),
		TrajexExtendLatencyMaxMs: t.stats.extendLatency.maxMs,
		TrajexExtendsByKind:      maps.Clone(t.stats.extendsByKind),
		TrajexBranchSlackMinMs:   t.stats.branchSlackMin(),
		SendToArmLatencyP50Ms:    t.stats.sendLatency.quantileMs(0.5),
		SendToArmLatencyP99Ms:    t.stats.sendLatency.quantileMs(0.99),
		SendToArmLatencyMaxMs:    t.stats.sendLatency.maxMs,

		JointPositionTargetsReceived: t.stats.targetsReceived,

		MaxJointDegPerSec:  t.stats.maxJointDegPerSec,
		MaxJointDegPerSec2: t.stats.maxJointDegPerSec2,
	}
}
