// Package builtin implements a motion service.
package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	pb "go.viam.com/api/service/motion/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/state"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func init() {
	resource.RegisterDefaultService(
		motion.API,
		resource.DefaultServiceModel,
		resource.Registration[motion.Service, *Config]{
			Constructor: NewBuiltIn,
			WeakDependencies: []resource.Matcher{
				resource.TypeMatcher{Type: resource.APITypeComponentName},
				resource.SubtypeMatcher{Subtype: slam.SubtypeName},
				resource.SubtypeMatcher{Subtype: vision.SubtypeName},
			},
		},
	)
}

// export keys to be used with DoCommand so they can be referenced by clients.
const (
	DoPlan    = "plan"
	DoExecute = "execute"
)

const (
	builtinOpLabel                     = "motion-service"
	maxTravelDistanceMM                = 5e6 // this is equivalent to 5km
	lookAheadDistanceMM        float64 = 5e6
	defaultSmoothIter                  = 30
	defaultAngularDegsPerSec           = 60.
	defaultLinearMPerSec               = 0.3
	defaultSlamPlanDeviationM          = 1.
	defaultGlobePlanDeviationM         = 2.6
)

var (
	defaultPositionPollingHz = 1.
	defaultObstaclePollingHz = 1.
)

var (
	stateTTL              = time.Hour * 24
	stateTTLCheckInterval = time.Minute
)

// inputEnabledActuator is an actuator that interacts with the frame system.
// This allows us to figure out where the actuator currently is and then
// move it. Input units are always in meters or radians.
type inputEnabledActuator interface {
	resource.Actuator
	framesystem.InputEnabled
}

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
) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	config, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}
	if config.LogFilePath != "" {
		logger, err := utils.NewFilePathDebugLogger(config.LogFilePath, "motion")
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
	if ms.state != nil {
		ms.state.Stop()
	}

	state, err := state.NewState(stateTTL, stateTTLCheckInterval, ms.logger)
	if err != nil {
		return err
	}
	ms.state = state
	return nil
}

type builtIn struct {
	resource.Named
	mu              sync.RWMutex
	fsService       framesystem.Service
	movementSensors map[resource.Name]movementsensor.MovementSensor
	slamServices    map[resource.Name]slam.Service
	visionServices  map[resource.Name]vision.Service
	components      map[resource.Name]resource.Resource
	logger          logging.Logger
	state           *state.State
}

func (ms *builtIn) Close(ctx context.Context) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.state != nil {
		ms.state.Stop()
	}
	return nil
}

func (ms *builtIn) Move(ctx context.Context, req motion.MoveReq) (bool, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	operation.CancelOtherWithLabel(ctx, builtinOpLabel)

	plan, err := ms.plan(ctx, req)
	if err != nil {
		return false, err
	}
	err = ms.execute(ctx, plan.Trajectory())
	return err == nil, err
}

func (ms *builtIn) MoveOnMap(ctx context.Context, req motion.MoveOnMapReq) (motion.ExecutionID, error) {
	if err := ctx.Err(); err != nil {
		return uuid.Nil, err
	}
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	ms.logger.CDebugf(ctx, "MoveOnMap called with %s", req)

	// TODO: Deprecated: remove once no motion apis use the opid system
	operation.CancelOtherWithLabel(ctx, builtinOpLabel)

	id, err := state.StartExecution(ctx, ms.state, req.ComponentName, req, ms.newMoveOnMapRequest)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
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
		v.extra = map[string]interface{}{"smooth_iter": defaultSmoothIter}
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

	if _, ok := extra["smooth_iter"]; !ok {
		extra["smooth_iter"] = defaultSmoothIter
	}

	return validatedExtra{
		maxReplans:       maxReplans,
		motionProfile:    motionProfile,
		replanCostFactor: replanCostFactor,
		extra:            extra,
	}, nil
}

func (ms *builtIn) MoveOnGlobe(ctx context.Context, req motion.MoveOnGlobeReq) (motion.ExecutionID, error) {
	if err := ctx.Err(); err != nil {
		return uuid.Nil, err
	}
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	ms.logger.CDebugf(ctx, "MoveOnGlobe called with %s", req)
	// TODO: Deprecated: remove once no motion apis use the opid system
	operation.CancelOtherWithLabel(ctx, builtinOpLabel)

	id, err := state.StartExecution(ctx, ms.state, req.ComponentName, req, ms.newMoveOnGlobeRequest)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (ms *builtIn) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*referenceframe.LinkInFrame,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
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
	if err := ctx.Err(); err != nil {
		return err
	}
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.state.StopExecutionByResource(req.ComponentName)
}

