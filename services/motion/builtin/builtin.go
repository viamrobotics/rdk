// Package builtin implements a motion service.
package builtin

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	servicepb "go.viam.com/api/service/motion/v1"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	registry.RegisterService(motion.Subtype, resource.DefaultServiceModel, registry.Service{
		RobotConstructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return NewBuiltIn(ctx, r, c, logger)
		},
	})
	resource.AddDefaultService(motion.Named(resource.DefaultServiceName))
}

// NewBuiltIn returns a new move and grab service for the given robot.
func NewBuiltIn(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (motion.Service, error) {
	return &builtIn{
		r:      r,
		logger: logger,
	}, nil
}

type builtIn struct {
	generic.Unimplemented
	r      robot.Robot
	logger golog.Logger
}

// Move takes a goal location and will plan and execute a movement to move a component specified by its name to that destination.
func (ms *builtIn) Move(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	constraints *servicepb.Constraints,
	slamName resource.Name,
	extra map[string]interface{},
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, "motion-service")
	logger := ms.r.Logger()

	// get goal frame
	goalFrameName := destination.Parent()
	logger.Debugf("goal given in frame of %q", goalFrameName)

	frameSys, err := framesystem.RobotFrameSystem(ctx, ms.r, worldState.Transforms)
	if err != nil {
		return false, err
	}

	// build maps of relevant components and inputs from initial inputs
	fsInputs, resources, err := framesystem.RobotFsCurrentInputs(ctx, ms.r, frameSys)
	if err != nil {
		return false, err
	}

	movingFrame := frameSys.Frame(componentName.ShortName())

	logger.Debugf("frame system inputs: %v", fsInputs)
	if movingFrame == nil {
		return false, fmt.Errorf("component named %s not found in robot frame system", componentName.ShortName())
	}

	// re-evaluate goalPose to be in the frame of World
	solvingFrame := referenceframe.World // TODO(erh): this should really be the parent of rootName
	tf, err := frameSys.Transform(fsInputs, destination, solvingFrame)
	if err != nil {
		return false, err
	}
	goalPose, _ := tf.(*referenceframe.PoseInFrame)

	// the goal is to move the component to goalPose which is specified in coordinates of goalFrameName
	output, err := motionplan.PlanMotion(ctx,
		logger,
		goalPose,
		movingFrame,
		fsInputs,
		frameSys,
		worldState,
		constraints,
		extra,
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
	worldState *referenceframe.WorldState,
	extra map[string]interface{},
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, "motion-service")
	logger := ms.r.Logger()

	components := robot.AllResourcesByName(ms.r, componentName.ShortName())
	if len(components) != 1 {
		return false, fmt.Errorf("got %d resources instead of 1 for (%s)", len(components), componentName.ShortName())
	}
	movableArm, ok := components[0].(arm.Arm)
	if !ok {
		return false, fmt.Errorf("%v(%T) is not an Arm and cannot MoveToPosition with a Pose", componentName.ShortName(), components[0])
	}

	// get destination pose in frame of movable component
	goalPose := destination.Pose()
	if destination.Parent() != componentName.ShortName() {
		logger.Debugf("goal given in frame of %q", destination.Parent())

		frameSys, err := framesystem.RobotFrameSystem(ctx, ms.r, worldState.Transforms)
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
		tf, err := frameSys.Transform(fsInputs, destination, componentName.ShortName()+"_origin")
		if err != nil {
			return false, err
		}
		goalPoseInFrame, _ := tf.(*referenceframe.PoseInFrame)
		goalPose = goalPoseInFrame.Pose()
		logger.Debugf("converted goal pose %q", spatialmath.PoseToProtobuf(goalPose))
	}
	err := movableArm.MoveToPosition(ctx, goalPose, worldState, extra)
	if err == nil {
		return true, nil
	}
	return false, err
}

func (ms *builtIn) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*referenceframe.LinkInFrame,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	if destinationFrame == "" {
		destinationFrame = referenceframe.World
	}
	return ms.r.TransformPose(
		ctx,
		referenceframe.NewPoseInFrame(
			componentName.ShortName(),
			spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0}),
		),
		destinationFrame,
		supplementalTransforms,
	)
}
