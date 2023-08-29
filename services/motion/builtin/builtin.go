// Package builtin implements a motion service.
package builtin

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	servicepb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/components/base"
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
	goutils "go.viam.com/utils"
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
	builtinOpLabel    = "motion-service"
	maxTravelDistance = 5e+6 // mm (or 5km)
)

// inputEnabledActuator is an actuator that interacts with the frame system.
// This allows us to figure out where the actuator currently is and then
// move it. Input units are always in meters or radians.
type inputEnabledActuator interface {
	resource.Actuator
	referenceframe.InputEnabled
}

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
	cancelFn        context.CancelFunc
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
	steps, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:          ms.logger,
		Goal:            goalPose,
		Frame:           movingFrame,
		Inputs:          fsInputs,
		FrameSystem:     frameSys,
		WorldState:      worldState,
		ConstraintSpecs: constraints,
		Options:         extra,
	})
	if err != nil {
		return false, err
	}

	// move all the components
	for _, step := range steps {
		// TODO(erh): what order? parallel?
		for name, inputs := range step {
			if len(inputs) == 0 {
				continue
			}
			r := resources[name]
			if err := r.GoToInputs(ctx, inputs); err != nil {
				// If there is an error on GoToInputs, stop the component if possible before returning the error
				if actuator, ok := r.(inputEnabledActuator); ok {
					if stopErr := actuator.Stop(ctx, nil); stopErr != nil {
						return false, errors.Wrap(err, stopErr.Error())
					}
				}
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
	kinematicsOptions := kinematicbase.NewKinematicBaseOptions()

	// make call to motionplan
	plan, kb, err := ms.planMoveOnMap(ctx, componentName, destination, slamName, kinematicsOptions, extra)
	if err != nil {
		return false, fmt.Errorf("error making plan for MoveOnMap: %w", err)
	}

	// execute the plan
	for i := 1; i < len(plan); i++ {
		if inputEnabledKb, ok := kb.(inputEnabledActuator); ok {
			if err := inputEnabledKb.GoToInputs(ctx, plan[i]); err != nil {
				// If there is an error on GoToInputs, stop the component if possible before returning the error
				if stopErr := kb.Stop(ctx, nil); stopErr != nil {
					return false, errors.Wrap(err, stopErr.Error())
				}
				return false, err
			}
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
	motionCfg *motion.MotionConfiguration,
	extra map[string]interface{},
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, builtinOpLabel)

	positionPollingPeriod := time.Duration(1000/motionCfg.PositionPollingFreqHz) * time.Millisecond
	obstaclePollingPeriod := time.Duration(1000/motionCfg.ObstaclePollingFreqHz) * time.Millisecond

	movementSensor, ok := ms.movementSensors[movementSensorName]
	if !ok {
		return false, resource.DependencyNotFoundError(movementSensorName)
	}

	planRequest, kb, err := ms.newMoveOnGlobeRequest(ctx, componentName, destination, movementSensor, obstacles, motionCfg, extra)
	if err != nil {
		return false, err
	}

	successChan := make(chan bool)
	defer close(successChan)

	planChan := make(chan motionplan.Plan)
	defer close(planChan)

	replanChan := make(chan bool, 1)
	defer close(replanChan)
	replanChan <- true

	errChan := make(chan error)
	defer close(errChan)

	var backgroundWorkers sync.WaitGroup
	defer backgroundWorkers.Wait()

	cancelCtx, cancelFn := context.WithCancel(context.Background())
	ms.cancelFn = cancelFn
	defer ms.cancelFn()

	// helper function to manage polling functions
	startPolling := func(ctx context.Context, period time.Duration, fn func(context.Context) error) {
		backgroundWorkers.Add(1)
		goutils.ManagedGo(func() {
			ticker := time.NewTicker(period)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := fn(ctx); err != nil {
						errChan <- err
					}
					return
				}
			}
		}, backgroundWorkers.Done)
	}

	// start goroutine to replan
	backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-successChan:
				return
			case <-replanChan:
				fmt.Println("replanning")

				ms.cancelFn()
				cancelCtx, cancelFn = context.WithCancel(ctx)
				ms.cancelFn = cancelFn

				inputs, err := kb.CurrentInputs(ctx)
				if err != nil {
					errChan <- err
					return
				}
				if len(kb.Kinematics().DoF()) == 2 {
					inputs = inputs[:2]
				}
				planRequest.Inputs = map[string][]referenceframe.Input{componentName.Name: inputs}

				plan, err := motionplan.PlanMotion(ctx, planRequest)
				if err != nil {
					errChan <- err
					return
				}
				planChan <- plan

				// drain the replanChan
				for len(replanChan) > 0 {
					<-replanChan
				}

				startPolling(cancelCtx, positionPollingPeriod, func(ctx context.Context) error {
					// TODO: the function that actually monitors position
					fmt.Println("position poll")
					return nil
				})
				startPolling(cancelCtx, obstaclePollingPeriod, func(ctx context.Context) error {
					// TODO: the function that actually monitors obstacles
					fmt.Println("obstacle poll")
					replanChan <- true
					return nil
				})
			}
		}
	}, backgroundWorkers.Done)

	// execution loop, exits when distance between GPS position reported by movement sensor is within PlanDeviationThreshold of the goal
	// after it exits, deferred statements will execute cleaning up the other threads
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case err := <-errChan:
			return false, err
		case plan := <-planChan:
			if err := ms.executePlan(cancelCtx, kb, plan); err != nil {
				return false, err
			}

			position, _, err := movementSensor.Position(cancelCtx, nil)
			if err != nil {
				return false, err
			}

			ms.logger.Infof("position: %v", position)
			// TODO: dont do the calculation to mm here
			if spatialmath.GeoPointToPose(position, destination).Point().Norm() <= 1e3*motionCfg.PlanDeviationM {
				successChan <- true
				return true, nil
			}
		}
	}
}

