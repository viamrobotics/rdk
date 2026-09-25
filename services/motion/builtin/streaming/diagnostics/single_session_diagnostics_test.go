package diagnostics

import (
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/utils"
)

const testWindowMs = 60_000

func durationPtr(d time.Duration) *time.Duration { return &d }

func TestDiagnosticsRecordAndReturnWindow(t *testing.T) {
	diagnostics := New(testWindowMs * time.Millisecond)
	diagnostics.RecordReceivedJointPositionTargetEvent()
	diagnostics.RecordArmRunway(40 * time.Millisecond)
	diagnostics.RecordTrajexSessionOpenEvent()
	diagnostics.RecordTrajexSessionCloseEvent()
	diagnostics.RecordArmStreamOpenEvent()
	diagnostics.RecordArmStreamCloseEvent()
	diagnostics.RecordTrajexExtend(time.Now(), 5*time.Millisecond, "pivot", durationPtr(12*time.Millisecond), durationPtr(300*time.Millisecond))
	diagnostics.RecordSendToArmLatency(time.Now(), 2*time.Millisecond)
	// Two joints: the second carries the larger |velocity|, so DegPerSec must pick it up rather
	// than just reporting the first/last joint.
	diagnostics.RecordSampledPVAT(
		[]float64{utils.DegToRad(10), utils.DegToRad(-20)},
		[]float64{utils.DegToRad(15), utils.DegToRad(-42)},
		[]float64{utils.DegToRad(30), utils.DegToRad(-90)},
		1500*time.Millisecond,
	)

	out := diagnostics.LastWindowDetails()
	test.That(t, len(out.JointPositionTargetReceived), test.ShouldEqual, 1)

	test.That(t, len(out.ArmRunway), test.ShouldEqual, 1)
	test.That(t, out.ArmRunway[0].SizeMs, test.ShouldEqual, 40.0)

	test.That(t, len(out.TrajexSessionOpen), test.ShouldEqual, 1)
	test.That(t, len(out.TrajexSessionClose), test.ShouldEqual, 1)
	test.That(t, len(out.ArmStreamOpen), test.ShouldEqual, 1)
	test.That(t, len(out.ArmStreamClose), test.ShouldEqual, 1)

	test.That(t, len(out.TrajexExtends), test.ShouldEqual, 1)
	test.That(t, out.TrajexExtends[0].DurationMs, test.ShouldEqual, 5.0)
	test.That(t, out.TrajexExtends[0].Kind, test.ShouldEqual, "pivot")
	test.That(t, *out.TrajexExtends[0].BranchSlackMs, test.ShouldEqual, 12.0)
	test.That(t, *out.TrajexExtends[0].DeltaActiveDurationMs, test.ShouldEqual, 300.0)
	test.That(t, len(out.SendToArmLatency), test.ShouldEqual, 1)
	test.That(t, out.SendToArmLatency[0].DurationMs, test.ShouldEqual, 2.0)

	test.That(t, len(out.SampledPVATs), test.ShouldEqual, 1)
	v := out.SampledPVATs[0]
	test.That(t, v.TrajectoryTimeMs, test.ShouldEqual, 1500.0)
	test.That(t, v.TimestampMs, test.ShouldBeGreaterThan, 1e12)
	test.That(t, len(v.JointDegPerSec), test.ShouldEqual, 2)
	test.That(t, v.JointDegPerSec[0], test.ShouldAlmostEqual, 15.0, 1e-9)
	test.That(t, v.JointDegPerSec[1], test.ShouldAlmostEqual, -42.0, 1e-9)
	test.That(t, len(v.JointDeg), test.ShouldEqual, 2)
	test.That(t, v.JointDeg[0], test.ShouldAlmostEqual, 10.0, 1e-9)
	test.That(t, v.JointDeg[1], test.ShouldAlmostEqual, -20.0, 1e-9)
	test.That(t, len(v.JointDegPerSec2), test.ShouldEqual, 2)
	test.That(t, v.JointDegPerSec2[0], test.ShouldAlmostEqual, 30.0, 1e-9)
	test.That(t, v.JointDegPerSec2[1], test.ShouldAlmostEqual, -90.0, 1e-9)

	// LastWindowDetails returns a copy: further recording must not mutate an earlier return.
	diagnostics.RecordReceivedJointPositionTargetEvent()
	test.That(t, len(out.JointPositionTargetReceived), test.ShouldEqual, 1)
}

