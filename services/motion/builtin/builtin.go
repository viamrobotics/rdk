// Package builtin implements a motion service.
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	pb "go.viam.com/api/service/motion/v1"
	vutils "go.viam.com/utils"
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
	defaultCollisionBuffer             = 150. // mm
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
	NumThreads  int    `json:"num_threads"`

	PlanFilePath           string `json:"plan_file_path"`
	LogPlannerErrors       bool   `json:"log_planner_errors"`
	LogSlowPlanThresholdMS int    `json:"log_slow_plan_threshold_ms"`
}

func (c *Config) shouldWritePlan(start time.Time, err error) bool {
	if err != nil && c.LogPlannerErrors {
		return true
	}

	if c.LogSlowPlanThresholdMS != 0 &&
		time.Since(start) > (time.Duration(c.LogSlowPlanThresholdMS)*time.Millisecond) {
		return true
	}

	return false
}

// Validate here adds a dependency on the internal framesystem service.
func (c *Config) Validate(path string) ([]string, []string, error) {
	if c.NumThreads < 0 {
		return nil, nil, fmt.Errorf("cannot configure with %d number of threads, number must be positive", c.NumThreads)
	}

	if c.LogPlannerErrors && c.PlanFilePath == "" {
		return nil, nil, fmt.Errorf("need a plan_file_path if you sent log_planner_errors to %v", c.LogPlannerErrors)
	}

	if c.LogSlowPlanThresholdMS != 0 && c.PlanFilePath == "" {
		return nil, nil, fmt.Errorf("need a plan_file_path if you sent LogSlowPlanThresholdMS to %v", c.LogSlowPlanThresholdMS)
	}

	return []string{framesystem.InternalServiceName.String()}, nil, nil
}

type builtIn struct {
	resource.Named
	conf                    *Config
	mu                      sync.RWMutex
	fsService               framesystem.Service
	movementSensors         map[resource.Name]movementsensor.MovementSensor
	slamServices            map[resource.Name]slam.Service
	visionServices          map[resource.Name]vision.Service
	components              map[string]resource.Resource
	logger                  logging.Logger
	state                   *state.State
	configuredDefaultExtras map[string]any
}

