// Package explore implements a motion service for exploration.
package explore

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	servicepb "go.viam.com/api/service/motion/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

var (
	model                   = resource.DefaultModelFamily.WithModel("explore")
	errUnimplemented        = errors.New("unimplemented")
	moveLimit               = 10000.
	validObstacleDistanceMM = 1000.
)

func init() {
	resource.RegisterDefaultService(
		motion.API, model,
		resource.Registration[motion.Service, *Config]{
			Constructor: NewExplore,
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

// NewExplore returns a new move and grab service for the given robot.
func NewExplore(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (motion.Service, error) {
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
	ms.obstacleChan = make(chan checkResponse)
	ms.responseChan = make(chan checkResponse)
	return nil
}

type explore struct {
	resource.Named
	resource.TriviallyCloseable
	fsService         framesystem.Service
	frameSystem       referenceframe.FrameSystem
	components        map[resource.Name]resource.Resource
	visionService     vision.Service
	camera            camera.Camera
	logger            golog.Logger
	lock              sync.Mutex
	obstacleChan      chan checkResponse
	responseChan      chan checkResponse
	kb                *kinematicbase.KinematicBase
	backgroundWorkers *sync.WaitGroup
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

// Move takes a goal location and will plan and execute a movement to move a component specified by its name to that destination.
func (ms *explore) Move(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	constraints *servicepb.Constraints,
	extra map[string]interface{},
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, exploreOpLabel)

	// Create kinematic base
	kb, err := ms.createKinematicBase(ctx, componentName, extra)
	if err != nil {
		return false, err
	}
	ms.kb = &kb

	// Create motionplan plan
	planInputs, err := ms.createMotionPlan(ctx, destination.Pose(), worldState, true, extra)
	if err != nil {
		return false, err
	}
	var plan motionplan.Plan
	for _, inputs := range planInputs {
		input := make(map[string][]referenceframe.Input)
		input[kb.Name().Name] = inputs
		plan = append(plan, input)
	}

	// Start background processes
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start polling for obstacles
	ms.backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		ms.checkForObstacles(cancelCtx, plan)
	}, ms.backgroundWorkers.Done)

	// Start executing plan
	ms.backgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		ms.executePlan(cancelCtx, plan)
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

		// if the checkPartialPlan process hit an error return it, otherwise exit
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

func (ms *explore) checkForObstacles(ctx context.Context, plan motionplan.Plan) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentPose := spatialmath.NewZeroPose()

			worldState, err := ms.updateWorldState(ctx)
			if err != nil {
				ms.obstacleChan <- checkResponse{err: err}
				return
			}

			collisionPose, err := motionplan.CheckPlan(
				(*ms.kb).Kinematics(),
				plan,
				worldState,
				ms.frameSystem,
				currentPose,
				[]referenceframe.Input{{Value: 0}, {Value: 0}},
				spatialmath.NewZeroPose(),
				ms.logger,
			)
			if err != nil {
				if collisionPose.Point().Distance(currentPose.Point()) < validObstacleDistanceMM {
					ms.logger.Debug("collision found")
					ms.obstacleChan <- checkResponse{success: true, err: err}
					return
				}
				ms.logger.Debug("collision found but outside of range")
				ms.obstacleChan <- checkResponse{success: false, err: err}
			} else {
				ms.obstacleChan <- checkResponse{success: false, err: err}
			}
		}
	}
}

