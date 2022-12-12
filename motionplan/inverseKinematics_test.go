package motionplan

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestIKConfiguration(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ikConfig := make(map[string]interface{}, 0)

	// test ability to change the number of threads used, this should also change which struct makes up the InverseKinematicsSolver
	threadCases := []struct {
		config   int
		expected int
	}{
		{defaultNumThreads, defaultNumThreads},
		{1, 1},
		{10, 10},
		{0, defaultNumThreads},
		{runtime.NumCPU() + 1, runtime.NumCPU() + 1},
	}
	for _, tc := range threadCases {
		t.Run(fmt.Sprintf("IK configured with %d threads", tc), func(t *testing.T) {
			ikConfig["num_threads"] = tc.config
			ik, err := newIKSolver(m, logger, ikConfig)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ik.options().NumThreads, test.ShouldEqual, tc.expected)
			if tc.expected == 1 {
				_, ok := ik.(*nloptIKSolver)
				test.That(t, ok, test.ShouldBeTrue)
			} else {
				ensemble, ok := ik.(*ensembleIKSolver)
				test.That(t, ok, test.ShouldBeTrue)
				test.That(t, len(ensemble.solvers), test.ShouldEqual, tc.expected)
			}
		})
	}

	t.Run("IK configured with user-specified opts", func(t *testing.T) {
		expected := defaultMinIkScore + 0.1
		ikConfig["min_ik_score"] = expected
		ik, err := newIKSolver(m, logger, ikConfig)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ik.options().MinScore, test.ShouldAlmostEqual, expected)
	})

	t.Run("IK configured with user-specified algOpts", func(t *testing.T) {
		expected := defaultMaxIterations + 1
		ikConfig["iterations"] = expected
		ikConfig["num_threads"] = 1
		ik, err := newIKSolver(m, logger, ikConfig)
		test.That(t, err, test.ShouldBeNil)
		nlopt, ok := ik.(*nloptIKSolver)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, nlopt.algOpts.MaxIterations, test.ShouldAlmostEqual, expected)
	})
}

func TestNLOptIKSolver(t *testing.T) {
	nSolutions := 10
	logger := golog.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	goal, err := ComputePosition(m, referenceframe.JointPositionsFromRadians([]float64{-4.128, 2.71, 2.798, 2.3, 1.291, 0.62}))
	test.That(t, err, test.ShouldBeNil)
	ikConfig := make(map[string]interface{}, 0)
	ikConfig["num_threads"] = 1
	solutions, err := BestIKSolutions(context.Background(), m, logger, goal, home, &referenceframe.WorldState{}, ikConfig, 1, nSolutions)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(solutions), test.ShouldEqual, nSolutions)
}

func TestEnsembleIKSolver(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/trossen/trossen_wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// Test ability to arrive at another position
	pos := spatialmath.NewPoseFromOrientation(
		r3.Vector{X: -46, Y: -133, Z: 372},
		&spatialmath.OrientationVectorDegrees{OX: 1.79, OY: -1.32, OZ: -1.11},
	)

	ctx := context.Background()
	solutions, err := BestIKSolutions(ctx, m, logger, pos, home, &referenceframe.WorldState{}, map[string]interface{}{}, 1, 1)
	test.That(t, err, test.ShouldBeNil)
	newPos, err := m.Transform(solutions[0])
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostEqualEps(pos, newPos, 1e-3), test.ShouldBeTrue)

	// Test moving forward 20 in X direction from previous position
	pos = spatialmath.NewPoseFromOrientation(
		r3.Vector{X: -66, Y: -133, Z: 372},
		&spatialmath.OrientationVectorDegrees{OX: 1.78, OY: -3.3, OZ: -1.11},
	)
	solutions, err = BestIKSolutions(ctx, m, logger, pos, solutions[0], &referenceframe.WorldState{}, map[string]interface{}{}, 1, 100)
	test.That(t, err, test.ShouldBeNil)
	for _, solution := range solutions {
		newPos, err := m.Transform(solution)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostEqualEps(pos, newPos, 1e-3), test.ShouldBeTrue)
	}
}
