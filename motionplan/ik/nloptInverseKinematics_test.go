package ik

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestCreateNloptSolver(t *testing.T) {
	logger := logging.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/example_kinematics/xarm6_kinematics_test.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateNloptSolver(m.DoF(), logger, -1, false, true)
	test.That(t, err, test.ShouldBeNil)
	ik.(*nloptIK).id = 1

	// matches xarm home end effector position
	pos := spatialmath.NewPoseFromPoint(r3.Vector{X: 207, Z: 112})
	seed := []float64{1, 1, -1, 1, 1, 0}
	solveFunc := NewMetricMinFunc(NewSquaredNormMetric(pos), m, logger)
	_, err = solveTest(context.Background(), ik, solveFunc, seed)
	test.That(t, err, test.ShouldBeNil)

	pos = spatialmath.NewPose(
		r3.Vector{X: -46, Y: -23, Z: 372},
		&spatialmath.OrientationVectorDegrees{Theta: 0, OX: 0, OY: 0, OZ: -1},
	)

	// Check unpacking from proto
	seed = referenceframe.InputsToFloats(m.InputFromProtobuf(&pb.JointPositions{Values: []float64{49, 28, -101, 0, -73, 0}}))
	solveFunc = NewMetricMinFunc(NewSquaredNormMetric(pos), m, logger)

	_, err = solveTest(context.Background(), ik, solveFunc, seed)
	test.That(t, err, test.ShouldBeNil)
}
