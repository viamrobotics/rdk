package sim

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestDedupWaypoints(t *testing.T) {
	in := [][]float64{
		{0, 0},
		{0, 0},
		{1, 0},
		{1, 0},
		{1, 1},
	}
	out := dedupWaypoints(in, defaultDedupToleranceRads)
	test.That(t, len(out), test.ShouldEqual, 3)
	test.That(t, out[0], test.ShouldResemble, []float64{0, 0})
	test.That(t, out[1], test.ShouldResemble, []float64{1, 0})
	test.That(t, out[2], test.ShouldResemble, []float64{1, 1})
}

func TestDedupWaypointsRespectsTolerance(t *testing.T) {
	// Two waypoints that differ by less than the tolerance should collapse.
	in := [][]float64{
		{0.0, 0.0},
		{0.0, 1e-7},
		{1.0, 0.0},
	}
	out := dedupWaypoints(in, 1e-5)
	test.That(t, len(out), test.ShouldEqual, 2)
}

func TestFakeSingleSegment(t *testing.T) {
	gen := newFakeTrajectoryGenerator(logging.NewTestLogger(t))

	waypoints := [][]float64{
		{0, 0, 0, 0, 0, 0},
		{1, -2, 0, 0, 0, 0},
	}
	traj, err := gen.Plan(context.Background(), waypoints, 1.0, 1000.0, 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, traj.nDof, test.ShouldEqual, 6)

	// Max excursion = 2 rad, velLimit = 1 rad/s => duration = 2 s.
	// n_samples = ceil(2 * 100) + 1 = 201; dt = 2 / 200 = 0.01 s.
	test.That(t, len(traj.sampleTimes), test.ShouldEqual, 201)
	test.That(t, traj.sampleTimes[0], test.ShouldEqual, 0.0)
	test.That(t, traj.sampleTimes[200], test.ShouldAlmostEqual, 2.0, 1e-9)
	test.That(t, traj.sampleTimes[100], test.ShouldAlmostEqual, 1.0, 1e-9)

	// At t=1.0 (half-way through the single segment): (0.5, -1, 0, 0, 0, 0).
	for j, want := range []float64{0.5, -1, 0, 0, 0, 0} {
		test.That(t, traj.sampleConfigs[100*6+j], test.ShouldAlmostEqual, want, 1e-9)
	}
	// Final sample is the target.
	for j, want := range []float64{1, -2, 0, 0, 0, 0} {
		test.That(t, traj.sampleConfigs[200*6+j], test.ShouldAlmostEqual, want, 1e-9)
	}
}

func TestFakeMultiSegment(t *testing.T) {
	gen := newFakeTrajectoryGenerator(logging.NewTestLogger(t))

	// Two segments: {0,0} -> {1,0} -> {1,1}.
	// Each segment has max excursion 1, duration 1 s. Total 2 s.
	waypoints := [][]float64{
		{0, 0},
		{1, 0},
		{1, 1},
	}
	traj, err := gen.Plan(context.Background(), waypoints, 1.0, 1.0, 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, traj.nDof, test.ShouldEqual, 2)
	test.That(t, len(traj.sampleTimes), test.ShouldEqual, 201)

	// Mid segment 0 (t=0.5): (0.5, 0).
	test.That(t, traj.sampleConfigs[50*2+0], test.ShouldAlmostEqual, 0.5, 1e-9)
	test.That(t, traj.sampleConfigs[50*2+1], test.ShouldAlmostEqual, 0.0, 1e-9)
	// Segment boundary (t=1.0): (1, 0).
	test.That(t, traj.sampleConfigs[100*2+0], test.ShouldAlmostEqual, 1.0, 1e-9)
	test.That(t, traj.sampleConfigs[100*2+1], test.ShouldAlmostEqual, 0.0, 1e-9)
	// Mid segment 1 (t=1.5): (1, 0.5).
	test.That(t, traj.sampleConfigs[150*2+0], test.ShouldAlmostEqual, 1.0, 1e-9)
	test.That(t, traj.sampleConfigs[150*2+1], test.ShouldAlmostEqual, 0.5, 1e-9)
	// Final sample (t=2.0): (1, 1).
	test.That(t, traj.sampleConfigs[200*2+0], test.ShouldAlmostEqual, 1.0, 1e-9)
	test.That(t, traj.sampleConfigs[200*2+1], test.ShouldAlmostEqual, 1.0, 1e-9)
}

