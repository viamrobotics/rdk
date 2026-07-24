package builtin

import (
	"testing"
	"time"

	"go.viam.com/test"
)

func TestPipelineTraceNilSafe(t *testing.T) {
	var trace *pipelineTrace
	trace.record(pipeChanArmPending, pipeOpDequeue, 1, 2)
	trace.recordEvent(pipeEventStreamOpen, "")
	trace.recordTiming(pipeTimingSendPoint, time.Millisecond)
	trace.recordVelocity(1.23)
	test.That(t, trace.snapshot(), test.ShouldResemble, pipelineTraceOutput{})
}

func TestPipelineTraceRecordsAndSnapshots(t *testing.T) {
	trace := newPipelineTrace()
	trace.record(pipeChanStreamQ, pipeOpEnqueue, 3, 10)
	trace.recordEvent(pipeEventSessionOpen, "")
	trace.recordTiming(pipeTimingExtend, 5*time.Millisecond)
	trace.recordVelocity(42.0)

	out := trace.snapshot()
	test.That(t, len(out.Samples), test.ShouldEqual, 1)
	test.That(t, out.Samples[0].Ch, test.ShouldEqual, pipeChanStreamQ)
	test.That(t, out.Samples[0].Op, test.ShouldEqual, pipeOpEnqueue)
	test.That(t, out.Samples[0].Len, test.ShouldEqual, 3)
	test.That(t, out.Samples[0].Cap, test.ShouldEqual, 10)

	test.That(t, len(out.Events), test.ShouldEqual, 1)
	test.That(t, out.Events[0].Kind, test.ShouldEqual, pipeEventSessionOpen)

	test.That(t, len(out.Timings), test.ShouldEqual, 1)
	test.That(t, out.Timings[0].Kind, test.ShouldEqual, pipeTimingExtend)
	test.That(t, out.Timings[0].Ms, test.ShouldAlmostEqual, 5.0, 0.5)

	test.That(t, len(out.Velocities), test.ShouldEqual, 1)
	test.That(t, out.Velocities[0].DegPerSec, test.ShouldEqual, 42.0)

	// snapshot returns a copy: further recording must not mutate the earlier snapshot.
	trace.record(pipeChanStreamQ, pipeOpDequeue, 0, 10)
	test.That(t, len(out.Samples), test.ShouldEqual, 1)
}
