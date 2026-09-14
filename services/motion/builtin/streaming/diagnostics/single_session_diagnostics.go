// Package diagnostics implements per-session diagnostics for arm streaming.
package diagnostics

import (
	"slices"
	"sync"
	"time"

	"go.viam.com/rdk/utils"
)

// SingleSessionDiagnostics collects the diagnostics of one arm-streaming session.
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

// RecordReceivedJointPositionTargetEvent records the arrival of one joint position target.
func (t *SingleSessionDiagnostics) RecordReceivedJointPositionTargetEvent() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.details.recordJointPositionTargetReceivedEvent()
	t.stats.recordJointPositionTargetReceived()
	t.mu.Unlock()
}

// RecordArmRunway records one reading of the estimated arm-side runway.
func (t *SingleSessionDiagnostics) RecordArmRunway(runway time.Duration) {
	if t == nil {
		return
	}
	ms := float64(runway.Microseconds()) / 1000.0
	t.mu.Lock()
	t.details.recordArmRunway(ms)
	t.stats.recordArmRunway(ms)
	t.mu.Unlock()
}

// RecordTrajexSessionOpenEvent records the trajex session opening.
func (t *SingleSessionDiagnostics) RecordTrajexSessionOpenEvent() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.details.recordTrajexSessionOpenEvent()
	t.mu.Unlock()
}

// RecordTrajexSessionCloseEvent records the trajex session closing.
func (t *SingleSessionDiagnostics) RecordTrajexSessionCloseEvent() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.details.recordTrajexSessionCloseEvent()
	t.mu.Unlock()
}

// RecordArmStreamOpenEvent records the arm stream RPC opening.
func (t *SingleSessionDiagnostics) RecordArmStreamOpenEvent() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.details.recordArmStreamOpenEvent()
	t.mu.Unlock()
}

// RecordArmStreamCloseEvent records the arm stream RPC closing.
func (t *SingleSessionDiagnostics) RecordArmStreamCloseEvent() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.details.recordArmStreamCloseEvent()
	t.mu.Unlock()
}

// RecordTrajexExtendLatency records the duration of one trajex Extend call that began at start.
func (t *SingleSessionDiagnostics) RecordTrajexExtendLatency(start time.Time, d time.Duration) {
	if t == nil {
		return
	}
	ms := float64(d.Microseconds()) / 1000.0
	t.mu.Lock()
	t.details.recordTrajexExtendLatency(unixMillisFloat(start), ms)
	t.stats.recordTrajexExtendLatency(ms)
	t.mu.Unlock()
}

// RecordSendToArmLatency records the duration of one batch send to the arm RPC that began at start.
func (t *SingleSessionDiagnostics) RecordSendToArmLatency(start time.Time, d time.Duration) {
	if t == nil {
		return
	}
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
	if t == nil {
		return
	}
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
	if t == nil {
		return SingleSessionLastWindowDetails{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.details.pruneBefore(unixMillisFloat(time.Now()) - t.details.windowMs)
	live := &t.details.SingleSessionLastWindowDetails
	return SingleSessionLastWindowDetails{
		JointPositionTargetReceived: slices.Clone(live.JointPositionTargetReceived),
		ArmRunway:                   slices.Clone(live.ArmRunway),
		TrajexExtendLatency:         slices.Clone(live.TrajexExtendLatency),
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
	if t == nil {
		return SingleSessionStats{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return SingleSessionStats{
		DurationMs: float64(time.Since(t.start).Microseconds()) / 1000.0,

		ArmRunwayMinMs: t.stats.armRunway.minMs,
		ArmRunwayMaxMs: t.stats.armRunway.maxMs,

		TrajexExtendLatencyP50Ms: t.stats.extendLatency.quantileMs(0.5),
		TrajexExtendLatencyP99Ms: t.stats.extendLatency.quantileMs(0.99),
		TrajexExtendLatencyMaxMs: t.stats.extendLatency.maxMs,
		SendToArmLatencyP50Ms:    t.stats.sendLatency.quantileMs(0.5),
		SendToArmLatencyP99Ms:    t.stats.sendLatency.quantileMs(0.99),
		SendToArmLatencyMaxMs:    t.stats.sendLatency.maxMs,

		JointPositionTargetsReceived: t.stats.targetsReceived,

		MaxJointDegPerSec:  t.stats.maxJointDegPerSec,
		MaxJointDegPerSec2: t.stats.maxJointDegPerSec2,
	}
}