func (ms *builtIn) ListPlanStatuses(
	ctx context.Context,
	req motion.ListPlanStatusesReq,
) ([]motion.PlanStatusWithID, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.state.ListPlanStatuses(req)
}

func (ms *builtIn) PlanHistory(
	ctx context.Context,
	req motion.PlanHistoryReq,
) ([]motion.PlanWithStatus, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.state.PlanHistory(req)
}

// DoCommand supports two commands which are specified through the command map
//   - DoPlan generates and returns a Trajectory for a given motionpb.MoveRequest without executing it
//     required key: DoPlan
//     input value: a motionpb.MoveRequest which will be used to create a Trajectory
//     output value: a motionplan.Trajectory specified as a map (the mapstructure.Decode function is useful for decoding this)
//   - DoExecute takes a Trajectory and executes it
//     required key: DoExecute
//     input value: a motionplan.Trajectory
//     output value: a bool
func (ms *builtIn) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	operation.CancelOtherWithLabel(ctx, builtinOpLabel)

	resp := make(map[string]interface{}, 0)
	if req, ok := cmd[DoPlan]; ok {
		s, err := utils.AssertType[string](req)
		if err != nil {
			return nil, err
		}
		var moveReqProto pb.MoveRequest
		err = protojson.Unmarshal([]byte(s), &moveReqProto)
		if err != nil {
			return nil, err
		}
		fields := moveReqProto.Extra.AsMap()
		if extra, err := utils.AssertType[map[string]interface{}](fields["fields"]); err == nil {
			v, err := structpb.NewStruct(extra)
			if err != nil {
				return nil, err
			}
			moveReqProto.Extra = v
		}
		moveReq, err := motion.MoveReqFromProto(&moveReqProto)
		if err != nil {
			return nil, err
		}
		plan, err := ms.plan(ctx, moveReq)
		if err != nil {
			return nil, err
		}
		resp[DoPlan] = plan.Trajectory()
	}
	if req, ok := cmd[DoExecute]; ok {
		var trajectory motionplan.Trajectory
		if err := mapstructure.Decode(req, &trajectory); err != nil {
			return nil, err
		}
		if err := ms.execute(ctx, trajectory); err != nil {
			return nil, err
		}
		resp[DoExecute] = true
	}
	return resp, nil
}

func (ms *builtIn) plan(ctx context.Context, req motion.MoveReq) (motionplan.Plan, error) {
	frameSys, err := ms.fsService.FrameSystem(ctx, req.WorldState.Transforms())
	if err != nil {
		return nil, err
	}

	// build maps of relevant components and inputs from initial inputs
	fsInputs, _, err := ms.fsService.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	ms.logger.CDebugf(ctx, "frame system inputs: %v", fsInputs)

	movingFrame := frameSys.Frame(req.ComponentName.ShortName())
	if movingFrame == nil {
		return nil, fmt.Errorf("component named %s not found in robot frame system", req.ComponentName.ShortName())
	}

	startState, waypoints, err := waypointsFromRequest(req, fsInputs)
	if err != nil {
		return nil, err
	}
	if len(waypoints) == 0 {
		return nil, errors.New("could not find any waypoints to plan for in MoveRequest. Fill in Destination or goal_state")
	}

	// re-evaluate goal poses to be in the frame of World
	// TODO (RSDK-8847) : this is a workaround to help account for us not yet being able to properly synchronize simultaneous motion across
	// multiple components. If we are moving component1, mounted on arm2, to a goal in frame of component2, which is mounted on arm2, then
	// passing that raw poseInFrame will certainly result in a plan which moves arm1 and arm2. We cannot guarantee that this plan is
	// collision-free until RSDK-8847 is complete. By transforming goals to world, only one arm should move for such a plan.
	worldWaypoints := []*motionplan.PlanState{}
	solvingFrame := referenceframe.World
	for _, wp := range waypoints {
		if wp.Poses() != nil {
			step := referenceframe.FrameSystemPoses{}
			for fName, destination := range wp.Poses() {
				tf, err := frameSys.Transform(fsInputs, destination, solvingFrame)
				if err != nil {
					return nil, err
				}
				goalPose, _ := tf.(*referenceframe.PoseInFrame)
				step[fName] = goalPose
			}
			worldWaypoints = append(worldWaypoints, motionplan.NewPlanState(step, wp.Configuration()))
		} else {
			worldWaypoints = append(worldWaypoints, wp)
		}
	}

	// the goal is to move the component to goalPose which is specified in coordinates of goalFrameName
	return motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:      ms.logger,
		Goals:       worldWaypoints,
		StartState:  startState,
		FrameSystem: frameSys,
		WorldState:  req.WorldState,
		Constraints: req.Constraints,
		Options:     req.Extra,
	})
}