func TestDiagnosticsRetainOnlyTheWindow(t *testing.T) {
	diagnostics := New(testWindowMs * time.Millisecond)

	// Inject entries well older than the window directly, then record fresh ones: the
	// record path must prune everything that has aged out of the window.
	oldT := float64(-2 * testWindowMs)
	diagnostics.details.JointPositionTargetReceived = append(diagnostics.details.JointPositionTargetReceived, Event{TimestampMs: oldT})
	diagnostics.details.ArmRunway = append(diagnostics.details.ArmRunway, BufferSize{TimestampMs: oldT})
	diagnostics.details.TrajexExtends = append(diagnostics.details.TrajexExtends, TrajexExtend{TimestampMs: oldT})
	diagnostics.details.SendToArmLatency = append(diagnostics.details.SendToArmLatency, Latency{TimestampMs: oldT})
	diagnostics.details.ArmStreamOpen = append(diagnostics.details.ArmStreamOpen, Event{TimestampMs: oldT})
	diagnostics.details.SampledPVATs = append(diagnostics.details.SampledPVATs, PVAT{TimestampMs: oldT})

	diagnostics.RecordReceivedJointPositionTargetEvent()
	diagnostics.RecordArmRunway(time.Millisecond)
	diagnostics.RecordArmStreamOpenEvent()
	diagnostics.RecordTrajexExtend(time.Now(), time.Millisecond, "noop", nil, nil)
	diagnostics.RecordSendToArmLatency(time.Now(), time.Millisecond)
	diagnostics.RecordSampledPVAT([]float64{0}, []float64{1}, []float64{2}, time.Millisecond)

	out := diagnostics.LastWindowDetails()
	test.That(t, len(out.JointPositionTargetReceived), test.ShouldEqual, 1)
	test.That(t, len(out.ArmRunway), test.ShouldEqual, 1)
	test.That(t, len(out.ArmStreamOpen), test.ShouldEqual, 1)
	test.That(t, len(out.TrajexExtends), test.ShouldEqual, 1)
	test.That(t, len(out.SendToArmLatency), test.ShouldEqual, 1)
	test.That(t, len(out.SampledPVATs), test.ShouldEqual, 1)

	// LastWindowDetails alone also prunes: entries recorded now age out once the wall clock
	// has moved a window past them, even with no further records. Simulate with a negative
	// window so the cutoff sits in the future and clears everything recorded above.
	diagnostics.details.windowMs = -testWindowMs
	out = diagnostics.LastWindowDetails()
	test.That(t, len(out.JointPositionTargetReceived), test.ShouldEqual, 0)
	test.That(t, len(out.ArmRunway), test.ShouldEqual, 0)
	test.That(t, len(out.ArmStreamOpen), test.ShouldEqual, 0)
	test.That(t, len(out.TrajexExtends), test.ShouldEqual, 0)
	test.That(t, len(out.SendToArmLatency), test.ShouldEqual, 0)
	test.That(t, len(out.SampledPVATs), test.ShouldEqual, 0)
}

func TestDiagnosticsZeroWindowDisablesDetailOnly(t *testing.T) {
	diagnostics := New(0)
	diagnostics.RecordReceivedJointPositionTargetEvent()
	diagnostics.RecordArmRunway(40 * time.Millisecond)
	diagnostics.RecordTrajexSessionOpenEvent()
	diagnostics.RecordTrajexSessionCloseEvent()
	diagnostics.RecordArmStreamOpenEvent()
	diagnostics.RecordArmStreamCloseEvent()
	diagnostics.RecordTrajexExtend(time.Now(), 5*time.Millisecond, "first_build", nil, durationPtr(time.Second))
	diagnostics.RecordSendToArmLatency(time.Now(), 2*time.Millisecond)
	diagnostics.RecordSampledPVAT([]float64{0}, []float64{1}, []float64{2}, time.Millisecond)

	test.That(t, diagnostics.LastWindowDetails(), test.ShouldResemble, SingleSessionLastWindowDetails{})

	stats := diagnostics.Stats()
	test.That(t, stats.JointPositionTargetsReceived, test.ShouldEqual, 1)
	test.That(t, stats.ArmRunwayMaxMs, test.ShouldEqual, 40.0)
	test.That(t, stats.TrajexExtendLatencyMaxMs, test.ShouldEqual, 5.0)
	test.That(t, stats.SendToArmLatencyMaxMs, test.ShouldEqual, 2.0)
}

