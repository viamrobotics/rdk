// Package explore implements a motion service for exploration. This motion service model is a temporary model
// that will be incorporated into the rdk:builtin:motion service in the future.
package explore

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	servicepb "go.viam.com/api/service/motion/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var (
	model            = resource.DefaultModelFamily.WithModel("explore")
	errUnimplemented = errors.New("unimplemented")
	// The distance a detected obstacle can be from a base to trigger the Move command to stop.
	lookAheadDistanceMM = 500.
	// successiveErrorLimit places a limit on the number of errors that can occur in a row when running
	// checkForObstacles.
	successiveErrorLimit = 5
	// Defines the limit on how far a potential kinematic base move action can be. For explore there is none.
	defaultMoveLimitMM = math.Inf(1)
	// The timeout for any individual move action on the kinematic base.
	defaultExploreTimeout = 100 * time.Second
	// The max angle the kinematic base spin action can be.
	defaultMaxSpinAngleDegs = 180.
)

func init() {
	resource.RegisterService(
		motion.API,
		model,
		resource.Registration[motion.Service, *Config]{
			Constructor: NewExplore,
			WeakDependencies: []resource.Matcher{
				resource.TypeMatcher{Type: resource.APITypeComponentName},
				resource.SubtypeMatcher{Subtype: vision.SubtypeName},
			},
		},
	)
}

const (
	exploreOpLabel = "explore-motion-service"
)

// moveResponse is the message type returned by both channels: obstacle detection and plan execution.
// It contains a bool stating if an obstacle was found or the execution was complete and/or any
// associated error.
type moveResponse struct {
	err     error
	success bool
}

// inputEnabledActuator is an actuator that interacts with the frame system.
// This allows us to figure out where the actuator currently is and then
// move it. Input units are always in meters or radians.
type inputEnabledActuator interface {
	resource.Actuator
	referenceframe.InputEnabled
}

// obstacleDetectorObject provides a map for matching vision services to any and all cameras names they use.
type obstacleDetectorObject map[vision.Service]resource.Name

// Config describes how to configure the service. As the explore motion service is not being surfaced to users, this is blank.
type Config struct{}

// Validate here adds a dependency on the internal framesystem service.
func (c *Config) Validate(path string) ([]string, error) {
	return []string{framesystem.InternalServiceName.String()}, nil
}

// NewExplore returns a new explore motion service for the given robot.
func NewExplore(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (motion.Service, error) {
	ms := &explore{
		Named:                 conf.ResourceName().AsNamed(),
		logger:                logger,
		obstacleResponseChan:  make(chan moveResponse),
		executionResponseChan: make(chan moveResponse),
		backgroundWorkers:     &sync.WaitGroup{},
	}

	if err := ms.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return ms, nil
}

// Reconfigure updates the explore motion service when the config has changed.
func (ms *explore) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) (err error) {
	ms.resourceMutex.Lock()
	defer ms.resourceMutex.Unlock()
	// Iterate over dependencies and store components and services along with the frame service directly
	components := make(map[resource.Name]resource.Resource)
	services := make(map[resource.Name]resource.Resource)
	for name, dep := range deps {
		switch dep := dep.(type) {
		case framesystem.Service:
			ms.fsService = dep
		case vision.Service:
			services[name] = dep
		default:
			components[name] = dep
		}
	}

	ms.components = components
	ms.services = services
	return nil
}

type explore struct {
	resource.Named
	logger logging.Logger

	processCancelFunc     context.CancelFunc
	obstacleResponseChan  chan moveResponse
	executionResponseChan chan moveResponse
	backgroundWorkers     *sync.WaitGroup

	// resourceMutex protects the connect to other resources, the frame service/system and prevent multiple
	// Move and/or Reconfigure actions from being performed simultaneously.
	resourceMutex sync.Mutex
	components    map[resource.Name]resource.Resource
	services      map[resource.Name]resource.Resource
	fsService     framesystem.Service
	frameSystem   referenceframe.FrameSystem
}

func (ms *explore) MoveOnMap(ctx context.Context, req motion.MoveOnMapReq) (motion.ExecutionID, error) {
	return uuid.Nil, errUnimplemented
}

