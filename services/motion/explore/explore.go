// Package builtin implements a motion service.
package explore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	servicepb "go.viam.com/api/service/motion/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

var errUnimplemented = errors.New("unimplemented")

func init() {
	resource.RegisterDefaultService(
		motion.API,
		resource.DefaultServiceModel,
		resource.Registration[motion.Service, *Config]{
			Constructor: New,
			WeakDependencies: []internal.ResourceMatcher{
				internal.ComponentDependencyWildcardMatcher,
			},
		})
}

const (
	exploreOpLabel = "explore-motion-service"
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
func New(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (motion.Service, error) {
	ms := &explore{
		Named:        conf.ResourceName().AsNamed(),
		logger:       logger,
		responseChan: make(chan checkResponse),
	}

	if err := ms.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return ms, nil
}

// Reconfigure updates the motion service when the config has changed.
func (ms *explore) Reconfigure(
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

	components := make(map[resource.Name]resource.Resource)
	for name, dep := range deps {
		switch dep := dep.(type) {
		case framesystem.Service:
			ms.fsService = dep
		default:
			components[name] = dep
		}
	}

	ms.backgroundWorkers = &sync.WaitGroup{}
	ms.components = components
	return nil
}

type explore struct {
	resource.Named
	resource.TriviallyCloseable
	fsService         framesystem.Service
	components        map[resource.Name]resource.Resource
	logger            golog.Logger
	lock              sync.Mutex
	obstacleChan      chan checkResponse
	responseChan      chan checkResponse
	kb                *kinematicbase.KinematicBase
	backgroundWorkers *sync.WaitGroup
	plan              motionplan.Plan //pointer?????
}

// Move takes a goal location and will plan and execute a movement to move a component specified by its name to that destination.

func (ms *explore) MoveOnMap(
	ctx context.Context,
	componentName resource.Name,
	destination spatialmath.Pose,
	slamName resource.Name,
	extra map[string]interface{},
) (bool, error) {
	return false, errUnimplemented
}

func (ms *explore) MoveOnGlobe(
	ctx context.Context,
	componentName resource.Name,
	destination *geo.Point,
	heading float64,
	movementSensorName resource.Name,
	obstacles []*spatialmath.GeoObstacle,
	motionCfg *motion.MotionConfiguration,
	extra map[string]interface{},
) (bool, error) {
	return false, errUnimplemented
}

func (ms *explore) MoveOnGlobeNew(
	ctx context.Context,
	req motion.MoveOnGlobeReq,
) (string, error) {
	return "", errUnimplemented
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

// PlanMoveOnMap returns the plan for MoveOnMap to execute.
func (ms *explore) planMoveOnMap(
	ctx context.Context,
	componentName resource.Name,
	destination spatialmath.Pose,
	kinematicsOptions kinematicbase.Options,
	extra map[string]interface{},
) ([][]referenceframe.Input, kinematicbase.KinematicBase, error) {

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

	fs, err := ms.fsService.FrameSystem(ctx, nil)
	if err != nil {
		return nil, nil, err
	}

	kb, err := kinematicbase.WrapWithKinematics(ctx, b, ms.logger, nil, nil, kinematicsOptions)
	if err != nil {
		return nil, nil, err
	}

	// replace original base frame with one that knows how to move itself and allow planning for
	if err = fs.ReplaceFrame(kb.Kinematics()); err != nil {
		return nil, nil, err
	}

	// get current position
	inputs, err := kb.CurrentInputs(ctx)
	if err != nil {
		return nil, nil, err
	}
	if kinematicsOptions.PositionOnlyMode && len(kb.Kinematics().DoF()) == 2 && len(inputs) == 3 {
		inputs = inputs[:2]
	}
	ms.logger.Debugf("base position: %v", inputs)

	dst := referenceframe.NewPoseInFrame(referenceframe.World, destination) // here

	f := kb.Kinematics()

	worldState, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{}, nil)
	if err != nil {
		return nil, nil, err
	}

	seedMap := map[string][]referenceframe.Input{f.Name(): inputs}

	ms.logger.Debugf("goal position: %v", dst.Pose().Point())
	plan, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:             ms.logger,
		Goal:               dst,
		Frame:              f,
		StartConfiguration: seedMap,
		FrameSystem:        fs,
		WorldState:         worldState,
		ConstraintSpecs:    nil,
		Options:            extra,
	})
	if err != nil {
		return nil, nil, err
	}
	steps, err := plan.GetFrameSteps(f.Name())
	return steps, kb, err
}

