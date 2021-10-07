package robotimpl

import (
	"context"
	"fmt"

	"go.viam.com/core/kinematics"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/robot"
	"go.viam.com/core/spatialmath"

	"github.com/go-errors/errors"
)

// MoveGripper needs a robot with exactly one arm and one gripper and will move the gripper position to the goalPose in the reference frame specified by goalFrameName
func MoveGripper(ctx context.Context, r robot.Robot, goalPose spatialmath.Pose, goalFrameName string) error {
	if len(r.ArmNames()) != 1 {
		return errors.New("robot needs exactly 1 arm for MoveGripper")
	}
	if len(r.GripperNames()) != 1 {
		return errors.New("robot needs exactly 1 gripper for MoveGripper")
	}

	// get all necessary parameters
	armName := r.ArmNames()[0]
	arm, ok := r.ArmByName(armName)
	if !ok {
		return fmt.Errorf("failed to find arm %q", armName)
	}

	gripperName := r.GripperNames()[0]
	if goalFrameName == gripperName {
		return errors.New("cannot move gripper with respect to gripper frame, gripper will always be at its own origin")
	}

	// get the frame system of the robot
	frameSys, err := r.FrameSystem(ctx)
	if err != nil {
		return err
	}
	solver := kinematics.NewSolvableFrameSystem(frameSys, r.Logger())
	// get the initial inputs
	input := referenceframe.StartPositions(solver)
	pos, err := arm.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}
	input[armName] = referenceframe.JointPosToInputs(pos)

	worldGoalPose, err := solver.TransformPose(input, goalPose, solver.GetFrame(goalFrameName), solver.World())
	if err != nil {
		return err
	}

	// update the goal orientation to match the current orientation, keep the point from goalPose
	armPose, err := solver.TransformFrame(input, solver.GetFrame(gripperName), solver.World())
	if err != nil {
		return err
	}
	newGoalPose := spatialmath.NewPoseFromOrientation(worldGoalPose.Point(), armPose.Orientation())

	// the goal is to move the gripper to newGoalPose (which is given in coord of frame goalFrameName).
	gripFrame := frameSys.GetFrame(gripperName)
	output, err := solver.SolvePose(ctx, input, newGoalPose, gripFrame, solver.World())
	if err != nil {
		return err
	}

	return arm.MoveToJointPositions(ctx, referenceframe.InputsToJointPos(output[armName]))

}
