package nloptik

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestNloptFixedJoint(t *testing.T) {
	logger := logging.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	seed := []float64{1, 1, -1, 1, 1, 0}
	pos := spatialmath.NewPoseFromPoint(r3.Vector{X: 207, Z: 112})
	solveFunc := ik.NewMetricMinFunc(motionplan.NewScaledSquaredNormMetric(pos, 100), m, logger)

	dof := m.DoF()
	limits := make([]referenceframe.Limit, len(dof))
	copy(limits, dof)
	limits[0] = referenceframe.Limit{Min: seed[0], Max: seed[0]}

	solver, err := CreateNloptSolver(logger, -1, false, true, time.Second)
	test.That(t, err, test.ShouldBeNil)

	var totalAttempts atomic.Int32
	solutions, _, err := ik.DoSolve(context.Background(), solver, &totalAttempts, solveFunc, [][]float64{seed}, [][]referenceframe.Limit{limits})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(solutions), test.ShouldBeGreaterThan, 0)
	for _, sol := range solutions {
		// joint 0 is pinned - it may move at most one epsilon nudge upward
		test.That(t, sol[0], test.ShouldBeGreaterThanOrEqualTo, seed[0])
		test.That(t, sol[0], test.ShouldBeLessThanOrEqualTo, seed[0]+defaultGoalThreshold)
	}
}

func TestCreateNloptSolver(t *testing.T) {
	logger := logging.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// matches xarm home end effector position
	pos := spatialmath.NewPoseFromPoint(r3.Vector{X: 207, Z: 112})
	seed := []float64{1, 1, -1, 1, 1, 0}
	solveFunc := ik.NewMetricMinFunc(motionplan.NewScaledSquaredNormMetric(pos, 100), m, logger)

	t.Run("not exact", func(t *testing.T) {
		solver, err := CreateNloptSolver(logger, -1, false, true, time.Second)
		test.That(t, err, test.ShouldBeNil)

		var totalAttempts atomic.Int32
		_, _, err = ik.DoSolve(context.Background(), solver, &totalAttempts, solveFunc, [][]float64{seed}, [][]referenceframe.Limit{m.DoF()})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("exact", func(t *testing.T) {
		solver, err := CreateNloptSolver(logger, -1, true, true, time.Second)
		test.That(t, err, test.ShouldBeNil)

		var totalAttempts atomic.Int32
		_, meta, err := ik.DoSolve(context.Background(), solver, &totalAttempts, solveFunc, [][]float64{seed}, [][]referenceframe.Limit{m.DoF()})
		test.That(t, err, test.ShouldBeNil)
		for idx, m := range meta {
			logger.Debugf("seed: %d %#v", idx, m)
		}
	})

	t.Run("Check unpacking from proto", func(t *testing.T) {
		pos = spatialmath.NewPose(
			r3.Vector{X: -46, Y: -23, Z: 372},
			&spatialmath.OrientationVectorDegrees{Theta: 0, OX: 0, OY: 0, OZ: -1},
		)

		seed = m.InputFromProtobuf(&pb.JointPositions{Values: []float64{49, 28, -101, 0, -73, 0}})
		solveFunc = ik.NewMetricMinFunc(motionplan.NewSquaredNormMetric(pos), m, logger)

		solver, err := CreateNloptSolver(logger, -1, false, true, time.Second)
		test.That(t, err, test.ShouldBeNil)

		var totalAttempts atomic.Int32
		_, _, err = ik.DoSolve(context.Background(), solver, &totalAttempts, solveFunc, [][]float64{seed}, [][]referenceframe.Limit{m.DoF()})
		test.That(t, err, test.ShouldBeNil)
	})
}