func (ms *explore) MoveOnGlobe(
	ctx context.Context,
	req motion.MoveOnGlobeReq,
) (motion.ExecutionID, error) {
	return uuid.Nil, errUnimplemented
}

func (ms *explore) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*referenceframe.LinkInFrame,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	return nil, errUnimplemented
}

func (ms *explore) StopPlan(
	ctx context.Context,
	req motion.StopPlanReq,
) error {
	return errUnimplemented
}

func (ms *explore) ListPlanStatuses(
	ctx context.Context,
	req motion.ListPlanStatusesReq,
) ([]motion.PlanStatusWithID, error) {
	return nil, errUnimplemented
}

func (ms *explore) PlanHistory(
	ctx context.Context,
	req motion.PlanHistoryReq,
) ([]motion.PlanWithStatus, error) {
	return nil, errUnimplemented
}

func (ms *explore) Close(ctx context.Context) error {
	ms.resourceMutex.Lock()
	defer ms.resourceMutex.Unlock()

	if ms.processCancelFunc != nil {
		ms.processCancelFunc()
	}
	utils.FlushChan(ms.obstacleResponseChan)
	utils.FlushChan(ms.executionResponseChan)
	ms.backgroundWorkers.Wait()
	return nil
}

// Move takes a goal location and will plan and execute a movement to move a component specified by its name
// to that destination.
func (ms *explore) Move(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	constraints *servicepb.Constraints,
	extra map[string]interface{},
) (bool, error) {
	ms.resourceMutex.Lock()
	defer ms.resourceMutex.Unlock()

	operation.CancelOtherWithLabel(ctx, exploreOpLabel)

	// Parse extras
	motionCfg, err := parseMotionConfig(extra)
	if err != nil {
		return false, err
	}

	// obstacleDetectors
	obstacleDetectors, err := ms.createObstacleDetectors(motionCfg)
	if err != nil {
		return false, err
	}

	// Create kinematic base
	kb, err := ms.createKinematicBase(ctx, componentName, motionCfg)
	if err != nil {
		return false, err
	}

	// Create motionplan plan
	plan, err := ms.createMotionPlan(ctx, kb, destination, extra)
	if err != nil {
		return false, err
	}

	// Start background processes
	cancelCtx, cancel := context.WithCancel(ctx)
	ms.processCancelFunc = cancel
	defer ms.processCancelFunc()

	// Start polling for obstacles
	ms.backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		ms.checkForObstacles(cancelCtx, obstacleDetectors, kb, plan, motionCfg.ObstaclePollingFreqHz)
	}, ms.backgroundWorkers.Done)

	// Start executing plan
	ms.backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		ms.executePlan(cancelCtx, kb, plan)
	}, ms.backgroundWorkers.Done)

	// this ensures that if the context is cancelled we always return early at the top of the loop
	if err := ctx.Err(); err != nil {
		return false, err
	}

	select {
	// if context was cancelled by the calling function, error out
	case <-ctx.Done():
		return false, ctx.Err()

	// once execution responds: return the result to the caller
	case resp := <-ms.executionResponseChan:
		ms.logger.CDebugf(ctx, "execution completed: %v", resp)
		if resp.err != nil {
			return resp.success, resp.err
		}
		if resp.success {
			return true, nil
		}

	// if the checkPartialPlan process hit an error return it, otherwise exit
	case resp := <-ms.obstacleResponseChan:
		ms.logger.CDebugf(ctx, "obstacle response: %v", resp)
		if resp.err != nil {
			return resp.success, resp.err
		}
		if resp.success {
			return false, nil
		}
	}
	return false, errors.New("no obstacle was seen during movement to goal and goal was not reached")
}

