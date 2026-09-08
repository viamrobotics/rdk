package ik

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
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestNloptFixedJoint(t *testing.T) {
	logger := logging.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// pin joint 0 at the model's real maximum, as happens when a planner freezes a
	// non-moving joint whose start position is at a limit (e.g. an open gripper). nlopt
	// can't handle zero-range bounds, so Solve nudges the window's upper bound past the
	// real max. The cost function pulls joint 0 toward a target beyond that limit, the
	// way a real goal metric can; emitted solutions must be clamped back to the real
	// limits or they fail the model's own validation downstream.
	dof := m.DoF()
	seed := []float64{dof[0].Max, 1, -1, 1, 1, 0}
	limits := make([]referenceframe.Limit, len(dof))
	copy(limits, dof)
	limits[0] = referenceframe.Limit{Min: seed[0], Max: seed[0]}

	target := append([]float64{}, seed...)
	target[0]++ // beyond the joint 0 limit
	solveFunc := func(_ context.Context, inputs []float64) float64 {
		total := 0.0
		for i, v := range inputs {
			d := v - target[i]
			total += d * d
		}
		return total
	}

	ik, err := CreateNloptSolver(logger, -1, false, true, time.Second)
	test.That(t, err, test.ShouldBeNil)

	var totalAttempts atomic.Int32
	solutions, _, err := DoSolve(context.Background(), ik, &totalAttempts, solveFunc, [][]float64{seed}, [][]referenceframe.Limit{limits})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(solutions), test.ShouldBeGreaterThan, 0)
	for _, sol := range solutions {
		test.That(t, sol[0], test.ShouldEqual, seed[0])
		_, err = m.Interpolate(sol, sol, 0.5)
		test.That(t, err, test.ShouldBeNil)
	}
}

func TestCreateNloptSolver(t *testing.T) {
	logger := logging.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// matches xarm home end effector position
	pos := spatialmath.NewPoseFromPoint(r3.Vector{X: 207, Z: 112})
	seed := []float64{1, 1, -1, 1, 1, 0}
	solveFunc := NewMetricMinFunc(motionplan.NewScaledSquaredNormMetric(pos, 100), m, logger)

	t.Run("not exact", func(t *testing.T) {
		ik, err := CreateNloptSolver(logger, -1, false, true, time.Second)
		test.That(t, err, test.ShouldBeNil)

		var totalAttempts atomic.Int32
		_, _, err = DoSolve(context.Background(), ik, &totalAttempts, solveFunc, [][]float64{seed}, [][]referenceframe.Limit{m.DoF()})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("exact", func(t *testing.T) {
		ik, err := CreateNloptSolver(logger, -1, true, true, time.Second)
		test.That(t, err, test.ShouldBeNil)

		var totalAttempts atomic.Int32
		_, meta, err := DoSolve(context.Background(), ik, &totalAttempts, solveFunc, [][]float64{seed}, [][]referenceframe.Limit{m.DoF()})
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
		solveFunc = NewMetricMinFunc(motionplan.NewSquaredNormMetric(pos), m, logger)

		ik, err := CreateNloptSolver(logger, -1, false, true, time.Second)
		test.That(t, err, test.ShouldBeNil)

		var totalAttempts atomic.Int32
		_, _, err = DoSolve(context.Background(), ik, &totalAttempts, solveFunc, [][]float64{seed}, [][]referenceframe.Limit{m.DoF()})
		test.That(t, err, test.ShouldBeNil)
	})
}

func BenchmarkNloptSolve(b *testing.B) {
	logger := logging.NewTestLogger(b)
	logger.SetLevel(logging.INFO)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "")
	test.That(b, err, test.ShouldBeNil)

	seed := []float64{1, 1, -1, 1, 1, 0}
	pos := spatialmath.NewPoseFromPoint(r3.Vector{X: 207, Z: 112})
	solveFunc := NewMetricMinFunc(motionplan.NewScaledSquaredNormMetric(pos, 100), m, logger)

	ik, err := CreateNloptSolver(logger, -1, false, true, time.Second)
	test.That(b, err, test.ShouldBeNil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var totalAttempts atomic.Int32
		_, _, err := DoSolve(context.Background(), ik, &totalAttempts, solveFunc,
			[][]float64{seed}, [][]referenceframe.Limit{m.DoF()})
		test.That(b, err, test.ShouldBeNil)
	}
}
