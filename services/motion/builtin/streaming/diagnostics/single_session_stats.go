package diagnostics

import (
	"math"
)

// SingleSessionStats is one session's whole-run aggregates.
type SingleSessionStats struct {
	DurationMs float64 `json:"duration_ms"`

	JointPositionTargetsReceived int64 `json:"joint_position_targets_received"`

	// Arm runway can go negative (the arm has outrun what was sent), so it is tracked as
	// exact extremes rather than a histogram.
	ArmRunwayMinMs float64 `json:"arm_runway_min_ms"`
	ArmRunwayMaxMs float64 `json:"arm_runway_max_ms"`

	TrajexExtendLatencyP50Ms float64 `json:"trajex_extend_latency_p50_ms"`
	TrajexExtendLatencyP99Ms float64 `json:"trajex_extend_latency_p99_ms"`
	TrajexExtendLatencyMaxMs float64 `json:"trajex_extend_latency_max_ms"`
	SendToArmLatencyP50Ms    float64 `json:"send_to_arm_latency_p50_ms"`
	SendToArmLatencyP99Ms    float64 `json:"send_to_arm_latency_p99_ms"`
	SendToArmLatencyMaxMs    float64 `json:"send_to_arm_latency_max_ms"`

	MaxJointDegPerSec  float64 `json:"max_joint_deg_per_sec"`
	MaxJointDegPerSec2 float64 `json:"max_joint_deg_per_sec2"`
}

// singleSessionStats is the whole-session accumulators backing Stats(); never pruned.
type singleSessionStats struct {
	targetsReceived    int64
	armRunway          extremes
	extendLatency      durationStats
	sendLatency        durationStats
	maxJointDegPerSec  float64
	maxJointDegPerSec2 float64
}

func (s *singleSessionStats) recordJointPositionTargetReceived() {
	s.targetsReceived++
}

func (s *singleSessionStats) recordArmRunway(ms float64) {
	s.armRunway.record(ms)
}

func (s *singleSessionStats) recordTrajexExtendLatency(ms float64) {
	s.extendLatency.record(ms)
}

func (s *singleSessionStats) recordSendToArmLatency(ms float64) {
	s.sendLatency.record(ms)
}

func (s *singleSessionStats) recordSampledPVAT(jointDegPerSec, jointDegPerSec2 []float64) {
	for _, v := range jointDegPerSec {
		s.maxJointDegPerSec = math.Max(s.maxJointDegPerSec, math.Abs(v))
	}
	for _, a := range jointDegPerSec2 {
		s.maxJointDegPerSec2 = math.Max(s.maxJointDegPerSec2, math.Abs(a))
	}
}

const (
	durationBucketCount = 24

	// durationBucketBaseMs is bucket 0's upper bound, doubled for each later bucket: 0.25ms
	// is below the smallest latency worth distinguishing, and 22 doublings reach ~17 minutes,
	// with the last bucket unbounded.
	durationBucketBaseMs = 0.25
)

// extremes tracks the exact min and max of one series in constant memory.
type extremes struct {
	count int64
	minMs float64
	maxMs float64
}

func (e *extremes) record(ms float64) {
	e.count++
	if e.count == 1 {
		e.minMs, e.maxMs = ms, ms
		return
	}
	e.minMs = math.Min(e.minMs, ms)
	e.maxMs = math.Max(e.maxMs, ms)
}

// durationStats accumulates one non-negative series in constant memory: exact count/min/max,
// plus a 24-bucket histogram whose bucket bounds double from durationBucketBaseMs, so
// quantileMs can answer p50/p99 by walking bucket counts — accurate to within one bucket,
// never above maxMs.
type durationStats struct {
	extremes
	buckets [durationBucketCount]int64
}

func (s *durationStats) record(ms float64) {
	s.extremes.record(ms)
	s.buckets[durationBucket(ms)]++
}

// durationBucket maps a value to the first bucket whose doubling upper bound covers it.
func durationBucket(ms float64) int {
	upper := durationBucketBaseMs
	for i := range durationBucketCount - 1 {
		if ms <= upper {
			return i
		}
		upper *= 2
	}
	// The last bucket is unbounded: everything past the second-to-last bucket's bound lands here.
	return durationBucketCount - 1
}

// quantileMs reports the upper bound of the bucket holding the q'th value, clamped to the true max.
func (s *durationStats) quantileMs(q float64) float64 {
	if s.count == 0 {
		return 0
	}
	// target is how many of the recorded values the q'th quantile must cover.
	// E.g. p50 over 9 recorded values must cover ceil(0.5*9) = 5 values.
	target := int64(math.Ceil(q * float64(s.count)))
	if target < 1 {
		target = 1
	}
	var cum int64
	upper := durationBucketBaseMs
	for i := range durationBucketCount - 1 {
		cum += s.buckets[i]
		if cum >= target {
			return math.Min(upper, s.maxMs)
		}
		upper *= 2
	}
	// The quantile is in the unbounded last bucket, so the only bound left is the exact max.
	return s.maxMs
}