func (ms *builtIn) execute(ctx context.Context, trajectory motionplan.Trajectory) error {
	// build maps of relevant components from initial inputs
	_, resources, err := ms.fsService.CurrentInputs(ctx)
	if err != nil {
		return err
	}

	// Batch GoToInputs calls if possible; components may want to blend between inputs
	combinedSteps := []map[string][][]referenceframe.Input{}
	currStep := map[string][][]referenceframe.Input{}
	for i, step := range trajectory {
		if i == 0 {
			for name, inputs := range step {
				if len(inputs) == 0 {
					continue
				}
				currStep[name] = append(currStep[name], inputs)
			}
			continue
		}
		changed := ""
		if len(currStep) > 0 {
			reset := false
			// Check if the current step moves only the same components as the previous step
			// If so, batch the inputs
			for name, inputs := range step {
				if len(inputs) == 0 {
					continue
				}
				if priorInputs, ok := currStep[name]; ok {
					for i, input := range inputs {
						if input != priorInputs[len(priorInputs)-1][i] {
							if changed == "" {
								changed = name
							}
							if changed != "" && changed != name {
								// If the current step moves different components than the previous step, reset the batch
								reset = true
								break
							}
						}
					}
				} else {
					// Previously moved components are no longer moving
					reset = true
				}
				if reset {
					break
				}
			}
			if reset {
				combinedSteps = append(combinedSteps, currStep)
				currStep = map[string][][]referenceframe.Input{}
			}
			for name, inputs := range step {
				if len(inputs) == 0 {
					continue
				}
				currStep[name] = append(currStep[name], inputs)
			}
		}
	}
	combinedSteps = append(combinedSteps, currStep)

	for _, step := range combinedSteps {
		for name, inputs := range step {
			if len(inputs) == 0 {
				continue
			}
			r, ok := resources[name]
			if !ok {
				return fmt.Errorf("plan had step for resource %s but no resource with that name found in framesystem", name)
			}
			if err := r.GoToInputs(ctx, inputs...); err != nil {
				// If there is an error on GoToInputs, stop the component if possible before returning the error
				if actuator, ok := r.(inputEnabledActuator); ok {
					if stopErr := actuator.Stop(ctx, nil); stopErr != nil {
						return errors.Wrap(err, stopErr.Error())
					}
				}
				return err
			}
		}
	}
	return nil
}

func waypointsFromRequest(
	req motion.MoveReq,
	fsInputs referenceframe.FrameSystemInputs,
) (*motionplan.PlanState, []*motionplan.PlanState, error) {
	var startState *motionplan.PlanState
	var waypoints []*motionplan.PlanState
	var err error

	if startStateIface, ok := req.Extra["start_state"]; ok {
		if startStateMap, ok := startStateIface.(map[string]interface{}); ok {
			startState, err = motionplan.DeserializePlanState(startStateMap)
			if err != nil {
				return nil, nil, err
			}
		} else {
			return nil, nil, errors.New("extras start_state could not be interpreted as map[string]interface{}")
		}
		if startState.Configuration() == nil {
			startState = motionplan.NewPlanState(startState.Poses(), fsInputs)
		}
	} else {
		startState = motionplan.NewPlanState(nil, fsInputs)
	}

	if waypointsIface, ok := req.Extra["waypoints"]; ok {
		if waypointsIfaceList, ok := waypointsIface.([]interface{}); ok {
			for _, wpIface := range waypointsIfaceList {
				if wpMap, ok := wpIface.(map[string]interface{}); ok {
					wp, err := motionplan.DeserializePlanState(wpMap)
					if err != nil {
						return nil, nil, err
					}
					waypoints = append(waypoints, wp)
				} else {
					return nil, nil, errors.New("element in extras waypoints could not be interpreted as map[string]interface{}")
				}
			}
		} else {
			return nil, nil, errors.New("Invalid 'waypoints' extra type. Expected an array")
		}
	}

	// If goal state is specified, it overrides the request goal
	if goalStateIface, ok := req.Extra["goal_state"]; ok {
		if goalStateMap, ok := goalStateIface.(map[string]interface{}); ok {
			goalState, err := motionplan.DeserializePlanState(goalStateMap)
			if err != nil {
				return nil, nil, err
			}
			waypoints = append(waypoints, goalState)
		} else {
			return nil, nil, errors.New("extras goal_state could not be interpreted as map[string]interface{}")
		}
	} else if req.Destination != nil {
		goalState := motionplan.NewPlanState(referenceframe.FrameSystemPoses{req.ComponentName.ShortName(): req.Destination}, nil)
		waypoints = append(waypoints, goalState)
	}
	return startState, waypoints, nil
}
