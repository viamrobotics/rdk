//go:build !windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt

package streaming

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
)

func testStreamOptions() StreamOptions {
	opts := NewDefaultOptions()
	opts.VelLimitDegPerSec = 90
	opts.AccelLimitDegPerSec2 = 90
	return opts
}

// sampleHorizon is far longer than any trajectory these tests plan, so sampling with it
// pulls out the session's entire remaining trajectory.
const sampleHorizon = time.Hour

func TestInputsWithin(t *testing.T) {
	a := []referenceframe.Input{0, 1, 2}
	test.That(t, inputsWithin(a, []referenceframe.Input{0, 1, 2}, 1e-4), test.ShouldBeTrue)
	test.That(t, inputsWithin(a, []referenceframe.Input{0, 1, 2.00005}, 1e-4), test.ShouldBeTrue)
	test.That(t, inputsWithin(a, []referenceframe.Input{0, 1, 2.001}, 1e-4), test.ShouldBeFalse)
	test.That(t, inputsWithin(a, []referenceframe.Input{0, 1}, 1e-4), test.ShouldBeFalse)
}

// TestTrajexSessionSamplesTowardTarget checks that a trajexSession started at a seed and extended
// toward a new target produces a non-empty stream of PVATs, at the configured dof, with
// non-decreasing trajectory time, that settles at the target.
func TestTrajexSessionSamplesTowardTarget(t *testing.T) {
	ctx := context.Background()
	seed := []referenceframe.Input{0, 0}
	target := []referenceframe.Input{0.5, -0.3}

	s := &trajexSession{opts: testStreamOptions(), diagnostics: diagnostics.New(0)}
	test.That(t, s.startSession(seed), test.ShouldBeNil)
	defer s.close()

	test.That(t, s.addJointPositionsToSession(ctx, target), test.ShouldBeNil)

	pvats, err := s.sampleAtLeast(ctx, sampleHorizon)
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
// waypointDedupEps of the current position is a no-op rather than an error.
func TestTrajexSessionAddJointPositionsDedups(t *testing.T) {
	ctx := context.Background()
	seed := []referenceframe.Input{0.2, 0.4}

	s := &trajexSession{opts: testStreamOptions(), diagnostics: diagnostics.New(0)}
	test.That(t, s.startSession(seed), test.ShouldBeNil)
	defer s.close()

	nearlyIdentical := []referenceframe.Input{0.2 + 1e-6, 0.4 - 1e-6}
	test.That(t, s.addJointPositionsToSession(ctx, nearlyIdentical), test.ShouldBeNil)
	test.That(t, s.lastJointPositions, test.ShouldResemble, seed)

	pvats, err := s.sampleAtLeast(ctx, sampleHorizon)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(pvats), test.ShouldEqual, 0)
}