// checkForObstacles will continuously monitor the generated transient worldState for obstacles in the given
// motionplan plan. A response will be sent through the channel if an error occurs, the motionplan plan
// completes or an obstacle is detected in the given range.
func (ms *explore) checkForObstacles(
	ctx context.Context,
	obstacleDetectors []obstacleDetectorObject,
	kb kinematicbase.KinematicBase,
	plan motionplan.Plan,
	obstaclePollingFrequencyHz float64,
) {
	ms.logger.Debug("Current Plan")
	ms.logger.Debug(plan)

	// Constantly check for obstacles in path at desired obstacle polling frequency
	ticker := time.NewTicker(time.Duration(int(1000/obstaclePollingFrequencyHz)) * time.Millisecond)
	defer ticker.Stop()

	var errCounterCurrentInputs, errCounterGenerateTransientWorldState int
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:

			currentInputs, err := kb.CurrentInputs(ctx)
			if err != nil {
				ms.logger.Debugf("issue occurred getting current inputs from kinematic base: %v", err)
				if errCounterCurrentInputs > successiveErrorLimit {
					ms.obstacleResponseChan <- moveResponse{success: false}
					return
				}
				errCounterCurrentInputs++
			} else {
				errCounterCurrentInputs = 0
			}

			currentPose := spatialmath.NewPose(
				r3.Vector{X: currentInputs[0].Value, Y: currentInputs[1].Value, Z: 0},
				&spatialmath.OrientationVector{OX: 0, OY: 0, OZ: 1, Theta: currentInputs[2].Value},
			)

			// Look for new transient obstacles and add to worldState
			worldState, err := ms.generateTransientWorldState(ctx, obstacleDetectors)
			if err != nil {
				ms.logger.CDebugf(ctx, "issue occurred generating transient worldState: %v", err)
				if errCounterGenerateTransientWorldState > successiveErrorLimit {
					ms.obstacleResponseChan <- moveResponse{success: false}
					return
				}
				errCounterGenerateTransientWorldState++
			} else {
				errCounterGenerateTransientWorldState = 0
			}

			// TODO: Generalize this fix to work for maps with non-transient obstacles. This current implementation
			// relies on the plan being two steps: a start position and a goal position.
			// JIRA Ticket: https://viam.atlassian.net/browse/RSDK-5964
			planTraj := plan.Trajectory()
			planTraj[0][kb.Name().ShortName()] = currentInputs
			ms.logger.Debugf("Current transient worldState: ", worldState.String())

			executionState, err := motionplan.NewExecutionState(
				plan,
				0,
				plan.Trajectory()[0],
				map[string]*referenceframe.PoseInFrame{
					kb.Kinematics().Name(): referenceframe.NewPoseInFrame(referenceframe.World, currentPose),
				},
			)
			if err != nil {
				ms.logger.Debugf("issue occurred getting execution state from kinematic base: %v", err)
				if errCounterCurrentInputs > successiveErrorLimit {
					ms.obstacleResponseChan <- moveResponse{success: false}
					return
				}
				errCounterCurrentInputs++
			}

			// Check plan for transient obstacles
			err = motionplan.CheckPlan(
				kb.Kinematics(),
				executionState,
				worldState,
				ms.frameSystem,
				lookAheadDistanceMM,
				ms.logger,
			)
			if err != nil {
				if strings.Contains(err.Error(), "found error between positions") {
					ms.logger.CDebug(ctx, "collision found in given range")
					ms.obstacleResponseChan <- moveResponse{success: true}
					return
				}
			}
		}
	}
}

// executePlan will carry out the desired motionplan plan.
func (ms *explore) executePlan(ctx context.Context, kb kinematicbase.KinematicBase, plan motionplan.Plan) {
	steps, err := plan.Trajectory().GetFrameInputs(kb.Name().Name)
	if err != nil {
		ms.logger.Debugf("error in executePlan: %s", err)
		return
	}
	for _, inputs := range steps {
		select {
		case <-ctx.Done():
			return
		default:
			if inputEnabledKb, ok := kb.(inputEnabledActuator); ok {
				if err := inputEnabledKb.GoToInputs(ctx, inputs); err != nil {
					// If there is an error on GoToInputs, stop the component if possible before returning the error
					if stopErr := kb.Stop(ctx, nil); stopErr != nil {
						ms.executionResponseChan <- moveResponse{err: err}
						return
					}
					// If the error was simply a cancellation of context return without erroring out
					if errors.Is(err, context.Canceled) {
						return
					}
					ms.executionResponseChan <- moveResponse{err: err}
					return
				}
			} else {
				ms.executionResponseChan <- moveResponse{err: errors.New("unable to cast kinematic base to inputEnabledActuator")}
				return
			}
		}
	}
	ms.executionResponseChan <- moveResponse{success: true}
}