func TestFakeScalesPerJointWithinSegment(t *testing.T) {
	gen := newFakeTrajectoryGenerator(logging.NewTestLogger(t))

	// Single segment {0,0} -> {1,2}. Max excursion = 2 (joint 1). Duration = 2 s.
	// Joint 0 travels at half the rate of joint 1.
	waypoints := [][]float64{
		{0, 0},
		{1, 2},
	}
	traj, err := gen.Plan(context.Background(), waypoints, 1.0, 1.0, 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(traj.sampleTimes), test.ShouldEqual, 201)

	// At t=1.0 (half-way): joint 0 = 0.5, joint 1 = 1.
	test.That(t, traj.sampleConfigs[100*2+0], test.ShouldAlmostEqual, 0.5, 1e-9)
	test.That(t, traj.sampleConfigs[100*2+1], test.ShouldAlmostEqual, 1.0, 1e-9)
}

func TestFakeDedupsInternally(t *testing.T) {
	gen := newFakeTrajectoryGenerator(logging.NewTestLogger(t))

	// First two waypoints are identical and should be collapsed.
	waypoints := [][]float64{
		{0, 0},
		{0, 0},
		{1, 0},
	}
	traj, err := gen.Plan(context.Background(), waypoints, 1.0, 1.0, 0)
	test.That(t, err, test.ShouldBeNil)
	// After dedup, single segment of duration 1 s.
	test.That(t, traj.sampleTimes[len(traj.sampleTimes)-1], test.ShouldAlmostEqual, 1.0, 1e-9)
}

func TestFakeAllDuplicatesReturnsTrivialTrajectory(t *testing.T) {
	gen := newFakeTrajectoryGenerator(logging.NewTestLogger(t))

	waypoints := [][]float64{
		{0, 0},
		{0, 0},
		{0, 0},
	}
	traj, err := gen.Plan(context.Background(), waypoints, 1.0, 1.0, 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(traj.sampleTimes), test.ShouldEqual, 1)
	test.That(t, traj.sampleTimes[0], test.ShouldEqual, 0.0)
	test.That(t, traj.sampleConfigs, test.ShouldResemble, []float64{0, 0})
}

func TestFakePathToleranceWarnsButDoesNotError(t *testing.T) {
	gen := newFakeTrajectoryGenerator(logging.NewTestLogger(t))

	// Two successive calls with positive pathTolerance should both succeed.
	// (The warn-once behavior itself isn't asserted; it's a UX nicety.)
	waypoints := [][]float64{{0}, {1}}
	_, err := gen.Plan(context.Background(), waypoints, 1.0, 1.0, 0.1)
	test.That(t, err, test.ShouldBeNil)
	_, err = gen.Plan(context.Background(), waypoints, 1.0, 1.0, 0.1)
	test.That(t, err, test.ShouldBeNil)
}

func TestFakeErrorsOnNonPositiveVelLimit(t *testing.T) {
	gen := newFakeTrajectoryGenerator(logging.NewTestLogger(t))

	_, err := gen.Plan(context.Background(), [][]float64{{0}, {1}}, 0, 1, 0)
	test.That(t, err, test.ShouldNotBeNil)

	_, err = gen.Plan(context.Background(), [][]float64{{0}, {1}}, -1, 1, 0)
	test.That(t, err, test.ShouldNotBeNil)
}
