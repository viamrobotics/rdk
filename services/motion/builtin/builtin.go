// Package builtin implements a motion service.
package builtin

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	registry.RegisterService(motion.Subtype, resource.DefaultModelName, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return NewBuiltIn(ctx, r, c, logger)
		},
	})
}

// NewBuiltIn returns a new move and grab service for the given robot.
func NewBuiltIn(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (motion.Service, error) {
	return &builtIn{
		r:      r,
		logger: logger,
	}, nil
}

type builtIn struct {
	r      robot.Robot
	logger golog.Logger
}

// Move takes a goal location and will plan and execute a movement to move a component specified by its name to that destination.
func (ms *builtIn) Move(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *commonpb.WorldState,
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, "motion-service")
	logger := ms.r.Logger()

	// get goal frame
	goalFrameName := destination.FrameName()
	logger.Debugf("goal given in frame of %q", goalFrameName)

	frameSys, err := framesystem.RobotFrameSystem(ctx, ms.r, worldState.GetTransforms())
	if err != nil {
		return false, err
	}
	solver := motionplan.NewSolvableFrameSystem(frameSys, logger)

	// build maps of relevant components and inputs from initial inputs
	fsInputs, resources, err := framesystem.RobotFsCurrentInputs(ctx, ms.r, solver)
	if err != nil {
		return false, err
	}

	logger.Debugf("frame system inputs: %v", fsInputs)

	// re-evaluate goalPose to be in the frame we're going to move in
	solvingFrame := referenceframe.World // TODO(erh): this should really be the parent of rootName
	tf, err := solver.Transform(fsInputs, destination, solvingFrame)
	if err != nil {
		return false, err
	}
	goalPose, _ := tf.(*referenceframe.PoseInFrame)

	// the goal is to move the component to goalPose which is specified in coordinates of goalFrameName
	output, err := solver.SolveWaypointsWithOptions(ctx,
		fsInputs,
		[]*referenceframe.PoseInFrame{goalPose},
		componentName.Name,
		worldState,
		[]map[string]interface{}{},
	)
	if err != nil {
		return false, err
	}

	// move all the components
	for _, step := range output {
		// TODO(erh): what order? parallel?
		for name, inputs := range step {
			if len(inputs) == 0 {
				continue
			}
			err := resources[name].GoToInputs(ctx, inputs)
			if err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

// MoveSingleComponent will pass through a move command to a component with a MoveToPosition method that takes a pose. Arms are the only
// component that supports this. This method will transform the destination pose, given in an arbitrary frame, into the pose of the arm.
// The arm will then move its most distal link to that pose. If you instead wish to move any other component than the arm end to that pose,
// then you must manually adjust the given destination by the transform from the arm end to the intended component.
func (ms *builtIn) MoveSingleComponent(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *commonpb.WorldState,
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, "motion-service")
	logger := ms.r.Logger()

	components := robot.AllResourcesByName(ms.r, componentName.Name)
	if len(components) != 1 {
		return false, fmt.Errorf("got %d resources instead of 1 for (%s)", len(components), componentName.Name)
	}
	movableArm, ok := components[0].(arm.Arm)
	if !ok {
		return false, fmt.Errorf("%v(%T) is not an Arm and cannot MoveToPosition with a Pose", componentName.Name, components[0])
	}

	// get destination pose in frame of movable component
	goalPose := destination.Pose()
	if destination.FrameName() != componentName.Name {
		logger.Debugf("goal given in frame of %q", destination.FrameName())

		frameSys, err := framesystem.RobotFrameSystem(ctx, ms.r, worldState.GetTransforms())
		if err != nil {
			return false, err
		}
		// get the initial inputs
		fsInputs, _, err := framesystem.RobotFsCurrentInputs(ctx, ms.r, frameSys)
		if err != nil {
			return false, err
		}
		logger.Debugf("frame system inputs: %v", fsInputs)

		// re-evaluate goalPose to be in the frame we're going to move in
		tf, err := frameSys.Transform(fsInputs, destination, componentName.Name+"_origin")
		if err != nil {
			return false, err
		}
		goalPoseInFrame, _ := tf.(*referenceframe.PoseInFrame)
		goalPose = goalPoseInFrame.Pose()
		logger.Debugf("converted goal pose %q", spatialmath.PoseToProtobuf(goalPose))
	}

	err := movableArm.MoveToPosition(ctx, spatialmath.PoseToProtobuf(goalPose), worldState, nil)
	if err == nil {
		return true, nil
	}
	return false, err
}

func (ms *builtIn) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	if destinationFrame == "" {
		destinationFrame = referenceframe.World
	}
	return ms.r.TransformPose(
		ctx,
		referenceframe.NewPoseInFrame(
			componentName.Name,
			spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0}),
		),
		destinationFrame,
		supplementalTransforms,
	)
}