// Move takes a goal location and will plan and execute a movement to move a component specified by its name to that destination.
func (ms *explore) Move(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	constraints *servicepb.Constraints,
	extra map[string]interface{},
) (bool, error) {

	// Note: can set linear speed, angular speed and obstacle polling frequency from extras until MotionCfg can be passed through
	operation.CancelOtherWithLabel(ctx, exploreOpLabel)
	opt := kinematicbase.NewKinematicBaseOptions()
	opt.NoSkidSteer = true
	//opt.UsePTGs = false
	extra["motion_profile"] = motionplan.PositionOnlyMotionProfile

	// Create plan to spin towards destination point
	planInputs, kb, err := ms.planMoveOnMap(ctx, componentName, destination.Pose(), opt, extra)
	if err != nil {
		return false, fmt.Errorf("error making plan for MoveOnMap: %w", err)
	}
	ms.kb = &kb

	var plan motionplan.Plan
	for _, inputs := range planInputs {
		input := make(map[string][]referenceframe.Input)
		input[kb.Name().Name] = inputs
		plan = append(plan, input)
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start polling for obstacles
	ms.backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		ms.checkPartialPlan(cancelCtx, worldState) //, waypoints, ma.waypointIndex)
	}, ms.backgroundWorkers.Done)

	// Start executing plan
	ms.backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		ms.executePlan(cancelCtx) //, waypoints, ma.waypointIndex)
	}, ms.backgroundWorkers.Done)

	for {
		// this ensures that if the context is cancelled we always return early at the top of the loop
		if err := ctx.Err(); err != nil {
			return false, err
		}

		select {
		// if context was cancelled by the calling function, error out
		case <-ctx.Done():
			return false, ctx.Err()

		// once execution responds: return the result to the caller
		case resp := <-ms.responseChan:
			ms.logger.Debugf("execution completed: %s", resp)
			return resp.success, resp.err

		// if the checkplan process hit an error return it, otherwise replan
		case resp := <-ms.obstacleChan:
			ms.logger.Debugf("obstacle response: %s", resp)
			if resp.err != nil {
				return resp.success, resp.err
			}
			if resp.success {
				return resp.success, nil /// successful edge case
			}
		}
	}
}

type checkResponse struct {
	err     error
	success bool
}

func (ms *explore) checkPartialPlan(ctx context.Context, worldState *referenceframe.WorldState) {

	fs, err := ms.fsService.FrameSystem(ctx, nil)
	if err != nil {
		ms.obstacleChan <- checkResponse{err: err}
	}
	pInFrame, err := (*ms.kb).CurrentPosition(ctx)
	if err != nil {
		ms.obstacleChan <- checkResponse{err: err}
	}
	currentInputs, err := (*ms.kb).CurrentInputs(ctx)
	if err != nil {
		ms.obstacleChan <- checkResponse{err: err}
	}

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	// this check ensures that if the context is cancelled we always return early at the top of the loop
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collisionPose, collision, err := motionplan.CheckPlan((*ms.kb).Kinematics(), ms.plan, worldState, fs, pInFrame.Pose(), currentInputs, nil, ms.logger)
			if err != nil {
				ms.obstacleChan <- checkResponse{err: err}
			}
			// Check if problem is immediate
			if collision {
				if collisionPose.Distance(r3.Vector{X: 0, Y: 0, Z: 0}) < 100 {
					ms.obstacleChan <- checkResponse{success: false, err: nil}
				}
			}
		}
	}
}

func createPartialPlan(plan motionplan.Plan) (motionplan.Plan, error) {
	var partialPlan motionplan.Plan

	// limit plan scope to 10 cm

	return partialPlan, nil
}

func (ms *explore) executePlan(ctx context.Context) {
	// background process carry out plan
	for i := 1; i < len(ms.plan); i++ {
		if inputEnabledKb, ok := (*ms.kb).(inputEnabledActuator); ok {
			if err := inputEnabledKb.GoToInputs(ctx, ms.plan[i][(*ms.kb).Name().Name]); err != nil {
				// If there is an error on GoToInputs, stop the component if possible before returning the error
				if stopErr := (*ms.kb).Stop(ctx, nil); stopErr != nil {
					ms.responseChan <- checkResponse{err: err}
				}
				ms.responseChan <- checkResponse{err: err}
			}
		}
	}
	ms.responseChan <- checkResponse{success: true}
}
