// Package builtin implements a motion service.
package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	servicepb "go.viam.com/api/service/motion/v1"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	resource.RegisterDefaultService(
		motion.API,
		resource.DefaultServiceModel,
		resource.Registration[motion.Service, *Config]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (motion.Service, error) {
				return NewBuiltIn(ctx, deps, conf, logger)
			},
			WeakDependencies: []internal.ResourceMatcher{internal.SLAMDependencyWildcardMatcher},
		})
}

// Config describes how to configure the service; currently only used for specifying dependency on framesystem service
type Config struct {
}

// Validate here adds a dependency on the internal framesystem service
func (c *Config) Validate(path string) ([]string, error) {
	return []string{framesystem.InternalServiceName.String()}, nil
}

// NewBuiltIn returns a new move and grab service for the given robot.
func NewBuiltIn(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (motion.Service, error) {
	ms := &builtIn{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	if err := ms.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return ms, nil
}

// Reconfigure updates the motion service when the config has changed.
func (ms *builtIn) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) (err error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	slamServices := make(map[resource.Name]slam.Service)
	for name, dep := range deps {
		switch dep := dep.(type) {
		case framesystem.Service:
			ms.fsService = dep
		case slam.Service:
			slamServices[name] = dep
		}
	}
	ms.slamServices = slamServices
	return nil
}

type builtIn struct {
	resource.Named
	resource.TriviallyCloseable
	fsService    framesystem.Service
	slamServices map[resource.Name]slam.Service
	logger       golog.Logger
	lock         sync.Mutex
}

// Move takes a goal location and will plan and execute a movement to move a component specified by its name to that destination.
func (ms *builtIn) Move(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	constraints *servicepb.Constraints,
	extra map[string]interface{},
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, "motion-service")
	logger := ms.logger

	// get goal frame
	goalFrameName := destination.Parent()
	logger.Debugf("goal given in frame of %q", goalFrameName)

	frameSys, err := ms.fsService.FrameSystem(ctx, worldState.Transforms())
	if err != nil {
		return false, err
	}

	// build maps of relevant components and inputs from initial inputs
	fsInputs, resources, err := ms.fsService.AllCurrentInputs(ctx)
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

// MoveOnMap will move the given component
func (ms *builtIn) MoveOnMap(
	ctx context.Context,
	componentName resource.Name,
	destination spatialmath.Pose,
	slamName resource.Name,
	extra map[string]interface{},
) (bool, error) {
	slamService, ok := ms.slamServices[slamName]
	if !ok {
		return false, errors.Wrap(resource.NewNotFoundError(slamName), "motion service missing weak dependency")
	}
	_ = slamService
	time.Sleep(time.Second * 10)
	return true, nil
}

// MoveSingleComponent will pass through a move command to a component with a MoveToPosition method that takes a pose. Arms are the only
// component that supports this. This method will transform the destination pose, given in an arbitrary frame, into the pose of the arm.
// The arm will then move its most distal link to that pose. If you instead wish to move any other component than the arm end to that pose,
// then you must manually adjust the given destination by the transform from the arm end to the intended component.
// Because this uses an arm's MoveToPosition method when issuing commands, it does not support obstacle avoidance.
func (ms *builtIn) MoveSingleComponent(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	extra map[string]interface{},
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, "motion-service")

	// Get the arm and all initial inputs
	fsInputs, allResources, err := ms.fsService.AllCurrentInputs(ctx)
	if err != nil {
		return false, err
	}
	ms.logger.Debugf("frame system inputs: %v", fsInputs)

	armResource, ok := allResources[componentName.ShortName()]
	if !ok {
		return false, fmt.Errorf("could not find a resource named %v", componentName.ShortName())
	}
	movableArm, ok := armResource.(arm.Arm)
	if !ok {
		return false, fmt.Errorf(
			"could not cast resource named %v to an arm. MoveSingleComponent only supports moving arms for now",
			componentName.ShortName(),
		)
	}

	// get destination pose in frame of movable component
	goalPose := destination.Pose()
	if destination.Parent() != componentName.ShortName() {
		ms.logger.Debugf("goal given in frame of %q", destination.Parent())

		frameSys, err := ms.fsService.FrameSystem(ctx, worldState.Transforms())
		if err != nil {
			return false, err
		}

		// re-evaluate goalPose to be in the frame we're going to move in
		tf, err := frameSys.Transform(fsInputs, destination, componentName.ShortName()+"_origin")
		if err != nil {
			return false, err
		}
		goalPoseInFrame, _ := tf.(*referenceframe.PoseInFrame)
		goalPose = goalPoseInFrame.Pose()
		ms.logger.Debugf("converted goal pose %q", spatialmath.PoseToProtobuf(goalPose))
	}
	err = movableArm.MoveToPosition(ctx, goalPose, extra)
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
	return ms.fsService.TransformPose(
		ctx,
		referenceframe.NewPoseInFrame(
			componentName.ShortName(),
			spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0}),
		),
		destinationFrame,
		supplementalTransforms,
	)
}
