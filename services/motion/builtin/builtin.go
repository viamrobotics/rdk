// Package builtin implements a motion service.
package builtin

import (
	"context"
	"fmt"
	"sync"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	servicepb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

var errUnimplemented = errors.New("unimplemented")

func init() {
	resource.RegisterDefaultService(
		motion.API,
		resource.DefaultServiceModel,
		resource.Registration[motion.Service, *Config]{
			Constructor: NewBuiltIn,
			WeakDependencies: []internal.ResourceMatcher{
				internal.SLAMDependencyWildcardMatcher,
				internal.ComponentDependencyWildcardMatcher,
				internal.VisionDependencyWildcardMatcher,
			},
		})
}

const (
	builtinOpLabel      = "motion-service"
	maxTravelDistanceMM = 5e6 // this is equivalent to 5km
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
type Config struct {
	LogFilePath string `json:"log_file_path"`
}

// Validate here adds a dependency on the internal framesystem service.
func (c *Config) Validate(path string) ([]string, error) {
	return []string{framesystem.InternalServiceName.String()}, nil
}

// NewBuiltIn returns a new move and grab service for the given robot.
func NewBuiltIn(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (motion.Service, error) {
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

	config, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}
	if config.LogFilePath != "" {
		logger, err := rdkutils.NewFilePathDebugLogger(config.LogFilePath, "motion")
		if err != nil {
			return err
		}
		ms.logger = logger
	}
	movementSensors := make(map[resource.Name]movementsensor.MovementSensor)
	slamServices := make(map[resource.Name]slam.Service)
	visionServices := make(map[resource.Name]vision.Service)
	components := make(map[resource.Name]resource.Resource)
	for name, dep := range deps {
		switch dep := dep.(type) {
		case framesystem.Service:
			ms.fsService = dep
		case movementsensor.MovementSensor:
			movementSensors[name] = dep
		case slam.Service:
			slamServices[name] = dep
		case vision.Service:
			visionServices[name] = dep
		default:
			components[name] = dep
		}
	}
	ms.movementSensors = movementSensors
	ms.slamServices = slamServices
	ms.visionServices = visionServices
	ms.components = components
	return nil
}

type builtIn struct {
	resource.Named
	resource.TriviallyCloseable
	fsService       framesystem.Service
	movementSensors map[resource.Name]movementsensor.MovementSensor
	slamServices    map[resource.Name]slam.Service
	visionServices  map[resource.Name]vision.Service
	components      map[resource.Name]resource.Resource
	logger          logging.Logger
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
		Logger:             ms.logger,
		Goal:               goalPose,
		Frame:              movingFrame,
		StartConfiguration: fsInputs,
		FrameSystem:        frameSys,
		WorldState:         worldState,
		ConstraintSpecs:    constraints,
		Options:            extra,
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
	valExtra, err := newValidatedExtra(extra)
	if err != nil {
		return false, err
	}
	// make call to motionplan
	mr, err := ms.newMoveOnMapRequest(ctx, componentName, destination, slamName, valExtra)
	if err != nil {
		return false, fmt.Errorf("error making plan for MoveOnMap: %w", err)
	}

	ma := newMoveAttempt(ctx, mr)
	if err := ma.start(); err != nil {
		return false, err
	}

	// this ensures that if the context is cancelled we always return early
	if err := ctx.Err(); err != nil {
		ma.cancel()
		return false, err
	}

	select {
	// if context was cancelled by the calling function, error out
	case <-ctx.Done():
		ma.cancel()
		return false, ctx.Err()

	// once execution responds: return the result to the caller
	case resp := <-ma.responseChan:
		mr.planRequest.Logger.Info("got move on map response")
		ms.logger.Debugf("execution completed: %s", resp)
		ma.cancel()
		return resp.success, resp.err
	}
}

type validatedExtra struct {
	maxReplans       int
	replanCostFactor float64
	motionProfile    string
	extra            map[string]interface{}
}

func newValidatedExtra(extra map[string]interface{}) (validatedExtra, error) {
	maxReplans := -1
	replanCostFactor := defaultReplanCostFactor
	motionProfile := ""
	v := validatedExtra{}
	if extra == nil {
		return v, nil
	}
	if replansRaw, ok := extra["max_replans"]; ok {
		if replans, ok := replansRaw.(int); ok {
			maxReplans = replans
		}
	}
	if profile, ok := extra["motion_profile"]; ok {
		motionProfile, ok = profile.(string)
		if !ok {
			return v, errors.New("could not interpret motion_profile field as string")
		}
	}
	if costFactorRaw, ok := extra["replan_cost_factor"]; ok {
		costFactor, ok := costFactorRaw.(float64)
		if !ok {
			return validatedExtra{}, errors.New("could not interpret replan_cost_factor field as float")
		}
		replanCostFactor = costFactor
	}
	return validatedExtra{
		maxReplans:       maxReplans,
		motionProfile:    motionProfile,
		replanCostFactor: replanCostFactor,
		extra:            extra,
	}, nil
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
	t := "MoveOnGlobe called for component: %s, destination: %+v, heading: %f, movementSensor: %s, obstacles: %v, motionCfg: %#v, extra: %s"
	ms.logger.Debugf(t,
		componentName,
		destination,
		heading,
		movementSensorName,
		obstacles,
		motionCfg,
		extra,
	)
	operation.CancelOtherWithLabel(ctx, builtinOpLabel)
	valExtra, err := newValidatedExtra(extra)
	if err != nil {
		return false, err
	}

	mr, err := ms.newMoveOnGlobeRequest(ctx, componentName, destination, movementSensorName, obstacles, motionCfg, nil, valExtra)
	if err != nil {
		return false, err
	}

	replanCount := 0
	// start a loop that plans every iteration and exits when something is read from the success channel
	for {
		ma := newMoveAttempt(ctx, mr)
		if err := ma.start(); err != nil {
			return false, err
		}

		// this ensures that if the context is cancelled we always return early at the top of the loop
		if err := ctx.Err(); err != nil {
			ma.cancel()
			return false, err
		}

		select {
		// if context was cancelled by the calling function, error out
		case <-ctx.Done():
			ma.cancel()
			return false, ctx.Err()

		// once execution responds: return the result to the caller
		case resp := <-ma.responseChan:
			ms.logger.Debugf("execution response: %s", resp)
			ma.cancel()

			// If we have a false `success` and nil error, that means replan
			if resp.success || resp.err != nil {
				return resp.success, resp.err
			}
			ms.logger.Info("reached end of plan, but not at goal; triggering a replan")

		// if the position poller hit an error return it, otherwise replan
		case resp := <-ma.position.responseChan:
			ms.logger.Debugf("position response: %s", resp)
			ma.cancel()
			if resp.err != nil {
				return false, resp.err
			}
			ms.logger.Info("position drift triggering a replan")

		// if the obstacle poller hit an error return it, otherwise replan
		case resp := <-ma.obstacle.responseChan:
			ms.logger.Debugf("obstacle response: %s", resp)
			ma.cancel()
			if resp.err != nil {
				return false, resp.err
			}
			ms.logger.Info("obstacle detection triggering a replan")
		}

		if valExtra.maxReplans >= 0 {
			replanCount++
			if replanCount > valExtra.maxReplans {
				return false, fmt.Errorf("exceeded maximum number of replans: %d", valExtra.maxReplans)
			}
		}
		// TODO: RSDK-4509 obstacles should include any transient obstacles which may have triggered a replan, if any.
		mr, err = ms.newMoveOnGlobeRequest(ctx, componentName, destination, movementSensorName, obstacles, motionCfg, mr.seedPlan, valExtra)
		if err != nil {
			return false, err
		}
	}
}

func (ms *builtIn) MoveOnGlobeNew(ctx context.Context, req motion.MoveOnGlobeReq) (string, error) {
	return "", errUnimplemented
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
			spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0}),
		),
		destinationFrame,
		supplementalTransforms,
	)
}

func (ms *builtIn) StopPlan(
	ctx context.Context,
	req motion.StopPlanReq,
) error {
	return errUnimplemented
}

func (ms *builtIn) ListPlanStatuses(
	ctx context.Context,
	req motion.ListPlanStatusesReq,
) ([]motion.PlanStatusWithID, error) {
	return nil, errUnimplemented
}

func (ms *builtIn) PlanHistory(
	ctx context.Context,
	req motion.PlanHistoryReq,
) ([]motion.PlanWithStatus, error) {
	return nil, errUnimplemented
}
