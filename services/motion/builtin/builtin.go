// Package builtin implements a motion service.
package builtin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	servicepb "go.viam.com/api/service/motion/v1"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/localizer"
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
			WeakDependencies: []internal.ResourceMatcher{
				internal.SLAMDependencyWildcardMatcher,
				internal.ComponentDependencyWildcardMatcher,
			},
		})
}

const motionOpId = "motion-service"

// ErrNotImplemented is thrown when an unreleased function is called
var ErrNotImplemented = errors.New("function coming soon but not yet implemented")

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

	localizers := make(map[resource.Name]localizer.Localizer)
	components := make(map[resource.Name]resource.Resource)
	for name, dep := range deps {
		switch dep := dep.(type) {
		case framesystem.Service:
			ms.fsService = dep
		case slam.Service, movementsensor.MovementSensor:
			localizer, err := ms.NewLocalizer(ctx, dep)
			if err != nil {
				return err
			}
			localizers[name] = localizer
		default:
			components[name] = dep
		}
	}
	ms.components = components
	ms.localizers = localizers
	return nil
}

type builtIn struct {
	resource.Named
	resource.TriviallyCloseable
	fsService  framesystem.Service
	localizers map[resource.Name]localizer.Localizer
	components map[resource.Name]resource.Resource
	logger     golog.Logger
	lock       sync.Mutex
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
	operation.CancelOtherWithLabel(ctx, motionOpId)

	// get goal frame
	goalFrameName := destination.Parent()
	ms.logger.Debugf("goal given in frame of %q", goalFrameName)

	frameSys, err := ms.fsService.FrameSystem(ctx, worldState.Transforms())
	if err != nil {
		return false, err
	}

	// build maps of relevant components and inputs from initial inputs
	fsInputs, resources, err := ms.fsService.CurrentInputs(ctx)
	if err != nil {
		return false, err
	}

	movingFrame := frameSys.Frame(componentName.ShortName())

	ms.logger.Debugf("frame system inputs: %v", fsInputs)
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
	output, err := motionplan.PlanMotion(ctx, ms.logger, goalPose, movingFrame, fsInputs, frameSys, worldState, constraints, extra)
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

// MoveOnMap will move the given component to the given destination on the slam map generated from a slam service specified by slamName.
// Bases are the only component that supports this.
func (ms *builtIn) MoveOnMap(
	ctx context.Context,
	componentName resource.Name,
	destination spatialmath.Pose,
	slamName resource.Name,
	extra map[string]interface{},
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, motionOpId)

	// get the SLAM Service from the slamName
	localizer, ok := ms.localizers[slamName]
	if !ok {
		return false, resource.DependencyNotFoundError(slamName)
	}
	ms.logger.Warn("This feature is currently experimental and does not support obstacle avoidance with SLAM maps yet")

	// assert localizer as a slam service and get map limits
	slamSvc, ok := localizer.(slam.Service)
	if !ok {
		return false, fmt.Errorf("cannot assert local of type %T as slam service", localizer)
	}

	// gets the extents of the SLAM map
	data, err := slam.GetPointCloudMapFull(ctx, slamSvc)
	if err != nil {
		return false, err
	}
	dims, err := pointcloud.GetPCDMetaData(bytes.NewReader(data))
	if err != nil {
		return false, err
	}
	limits := []referenceframe.Limit{
		{Min: dims.MinX, Max: dims.MaxX},
		{Min: dims.MinY, Max: dims.MaxY},
		{Min: -2 * math.Pi, Max: 2 * math.Pi},
	}

	// create a KinematicBase from the componentName
	component, ok := ms.components[componentName]
	if !ok {
		return false, resource.DependencyNotFoundError(componentName)
	}
	kw, ok := component.(base.KinematicWrappable)
	if !ok {
		return false, fmt.Errorf("cannot move component of type %T because it is not a KinematicWrappable Base", component)
	}
	kb, err := kw.WrapWithKinematics(ctx, localizer, limits)
	if err != nil {
		return false, err
	}

	// get current position
	inputs, err := kb.CurrentInputs(ctx)
	if err != nil {
		return false, err
	}
	ms.logger.Debugf("base position: %v", inputs)

	// make call to motionplan
	dst := spatialmath.NewPoseFromPoint(destination.Point())
	ms.logger.Debugf("goal position: %v", dst)
	plan, err := motionplan.PlanFrameMotion(ctx, ms.logger, dst, kb.ModelFrame(), inputs, nil, extra)
	if err != nil {
		return false, err
	}

	// execute the plan
	for i := 1; i < len(plan); i++ {
		if err := kb.GoToInputs(ctx, plan[i]); err != nil {
			return false, err
		}
	}
	return true, nil
}

