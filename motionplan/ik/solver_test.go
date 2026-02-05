package ik

import (
	"context"
	"math"
	"runtime"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var (
	home = [][]float64{{0, 0, 0, 0, 0, 0}}
	nCPU = int(math.Max(1.0, float64(runtime.NumCPU()/4)))
)

func TestCombinedIKinematics(t *testing.T) {
	logger := logging.NewTestLogger(t)
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(logger, nCPU, defaultGoalThreshold, time.Second)
	test.That(t, err, test.ShouldBeNil)

	// Test ability to arrive at another position
	pos := spatial.NewPose(
		r3.Vector{X: -46, Y: -133, Z: 372},
		&spatial.OrientationVectorDegrees{OX: 1.79, OY: -1.32, OZ: -1.11},
	)
	solveFunc := NewMetricMinFunc(motionplan.NewSquaredNormMetric(pos), m, logger)
	solution, _, err := DoSolve(context.Background(), ik, solveFunc, home, [][]frame.Limit{m.DoF()})
	test.That(t, err, test.ShouldBeNil)

	// Test moving forward 20 in X direction from previous position
	pos = spatial.NewPose(
		r3.Vector{X: -66, Y: -133, Z: 372},
		&spatial.OrientationVectorDegrees{OX: 1.78, OY: -3.3, OZ: -1.11},
	)
	solveFunc = NewMetricMinFunc(motionplan.NewSquaredNormMetric(pos), m, logger)
	_, _, err = DoSolve(context.Background(), ik, solveFunc, solution, [][]frame.Limit{m.DoF()})
	test.That(t, err, test.ShouldBeNil)
}

func TestUR5NloptIKinematics(t *testing.T) {
	logger := logging.NewTestLogger(t)

	m, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(logger, nCPU, defaultGoalThreshold, time.Second)
	test.That(t, err, test.ShouldBeNil)

	goalJP := frame.JointPositionsFromRadians([]float64{-4.128, 2.71, 2.798, 2.3, 1.291, 0.62})
	goal, err := m.Transform(m.InputFromProtobuf(goalJP))
	test.That(t, err, test.ShouldBeNil)
	solveFunc := NewMetricMinFunc(motionplan.NewSquaredNormMetric(goal), m, logger)
	_, _, err = DoSolve(context.Background(), ik, solveFunc, home, [][]frame.Limit{m.DoF()})
	test.That(t, err, test.ShouldBeNil)
}
