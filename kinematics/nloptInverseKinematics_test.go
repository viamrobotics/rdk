package kinematics

import (
	"context"
	"testing"

	commonpb "go.viam.com/core/proto/api/common/v1"
	pb "go.viam.com/core/proto/api/component/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestCreateNloptIKSolver(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateNloptIKSolver(m, logger)
	test.That(t, err, test.ShouldBeNil)
	ik.id = 1

	pos := &commonpb.Pose{X: 360, Z: 362}
	seed := frame.FloatsToInputs([]float64{1, 1, 1, 1, 1, 0})

	_, err = solveTest(context.Background(), ik, pos, seed)
	test.That(t, err, test.ShouldBeNil)

	pos = &commonpb.Pose{X: -46, Y: -23, Z: 372, Theta: utils.RadToDeg(3.92), OX: -0.46, OY: 0.84, OZ: 0.28}

	seed = frame.JointPosToInputs(&pb.ArmJointPositions{Degrees: []float64{49, 28, -101, 0, -73, 0}})

	_, err = solveTest(context.Background(), ik, pos, seed)
	test.That(t, err, test.ShouldBeNil)
}