// TestTrajexSessionRunwayTracksStagedBacklog covers the runway the gate reads in both of the
// states a session can be in: with unsampled active trajectory, which the session reports
// exactly, and with staged batches, which the session cannot put a duration on and which the
// velocity-limit estimate covers until the rebase absorbs them. Without the estimate the gate
// would reopen while the active trajectory drained even as staged backlog kept growing, which
// is the field failure this exists to prevent.
func TestTrajexSessionRunwayTracksStagedBacklog(t *testing.T) {
	ctx := context.Background()
	opts := testStreamOptions()
	opts.VelLimitDegPerSec = 10 // slow, so trajectories are long relative to sampling noise

	s := &trajexSession{opts: opts, diagnostics: diagnostics.New(0)}
	test.That(t, s.startSession([]referenceframe.Input{0}), test.ShouldBeNil)
	defer s.close()
	test.That(t, s.trajexRunway(), test.ShouldEqual, time.Duration(0))

	// A 0.35 rad move at a 10 deg/s limit is roughly 2s of backlog, none of it sampled, and
	// all of it in the active trajectory.
	test.That(t, s.addJointPositionsToSession(ctx, []referenceframe.Input{0.35}), test.ShouldBeNil)
	test.That(t, s.stagedEstimate, test.ShouldEqual, time.Duration(0))
	test.That(t, s.trajexRunway(), test.ShouldBeGreaterThan, 1500*time.Millisecond)

	// Sampling shrinks it.
	_, err := s.sampleAtLeast(ctx, s.trajexRunway()-100*time.Millisecond)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s.trajexRunway(), test.ShouldBeLessThan, 300*time.Millisecond)

	// Drain the active trajectory entirely, then extend twice: each branch necessarily falls
	// at or behind the watermark, so both batches stage. The session's own remainder reads
	// zero, but the runway must still report the staged motion: 0.35 rad back to 0 and then
	// 0.2 rad on, at 10 deg/s, is a 3.15s velocity-limit bound.
	_, err = s.sampleAtLeast(ctx, sampleHorizon)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s.addJointPositionsToSession(ctx, []referenceframe.Input{0}), test.ShouldBeNil)
	test.That(t, s.sess.RemainingActiveDuration(), test.ShouldBeLessThan, time.Millisecond)
	test.That(t, s.trajexRunway(), test.ShouldBeGreaterThan, 1900*time.Millisecond)
	test.That(t, s.trajexRunway(), test.ShouldBeLessThan, 2100*time.Millisecond)
	test.That(t, s.addJointPositionsToSession(ctx, []referenceframe.Input{0.2}), test.ShouldBeNil)
	staged := s.trajexRunway()
	test.That(t, staged, test.ShouldBeGreaterThan, 3000*time.Millisecond)
	test.That(t, staged, test.ShouldBeLessThan, 3300*time.Millisecond)

	// The next sample fires the rebase: the staged batches become the active trajectory, the
	// estimate is dropped, and the exact remainder takes over. Acceleration ramps only add
	// time, so (allowing for the ~10ms just sampled) the estimate must not have exceeded what
	// TOTG actually assigned.
	pvats, err := s.sampleAtLeast(ctx, 10*time.Millisecond)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(pvats), test.ShouldBeGreaterThan, 0)
	test.That(t, s.stagedEstimate, test.ShouldEqual, time.Duration(0))
	test.That(t, s.trajexRunway(), test.ShouldBeGreaterThan, staged-100*time.Millisecond)

	// Draining the rebased trajectory takes it back to zero.
	_, err = s.sampleAtLeast(ctx, sampleHorizon)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s.trajexRunway(), test.ShouldBeLessThan, 20*time.Millisecond)
}

// TestTrajexSessionSampleHorizon checks that sampleAtLeast advances the watermark by only
// (approximately) the requested horizon per call rather than sampling the full trajectory, and
// that consecutive calls continue from where the previous one left off.
func TestTrajexSessionSampleHorizon(t *testing.T) {
	ctx := context.Background()
	seed := []referenceframe.Input{0, 0}
	target := []referenceframe.Input{0.5, 0.5}

	s := &trajexSession{opts: testStreamOptions(), diagnostics: diagnostics.New(0)}
	test.That(t, s.startSession(seed), test.ShouldBeNil)
	defer s.close()

	test.That(t, s.addJointPositionsToSession(ctx, target), test.ShouldBeNil)

	horizon := 20 * time.Millisecond
	first, err := s.sampleAtLeast(ctx, horizon)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(first), test.ShouldBeGreaterThan, 0)
	test.That(t, len(first[0].positions), test.ShouldEqual, len(seed))
	// The last sample reaches the horizon but not far past it (within one sample period).
	test.That(t, first[len(first)-1].time, test.ShouldBeGreaterThanOrEqualTo, horizon)
	test.That(t, first[len(first)-1].time, test.ShouldBeLessThan, horizon+20*time.Millisecond)

	// A second call continues from the watermark rather than restarting.
	second, err := s.sampleAtLeast(ctx, horizon)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(second), test.ShouldBeGreaterThan, 0)
	test.That(t, second[0].time, test.ShouldBeGreaterThan, first[len(first)-1].time)
}