// NewBuiltIn returns a new move and grab service for the given robot.
func NewBuiltIn(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (motion.Service, error) {
	ms := &builtIn{
		Named:                   conf.ResourceName().AsNamed(),
		logger:                  logger,
		configuredDefaultExtras: make(map[string]any),
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
	ms.conf = config

	if config.LogFilePath != "" {
		fileAppender, _ := logging.NewFileAppender(config.LogFilePath)
		ms.logger.AddAppender(fileAppender)
	}
	if config.NumThreads > 0 {
		ms.configuredDefaultExtras["num_threads"] = config.NumThreads
	}

	movementSensors := make(map[resource.Name]movementsensor.MovementSensor)
	slamServices := make(map[resource.Name]slam.Service)
	visionServices := make(map[resource.Name]vision.Service)
	componentMap := make(map[string]resource.Resource)
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
			componentMap[name.ShortName()] = dep
		}
	}
	ms.movementSensors = movementSensors
	ms.slamServices = slamServices
	ms.visionServices = visionServices
	ms.components = componentMap
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

	ms.applyDefaultExtras(req.Extra)
	plan, err := ms.plan(ctx, req, ms.logger)
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

	ms.applyDefaultExtras(req.Extra)
	id, err := state.StartExecution(ctx, ms.state, req.ComponentName, req, ms.newMoveOnMapRequest)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

type validatedExtra struct {
	maxReplans       int
	replanCostFactor float64
	motionProfile    motionplan.MotionProfile
	extra            map[string]interface{}
}

func newValidatedExtra(extra map[string]interface{}) (validatedExtra, error) {
	maxReplans := -1
	replanCostFactor := defaultReplanCostFactor
	motionProfile := motionplan.FreeMotionProfile
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
		motionProfile, ok = profile.(motionplan.MotionProfile)
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
	if _, ok := extra["collision_buffer_mm"]; !ok {
		extra["collision_buffer_mm"] = defaultCollisionBuffer
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

	ms.applyDefaultExtras(req.Extra)
	id, err := state.StartExecution(ctx, ms.state, req.ComponentName, req, ms.newMoveOnGlobeRequest)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

// GetPose is deprecated.
func (ms *builtIn) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*referenceframe.LinkInFrame,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	ms.logger.Warn("GetPose is deprecated. Please switch to using the GetPose method defined on the FrameSystem service")
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.fsService.GetPose(ctx, componentName.ShortName(), destinationFrame, supplementalTransforms, extra)
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
		// Special handling: we want to observe the logs just for the DoCommand
		obsLogger := ms.logger.Sublogger("observed-" + uuid.New().String())
		observerCore, observedLogs := observer.New(zap.LevelEnablerFunc(zapcore.InfoLevel.Enabled))
		obsLogger.AddAppender(observerCore)

		moveReq, err := motion.MoveReqFromProto(&moveReqProto)
		if err != nil {
			return nil, err
		}
		plan, err := ms.plan(ctx, moveReq, obsLogger)
		if err != nil {
			return nil, err
		}

		partialLogString := "returning partial plan up to waypoint"
		partialLogs := observedLogs.FilterMessageSnippet(partialLogString).All()
		if len(partialLogs) > 0 {
			// Extract the waypoint number from the partial log
			if len(partialLogs) == 1 {
				logMsg := partialLogs[0].Message
				// Find the waypoint number after the partial log string
				waypointStr := strings.TrimPrefix(logMsg, partialLogString)
				// Extract just the number
				waypointNum, err := strconv.Atoi(strings.Split(strings.TrimSpace(waypointStr), " ")[0])
				if err == nil {
					resp[DoPlan+"_partialwp"] = waypointNum
				} else {
					obsLogger.CWarnf(ctx, "error parsing log string: %s", logMsg)
					obsLogger.CWarn(ctx, err)
				}
			} else {
				obsLogger.CWarnf(ctx, "Unexpected number of partial logs: %d", len(partialLogs))
			}
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

func (ms *builtIn) plan(ctx context.Context, req motion.MoveReq, logger logging.Logger) (motionplan.Plan, error) {
	frameSys, err := framesystem.NewFromService(ctx, ms.fsService, req.WorldState.Transforms())
	if err != nil {
		return nil, err
	}

	// build maps of relevant components and inputs from initial inputs
	fsInputs, err := ms.fsService.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	logger.CDebugf(ctx, "frame system inputs: %v", fsInputs)

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
	// The contents of waypoints can be gigantic, and if so, making copies of `extra` becomes the majority of motion planning runtime.
	// As the meaning from `waypoints` has already been extracted above into its proper data structure, there is no longer a need to
	// keep it in `extra`.
	if req.Extra != nil {
		req.Extra["waypoints"] = nil
	}

	// TODO: RSDK-11198 REMOVE THIS IN THE RELEASE AFTER THE RELEASE THAT INTRODUCED THIS CODE. It is only here
	// to deprecate rather than break the "planning_alg" key and give us time to inform users/engineers
	// how to use the "planning_algorithm_settings" option
	if deprecatedKeyAlg, ok := req.Extra["planning_alg"]; ok {
		logger.Warn("the 'planning_alg' key is deprecated and will soon no longer be accepted, please use 'planning_algorithm_settings' instead")
		if deprecatedKeyAlgParsed, ok := deprecatedKeyAlg.(string); ok {
			req.Extra["planning_algorithm_settings"] = map[string]interface{}{
				"algorithm": deprecatedKeyAlgParsed,
			}
		}
	}

	// TODO: RSDK-11198 Similar to above
	if deprecatedKeySeed, ok := req.Extra["solutions_to_seed"]; ok {
		logger.Warn(
			"the top level 'solutions_to_seed' key is deprecated and will soon no longer be accepted," +
				"please include it in 'planning_algorithm_settings' instead",
		)
		if deprecatedSeedParsed, ok := deprecatedKeySeed.(int); ok {
			req.Extra["planning_algorithm_settings"] = map[string]interface{}{
				"algorithm": motionplan.CBiRRT,
				"cbirrt_settings": map[string]interface{}{
					"solutions_to_seed": deprecatedSeedParsed,
				},
			}
		}
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

	planOpts, err := motionplan.NewPlannerOptionsFromExtra(req.Extra)
	if err != nil {
		return nil, err
	}

	// the goal is to move the component to goalPose which is specified in coordinates of goalFrameName

	planRequest := &motionplan.PlanRequest{
		FrameSystem:    frameSys,
		Goals:          worldWaypoints,
		StartState:     startState,
		WorldState:     req.WorldState,
		Constraints:    req.Constraints,
		PlannerOptions: planOpts,
	}

	start := time.Now()
	plan, err := motionplan.PlanMotion(ctx, logger, planRequest)
	if ms.conf.shouldWritePlan(start, err) {
		err := ms.writePlanRequest(planRequest)
		if err != nil {
			ms.logger.Warnf("couldn't write plan: %v", err)
		}
	}
	return plan, err
}

func (ms *builtIn) execute(ctx context.Context, trajectory motionplan.Trajectory) error {
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
			r, ok := ms.components[name]
			if !ok {
				return fmt.Errorf("plan had step for resource %s but it was not found in the motion", name)
			}
			ie, err := utils.AssertType[framesystem.InputEnabled](r)
			if err != nil {
				return err
			}
			if err := ie.GoToInputs(ctx, inputs...); err != nil {
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

// applyDefaultExtras iterates through the list of default extras configured on the builtIn motion service and adds them to the
// given map of extras if the key does not already exist.
func (ms *builtIn) applyDefaultExtras(extras map[string]any) {
	if extras == nil {
		extras = make(map[string]any)
	}
	for key, val := range ms.configuredDefaultExtras {
		if _, ok := extras[key]; !ok {
			extras[key] = val
		}
	}
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

func (ms *builtIn) writePlanRequest(req *motionplan.PlanRequest) error {
	fn := filepath.Join(ms.conf.PlanFilePath, fmt.Sprintf("plan-%s.json", time.Now().Format(time.RFC3339)))
	ms.logger.Infof("writing plan to %s", fn)

	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Clean(fn), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer vutils.UncheckedErrorFunc(file.Close)
	_, err = file.Write(data)
	if err != nil {
		return err
	}
	return nil
}
