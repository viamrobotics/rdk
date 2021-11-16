package objectmanipulation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	"go.viam.com/core/config"
	"go.viam.com/core/kinematics"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/spatialmath"
)

const frameSystemName = "move_gripper"

func init() {
	registry.RegisterService(Type, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	})
}

// A Service controls the flow of manipulating other objects with a robot's gripper.
type Service interface {
	DoGrab(ctx context.Context, gripperName, armName, cameraName string, cameraPoint *r3.Vector) (bool, error)
}

// Type is the type of service.
const Type = config.ServiceType("object_manipulation")

// New returns a new move and grab service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	return &objectMService{
		r:      r,
		logger: logger,
	}, nil
}

type objectMService struct {
	r      robot.Robot
	logger golog.Logger
}

// DoGrab takes a camera point of an object's location and both moves the gripper
// to that location and commands it to grab the object
func (mgs objectMService) DoGrab(ctx context.Context, gripperName, rootName, cameraName string, cameraPoint *r3.Vector) (bool, error) {
	// get gripper component
	gripper, ok := mgs.r.GripperByName(gripperName)
	if !ok {
		return false, fmt.Errorf("failed to find gripper %q", gripperName)
	}
	// do gripper movement
	err := gripper.Open(ctx)
	if err != nil {
		return false, err
	}
	cameraPose := spatialmath.NewPoseFromPoint(*cameraPoint)
	err = mgs.moveGripper(ctx, gripperName, rootName, cameraPose, cameraName)
	if err != nil {
		return false, err
	}
	return gripper.Grab(ctx)
}

// moveGripper needs a robot with exactly one arm and one gripper and will move the gripper position to the goalPose in the reference frame specified by goalFrameName
func (mgs objectMService) moveGripper(ctx context.Context, gripperName, rootName string, goalPose spatialmath.Pose, goalFrameName string) error {
	r := mgs.r
	logger := r.Logger()
	logger.Debugf("goal given in frame of %q", goalFrameName)

	if goalFrameName == gripperName {
		return errors.New("cannot move gripper with respect to gripper frame, gripper will always be at its own origin")
	}
	logger.Debugf("using gripper %q", gripperName)

	// get the frame system of the robot
	frameSys, err := r.FrameSystem(ctx, frameSystemName, "")
	if err != nil {
		return err
	}
	solver := kinematics.NewSolvableFrameSystem(frameSys, r.Logger())
	// get the initial inputs
	input := referenceframe.StartPositions(solver)

	allOriginals := map[string][]referenceframe.Input{}
	resources := map[string]referenceframe.InputEnabled{}

	for k, original := range input {
		if strings.HasSuffix(k, "_offset") {
			continue
		}
		if len(original) == 0 {
			continue
		}

		allOriginals[k] = original

		all := robot.AllResourcesByName(r, k)
		if len(all) != 1 {
			return fmt.Errorf("got %d resources instead of 1 for (%s)", len(all), k)
		}

		ii, ok := all[0].(referenceframe.InputEnabled)
		if !ok {
			return fmt.Errorf("%v(%T) is not InputEnabled", k, all[0])
		}

		resources[k] = ii

		pos, err := ii.CurrentInputs(ctx)
		if err != nil {
			return err
		}
		input[k] = pos
	}
	logger.Debugf("frame system inputs: %v", input)

	solvingFrame := solver.World() // TODO(erh): this should really be the parent of rootName

	// re-evaluate goalPose to be in the frame we're going to move in
	goalPose, err = solver.TransformPose(input, goalPose, solver.GetFrame(goalFrameName), solvingFrame)
	if err != nil {
		return err
	}

	if true { // if we want to keep the orientation of the gripper the same
		// TODO(erh): this is often desirable, but not necessarily, and many times will be wrong.
		// update the goal orientation to match the current orientation, keep the point from goalPose
		armPose, err := solver.TransformFrame(input, solver.GetFrame(gripperName), solvingFrame)
		if err != nil {
			return err
		}
		goalPose = spatialmath.NewPoseFromOrientation(goalPose.Point(), armPose.Orientation())
	}

	// the goal is to move the gripper to goalPose (which is given in coord of frame goalFrameName).
	output, err := solver.SolvePose(ctx, input, goalPose, solver.GetFrame(gripperName), solvingFrame)
	if err != nil {
		return err
	}

	// TODO(erh): what order? parallel?
	for n, v := range output {
		if len(v) == 0 {
			continue
		}
		err := resources[n].GoToInputs(ctx, v)
		if err != nil {
			return err
		}
	}

	return nil
}
