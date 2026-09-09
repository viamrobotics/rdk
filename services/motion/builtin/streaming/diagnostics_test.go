// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/utils"
)

func TestStreamDiagnosticsNilSafe(t *testing.T) {
	var diagnostics *StreamDiagnostics
	diagnostics.record(diagChanArmPending, diagOpDequeue, 1, 2)
	diagnostics.recordEvent(diagEventStreamOpen, "")
	diagnostics.recordTiming(diagTimingSendPoint, time.Millisecond)
	diagnostics.recordKinematics([]float64{0.1}, []float64{0.2}, []float64{0.3})
	test.That(t, diagnostics.Snapshot(), test.ShouldResemble, StreamDiagnosticsOutput{})
}

func TestStreamDiagnosticsRecordsAndSnapshots(t *testing.T) {
	diagnostics := NewStreamDiagnostics()
	diagnostics.record(diagChanPlanQ, diagOpEnqueue, 3, 10)
	diagnostics.recordEvent(diagEventTrajexSessionOpen, "")
	diagnostics.recordTiming(diagTimingExtend, 5*time.Millisecond)
	// Two joints: the second carries the larger |velocity|, so DegPerSec must pick it up rather
	// than just reporting the first/last joint.
	diagnostics.recordKinematics(
		[]float64{utils.DegToRad(10), utils.DegToRad(-20)},
		[]float64{utils.DegToRad(15), utils.DegToRad(-42)},
		[]float64{utils.DegToRad(30), utils.DegToRad(-90)},
	)

	out := diagnostics.Snapshot()
	test.That(t, len(out.Samples), test.ShouldEqual, 1)
	test.That(t, out.Samples[0].Ch, test.ShouldEqual, diagChanPlanQ)
	test.That(t, out.Samples[0].Op, test.ShouldEqual, diagOpEnqueue)
	test.That(t, out.Samples[0].Len, test.ShouldEqual, 3)
	test.That(t, out.Samples[0].Cap, test.ShouldEqual, 10)

	test.That(t, len(out.Events), test.ShouldEqual, 1)
	test.That(t, out.Events[0].Kind, test.ShouldEqual, diagEventTrajexSessionOpen)

	test.That(t, len(out.Timings), test.ShouldEqual, 1)
	test.That(t, out.Timings[0].Kind, test.ShouldEqual, diagTimingExtend)
	test.That(t, out.Timings[0].Ms, test.ShouldAlmostEqual, 5.0, 0.5)

	test.That(t, len(out.Kinematics), test.ShouldEqual, 1)
	v := out.Kinematics[0]
	test.That(t, v.DegPerSec, test.ShouldAlmostEqual, 42.0, 1e-9)
	test.That(t, len(v.JointDegPerSec), test.ShouldEqual, 2)
	test.That(t, v.JointDegPerSec[0], test.ShouldAlmostEqual, 15.0, 1e-9)
	test.That(t, v.JointDegPerSec[1], test.ShouldAlmostEqual, -42.0, 1e-9)
	test.That(t, len(v.JointPositionsDeg), test.ShouldEqual, 2)
	test.That(t, v.JointPositionsDeg[0], test.ShouldAlmostEqual, 10.0, 1e-9)
	test.That(t, v.JointPositionsDeg[1], test.ShouldAlmostEqual, -20.0, 1e-9)
	test.That(t, len(v.JointAccelDegPerSec2), test.ShouldEqual, 2)
	test.That(t, v.JointAccelDegPerSec2[0], test.ShouldAlmostEqual, 30.0, 1e-9)
	test.That(t, v.JointAccelDegPerSec2[1], test.ShouldAlmostEqual, -90.0, 1e-9)

	// Snapshot returns a copy: further recording must not mutate the earlier snapshot.
	diagnostics.record(diagChanPlanQ, diagOpDequeue, 0, 10)
	test.That(t, len(out.Samples), test.ShouldEqual, 1)
}

func TestStreamDiagnosticsRetainsOnlyTheWindow(t *testing.T) {
	diagnostics := NewStreamDiagnostics()

	// Inject entries well older than the window directly, then record fresh ones: the
	// record path must prune everything that has aged out of the window.
	oldT := float64(-2 * diagnosticsWindowMs)
	diagnostics.samples = append(diagnostics.samples, DiagnosticsSample{TMs: oldT, Ch: "old", Op: "enq"})
	diagnostics.events = append(diagnostics.events, DiagnosticsEvent{TMs: oldT, Kind: "old"})
	diagnostics.timings = append(diagnostics.timings, DiagnosticsTiming{TMs: oldT, Kind: "old"})
	diagnostics.kinematics = append(diagnostics.kinematics, DiagnosticsKinematics{TMs: oldT})

	diagnostics.record("fresh", diagOpEnqueue, 1, 2)
	diagnostics.recordEvent("fresh", "")
	diagnostics.recordTiming("fresh", time.Millisecond)
	diagnostics.recordKinematics([]float64{0}, []float64{1}, []float64{2})

	out := diagnostics.Snapshot()
	test.That(t, len(out.Samples), test.ShouldEqual, 1)
	test.That(t, out.Samples[0].Ch, test.ShouldEqual, "fresh")
	test.That(t, len(out.Events), test.ShouldEqual, 1)
	test.That(t, out.Events[0].Kind, test.ShouldEqual, "fresh")
	test.That(t, len(out.Timings), test.ShouldEqual, 1)
	test.That(t, out.Timings[0].Kind, test.ShouldEqual, "fresh")
	test.That(t, len(out.Kinematics), test.ShouldEqual, 1)

	// Snapshot alone also prunes: entries recorded now age out once the wall clock has
	// moved a window past them, even with no further records. Simulate by backdating the
	// recorder's start so everything recorded above is now older than the window.
	diagnostics.start = diagnostics.start.Add(-2 * diagnosticsWindowMs * time.Millisecond)
	out = diagnostics.Snapshot()
	test.That(t, len(out.Samples), test.ShouldEqual, 0)
	test.That(t, len(out.Events), test.ShouldEqual, 0)
	test.That(t, len(out.Timings), test.ShouldEqual, 0)
	test.That(t, len(out.Kinematics), test.ShouldEqual, 0)
}