// generateTransientWorldState will create a new worldState with transient obstacles generated by the provided
// obstacleDetectors.
func (ms *explore) generateTransientWorldState(
	ctx context.Context,
	obstacleDetectors []obstacleDetectorObject,
) (*referenceframe.WorldState, error) {
	geometriesInFrame := []*referenceframe.GeometriesInFrame{}

	// Iterate through provided obstacle detectors and their associated vision service and cameras
	for _, obstacleDetector := range obstacleDetectors {
		for visionService, cameraName := range obstacleDetector {
			// Get detections as vision objects
			detections, err := visionService.GetObjectPointClouds(ctx, cameraName.Name, nil)
			if err != nil && strings.Contains(err.Error(), "does not implement a 3D segmenter") {
				ms.logger.CInfof(ctx, "cannot call GetObjectPointClouds on %q as it does not implement a 3D segmenter",
					visionService.Name())
			} else if err != nil {
				return nil, err
			}

			// Extract geometries from vision objects
			geometries := []spatialmath.Geometry{}
			for i, detection := range detections {
				geometry := detection.Geometry
				label := cameraName.Name + "_transientObstacle_" + strconv.Itoa(i)
				if geometry.Label() != "" {
					label += "_" + geometry.Label()
				}
				geometry.SetLabel(label)
				geometries = append(geometries, geometry)
			}
			geometriesInFrame = append(geometriesInFrame,
				referenceframe.NewGeometriesInFrame(cameraName.Name, geometries),
			)
		}
	}

	// Add geometries to worldState
	worldState, err := referenceframe.NewWorldState(geometriesInFrame, nil)
	if err != nil {
		return nil, err
	}
	return worldState, nil
}

// createKinematicBase will instantiate a kinematic base from the provided base name and motionCfg with
// associated kinematic base options. This will be a differential drive kinematic base.
func (ms *explore) createKinematicBase(
	ctx context.Context,
	baseName resource.Name,
	motionCfg motion.MotionConfiguration,
) (kinematicbase.KinematicBase, error) {
	// Select the base from the component list using the baseName
	component, ok := ms.components[baseName]
	if !ok {
		return nil, resource.DependencyNotFoundError(baseName)
	}

	b, ok := component.(base.Base)
	if !ok {
		return nil, fmt.Errorf("cannot get component of type %T because it is not a Case", component)
	}

	// Generate kinematic base options from motionCfg
	kinematicsOptions := kinematicbase.NewKinematicBaseOptions()
	kinematicsOptions.NoSkidSteer = true
	kinematicsOptions.UsePTGs = false
	// PositionOnlyMode needs to be false to allow orientation info to be available in CurrentInputs as
	// we need that to correctly place obstacles in world's reference frame.
	kinematicsOptions.PositionOnlyMode = false
	kinematicsOptions.AngularVelocityDegsPerSec = motionCfg.AngularDegsPerSec
	kinematicsOptions.LinearVelocityMMPerSec = motionCfg.LinearMPerSec * 1000
	kinematicsOptions.Timeout = defaultExploreTimeout
	kinematicsOptions.MaxSpinAngleDeg = defaultMaxSpinAngleDegs
	kinematicsOptions.MaxMoveStraightMM = defaultMoveLimitMM

	// Create new kinematic base (differential drive)
	kb, err := kinematicbase.WrapWithKinematics(
		ctx,
		b,
		ms.logger,
		nil,
		[]referenceframe.Limit{
			{Min: -defaultMoveLimitMM, Max: defaultMoveLimitMM},
			{Min: -defaultMoveLimitMM, Max: defaultMoveLimitMM},
			{Min: -2 * math.Pi, Max: 2 * math.Pi},
		},
		kinematicsOptions,
	)
	if err != nil {
		return nil, err
	}

	return kb, nil
}

