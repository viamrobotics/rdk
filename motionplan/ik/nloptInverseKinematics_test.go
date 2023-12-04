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

func TestCreateNloptIKSolver(t *testing.T) {
	logger := logging.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateNloptIKSolver(m, logger, -1, false, DefaultJump)
	test.That(t, err, test.ShouldBeNil)
	ik.id = 1

	// matches xarm home end effector position
	pos := spatialmath.NewPoseFromPoint(r3.Vector{X: 207, Z: 112})
	seed := referenceframe.FloatsToInputs([]float64{1, 1, 1, 1, 1, 0})
	_, err = solveTest(context.Background(), ik, pos, seed)
	test.That(t, err, test.ShouldBeNil)

	pos = spatialmath.NewPose(
		r3.Vector{X: -46, Y: -23, Z: 372},
		&spatialmath.OrientationVectorDegrees{Theta: 0, OX: 0, OY: 0, OZ: -1},
	)

	seed = m.InputFromProtobuf(&pb.JointPositions{Values: []float64{49, 28, -101, 0, -73, 0}})

	_, err = solveTest(context.Background(), ik, pos, seed)
	test.That(t, err, test.ShouldBeNil)
}