// MoveOnGlobe will move the given component to the given destination on the globe.
// Bases are the only component that supports this.
func (ms *builtIn) MoveOnGlobe(
	ctx context.Context,
	componentName resource.Name,
	destination *geo.Point,
	heading float64,
	movementSensorName resource.Name,
	obstacles []*spatialmath.GeoObstacle,
	linearVelocity float64,
	angularVelocity float64,
	extra map[string]interface{},
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, motionOpId)

	// create a new empty framesystem which we will add our base to
	fs := referenceframe.NewEmptyFrameSystem("")

	// build maps of input enabled resources
	_, resources, err := ms.fsService.CurrentInputs(ctx)
	if err != nil {
		return false, err
	}

	// get the localizer
	local, ok := ms.localizers[movementSensorName]
	if !ok {
		return false, resource.DependencyNotFoundError(movementSensorName)
	}

	// assert localizer as a movementSensor to get map limits
	movementSensor, ok := local.(movementsensor.MovementSensor)
	if !ok {
		return false, fmt.Errorf("cannot assert local of type %T as a movementSensor", local)
	}

	currentPosition, _, err := movementSensor.Position(ctx, nil)
	if err != nil {
		return false, err
	}
	currentPose := spatialmath.GeoPointToPose(currentPosition)

	// convert destination into a spatialmath pose in frame with respect to 0 latitude, 0 longitude
	dstPose := spatialmath.GeoPointToPose(destination)
	dstPIF := referenceframe.NewPoseInFrame(referenceframe.World, dstPose)

	// convert GeoObstacles into GeometriesInFrame then into a Worldstate
	geoms := spatialmath.GeoObstaclesToGeometries(obstacles)
	gif := referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)
	wrldst, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{gif}, nil)
	if err != nil {
		return false, err
	}

	// construct naive limits from destintion and current position
	minX := math.Min(currentPose.Point().X, dstPose.Point().X)
	maxX := math.Max(currentPose.Point().X, dstPose.Point().X)
	minY := math.Min(currentPose.Point().Y, dstPose.Point().Y)
	maxY := math.Max(currentPose.Point().Y, dstPose.Point().Y)
	limits := []referenceframe.Limit{
		{Min: minX, Max: maxX},
		{Min: minY, Max: maxY},
		{Min: -2 * math.Pi, Max: 2 * math.Pi},
	}

	// recalculate limits with respect to worldstate
	limits = wrldst.BoundingBox(ctx, limits)

	// create a KinematicBase from the componentName
	baseComponent, ok := ms.components[componentName]
	if !ok {
		return false, fmt.Errorf("only Base components are supported for MoveOnGlobe: could not find an Base named %v", componentName)
	}
	kw, ok := baseComponent.(base.KinematicWrappable)
	if !ok {
		return false, fmt.Errorf("cannot move base of type %T because it is not KinematicWrappable", baseComponent)
	}
	kb, err := kw.WrapWithKinematics(ctx, local, limits)
	if err != nil {
		return false, err
	}

	// check for cancelled context before we are start planning
	if ctx.Err() != nil {
		ms.logger.Info("successfully canceled motion service MoveOnGlobe")
		return true, ctx.Err()
	}

	// get current position
	inputs, err := kb.CurrentInputs(ctx)
	if err != nil {
		return false, err
	}
	seedMap := map[string][]referenceframe.Input{kb.ModelFrame().Name(): inputs}

	// Add the kinematic wheeled base to the framesystem
	if err := fs.AddFrame(kb.ModelFrame(), fs.World()); err != nil {
		return false, err
	}

	// make call to motionplan
	plan, err := motionplan.PlanMotion(ctx, ms.logger, dstPIF, kb.ModelFrame(), seedMap, fs, wrldst, nil, extra)
	if err != nil {
		return false, err
	}

	// check for cancelled context after we are done planning
	if ctx.Err() != nil {
		ms.logger.Info("successfully canceled motion service MoveOnGlobe")
		return true, ctx.Err()
	}

	// execute the plan
	for _, step := range plan {
		for name, inputs := range step {
			if len(inputs) == 0 {
				continue
			}
			if err := resources[name].GoToInputs(ctx, inputs); err != nil {
				// if context is cancelled we only stop the base
				kb.Stop(ctx, nil)
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
// Because this uses an arm's MoveToPosition method when issuing commands, it does not support obstacle avoidance.
func (ms *builtIn) MoveSingleComponent(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	extra map[string]interface{},
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, motionOpId)

	// Get the arm and all initial inputs
	fsInputs, _, err := ms.fsService.CurrentInputs(ctx)
	if err != nil {
		return false, err
	}
	ms.logger.Debugf("frame system inputs: %v", fsInputs)

	armResource, ok := ms.components[componentName]
	if !ok {
		return false, fmt.Errorf("could not find a resource named %v", componentName.ShortName())
	}
	movableArm, ok := armResource.(arm.Arm)
	if !ok {
		return false, fmt.Errorf(
			"could not cast resource named %v to an arm. MoveSingleComponent only supports moving arms for now",
			componentName,
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
	return err == nil, err
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

// NewLocalizer constructs either a SLAMLocalizer or MovementSensorLocalizer from the given resource
func (ms *builtIn) NewLocalizer(ctx context.Context, res resource.Resource) (localizer.Localizer, error) {
	switch res := res.(type) {
	case slam.Service:
		return &localizer.SLAMLocalizer{Service: res}, nil
	case movementsensor.MovementSensor:
		return &localizer.MovementSensorLocalizer{MovementSensor: res}, nil
	default:
		return nil, fmt.Errorf("cannot localize on resource of type %T", res)
	}
}

func boundingBox(ctx context.Context, start, goal spatialmath.Pose, wrldst referenceframe.WorldState) []referenceframe.Limit {
	minX := math.Min(start.Point().X, goal.Point().X)
	maxX := math.Min(start.Point().X, goal.Point().X)
	minY := math.Min(start.Point().Y, goal.Point().Y)
	maxY := math.Min(start.Point().Y, goal.Point().Y)

	return []referenceframe.Limit{
		{Min: minX, Max: maxX},
		{Min: minY, Max: maxY},
		{Min: -2 * math.Pi, Max: 2 * math.Pi},
	}

}
