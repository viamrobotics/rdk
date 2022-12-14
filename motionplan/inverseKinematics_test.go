package motionplan

import (
	"context"
	"fmt"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestIKConfiguration(t *testing.T) {
	// create function to get ik object constructed from config map
	newTestIKSolver := func(cfg map[string]interface{}) (inverseKinematicsSolver, error) {
		logger := golog.NewTestLogger(t)
		goal := spatialmath.NewZeroPose()
		m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/universalrobots/ur5e.json"), "")
		if err != nil {
			return nil, err
		}
		fs := referenceframe.NewEmptySimpleFrameSystem("")
		if err = fs.AddFrame(m, fs.Frame(referenceframe.World)); err != nil {
			return nil, err
		}
		inputMap := referenceframe.StartPositions(fs)
		opt, err := newPlanManager(logger, fs, m, inputMap, goal, &referenceframe.WorldState{}, 1, cfg).plannerOptionsFromConfig(nil, goal, cfg)
		if err != nil {
			return nil, err
		}
		ik, err := newIKSolver(m, logger, opt.ikOptions)
		if err != nil {
			return nil, err
		}
		return ik, err
	}

	// test ability to change the number of threads used, this should also change which struct makes up the InverseKinematicsSolver
	threadCases := []struct {
		config   int
		expected int
	}{
		{defaultNumThreads, defaultNumThreads},
		{1, 1},
		{10, 10},
		{0, defaultNumThreads},
	}
	for _, tc := range threadCases {
		t.Run(fmt.Sprintf("IK configured with %d threads", tc), func(t *testing.T) {
			ikConfig := map[string]interface{}{"num_threads": tc.config}
			ik, err := newTestIKSolver(ikConfig)
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
		ikConfig := map[string]interface{}{"min_ik_score": expected}
		ik, err := newTestIKSolver(ikConfig)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ik.options().MinScore, test.ShouldAlmostEqual, expected)
	})

	t.Run("IK configured with user-specified algOpts", func(t *testing.T) {
		expected := defaultMaxIterations + 1
		ikConfig := map[string]interface{}{"num_threads": 1, "iterations": expected}
		ik, err := newTestIKSolver(ikConfig)
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
	config := make(map[string]interface{}, 0)
	config["num_threads"] = 1
	config["max_ik_solutions"] = nSolutions
	fs := referenceframe.NewEmptySimpleFrameSystem("test")
	test.That(t, fs.AddFrame(m, fs.Frame(referenceframe.World)), test.ShouldBeNil)
	inputMap := referenceframe.StartPositions(fs)
	solutions, err := BestIKSolutions(context.Background(), logger, fs, m, inputMap, goal, &referenceframe.WorldState{}, 1, config)
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

	fs := referenceframe.NewEmptySimpleFrameSystem("test")
	test.That(t, fs.AddFrame(m, fs.Frame(referenceframe.World)), test.ShouldBeNil)
	inputMap := referenceframe.StartPositions(fs)
	motionConfig := make(map[string]interface{})
	motionConfig["max_ik_solutions"] = 1

	ctx := context.Background()
	solutions, err := BestIKSolutions(ctx, logger, fs, m, inputMap, pos, &referenceframe.WorldState{}, 1, motionConfig)
	test.That(t, err, test.ShouldBeNil)
	newPos, err := m.Transform(solutions[0])
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostEqualEps(pos, newPos, 1e-3), test.ShouldBeTrue)

	// Test moving forward 20 in X direction from previous position
	pos = spatialmath.NewPoseFromOrientation(

		r3.Vector{X: -66, Y: -133, Z: 372},
		&spatialmath.OrientationVectorDegrees{OX: 1.78, OY: -3.3, OZ: -1.11},
	)
	motionConfig["max_ik_solutions"] = 10
	inputMap[m.Name()] = solutions[0]
	solutions, err = BestIKSolutions(ctx, logger, fs, m, inputMap, pos, &referenceframe.WorldState{}, 1, motionConfig)
	test.That(t, err, test.ShouldBeNil)

	for _, solution := range solutions {
		newPos, err := m.Transform(solution)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostEqualEps(pos, newPos, 1e-3), test.ShouldBeTrue)
	}
}
