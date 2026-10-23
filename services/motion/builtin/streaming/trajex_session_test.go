//go:build !windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt

package streaming

import (
	"context"
	"errors"
	"testing"
	"time"

	trajex "github.com/viam-modules/trajex/go"
	totgstream "github.com/viam-modules/trajex/go/totg/streaming"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
)

func testStreamOptions() StreamOptions {
	return NewStreamOptions(motion.TempStreamOptions{
		MoveOptions: &arm.MoveOptions{MaxVelRads: testVelLimitRadPerSec, MaxAccRads: testAccelLimitRadPerSec2},
	})
}

const testMaxTargetsDroppedInARowDueToTrajexExtendError = 3

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

// TestTrajexSessionStartSessionRejectsMismatchedJointLimits checks that startSession errors when
// MaxVelRadsJoints/MaxAccRadsJoints don't have exactly one entry per joint.
func TestTrajexSessionStartSessionRejectsMismatchedJointLimits(t *testing.T) {
	seed := []referenceframe.Input{0, 0}

	opts := testStreamOptions()
	opts.MoveOptions.MaxVelRadsJoints = []float64{1, 2, 3}
	s := &trajexSession{opts: opts, diagnostics: diagnostics.New(0)}
	test.That(t, s.startSession(seed), test.ShouldNotBeNil)

	opts = testStreamOptions()
	opts.MoveOptions.MaxAccRadsJoints = []float64{1, 2, 3}
	s = &trajexSession{opts: opts, diagnostics: diagnostics.New(0)}
	test.That(t, s.startSession(seed), test.ShouldNotBeNil)
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

// failingExtend wraps the session's real Extend so the calls whose (1-based) index is in failAt
// return err, leaving the session untouched as a failed Extend does.
func failingExtend(s *trajexSession, err error, failAt ...int) {
	realExtend := s.extend
	calls := 0
	s.extend = func(ctx context.Context, batch *trajex.TensorMap) (totgstream.ExtendResult, error) {
		calls++
		for _, n := range failAt {
			if calls == n {
				return totgstream.ExtendResult{}, err
			}
		}
		return realExtend(ctx, batch)
	}
}

func TestTrajexSessionDropsTargetsOnTrajexExtendError(t *testing.T) {
	ctx := context.Background()
	extendErr := errors.New("trajex/totg/streaming: Extend failed: simulated trajex error")

	newSession := func(t *testing.T) *trajexSession {
		t.Helper()
		opts := testStreamOptions()
		opts.MaxTargetsDroppedInARowDueToTrajexExtendError = testMaxTargetsDroppedInARowDueToTrajexExtendError
		s := &trajexSession{logger: logging.NewTestLogger(t), opts: opts, diagnostics: diagnostics.New(0)}
		test.That(t, s.startSession([]referenceframe.Input{0, 0}), test.ShouldBeNil)
		t.Cleanup(s.close)
		return s
	}

	t.Run("a rejected target is skipped and the session continues from the last accepted one", func(t *testing.T) {
		s := newSession(t)
		failingExtend(s, extendErr, 2)

		test.That(t, s.addJointPositionsToSession(ctx, []referenceframe.Input{0.2, 0.1}), test.ShouldBeNil)
		test.That(t, s.diagnostics.Stats().JointPositionTargetsDroppedDueToTrajexExtendError, test.ShouldEqual, 0)

		test.That(t, s.addJointPositionsToSession(ctx, []referenceframe.Input{0.4, 0.2}), test.ShouldBeNil)
		test.That(t, s.diagnostics.Stats().JointPositionTargetsDroppedDueToTrajexExtendError, test.ShouldEqual, 1)
		test.That(t, s.lastJointPositions, test.ShouldResemble, []referenceframe.Input{0.2, 0.1})

		test.That(t, s.addJointPositionsToSession(ctx, []referenceframe.Input{0.6, 0.3}), test.ShouldBeNil)
		test.That(t, s.diagnostics.Stats().JointPositionTargetsDroppedDueToTrajexExtendError, test.ShouldEqual, 1)
		test.That(t, s.lastJointPositions, test.ShouldResemble, []referenceframe.Input{0.6, 0.3})

		pvats, err := s.sampleAtLeast(ctx, sampleHorizon)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(pvats), test.ShouldBeGreaterThan, 0)
	})

	t.Run("with no skips allowed, an Extend error is returned", func(t *testing.T) {
		s := newSession(t)
		s.opts.MaxTargetsDroppedInARowDueToTrajexExtendError = 0
		failingExtend(s, errors.New("arm faulted"), 1)

		test.That(t, s.addJointPositionsToSession(ctx, []referenceframe.Input{0.2, 0.1}), test.ShouldNotBeNil)
		test.That(t, s.diagnostics.Stats().JointPositionTargetsDroppedDueToTrajexExtendError, test.ShouldEqual, 0)
	})

	t.Run("targets rejected too many times in a row end the session", func(t *testing.T) {
		s := newSession(t)
		failingExtend(s, extendErr, 1, 2, 3, 4)

		maxDropped := s.opts.MaxTargetsDroppedInARowDueToTrajexExtendError
		for i := range maxDropped {
			test.That(t, s.addJointPositionsToSession(ctx, []referenceframe.Input{float64(i+1) * 0.1, 0}), test.ShouldBeNil)
		}
		test.That(t, s.diagnostics.Stats().JointPositionTargetsDroppedDueToTrajexExtendError, test.ShouldEqual, maxDropped)
		test.That(t, s.addJointPositionsToSession(ctx, []referenceframe.Input{0.9, 0}), test.ShouldNotBeNil)
	})

	t.Run("an accepted target resets the count", func(t *testing.T) {
		s := newSession(t)
		failingExtend(s, extendErr, 1, 2, 3, 5, 6, 7)

		wantDropped := int64(0)
		for _, step := range []struct {
			target  []referenceframe.Input
			dropped bool
		}{
			{[]referenceframe.Input{0.1, 0}, true},
			{[]referenceframe.Input{0.2, 0}, true},
			{[]referenceframe.Input{0.3, 0}, true},
			{[]referenceframe.Input{0.4, 0}, false},
			{[]referenceframe.Input{0.5, 0}, true},
			{[]referenceframe.Input{0.6, 0}, true},
			{[]referenceframe.Input{0.7, 0}, true},
		} {
			test.That(t, s.addJointPositionsToSession(ctx, step.target), test.ShouldBeNil)
			if step.dropped {
				wantDropped++
			}
			test.That(t, s.diagnostics.Stats().JointPositionTargetsDroppedDueToTrajexExtendError, test.ShouldEqual, wantDropped)
		}
	})
}

