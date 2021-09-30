package robot

import (
	"context"
	"fmt"

	"go.viam.com/core/kinematics"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/spatialmath"

	"github.com/go-errors/errors"
)

// MoveGripper needs a robot with exactly one arm and one gripper and will move the gripper position to the goalPose in the reference frame specified by goalFrameName
func MoveGripper(ctx context.Context, r Robot, goalPose spatialmath.Pose, goalFrameName string) error {
	if len(r.ArmNames()) != 1 {
		return errors.New("robot needs exactly 1 arm to do grabAt")
	}
	if len(r.GripperNames()) != 1 {
		return errors.New("robot needs exactly 1 gripper to do grabAt")
	}

	// get all necessary parameters
	armName := r.ArmNames()[0]
	arm, ok := r.ArmByName(armName)
	if !ok {
		return fmt.Errorf("failed to find arm %q", armName)
	}
	gripperName := r.GripperNames()[0]

	// get the frame system of the robot
	frameSys, err := r.FrameSystem(ctx)
	if err != nil {
		return err
	}
	// get the initial inputs
	input := referenceframe.StartPositions(frameSys)
	pos, err := arm.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}
	input[armName] = referenceframe.JointPosToInputs(pos)

	// the goal is to move the gripper to goalPose (which is given in coord of frame goalFrameName).
	solver := kinematics.NewSolvableFrameSystem(frameSys, r.Logger())
	output, err := solver.SolvePose(ctx, input, goalPose, frameSys.GetFrame(gripperName), frameSys.GetFrame(goalFrameName))
	if err != nil {
		return err
	}

	return arm.MoveToJointPositions(ctx, referenceframe.InputsToJointPos(output[armName]))

}
