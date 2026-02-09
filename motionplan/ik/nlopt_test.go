package ik

import (
	"context"
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

func TestCreateNloptSolver(t *testing.T) {
	logger := logging.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// matches xarm home end effector position
	pos := spatialmath.NewPoseFromPoint(r3.Vector{X: 207, Z: 112})
	seed := []float64{1, 1, -1, 1, 1, 0}
	solveFunc := NewMetricMinFunc(motionplan.NewScaledSquaredNormMetric(pos, 10, motionplan.TranslationCloud{}), m, logger)

	t.Run("not exact", func(t *testing.T) {
		ik, err := CreateNloptSolver(logger, -1, false, true, time.Second)
		test.That(t, err, test.ShouldBeNil)

		_, _, err = DoSolve(context.Background(), ik, solveFunc, [][]float64{seed}, [][]referenceframe.Limit{m.DoF()})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("exact", func(t *testing.T) {
		ik, err := CreateNloptSolver(logger, -1, true, true, time.Second)
		test.That(t, err, test.ShouldBeNil)

		_, meta, err := DoSolve(context.Background(), ik, solveFunc, [][]float64{seed}, [][]referenceframe.Limit{m.DoF()})
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

		_, _, err = DoSolve(context.Background(), ik, solveFunc, [][]float64{seed}, [][]referenceframe.Limit{m.DoF()})
		test.That(t, err, test.ShouldBeNil)
	})
}
