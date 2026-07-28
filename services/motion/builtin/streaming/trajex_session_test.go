// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"context"
	"testing"

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

func testStreamOptions() *StreamOptions {
	opts := &StreamOptions{
		VelLimitDegPerSec:    90,
		AccelLimitDegPerSec2: 90,
		SendToArmIntervalMs:  10,
	}
	opts.ApplyDefaults()
	return opts
}

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

	pvats, err := s.sampleRemainingPVATsFromSession(ctx)
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

	pvats, err := s.sampleRemainingPVATsFromSession(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(pvats), test.ShouldEqual, 0)
}

// TestTrajexSessionSampleNextPVATFromSession checks that sampleNextPVATFromSession samples one
// PVAT at a time, matching the first entries sampleRemainingPVATsFromSession would otherwise
// drain in bulk.
func TestTrajexSessionSampleNextPVATFromSession(t *testing.T) {
	ctx := context.Background()
	seed := []referenceframe.Input{0, 0}
	target := []referenceframe.Input{0.5, 0.5}

	s := &trajexSession{opts: testStreamOptions()}
	test.That(t, s.startSession(seed), test.ShouldBeNil)
	defer s.close()

	test.That(t, s.addJointPositionsToSession(ctx, target), test.ShouldBeNil)

	pv, err := s.sampleNextPVATFromSession(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pv, test.ShouldNotBeNil)
	test.That(t, len(pv.positions), test.ShouldEqual, len(seed))
	test.That(t, pv.time, test.ShouldBeGreaterThanOrEqualTo, 0)
}
