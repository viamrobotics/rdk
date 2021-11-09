package moveandgrab

import (
	"context"
	"errors"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	"go.viam.com/core/config"
	"go.viam.com/core/kinematics"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/spatialmath"
)

func init() {
	registry.RegisterService(Type, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	})
}

// A Service controls the move and grab flow for a robot's gripper.
type Service interface {
	DoGrab(ctx context.Context, cameraName string, x, y, z float64) (bool, error)
	MoveGripper(ctx context.Context, goalPose spatialmath.Pose, goalFrameName string) error
}

// Type is the type of service.
const Type = config.ServiceType("moveandgrab")

// New returns a new move and grab service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	return &MGService{
		r:      r,
		logger: logger,
	}, nil
}

// MGService is an implementation of a move and grab service
// as defined above
type MGService struct {
	r      robot.Robot
	logger golog.Logger
}

// DoGrab takes a camera point of an object's location and both moves the gripper
// to that location and commands it to grab the object
func (mgs MGService) DoGrab(ctx context.Context, cameraName string, x, y, z float64) (bool, error) {
	// get gripper component
	if len(mgs.r.GripperNames()) != 1 {
		return false, errors.New("robot needs exactly 1 arm for doGrab")
	}
	gripperName := mgs.r.GripperNames()[0]
	gripper, ok := mgs.r.GripperByName(gripperName)
	if !ok {
		return false, fmt.Errorf("failed to find gripper %q", gripperName)
	}
	// do gripper movement
	err := gripper.Open(ctx)
	if err != nil {
		return false, err
	}
	cameraPoint := r3.Vector{x, y, z}
	cameraPose := spatialmath.NewPoseFromPoint(cameraPoint)
	err = mgs.MoveGripper(ctx, cameraPose, cameraName)
	if err != nil {
		return false, err
	}
	return gripper.Grab(ctx)
}

// MoveGripper needs a robot with exactly one arm and one gripper and will move the gripper position to the goalPose in the reference frame specified by goalFrameName
func (mgs MGService) MoveGripper(ctx context.Context, goalPose spatialmath.Pose, goalFrameName string) error {
	r := mgs.r
	logger := r.Logger()
	logger.Debugf("goal given in frame of %q", goalFrameName)
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
	logger.Debugf("using arm %q", armName)

	gripperName := r.GripperNames()[0]
	if goalFrameName == gripperName {
		return errors.New("cannot move gripper with respect to gripper frame, gripper will always be at its own origin")
	}
	logger.Debugf("using gripper %q", gripperName)

	// get the frame system of the robot
	frameSys, err := r.FrameSystem(ctx, "move_gripper", "")
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
	logger.Debugf("frame system inputs: %v", input)

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
	output, err := solver.SolvePose(ctx, input, newGoalPose, solver.GetFrame(gripperName), solver.World())
	if err != nil {
		return err
	}

	return arm.MoveToJointPositions(ctx, referenceframe.InputsToJointPos(output[armName]))

}