// createObstacleDetectors will generate the list of obstacle detectors from the camera and vision services
// names provided in them motionCfg.
func (ms *explore) createObstacleDetectors(motionCfg motion.MotionConfiguration) ([]obstacleDetectorObject, error) {
	var obstacleDetectors []obstacleDetectorObject
	// Iterate through obstacleDetectorsNames
	for _, obstacleDetectorsName := range motionCfg.ObstacleDetectors {
		// Select the vision service from the service list using the vision service name in obstacleDetectorsNames
		visionServiceResource, ok := ms.services[obstacleDetectorsName.VisionServiceName]
		if !ok {
			return nil, resource.DependencyNotFoundError(obstacleDetectorsName.VisionServiceName)
		}
		visionService, ok := visionServiceResource.(vision.Service)
		if !ok {
			return nil, fmt.Errorf("cannot get service of type %T because it is not a vision service",
				visionServiceResource,
			)
		}

		// Select the camera from the component list using the camera name in obstacleDetectorsNames
		// Note: May need to be converted to a for loop if we accept multiple cameras for each vision service
		cameraResource, ok := ms.components[obstacleDetectorsName.CameraName]
		if !ok {
			return nil, resource.DependencyNotFoundError(obstacleDetectorsName.CameraName)
		}
		cam, ok := cameraResource.(camera.Camera)
		if !ok {
			return nil, fmt.Errorf("cannot get component of type %T because it is not a camera", cameraResource)
		}
		obstacleDetectors = append(obstacleDetectors, obstacleDetectorObject{visionService: cam.Name()})
	}

	return obstacleDetectors, nil
}

// createMotionPlan will construct a motion plan towards a specified destination using the given kinematic base.
// No position knowledge is assumed so every plan uses the base, and therefore world, origin as the starting location.
func (ms *explore) createMotionPlan(
	ctx context.Context,
	kb kinematicbase.KinematicBase,
	destination *referenceframe.PoseInFrame,
	extra map[string]interface{},
) (motionplan.Plan, error) {
	if destination.Pose().Point().Norm() >= defaultMoveLimitMM {
		return nil, errors.Errorf("destination %v is above the defined limit of %v", destination.Pose().Point().String(), defaultMoveLimitMM)
	}

	worldState, err := referenceframe.NewWorldState(nil, nil)
	if err != nil {
		return nil, err
	}

	ms.frameSystem, err = ms.fsService.FrameSystem(ctx, worldState.Transforms())
	if err != nil {
		return nil, err
	}

	// replace origin base frame to remove original differential drive kinematic base geometry as it is overwritten by a bounding sphere
	// during kinematic base creation
	if err := ms.frameSystem.ReplaceFrame(referenceframe.NewZeroStaticFrame(kb.Kinematics().Name() + "_origin")); err != nil {
		return nil, err
	}

	// replace original base frame with one that knows how to move itself and allow planning for
	if err := ms.frameSystem.ReplaceFrame(kb.Kinematics()); err != nil {
		return nil, err
	}

	inputs := make([]referenceframe.Input, len(kb.Kinematics().DoF()))

	f := kb.Kinematics()

	seedMap := map[string][]referenceframe.Input{f.Name(): inputs}

	goalPose, err := ms.fsService.TransformPose(ctx, destination, ms.frameSystem.World().Name(), nil)
	if err != nil {
		return nil, err
	}

	ms.logger.CDebugf(ctx, "goal position: %v", goalPose.Pose().Point())
	return motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:             ms.logger,
		Goal:               goalPose,
		Frame:              f,
		StartConfiguration: seedMap,
		FrameSystem:        ms.frameSystem,
		WorldState:         worldState,
		ConstraintSpecs:    nil,
		Options:            extra,
	})
}

// parseMotionConfig extracts the MotionConfiguration from extra's.
func parseMotionConfig(extra map[string]interface{}) (motion.MotionConfiguration, error) {
	motionCfgInterface, ok := extra["motionCfg"]
	if !ok {
		return motion.MotionConfiguration{}, errors.New("no motionCfg provided")
	}

	motionCfg, ok := motionCfgInterface.(motion.MotionConfiguration)
	if !ok {
		return motion.MotionConfiguration{}, errors.New("could not interpret motionCfg field as an MotionConfiguration")
	}
	return motionCfg, nil
}
