// Package builtin implements a motion service.
package builtin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

	movementSensors := make(map[resource.Name]movementsensor.MovementSensor)
	slamServices := make(map[resource.Name]slam.Service)
	components := make(map[resource.Name]resource.Resource)
	for name, dep := range deps {
		switch dep := dep.(type) {
		case framesystem.Service:
			ms.fsService = dep
		case movementsensor.MovementSensor:
			movementSensors[name] = dep
		case slam.Service:
			slamServices[name] = dep
		default:
			components[name] = dep
		}
	}
	ms.movementSensors = movementSensors
	ms.slamServices = slamServices
	ms.components = components
	return nil
}

type builtIn struct {
	resource.Named
	resource.TriviallyCloseable
	fsService       framesystem.Service
	movementSensors map[resource.Name]movementsensor.MovementSensor
	slamServices    map[resource.Name]slam.Service
	components      map[resource.Name]resource.Resource
	logger          golog.Logger
	lock            sync.Mutex
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

// // PlanMoveOnMap returns the plan for MoveOnMap to execute
// func (ms *builtIn) planMoveOnGlobeRay(
// 	ctx context.Context,
// 	componentName resource.Name,
// 	destination *geo.Point,
// 	movementSensorName resource.Name,
// 	obstacles []*spatialmath.GeoObstacle,
// 	linearVelocityMillisPerSec float64,
// 	angularVelocityDegsPerSec float64,
// 	extra map[string]interface{},
// ) ([][]referenceframe.Input, kinematicbase.KinematicBase, error) {
// 	operation.CancelOtherWithLabel(ctx, builtinOpLabel)

// 	// get relevant components
// 	localizer, ok := ms.movementSensors[movementSensorName]
// 	if !ok {
// 		return nil, nil, resource.DependencyNotFoundError(movementSensorName)
// 	}
// 	baseComponent, ok := ms.components[componentName]
// 	if !ok {
// 		return nil, nil, fmt.Errorf("only Base components are supported for MoveOnGlobe: could not find an Base named %v", componentName)
// 	}
// 	b, ok := baseComponent.(base.Base)
// 	if !ok {
// 		return nil, nil, fmt.Errorf("cannot move base of type %T because it is not a Base", baseComponent)
// 	}

// 	// make a kinematic base by wrapping the original base, limits of this base's frame are unrestricted
// 	var kb kinematicbase.KinematicBase
// 	var err error
// 	limits := []referenceframe.Limit{{Min: math.Inf(-1), Max: math.Inf(1)}, {Min: math.Inf(-1), Max: math.Inf(1)}}
// 	if fake, ok := b.(*fake.Base); ok {
// 		kb, err = kinematicbase.WrapWithFakeKinematics(ctx, fake, localizer, limits)
// 	} else {
// 		kb, err = kinematicbase.WrapWithKinematics(ctx, b, localizer, limits, linearVelocityMillisPerSec, angularVelocityDegsPerSec)
// 	}
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	// get current location of the base
// 	currentInputs, err := kb.CurrentInputs(ctx)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	// TODO: better practice to get this from frame.Transform(input)
// 	currentPoint := r3.Vector{X: currentInputs[0].Value, Y: currentInputs[1].Value}

// 	// convert destination into spatialmath.Pose with respect to lat = 0 = lng
// 	relativeDst := spatialmath.GeoPointToPose(destination).Point().Sub(currentPoint)
// 	distance := relativeDst.Norm()

// 	// convert GeoObstacles into GeometriesInFrame with respect to the base's starting point
// 	geoms := spatialmath.GeoObstaclesToGeometries(obstacles, currentPoint)
// 	gif := referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)
// 	wrldst, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{gif}, nil)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	// Add the kinematic wheeled base frame to the framesystem, use smaller limits to make planning easier
// 	fs := referenceframe.NewEmptyFrameSystem("")
// 	f, err := kb.Kinematics([]referenceframe.Limit{
// 		{Min: -3 * distance, Max: 3 * distance},
// 		{Min: -3 * distance, Max: 3 * distance},
// 	})
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	if err := fs.AddFrame(f, fs.World()); err != nil {
// 		return nil, nil, err
// 	}

// 	// make call to motionplan to get the plan
// 	solutionMap, err := motionplan.PlanMotion(
// 		ctx,
// 		ms.logger,
// 		referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPoseFromPoint(relativeDst)),
// 		f,
// 		referenceframe.StartPositions(fs),
// 		fs,
// 		wrldst,
// 		nil,
// 		extra,
// 	)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	plan, err := motionplan.FrameStepsFromRobotPath(f.Name(), solutionMap)

// 	// return plan in terms of gps coordinates
// 	for _, step := range plan {
// 		step[0].Value += currentPoint.X
// 		step[1].Value += currentPoint.Y
// 	}
// 	return plan, kb, err
// }

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

	plan, kb, err := ms.planMoveOnGlobeNick(
		ctx,
		componentName,
		destination,
		movementSensorName,
		obstacles,
		linearVelocityMillisPerSec,
		angularVelocityDegsPerSec,
		extra,
	)
	if err != nil {
		return false, fmt.Errorf("error making plan for MoveOnMap: %v", err)
	}

	// execute the plan
	for i := 1; i < len(plan); i++ {
		ms.logger.Info(plan[i])
		if err := kb.GoToInputs(ctx, plan[i]); err != nil {
			return false, err
		}
	}
	return true, nil
}

// planMoveOnGlobe returns the plan for MoveOnGlobe to execute
func (ms *builtIn) planMoveOnGlobeNick(
	ctx context.Context,
	componentName resource.Name,
	destination *geo.Point,
	movementSensorName resource.Name,
	obstacles []*spatialmath.GeoObstacle,
	linearVelocityMillisPerSec float64,
	angularVelocityDegsPerSec float64,
	extra map[string]interface{},
) ([][]referenceframe.Input, kinematicbase.KinematicBase, error) {
	// build the localizer from the movement sensor
	movementSensor, ok := ms.movementSensors[movementSensorName]
	if !ok {
		return nil, nil, resource.DependencyNotFoundError(movementSensorName)
	}
	origin, _, err := movementSensor.Position(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	localizer := motion.NewMovementSensorLocalizer(movementSensor, origin)

	// convert destination into spatialmath.Pose with respect to where the localizer was initialized
	goal := spatialmath.GeoPointToPose(destination, origin)

	// convert GeoObstacles into GeometriesInFrame with respect to the base's starting point
	geoms := spatialmath.GeoObstaclesToGeometries(obstacles, origin)

	gif := referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)
	wrldst, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{gif}, nil)
	if err != nil {
		return nil, nil, err
	}

	// construct limits
	straightlineDistance := goal.Point().Norm()
	limits := []referenceframe.Limit{
		{Min: -straightlineDistance * 3, Max: straightlineDistance * 3},
		{Min: -straightlineDistance * 3, Max: straightlineDistance * 3},
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

	inputMap := map[string][]referenceframe.Input{componentName.Name: make([]referenceframe.Input, 3)}

	// create a new empty framesystem which we add the kinematic base to
	fs := referenceframe.NewEmptyFrameSystem("")
	kbf := kb.Kinematics()
	if err != nil {
		return nil, nil, err
	}
	if err := fs.AddFrame(kbf, fs.World()); err != nil {
		return nil, nil, err
	}

	// add an offset between the movement sensor and the base if it is applicable
	baseOrigin := referenceframe.NewPoseInFrame(componentName.ShortName(), spatialmath.NewZeroPose())
	movementSensorToBase, err := ms.fsService.TransformPose(ctx, baseOrigin, movementSensorName.ShortName(), nil)
	if err != nil {
		movementSensorToBase = baseOrigin
	}
	// TODO(RSDK-3407): this does not adequately account for geometries right now since it is a transformation after the fact.
	// This is probably acceptable for the time being, but long term the construction of the frame system for the kinematic base should
	// be moved under the purview of the kinematic base wrapper instead of being done here.
	offsetFrame, err := referenceframe.NewStaticFrame("offset", movementSensorToBase.Pose())
	if err := fs.AddFrame(offsetFrame, kbf); err != nil {
		return nil, nil, err
	}

	// make call to motionplan
	solutionMap, err := motionplan.PlanMotion(
		ctx,
		ms.logger,
		referenceframe.NewPoseInFrame(referenceframe.World, goal),
		offsetFrame,
		inputMap,
		fs,
		wrldst,
		nil,
		extra,
	)
	if err != nil {
		return nil, nil, err
	}

	plan, err := motionplan.FrameStepsFromRobotPath(kbf.Name(), solutionMap)
	if err != nil {
		return nil, nil, err
	}
	for _, step := range plan {
		ms.logger.Info(step)
	}
	return plan, kb, nil
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
	slamSvc, ok := ms.slamServices[slamName]
	if !ok {
		return nil, nil, resource.DependencyNotFoundError(slamName)
	}

	// gets the extents of the SLAM map
	limits, err := slam.GetLimits(ctx, slamSvc)
	if err != nil {
		return nil, nil, err
	}

	// create a KinematicBase from the componentName
	component, ok := ms.components[componentName]
	if !ok {
		return nil, nil, resource.DependencyNotFoundError(componentName)
	}
	b, ok := component.(base.Base)
	if !ok {
		return nil, nil, fmt.Errorf("cannot move component of type %T because it is not a Base", component)
	}
	var kb kinematicbase.KinematicBase
	if fake, ok := b.(*fake.Base); ok {
		kb, err = kinematicbase.WrapWithFakeKinematics(ctx, fake, motion.NewSLAMLocalizer(slamSvc), limits)
	} else {
		kb, err = kinematicbase.WrapWithKinematics(
			ctx,
			b,
			motion.NewSLAMLocalizer(slamSvc),
			limits,
			defaultLinearVelocityMillisPerSec,
			defaultAngularVelocityDegsPerSec,
		)
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
	if err != nil {
		return nil, nil, err
	}
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
