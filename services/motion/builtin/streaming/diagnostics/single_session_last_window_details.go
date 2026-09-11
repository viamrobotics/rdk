package diagnostics

import (
	"time"
)

// Event is one moment something happened at, as a Unix timestamp in milliseconds.
type Event struct {
	TimestampMs float64 `json:"timestamp_ms"`
}

// Latency is one measured call latency, stamped with the Unix time in milliseconds the
// call started.
type Latency struct {
	TimestampMs float64 `json:"timestamp_ms"`
	DurationMs  float64 `json:"duration_ms"`
}

// BufferSize is one reading of a buffer's size in milliseconds of trajectory, stamped with
// the Unix time in milliseconds it was read.
type BufferSize struct {
	TimestampMs float64 `json:"timestamp_ms"`
	SizeMs      float64 `json:"size_ms"`
}

// PVAT is one sampled PVAT in degrees.
type PVAT struct {
	// TimestampMs is the Unix time in milliseconds the PVAT was sampled; it exists so
	// PVATs age out of the window on the same clock as every other series.
	TimestampMs float64 `json:"timestamp_ms"`
	// TrajectoryTimeMs is the PVAT's time on the trajectory clock. Once the arm API
	// includes the Unix time the arm started executing, this can be used to determine
	// the arm's position/velocity/acceleration in wall clock time.
	TrajectoryTimeMs float64   `json:"trajectory_time_ms"`
	JointDeg         []float64 `json:"joint_deg"`
	JointDegPerSec   []float64 `json:"joint_deg_per_sec"`
	JointDegPerSec2  []float64 `json:"joint_deg_per_sec2"`
}

// SingleSessionLastWindowDetails is the rolling-window detail returned to callers.
type SingleSessionLastWindowDetails struct {
	ArmRunway []BufferSize `json:"arm_runway"`

	TrajexExtendLatency []Latency `json:"trajex_extend_latency"`
	SendToArmLatency    []Latency `json:"send_to_arm_latency"`

	TrajexSessionOpen           []Event `json:"trajex_session_open"`
	TrajexSessionClose          []Event `json:"trajex_session_close"`
	ArmStreamOpen               []Event `json:"arm_stream_open"`
	ArmStreamClose              []Event `json:"arm_stream_close"`
	JointPositionTargetReceived []Event `json:"joint_position_target_received"`

	SampledPVATs []PVAT `json:"sampled_pvats"`
}

// singleSessionLastWindowDetails is the rolling window of full-detail entries backing
// LastWindowDetails().
type singleSessionLastWindowDetails struct {
	windowMs float64

	SingleSessionLastWindowDetails
}

func (d *singleSessionLastWindowDetails) recordArmRunway(ms float64) {
	now := unixMillisFloat(time.Now())
	d.ArmRunway = append(d.ArmRunway, BufferSize{TimestampMs: now, SizeMs: ms})
	d.pruneBefore(now - d.windowMs)
}

func (d *singleSessionLastWindowDetails) recordTrajexExtendLatency(startTimestampMs, ms float64) {
	d.TrajexExtendLatency = append(d.TrajexExtendLatency, Latency{TimestampMs: startTimestampMs, DurationMs: ms})
	d.pruneBefore(unixMillisFloat(time.Now()) - d.windowMs)
}

func (d *singleSessionLastWindowDetails) recordSendToArmLatency(startTimestampMs, ms float64) {
	d.SendToArmLatency = append(d.SendToArmLatency, Latency{TimestampMs: startTimestampMs, DurationMs: ms})
	d.pruneBefore(unixMillisFloat(time.Now()) - d.windowMs)
}

func (d *singleSessionLastWindowDetails) recordTrajexSessionOpenEvent() {
	now := unixMillisFloat(time.Now())
	d.TrajexSessionOpen = append(d.TrajexSessionOpen, Event{TimestampMs: now})
	d.pruneBefore(now - d.windowMs)
}

func (d *singleSessionLastWindowDetails) recordTrajexSessionCloseEvent() {
	now := unixMillisFloat(time.Now())
	d.TrajexSessionClose = append(d.TrajexSessionClose, Event{TimestampMs: now})
	d.pruneBefore(now - d.windowMs)
}

func (d *singleSessionLastWindowDetails) recordArmStreamOpenEvent() {
	now := unixMillisFloat(time.Now())
	d.ArmStreamOpen = append(d.ArmStreamOpen, Event{TimestampMs: now})
	d.pruneBefore(now - d.windowMs)
}

func (d *singleSessionLastWindowDetails) recordArmStreamCloseEvent() {
	now := unixMillisFloat(time.Now())
	d.ArmStreamClose = append(d.ArmStreamClose, Event{TimestampMs: now})
	d.pruneBefore(now - d.windowMs)
}

func (d *singleSessionLastWindowDetails) recordJointPositionTargetReceivedEvent() {
	now := unixMillisFloat(time.Now())
	d.JointPositionTargetReceived = append(d.JointPositionTargetReceived, Event{TimestampMs: now})
	d.pruneBefore(now - d.windowMs)
}

func (d *singleSessionLastWindowDetails) recordSampledPVAT(sampled PVAT) {
	now := unixMillisFloat(time.Now())
	sampled.TimestampMs = now
	d.SampledPVATs = append(d.SampledPVATs, sampled)
	d.pruneBefore(now - d.windowMs)
}

func (d *singleSessionLastWindowDetails) pruneBefore(timestampMs float64) {
	timestampMsOfEvent := func(e Event) float64 { return e.TimestampMs }
	timestampMsOfLatency := func(l Latency) float64 { return l.TimestampMs }
	timestampMsOfBufferSize := func(b BufferSize) float64 { return b.TimestampMs }
	timestampMsOfPVAT := func(v PVAT) float64 { return v.TimestampMs }
	d.JointPositionTargetReceived = pruneBefore(d.JointPositionTargetReceived, timestampMsOfEvent, timestampMs)
	d.ArmRunway = pruneBefore(d.ArmRunway, timestampMsOfBufferSize, timestampMs)
	d.TrajexExtendLatency = pruneBefore(d.TrajexExtendLatency, timestampMsOfLatency, timestampMs)
	d.SendToArmLatency = pruneBefore(d.SendToArmLatency, timestampMsOfLatency, timestampMs)
	d.TrajexSessionOpen = pruneBefore(d.TrajexSessionOpen, timestampMsOfEvent, timestampMs)
	d.TrajexSessionClose = pruneBefore(d.TrajexSessionClose, timestampMsOfEvent, timestampMs)
	d.ArmStreamOpen = pruneBefore(d.ArmStreamOpen, timestampMsOfEvent, timestampMs)
	d.ArmStreamClose = pruneBefore(d.ArmStreamClose, timestampMsOfEvent, timestampMs)
	d.SampledPVATs = pruneBefore(d.SampledPVATs, timestampMsOfPVAT, timestampMs)
}

func pruneBefore[T any](items []T, timestampMsOf func(T) float64, timestampMs float64) []T {
	i := 0
	for i < len(items) && timestampMsOf(items[i]) < timestampMs {
		i++
	}
	return items[i:]
}

func unixMillisFloat(t time.Time) float64 {
	return float64(t.UnixMicro()) / 1000.0
}