func TestTrajexSessionStagesAfterSlowExtend(t *testing.T) {
	ctx := context.Background()
	diag := diagnostics.New(time.Minute)
	opts := testStreamOptions()
	opts.ForceRebaseIfExtendLatencyExceedsMs = 15
	maxExtendLatency := time.Duration(opts.ForceRebaseIfExtendLatencyExceedsMs) * time.Millisecond
	s := &trajexSession{opts: opts, diagnostics: diag}
	test.That(t, s.startSession([]referenceframe.Input{0}), test.ShouldBeNil)
	defer s.close()

	lastKind := func() string {
		extends := diag.LastWindowDetails().TrajexExtends
		return extends[len(extends)-1].Kind
	}

	test.That(t, s.addJointPositionsToSession(ctx, []referenceframe.Input{0.2}), test.ShouldBeNil)
	s.stageIfSlow(maxExtendLatency / 2)
	test.That(t, s.addJointPositionsToSession(ctx, []referenceframe.Input{0.4}), test.ShouldBeNil)
	test.That(t, lastKind(), test.ShouldEqual, "pivot")

	s.stageIfSlow(2 * maxExtendLatency)
	test.That(t, s.addJointPositionsToSession(ctx, []referenceframe.Input{0.6}), test.ShouldBeNil)
	test.That(t, lastKind(), test.ShouldEqual, "staged_again")

	// Draining the session leaves nothing to pivot from, so the next batch stages; sampling it
	// is what rebases, after which Extend pivots again.
	_, err := s.sampleAtLeast(ctx, sampleHorizon)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s.addJointPositionsToSession(ctx, []referenceframe.Input{0.8}), test.ShouldBeNil)
	test.That(t, lastKind(), test.ShouldEqual, "staged_branch_sampled")
	_, err = s.sampleAtLeast(ctx, 50*time.Millisecond)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s.addJointPositionsToSession(ctx, []referenceframe.Input{1.0}), test.ShouldBeNil)
	test.That(t, lastKind(), test.ShouldEqual, "pivot")
}