func (ms *builtIn) newMoveOnGlobeRequest(
	ctx context.Context,
	componentName resource.Name,
	destination *geo.Point,
	movementSensor movementsensor.MovementSensor,
	obstacles []*spatialmath.GeoObstacle,
	motionCfg *motion.MotionConfiguration,
	extra map[string]interface{},
) (*motionplan.PlanRequest, kinematicbase.KinematicBase, error) {
	kinematicsOptions := kinematicbase.NewKinematicBaseOptions()
	if motionCfg.LinearMPerSec != 0 {
		kinematicsOptions.LinearVelocityMMPerSec = motionCfg.LinearMPerSec * 1000
	}
	if motionCfg.AngularDegsPerSec != 0 {
		kinematicsOptions.AngularVelocityDegsPerSec = motionCfg.AngularDegsPerSec
	}
	if motionCfg.PlanDeviationM != 0 {
		kinematicsOptions.PlanDeviationThresholdMM = motionCfg.PlanDeviationM * 1000
	}
	kinematicsOptions.GoalRadiusMM = math.Min(motionCfg.PlanDeviationM*1000, 3000)
	kinematicsOptions.HeadingThresholdDegrees = 8

	// build the localizer from the movement sensor
	origin, _, err := movementSensor.Position(ctx, nil)
	if err != nil {
		return nil, nil, err
	}

	// add an offset between the movement sensor and the base if it is applicable
	baseOrigin := referenceframe.NewPoseInFrame(componentName.ShortName(), spatialmath.NewZeroPose())
	movementSensorToBase, err := ms.fsService.TransformPose(ctx, baseOrigin, movementSensor.Name().ShortName(), nil)
	if err != nil {
		movementSensorToBase = baseOrigin
	}
	localizer := motion.NewMovementSensorLocalizer(movementSensor, origin, movementSensorToBase.Pose())

	// convert destination into spatialmath.Pose with respect to where the localizer was initialized
	goal := spatialmath.GeoPointToPose(destination, origin)

	// convert GeoObstacles into GeometriesInFrame with respect to the base's starting point
	geoms := spatialmath.GeoObstaclesToGeometries(obstacles, origin)

	gif := referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)
	worldState, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{gif}, nil)
	if err != nil {
		return nil, nil, err
	}

	// construct limits
	straightlineDistance := goal.Point().Norm()
	if straightlineDistance > maxTravelDistance {
		return nil, nil, fmt.Errorf("cannot move more than %d kilometers", int(maxTravelDistance*1e-6))
	}
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
			kinematicsOptions.PositionOnlyMode = motionProfile == motionplan.PositionOnlyMotionProfile
		}
	}
	ms.logger.Debugf("base limits: %v", limits)

	// create a KinematicBase from the componentName
	baseComponent, ok := ms.components[componentName]
	if !ok {
		return nil, nil, resource.NewNotFoundError(componentName)
	}
	b, ok := baseComponent.(base.Base)
	if !ok {
		return nil, nil, fmt.Errorf("cannot move component of type %T because it is not a Base", baseComponent)
	}

	kb, err := kinematicbase.WrapWithKinematics(ctx, b, ms.logger, localizer, limits, kinematicsOptions)
	if err != nil {
		return nil, nil, err
	}

	// create a new empty framesystem which we add the kinematic base to
	fs := referenceframe.NewEmptyFrameSystem("")
	kbf := kb.Kinematics()
	if err := fs.AddFrame(kbf, fs.World()); err != nil {
		return nil, nil, err
	}

	// TODO(RSDK-3407): this does not adequately account for geometries right now since it is a transformation after the fact.
	// This is probably acceptable for the time being, but long term the construction of the frame system for the kinematic base should
	// be moved under the purview of the kinematic base wrapper instead of being done here.
	offsetFrame, err := referenceframe.NewStaticFrame("offset", movementSensorToBase.Pose())
	if err != nil {
		return nil, nil, err
	}
	if err := fs.AddFrame(offsetFrame, kbf); err != nil {
		return nil, nil, err
	}

	return &motionplan.PlanRequest{
		Logger:      ms.logger,
		Goal:        referenceframe.NewPoseInFrame(referenceframe.World, goal),
		Frame:       offsetFrame,
		FrameSystem: fs,
		Inputs:      referenceframe.StartPositions(fs),
		WorldState:  worldState,
		Options:     extra,
	}, kb, nil
}

