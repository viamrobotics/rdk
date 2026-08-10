//go:build !windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt

package streaming

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
)

func TestInputsWithin(t *testing.T) {
	a := []referenceframe.Input{0, 1, 2}
	test.That(t, inputsWithin(a, []referenceframe.Input{0, 1, 2}, 1e-4), test.ShouldBeTrue)
	test.That(t, inputsWithin(a, []referenceframe.Input{0, 1, 2.00005}, 1e-4), test.ShouldBeTrue)
	test.That(t, inputsWithin(a, []referenceframe.Input{0, 1, 2.001}, 1e-4), test.ShouldBeFalse)
	test.That(t, inputsWithin(a, []referenceframe.Input{0, 1}, 1e-4), test.ShouldBeFalse)
}

func testStreamOptions() StreamOptions {
	opts := NewDefaultOptions()
	opts.VelLimitDegPerSec = 90
	opts.AccelLimitDegPerSec2 = 90
	return opts
}

// drainHorizon is far longer than any trajectory these tests plan, so sampling with it drains
// the session's entire remaining trajectory.
const drainHorizon = time.Hour

// TestTrajexSessionSamplesTowardTarget checks that a trajexSession started at a seed and extended
// toward a new target produces a non-empty stream of PVATs, at the configured dof, with
// non-decreasing trajectory time, that settles at the target.
func TestTrajexSessionSamplesTowardTarget(t *testing.T) {
	ctx := context.Background()
	seed := []referenceframe.Input{0, 0}
	target := []referenceframe.Input{0.5, -0.3}

	s := &trajexSession{opts: testStreamOptions()}
	test.That(t, s.startSession(seed), test.ShouldBeNil)
	defer s.close()

	test.That(t, s.addJointPositionsToSession(ctx, target), test.ShouldBeNil)

	pvats, err := s.sample(ctx, drainHorizon)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(pvats), test.ShouldBeGreaterThan, 0)

	lastTime := pvats[0].time
	for _, pv := range pvats {
		test.That(t, len(pv.positions), test.ShouldEqual, len(seed))
		test.That(t, pv.time, test.ShouldBeGreaterThanOrEqualTo, lastTime)
		lastTime = pv.time
	}

	last := pvats[len(pvats)-1]
	for i, p := range last.positions {
		test.That(t, p, test.ShouldAlmostEqual, float64(target[i]), 1e-2)
	}
}

// TestTrajexSessionAddJointPositionsDedups checks that extending toward a target within
// waypointDedupEps of the current position is a no-op rather than an error (a zero-length
// waypoint segment would otherwise be sent to the trajex library).
func TestTrajexSessionAddJointPositionsDedups(t *testing.T) {
	ctx := context.Background()
	seed := []referenceframe.Input{0.2, 0.4}

	s := &trajexSession{opts: testStreamOptions()}
	test.That(t, s.startSession(seed), test.ShouldBeNil)
	defer s.close()

	nearlyIdentical := []referenceframe.Input{0.2 + 1e-6, 0.4 - 1e-6}
	test.That(t, s.addJointPositionsToSession(ctx, nearlyIdentical), test.ShouldBeNil)
	test.That(t, s.lastJointPositions, test.ShouldResemble, seed)

	pvats, err := s.sample(ctx, drainHorizon)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(pvats), test.ShouldEqual, 0)
}

// TestTrajexSessionSampleHorizon checks that sample advances the watermark by only
// (approximately) the requested horizon per call rather than draining the trajectory, and that
// consecutive calls continue from where the previous one left off. This is the property Run's
// deficit-driven sampling relies on to commit no more trajectory than the arm's runway needs.
func TestTrajexSessionSampleHorizon(t *testing.T) {
	ctx := context.Background()
	seed := []referenceframe.Input{0, 0}
	target := []referenceframe.Input{0.5, 0.5}

	s := &trajexSession{opts: testStreamOptions()}
	test.That(t, s.startSession(seed), test.ShouldBeNil)
	defer s.close()

	test.That(t, s.addJointPositionsToSession(ctx, target), test.ShouldBeNil)

	horizon := 20 * time.Millisecond
	first, err := s.sample(ctx, horizon)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(first), test.ShouldBeGreaterThan, 0)
	test.That(t, len(first[0].positions), test.ShouldEqual, len(seed))
	// The last sample reaches the horizon but not far past it (within one sample period).
	test.That(t, first[len(first)-1].time, test.ShouldBeGreaterThanOrEqualTo, horizon)
	test.That(t, first[len(first)-1].time, test.ShouldBeLessThan, horizon+20*time.Millisecond)

	// A second call continues from the watermark rather than restarting.
	second, err := s.sample(ctx, horizon)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(second), test.ShouldBeGreaterThan, 0)
	test.That(t, second[0].time, test.ShouldBeGreaterThan, first[len(first)-1].time)
}