func TestStats(t *testing.T) {
	diagnostics := New(testWindowMs * time.Millisecond)
	for range 3 {
		diagnostics.RecordReceivedJointPositionTargetEvent()
	}
	diagnostics.RecordSendToArmLatency(time.Now(), time.Millisecond)
	diagnostics.RecordSendToArmLatency(time.Now(), 2*time.Millisecond)
	diagnostics.RecordSendToArmLatency(time.Now(), 20*time.Millisecond)
	// A pivot with slack to spare, a stage that missed by 30ms, and a locked-out stage that
	// compared nothing: the kind tally sees all three, the slack minimum only the first two.
	diagnostics.RecordTrajexExtend(time.Now(), 4*time.Millisecond, "pivot", durationPtr(12*time.Millisecond), durationPtr(250*time.Millisecond))
	diagnostics.RecordTrajexExtend(time.Now(), 3*time.Millisecond, "staged_branch_sampled", durationPtr(-30*time.Millisecond), nil)
	diagnostics.RecordTrajexExtend(time.Now(), 2*time.Millisecond, "staged_again", nil, nil)
	diagnostics.RecordArmRunway(40 * time.Millisecond)
	diagnostics.RecordArmRunway(-5 * time.Millisecond)
	diagnostics.RecordSampledPVAT(
		[]float64{0, 0},
		[]float64{utils.DegToRad(50), utils.DegToRad(-80)},
		[]float64{utils.DegToRad(30), utils.DegToRad(-100)},
		0,
	)

	// Backdate the clock so DurationMs is deterministic rather than racing sub-microsecond
	// test execution against the Microseconds() truncation.
	diagnostics.start = diagnostics.start.Add(-time.Second)
	stats := diagnostics.Stats()
	test.That(t, stats.DurationMs, test.ShouldBeGreaterThan, 999.0)

	test.That(t, stats.JointPositionTargetsReceived, test.ShouldEqual, 3)
	test.That(t, stats.SendToArmLatencyMaxMs, test.ShouldEqual, 20.0)
	// Histogram quantiles land on power-of-two bucket bounds, clamped to the true max.
	test.That(t, stats.SendToArmLatencyP50Ms, test.ShouldEqual, 2.0)
	test.That(t, stats.SendToArmLatencyP99Ms, test.ShouldEqual, 20.0)

	test.That(t, stats.TrajexExtendLatencyMaxMs, test.ShouldEqual, 4.0)
	test.That(t, stats.TrajexExtendLatencyP50Ms, test.ShouldEqual, 4.0)
	test.That(t, stats.TrajexExtendsByKind, test.ShouldResemble, map[string]int64{"pivot": 1, "staged_branch_sampled": 1, "staged_again": 1})
	test.That(t, *stats.TrajexBranchSlackMinMs, test.ShouldEqual, -30.0)

	test.That(t, stats.ArmRunwayMinMs, test.ShouldEqual, -5.0)
	test.That(t, stats.ArmRunwayMaxMs, test.ShouldEqual, 40.0)
	test.That(t, stats.MaxJointDegPerSec, test.ShouldAlmostEqual, 80.0, 1e-9)
	test.That(t, stats.MaxJointDegPerSec2, test.ShouldAlmostEqual, 100.0, 1e-9)

	// The stats survive window pruning: age everything out of the window, then check
	// the aggregates are unchanged while the snapshot is empty.
	diagnostics.details.windowMs = -testWindowMs
	out := diagnostics.LastWindowDetails()
	test.That(t, len(out.SendToArmLatency), test.ShouldEqual, 0)
	stats = diagnostics.Stats()
	test.That(t, stats.JointPositionTargetsReceived, test.ShouldEqual, 3)
}