func (ms *explore) executePlan(ctx context.Context, plan motionplan.Plan) {
	// background process carry out plan
	for i := 1; i < len(plan); i++ {
		if inputEnabledKb, ok := (*ms.kb).(inputEnabledActuator); ok {
			if err := inputEnabledKb.GoToInputs(ctx, plan[i][(*ms.kb).Name().Name]); err != nil {
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

func (ms *explore) updateWorldState(ctx context.Context) (*referenceframe.WorldState, error) {
	detections, err := ms.visionService.GetObjectPointClouds(ctx, ms.camera.Name().Name, nil)
	if err != nil && strings.Contains(err.Error(), "does not implement a 3D segmenter") {
		ms.logger.Infof("cannot call GetObjectPointClouds on %q as it does not implement a 3D segmenter", ms.visionService.Name())
	} else if err != nil {
		return nil, err
	}

	geoms := []spatialmath.Geometry{}
	for i, detection := range detections {
		geometry := detection.Geometry
		label := ms.camera.Name().Name + "_transientObstacle_" + strconv.Itoa(i)
		if geometry.Label() != "" {
			label += "_" + geometry.Label()
		}
		geometry.SetLabel(label)
		geoms = append(geoms, geometry)
	}
	// consider having geoms be from the frame of world
	// to accomplish this we need to know the transform from the base to the camera
	gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame((referenceframe.World), geoms)}
	worldState, err := referenceframe.NewWorldState(gifs, nil)
	if err != nil {
		return nil, err
	}
	return worldState, nil
}

func createKBOps(extra map[string]interface{}) (kinematicbase.Options, error) {
	opt := kinematicbase.NewKinematicBaseOptions()
	opt.NoSkidSteer = true
	opt.UsePTGs = false

	extra["motion_profile"] = motionplan.PositionOnlyMotionProfile

	if degsPerSec, ok := extra["angular_degs_per_sec"]; ok {
		angularDegsPerSec, ok := degsPerSec.(float64)
		if !ok {
			return kinematicbase.Options{}, errors.New("could not interpret motion_profile field as string")
		}
		opt.AngularVelocityDegsPerSec = angularDegsPerSec
	}

	if mPerSec, ok := extra["linear_m_per_sec"]; ok {
		linearMPerSec, ok := mPerSec.(float64)
		if !ok {
			return kinematicbase.Options{}, errors.New("could not interpret motion_profile field as string")
		}
		opt.LinearVelocityMMPerSec = linearMPerSec
	}

	if profile, ok := extra["motion_profile"]; ok {
		motionProfile, ok := profile.(string)
		if !ok {
			return kinematicbase.Options{}, errors.New("could not interpret motion_profile field as string")
		}
		opt.PositionOnlyMode = motionProfile == motionplan.PositionOnlyMotionProfile
	}

	return opt, nil
}

// PlanMoveOnMap returns the plan for MoveOnMap to execute.
func (ms *explore) createKinematicBase(
	ctx context.Context,
	componentName resource.Name,
	extra map[string]interface{},
) (kinematicbase.KinematicBase, error) {
	// create a KinematicBase from the componentName
	component, ok := ms.components[componentName]
	if !ok {
		return nil, resource.DependencyNotFoundError(componentName)
	}

	b, ok := component.(base.Base)
	if !ok {
		return nil, fmt.Errorf("cannot move component of type %T because it is not a Base", component)
	}

	kinematicsOptions, err := createKBOps(extra)
	if err != nil {
		return nil, err
	}

	kb, err := kinematicbase.WrapWithKinematics(
		ctx,
		b,
		ms.logger,
		nil,
		[]referenceframe.Limit{{Min: -moveLimit, Max: moveLimit}, {Min: -moveLimit, Max: moveLimit}},
		kinematicsOptions,
	)
	if err != nil {
		return nil, err
	}

	return kb, nil
}

func (ms *explore) createMotionPlan(
	ctx context.Context,
	destination spatialmath.Pose,
	worldState *referenceframe.WorldState,
	positionOnlyMode bool,
	extra map[string]interface{},
) ([][]referenceframe.Input, error) {
	fs, err := ms.fsService.FrameSystem(ctx, worldState.Transforms())
	if err != nil {
		return nil, err
	}

	// replace original base frame with one that knows how to move itself and allow planning for
	if err := fs.ReplaceFrame((*ms.kb).Kinematics()); err != nil {
		return nil, err
	}

	ms.frameSystem = fs

	inputs := []referenceframe.Input{{Value: 0}, {Value: 0}, {Value: 0}}

	if positionOnlyMode && len((*ms.kb).Kinematics().DoF()) == 2 && len(inputs) == 3 {
		inputs = inputs[:2]
	}

	dst := referenceframe.NewPoseInFrame(referenceframe.World, destination)

	f := (*ms.kb).Kinematics()

	worldStateNew, err := referenceframe.NewWorldState(nil, nil)
	if err != nil {
		return nil, err
	}

	seedMap := map[string][]referenceframe.Input{f.Name(): inputs}

	ms.logger.Debugf("goal position: %v", dst.Pose().Point())
	plan, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:             ms.logger,
		Goal:               dst,
		Frame:              f,
		StartConfiguration: seedMap,
		FrameSystem:        fs,
		WorldState:         worldStateNew,
		ConstraintSpecs:    nil,
		Options:            extra,
	})
	if err != nil {
		return nil, err
	}
	steps, err := plan.GetFrameSteps(f.Name())
	return steps, err
}
