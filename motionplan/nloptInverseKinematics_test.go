package motionplan

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

func TestCreateNloptIKSolver(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("component/arm/wx250s/wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateNloptIKSolver(m, logger, -1)
	test.That(t, err, test.ShouldBeNil)
	ik.id = 1

	pos := &commonpb.Pose{X: 360, Z: 362}
	seed := referenceframe.FloatsToInputs([]float64{1, 1, 1, 1, 1, 0})

	_, err = solveTest(context.Background(), ik, pos, seed)
	test.That(t, err, test.ShouldBeNil)

	pos = &commonpb.Pose{X: -46, Y: -23, Z: 372, Theta: utils.RadToDeg(3.92), OX: -0.46, OY: 0.84, OZ: 0.28}

	jointPos := []*pb.JointPosition{
		{
			JointType:  pb.JointPosition_JOINT_TYPE_REVOLUTE,
			Parameters: []float64{49},
		},
		{
			JointType:  pb.JointPosition_JOINT_TYPE_REVOLUTE,
			Parameters: []float64{28},
		},
		{
			JointType:  pb.JointPosition_JOINT_TYPE_REVOLUTE,
			Parameters: []float64{-101},
		},
		{
			JointType:  pb.JointPosition_JOINT_TYPE_REVOLUTE,
			Parameters: []float64{0},
		},
		{
			JointType:  pb.JointPosition_JOINT_TYPE_REVOLUTE,
			Parameters: []float64{-73},
		},
		{
			JointType:  pb.JointPosition_JOINT_TYPE_REVOLUTE,
			Parameters: []float64{0},
		},
	}
	seed, err = referenceframe.JointPosToInputs(jointPos)
	test.That(t, err, test.ShouldBeNil)

	_, err = solveTest(context.Background(), ik, pos, seed)
	test.That(t, err, test.ShouldBeNil)
}
