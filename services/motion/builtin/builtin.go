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
	"go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/pointcloud"
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
			Constructor: NewBuiltIn,
			WeakDependencies: []internal.ResourceMatcher{
				internal.SLAMDependencyWildcardMatcher,
				internal.ComponentDependencyWildcardMatcher,
			},
		})
}

const (
	builtinOpLabel                    = "motion-service"
	defaultLinearVelocityMillisPerSec = 300 // mm per second; used for bases only
	defaultAngularVelocityDegsPerSec  = 60  // degrees per second; used for bases only
)

// ErrNotImplemented is thrown when an unreleased function is called.
var ErrNotImplemented = errors.New("function coming soon but not yet implemented")

// Config describes how to configure the service; currently only used for specifying dependency on framesystem service.
type Config struct{}

// Validate here adds a dependency on the internal framesystem service.
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

	localizers := make(map[resource.Name]motion.Localizer)
	components := make(map[resource.Name]resource.Resource)
	for name, dep := range deps {
		switch dep := dep.(type) {
		case framesystem.Service:
			ms.fsService = dep
		case slam.Service, movementsensor.MovementSensor:
			localizer, err := motion.NewLocalizer(ctx, dep)
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
	localizers map[resource.Name]motion.Localizer
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
	operation.CancelOtherWithLabel(ctx, builtinOpLabel)

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
	operation.CancelOtherWithLabel(ctx, builtinOpLabel)

	// make call to motionplan
	plan, kb, err := ms.planMoveOnMap(ctx, componentName, destination, slamName, extra)
	if err != nil {
		return false, fmt.Errorf("error making plan for MoveOnMap: %w", err)
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
	linearVelocityMillisPerSec float64,
	angularVelocityDegsPerSec float64,
	extra map[string]interface{},
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, builtinOpLabel)

	// get the localizer from the componentName
	localizer, ok := ms.localizers[movementSensorName]
	if !ok {
		return false, resource.DependencyNotFoundError(movementSensorName)
	}

	currentPosition, dstPIF, err := ms.getRelativePositionAndDestination(ctx, localizer, componentName, movementSensorName, *destination)
	if err != nil {
		return false, err
	}

	plan, kb, err := ms.planMoveOnGlobe(ctx,
		componentName,
		currentPosition,
		dstPIF,
		localizer,
		obstacles,
		linearVelocityMillisPerSec,
		angularVelocityDegsPerSec,
		extra,
	)
	if err != nil {
		return false, err
	}

	// execute the plan
	for _, step := range plan {
		for _, inputs := range step {
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			if len(inputs) == 0 {
				continue
			}
			if err := kb.GoToInputs(ctx, inputs); err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

// planMoveOnGlobe returns the plan for MoveOnGlobe to execute.
func (ms *builtIn) planMoveOnGlobe(
	ctx context.Context,
	componentName resource.Name,
	currentPosition r3.Vector,
	dstPIF *referenceframe.PoseInFrame,
	localizer motion.Localizer,
	obstacles []*spatialmath.GeoObstacle,
	linearVelocityMillisPerSec float64,
	angularVelocityDegsPerSec float64,
	extra map[string]interface{},
) ([]map[string][]referenceframe.Input, kinematicbase.KinematicBase, error) {
	// create a new empty framesystem which we add our base to
	fs := referenceframe.NewEmptyFrameSystem("")

	// convert GeoObstacles into GeometriesInFrame with respect to the base's starting point
	geoms := spatialmath.GeoObstaclesToGeometries(obstacles, currentPosition)

	gif := referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)
	wrldst, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{gif}, nil)
	if err != nil {
		return nil, nil, err
	}

	// construct limits
	straightlineDistance := dstPIF.Pose().Point().Norm()
	limits := []referenceframe.Limit{
		{Min: -straightlineDistance * 3, Max: straightlineDistance * 3},
		{Min: -straightlineDistance * 3, Max: straightlineDistance * 3},
		{Min: -2 * math.Pi, Max: 2 * math.Pi},
	}

	if extra != nil {
		if profile, ok := extra["motion_profile"]; ok {
			motionProfile, ok := profile.(string)
			if !ok {
				return nil, nil, errors.New("could not interpret motion_profile field as string")
			}
			if motionProfile == motionplan.PositionOnlyMotionProfile {
				limits = limits[0:2] // remove theta limit if in position only mode
			}
		}
	}
	ms.logger.Debugf("base limits: %v", limits)

	// create a KinematicBase from the componentName
	baseComponent, ok := ms.components[componentName]
	if !ok {
		return nil, nil, fmt.Errorf("only Base components are supported for MoveOnGlobe: could not find an Base named %v", componentName)
	}
	b, ok := baseComponent.(base.Base)
	if !ok {
		return nil, nil, fmt.Errorf("cannot move base of type %T because it is not a Base", baseComponent)
	}
	var kb kinematicbase.KinematicBase
	if fake, ok := b.(*fake.Base); ok {
		kb, err = kinematicbase.WrapWithFakeKinematics(ctx, fake, localizer, limits)
	} else {
		kb, err = kinematicbase.WrapWithKinematics(ctx, b, localizer, limits,
			linearVelocityMillisPerSec, angularVelocityDegsPerSec)
	}
	if err != nil {
		return nil, nil, err
	}

	inputs, err := kb.CurrentInputs(ctx)
	if err != nil {
		return nil, nil, err
	}
	inputMap := map[string][]referenceframe.Input{componentName.Name: inputs}

	// Add the kinematic wheeled base to the framesystem
	if err := fs.AddFrame(kb.Kinematics(), fs.World()); err != nil {
		return nil, nil, err
	}

	// make call to motionplan
	plan, err := motionplan.PlanMotion(ctx, ms.logger, dstPIF, kb.Kinematics(), inputMap, fs, wrldst, nil, extra)
	if err != nil {
		return nil, nil, err
	}
	return plan, kb, nil
}

// getRelativePositionAndDestination returns the position of the base relative to the localizer only if there is
// information about their spatial relationship within the framesystem and the resulting pose in frame
// as the destination relative to the base to plan for.
func (ms *builtIn) getRelativePositionAndDestination(
	ctx context.Context,
	localizer motion.Localizer,
	componentName resource.Name,
	movementSensorName resource.Name,
	destination geo.Point,
) (r3.Vector, *referenceframe.PoseInFrame, error) {
	var currentPosition r3.Vector

	// get localizer current pose in frame
	currentPIF, err := localizer.CurrentPosition(ctx)
	if err != nil {
		return currentPosition, nil, err
	}

	currentPosition = currentPIF.Pose().Point()
	ms.logger.Debugf("current position: %v", currentPosition)

	// get position of localizer relative to base
	robotFS, err := ms.fsService.FrameSystem(ctx, nil)
	if err != nil {
		return currentPosition, nil, err
	}

	localizerFrame := robotFS.Frame(movementSensorName.ShortName())
	if localizerFrame != nil {
		// build maps of relevant components and inputs from initial inputs
		fsInputs, _, err := ms.fsService.CurrentInputs(ctx)
		if err != nil {
			return currentPosition, nil, err
		}

		// transform currentPIF by the movementsensor translation specified for its frame
		destinationFrameName := componentName.Name
		tf, err := robotFS.Transform(fsInputs, &currentPIF, destinationFrameName)
		if err != nil {
			return currentPosition, nil, err
		}
		currentPosition = tf.(*referenceframe.PoseInFrame).Pose().Point()
		ms.logger.Debugf("corrected current position: %v", currentPosition)
	}

	// convert destination into spatialmath.Pose with respect to lat = 0 = lng
	dstPose := spatialmath.GeoPointToPose(&destination)
	ms.logger.Debugf("destination as geo point and pose: %v, %v", destination, dstPose.Point())

	// convert the destination to be relative to the currentPosition
	relativeDestinationPt := r3.Vector{
		X: dstPose.Point().X - currentPosition.X,
		Y: dstPose.Point().Y - currentPosition.Y,
		Z: 0,
	}
	ms.logger.Debugf("destination pose relative to current position: %v", relativeDestinationPt)

	relativeDstPose := spatialmath.NewPoseFromPoint(relativeDestinationPt)
	dstPIF := referenceframe.NewPoseInFrame(referenceframe.World, relativeDstPose)

	return currentPosition, dstPIF, nil
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
	operation.CancelOtherWithLabel(ctx, builtinOpLabel)

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

// PlanMoveOnMap returns the plan for MoveOnMap to execute.
func (ms *builtIn) planMoveOnMap(
	ctx context.Context,
	componentName resource.Name,
	destination spatialmath.Pose,
	slamName resource.Name,
	extra map[string]interface{},
) ([][]referenceframe.Input, kinematicbase.KinematicBase, error) {
	// get the SLAM Service from the slamName
	localizer, ok := ms.localizers[slamName]
	if !ok {
		return nil, nil, resource.DependencyNotFoundError(slamName)
	}

	// assert localizer as a slam service and get map limits
	slamSvc, ok := localizer.(slam.Service)
	if !ok {
		return nil, nil, fmt.Errorf("cannot assert localizer of type %T as slam service", localizer)
	}

	// gets the extents of the SLAM map
	limits, err := slam.GetLimits(ctx, slamSvc)
	if err != nil {
		return nil, nil, err
	}
	limits = append(limits, referenceframe.Limit{Min: -2 * math.Pi, Max: 2 * math.Pi})

	// create a KinematicBase from the componentName
	component, ok := ms.components[componentName]
	if !ok {
		return nil, nil, resource.DependencyNotFoundError(componentName)
	}
	b, ok := component.(base.Base)
	if !ok {
		return nil, nil, fmt.Errorf("cannot move component of type %T because it is not a Base", component)
	}

	if extra != nil {
		if profile, ok := extra["motion_profile"]; ok {
			motionProfile, ok := profile.(string)
			if !ok {
				return nil, nil, errors.New("could not interpret motion_profile field as string")
			}
			if motionProfile == motionplan.PositionOnlyMotionProfile {
				limits = limits[0:2] // remove theta limit if in position only mode
			}
		}
	}

	var kb kinematicbase.KinematicBase
	if fake, ok := b.(*fake.Base); ok {
		kb, err = kinematicbase.WrapWithFakeKinematics(ctx, fake, localizer, limits)
	} else {
		kb, err = kinematicbase.WrapWithKinematics(ctx, b, localizer, limits,
			defaultLinearVelocityMillisPerSec, defaultAngularVelocityDegsPerSec)
	}
	if err != nil {
		return nil, nil, err
	}

	// get point cloud data in the form of bytes from pcd
	pointCloudData, err := slam.GetPointCloudMapFull(ctx, slamSvc)
	if err != nil {
		return nil, nil, err
	}
	// store slam point cloud data  in the form of a recursive octree for collision checking
	octree, err := pointcloud.ReadPCDToBasicOctree(bytes.NewReader(pointCloudData))
	if err != nil {
		return nil, nil, err
	}

	if extra == nil {
		extra = make(map[string]interface{})
	}
	extra["planning_alg"] = "rrtstar"

	// get current position
	inputs, err := kb.CurrentInputs(ctx)
	if err != nil {
		return nil, nil, err
	}
	ms.logger.Debugf("base position: %v", inputs)

	dst := referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPoseFromPoint(destination.Point()))

	f := kb.Kinematics()
	fs := referenceframe.NewEmptyFrameSystem("")
	if err := fs.AddFrame(f, fs.World()); err != nil {
		return nil, nil, err
	}

	worldState, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{
		referenceframe.NewGeometriesInFrame(referenceframe.World, []spatialmath.Geometry{octree}),
	}, nil)
	if err != nil {
		return nil, nil, err
	}

	seedMap := map[string][]referenceframe.Input{f.Name(): inputs}

	ms.logger.Debugf("goal position: %v", dst.Pose().Point())
	solutionMap, err := motionplan.PlanMotion(ctx, ms.logger, dst, f, seedMap, fs, worldState, nil, extra)
	if err != nil {
		return nil, nil, err
	}
	plan, err := motionplan.FrameStepsFromRobotPath(f.Name(), solutionMap)
	return plan, kb, err
}