func (ms *builtIn) executePlan(ctx context.Context, kinematicBase kinematicbase.KinematicBase, plan motionplan.Plan) error {
	waypoints, err := plan.GetFrameSteps(kinematicBase.Name().Name)
	if err != nil {
		return err
	}

	for i := 1; i < len(waypoints); i++ {
		select {
		case <-ctx.Done():
			return nil
		default:
			ms.logger.Info(waypoints[i])
			segment := []referenceframe.Input{
				{Value: waypoints[i][0].Value - waypoints[i-1][0].Value},
				{Value: waypoints[i][1].Value - waypoints[i-1][1].Value},
			}
			if err := kinematicBase.GoToInputs(ctx, segment); err != nil {
				// If there is an error on GoToInputs, stop the component if possible before returning the error
				if stopErr := kinematicBase.Stop(ctx, nil); stopErr != nil {
					return errors.Wrap(err, stopErr.Error())
				}
				return err
			}
		}
	}
	return nil
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
	kinematicsOptions kinematicbase.Options,
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
			kinematicsOptions.PositionOnlyMode = motionProfile == motionplan.PositionOnlyMotionProfile
		}
	}

	kb, err := kinematicbase.WrapWithKinematics(ctx, b, ms.logger, motion.NewSLAMLocalizer(slamSvc), limits, kinematicsOptions)
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

	// get current position
	inputs, err := kb.CurrentInputs(ctx)
	if err != nil {
		return nil, nil, err
	}
	if kinematicsOptions.PositionOnlyMode {
		inputs = inputs[:2]
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
	plan, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:          ms.logger,
		Goal:            dst,
		Frame:           f,
		Inputs:          seedMap,
		FrameSystem:     fs,
		WorldState:      worldState,
		ConstraintSpecs: nil,
		Options:         extra,
	})
	if err != nil {
		return nil, nil, err
	}
	steps, err := plan.GetFrameSteps(f.Name())
	return steps, kb, err
}
