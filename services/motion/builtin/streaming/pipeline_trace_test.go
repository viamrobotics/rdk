// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/utils"
)

func TestPipelineTraceNilSafe(t *testing.T) {
	var trace *PipelineTrace
	trace.record(pipeChanArmPending, pipeOpDequeue, 1, 2)
	trace.recordEvent(pipeEventStreamOpen, "")
	trace.recordTiming(pipeTimingSendPoint, time.Millisecond)
	trace.recordVelocity([]float64{0.1}, []float64{0.2})
	test.That(t, trace.Snapshot(), test.ShouldResemble, PipelineTraceOutput{})
}

func TestPipelineTraceRecordsAndSnapshots(t *testing.T) {
	trace := NewPipelineTrace()
	trace.record(pipeChanPlanQ, pipeOpEnqueue, 3, 10)
	trace.recordEvent(pipeEventSessionOpen, "")
	trace.recordTiming(pipeTimingExtend, 5*time.Millisecond)
	// Two joints: the second carries the larger |velocity|, so DegPerSec must pick it up rather
	// than just reporting the first/last joint.
	trace.recordVelocity(
		[]float64{utils.DegToRad(10), utils.DegToRad(-20)},
		[]float64{utils.DegToRad(15), utils.DegToRad(-42)},
	)

	out := trace.Snapshot()
	test.That(t, len(out.Samples), test.ShouldEqual, 1)
	test.That(t, out.Samples[0].Ch, test.ShouldEqual, pipeChanPlanQ)
	test.That(t, out.Samples[0].Op, test.ShouldEqual, pipeOpEnqueue)
	test.That(t, out.Samples[0].Len, test.ShouldEqual, 3)
	test.That(t, out.Samples[0].Cap, test.ShouldEqual, 10)

	test.That(t, len(out.Events), test.ShouldEqual, 1)
	test.That(t, out.Events[0].Kind, test.ShouldEqual, pipeEventSessionOpen)

	test.That(t, len(out.Timings), test.ShouldEqual, 1)
	test.That(t, out.Timings[0].Kind, test.ShouldEqual, pipeTimingExtend)
	test.That(t, out.Timings[0].Ms, test.ShouldAlmostEqual, 5.0, 0.5)

	test.That(t, len(out.Velocities), test.ShouldEqual, 1)
	v := out.Velocities[0]
	test.That(t, v.DegPerSec, test.ShouldAlmostEqual, 42.0, 1e-9)
	test.That(t, len(v.JointDegPerSec), test.ShouldEqual, 2)
	test.That(t, v.JointDegPerSec[0], test.ShouldAlmostEqual, 15.0, 1e-9)
	test.That(t, v.JointDegPerSec[1], test.ShouldAlmostEqual, -42.0, 1e-9)
	test.That(t, len(v.JointPositionsDeg), test.ShouldEqual, 2)
	test.That(t, v.JointPositionsDeg[0], test.ShouldAlmostEqual, 10.0, 1e-9)
	test.That(t, v.JointPositionsDeg[1], test.ShouldAlmostEqual, -20.0, 1e-9)

	// Snapshot returns a copy: further recording must not mutate the earlier snapshot.
	trace.record(pipeChanPlanQ, pipeOpDequeue, 0, 10)
	test.That(t, len(out.Samples), test.ShouldEqual, 1)
}
