package builtin

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/state"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
)

const (
	defaultReplanCostFactor = 1.0
	defaultMaxReplans       = -1 // Values below zero will replan infinitely
	baseStopTimeout         = time.Second * 5
)

// validatedMotionConfiguration is a copy of the motion.MotionConfiguration type
// which has been validated to conform to the expectations of the builtin
// motion service.
type validatedMotionConfiguration struct {
	obstacleDetectors     []motion.ObstacleDetectorName
	positionPollingFreqHz float64
	obstaclePollingFreqHz float64
	planDeviationMM       float64
	linearMPerSec         float64
	angularDegsPerSec     float64
}

type requestType uint8

const (
	requestTypeUnspecified requestType = iota
	requestTypeMoveOnGlobe
	requestTypeMoveOnMap
)

// moveRequest is a structure that contains all the information necessary for to make a move call.
type moveRequest struct {
	requestType requestType
	// geoPoseOrigin is only set if requestType == requestTypeMoveOnGlobe
	geoPoseOrigin     *spatialmath.GeoPose
	logger            logging.Logger
	config            *validatedMotionConfiguration
	planRequest       *motionplan.PlanRequest
	seedPlan          motionplan.Plan
	kinematicBase     kinematicbase.KinematicBase
	obstacleDetectors map[vision.Service][]resource.Name
	replanCostFactor  float64
	// TODO(RSDK-8683): remove atGoalCheck and put it in the motionplan package
	// atGoalCheck func(basePose spatialmath.Pose) *state.ExecuteResponse
	atGoalCheck func(basePose spatialmath.Pose) bool
	fsService   framesystem.Service
	// localizingFS is used for placing observed transient obstacles into their absolute world position when
	// they are observed. It is also used by CheckPlan to perform collision checking.
	// The localizingFS combines a kinematic bases localization and kinematics(aka execution) frames into a
	// singulare frame moniker wrapperFrame. The wrapperFrame allows us to position the geometries of the base
	// using only provided referenceframe.Input values and we do not have to compose with a separate pose
	/// to absolutely position ourselves in the world frame.
	localizingFS referenceframe.FrameSystem

	executeBackgroundWorkers *sync.WaitGroup
	responseChan             chan moveResponse
	// replanners for the move request
	// if we ever have to add additional instances we should figure out how to make this more scalable
	position, obstacle *replanner
}

// plan creates a plan using the currentInputs of the robot and the moveRequest's planRequest.
func (mr *moveRequest) Plan(ctx context.Context) (motionplan.Plan, error) {
	inputs, err := mr.kinematicBase.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	// TODO: this is really hacky and we should figure out a better place to store this information
	if len(mr.kinematicBase.Kinematics().DoF()) == 2 {
		inputs = inputs[:2]
	}
	startConf := referenceframe.FrameSystemInputs{mr.kinematicBase.Kinematics().Name(): inputs}
	mr.planRequest.StartState = motionplan.NewPlanState(mr.planRequest.StartState.Poses(), startConf)

	// get existing elements of the worldstate
	existingGifs, err := mr.planRequest.WorldState.ObstaclesInWorldFrame(mr.planRequest.FrameSystem, startConf)
	if err != nil {
		return nil, err
	}

	// get transient detections
	gifs := []*referenceframe.GeometriesInFrame{}
	for visSrvc, cameraNames := range mr.obstacleDetectors {
		for _, camName := range cameraNames {
			transientGifs, err := mr.getTransientDetections(ctx, visSrvc, camName)
			if err != nil {
				return nil, err
			}
			gifs = append(gifs, transientGifs)
		}
	}
	gifs = append(gifs, existingGifs)

	// update worldstate to include transient detections
	planRequestCopy := *mr.planRequest
	planRequestCopy.WorldState, err = referenceframe.NewWorldState(gifs, nil)
	if err != nil {
		return nil, err
	}

	// TODO(RSDK-5634): this should pass in mr.seedplan and the appropriate replanCostFactor once this bug is found and fixed.
	return motionplan.Replan(ctx, &planRequestCopy, nil, 0)
}

func (mr *moveRequest) Execute(ctx context.Context, plan motionplan.Plan) (state.ExecuteResponse, error) {
	defer mr.executeBackgroundWorkers.Wait()
	cancelCtx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	mr.start(cancelCtx, plan)
	return mr.listen(cancelCtx)
}

func (mr *moveRequest) AnchorGeoPose() *spatialmath.GeoPose {
	return mr.geoPoseOrigin
}

// execute attempts to follow a given Plan starting from the index percribed by waypointIndex.
// Note that waypointIndex is an atomic int that is incremented in this function after each waypoint has been successfully reached.
func (mr *moveRequest) execute(ctx context.Context, plan motionplan.Plan) (state.ExecuteResponse, error) {
	// Determine if we already are at the goal
	// If our motion profile is position_only then, we only check against our current & desired position
	// Conversely if our motion profile is anything else, then we also need to check again our
	// current & desired orientation
	if resp := mr.atGoalCheck(mr.planRequest.StartState.Poses()[mr.kinematicBase.Name().ShortName()].Pose()); resp {
		mr.logger.Info("no need to move, already within planDeviationMM of the goal")
		return state.ExecuteResponse{Replan: false}, nil
	}

	waypoints, err := plan.Trajectory().GetFrameInputs(mr.kinematicBase.Name().ShortName())
	if err != nil {
		return state.ExecuteResponse{}, err
	}

	if err := mr.kinematicBase.GoToInputs(ctx, waypoints...); err != nil {
		// If there is an error on GoToInputs, stop the component if possible before returning the error
		mr.logger.CDebugf(ctx, "calling kinematicBase.Stop due to %s\n", err)
		if stopErr := mr.stop(); stopErr != nil {
			return state.ExecuteResponse{}, errors.Wrap(err, stopErr.Error())
		}
		return state.ExecuteResponse{}, err
	}

	// the plan has been fully executed, so check to see if where we are at is close enough to the goal
	executionState, err := mr.kinematicBase.ExecutionState(ctx)
	if err != nil {
		return state.ExecuteResponse{}, err
	}
	currentPosition, ok := executionState.CurrentPoses()[mr.kinematicBase.LocalizationFrame().Name()]
	if !ok {
		return state.ExecuteResponse{}, errors.New("exeuctionState.CurrentPoses() does not contain an entry for the LocalizationFrame")
	}
	if resp := mr.atGoalCheck(currentPosition.Pose()); !resp {
		return state.ExecuteResponse{Replan: true, ReplanReason: "issuing a replan since we are not within planDeviationMM of the goal"}, nil
	}
	return state.ExecuteResponse{Replan: false}, nil
}

// deviatedFromPlan takes a plan and an index of a waypoint on that Plan and returns whether or not it is still
// following the plan as described by the PlanDeviation specified for the moveRequest.
func (mr *moveRequest) deviatedFromPlan(ctx context.Context, plan motionplan.Plan) (state.ExecuteResponse, error) {
	// calculate the error state
	executionState, err := mr.kinematicBase.ExecutionState(ctx)
	if err != nil {
		return state.ExecuteResponse{}, err
	}
	errorState, err := motionplan.CalculateFrameErrorState(executionState, mr.kinematicBase.Kinematics(), mr.kinematicBase.LocalizationFrame())
	if err != nil {
		return state.ExecuteResponse{}, err
	}

	// check if the error state is outside the acceptable bounds
	if errorState.Point().Norm() > mr.config.planDeviationMM {
		msg := "error state exceeds planDeviationMM; planDeviationMM: %f, errorstate.Point().Norm(): %f, errorstate.Point(): %#v "
		reason := fmt.Sprintf(msg, mr.config.planDeviationMM, errorState.Point().Norm(), errorState.Point())
		return state.ExecuteResponse{Replan: true, ReplanReason: reason}, nil
	}
	return state.ExecuteResponse{}, nil
}

// getTransientDetections returns a list of geometries as observed by the provided vision service and camera.
// Depending on the caller, the geometries returned are either in their relative position
// with respect to the base or in their absolute position with respect to the world.
func (mr *moveRequest) getTransientDetections(
	ctx context.Context,
	visSrvc vision.Service,
	camName resource.Name,
) (*referenceframe.GeometriesInFrame, error) {
	mr.logger.CDebugf(ctx,
		"proceeding to get detections from vision service: %s with camera: %s",
		visSrvc.Name().ShortName(),
		camName.ShortName(),
	)

	baseExecutionState, err := mr.kinematicBase.ExecutionState(ctx)
	if err != nil {
		return nil, err
	}
	// the inputMap informs where we are in the world
	// the inputMap will be used downstream to transform the observed geometry from the camera frame
	// into the world frame
	inputMap, _, err := mr.fsService.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	kbInputs := make([]referenceframe.Input, len(mr.kinematicBase.Kinematics().DoF()))
	kbInputs = append(kbInputs, referenceframe.PoseToInputs(
		baseExecutionState.CurrentPoses()[mr.kinematicBase.LocalizationFrame().Name()].Pose(),
	)...)
	inputMap[mr.kinematicBase.Name().ShortName()] = kbInputs

	detections, err := visSrvc.GetObjectPointClouds(ctx, camName.Name, nil)
	if err != nil {
		return nil, err
	}

	// transformed detections
	transientGeoms := []spatialmath.Geometry{}
	for i, detection := range detections {
		geometry := detection.Geometry
		// update the label of the geometry so we know it is transient
		label := camName.ShortName() + "_transientObstacle_" + strconv.Itoa(i)
		if geometry.Label() != "" {
			label += "_" + geometry.Label()
		}
		geometry.SetLabel(label)

		// the geometry is originally in the frame of the camera that observed it
		// here we use a framesystem which has the wrapper frame to position the geometry
		// in the world frame
		tf, err := mr.localizingFS.Transform(
			inputMap,
			referenceframe.NewGeometriesInFrame(camName.ShortName(), []spatialmath.Geometry{geometry}),
			referenceframe.World,
		)
		if err != nil {
			return nil, err
		}
		worldGifs, ok := tf.(*referenceframe.GeometriesInFrame)
		if !ok {
			return nil, errors.New("unable to assert referenceframe.Transformable into *referenceframe.GeometriesInFrame")
		}
		transientGeoms = append(transientGeoms, worldGifs.Geometries()...)
	}
	return referenceframe.NewGeometriesInFrame(referenceframe.World, transientGeoms), nil
}

// obstaclesIntersectPlan takes a list of waypoints and an index of a waypoint on that Plan and reports an error indicating
// whether or not any obstacle detectors report geometries in positions which would cause a collision with the executor
// following the Plan.
func (mr *moveRequest) obstaclesIntersectPlan(
	ctx context.Context,
	plan motionplan.Plan,
) (state.ExecuteResponse, error) {
	// if the camera is mounted on something InputEnabled that isn't the base, then that
	// input needs to be known in order to properly calculate the pose of the obstacle
	// furthermore, if that InputEnabled thing has moved since this moveRequest was initialized
	// (due to some other non-motion call for example), then we can't just get current inputs
	// we need the original input to place that thing in its original position
	// hence, cached CurrentInputs from the start are used i.e. mr.planRequest.StartConfiguration
	existingGifs, err := mr.planRequest.WorldState.ObstaclesInWorldFrame(
		mr.planRequest.FrameSystem, mr.planRequest.StartState.Configuration(),
	)
	if err != nil {
		return state.ExecuteResponse{}, err
	}

	for visSrvc, cameraNames := range mr.obstacleDetectors {
		for _, camName := range cameraNames {
			// Note: detections are initially observed from the camera frame but must be transformed to be in
			// world frame. We cannot use the inputs of the base to transform the detections since they are relative.
			gifs, err := mr.getTransientDetections(ctx, visSrvc, camName)
			if err != nil {
				return state.ExecuteResponse{}, err
			}
			if len(gifs.Geometries()) == 0 {
				mr.logger.CDebug(ctx, "no obstacles detected")
				continue
			}

			// construct new worldstate
			worldState, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{existingGifs, gifs}, nil)
			if err != nil {
				return state.ExecuteResponse{}, err
			}

			// get the execution state of the base
			baseExecutionState, err := mr.kinematicBase.ExecutionState(ctx)
			if err != nil {
				return state.ExecuteResponse{}, err
			}

			// build representation of frame system's inputs
			// TODO(pl): in the case where we have e.g. an arm (not moving) mounted on a base, we should be passing its current
			// configuration rather than the zero inputs
			updatedBaseExecutionState := baseExecutionState
			_, ok := mr.kinematicBase.Kinematics().(tpspace.PTGProvider)
			if ok {
				updatedBaseExecutionState, err = mr.augmentBaseExecutionState(baseExecutionState)
				if err != nil {
					return state.ExecuteResponse{}, err
				}
			}

			mr.logger.CDebugf(ctx, "CheckPlan inputs: \n currentPosition: %v\n currentInputs: %v\n worldstate: %s",
				spatialmath.PoseToProtobuf(updatedBaseExecutionState.CurrentPoses()[mr.kinematicBase.Kinematics().Name()].Pose()),
				updatedBaseExecutionState.CurrentInputs(),
				worldState.String(),
			)
			if err := motionplan.CheckPlan(
				mr.localizingFS.Frame(mr.kinematicBase.Kinematics().Name()), // frame we wish to check for collisions
				updatedBaseExecutionState,
				worldState, // detected obstacles by this instance of camera + service
				mr.localizingFS,
				lookAheadDistanceMM,
				mr.planRequest.Logger,
			); err != nil {
				mr.planRequest.Logger.CInfo(ctx, err.Error())
				return state.ExecuteResponse{Replan: true, ReplanReason: err.Error()}, nil
			}
		}
	}
	return state.ExecuteResponse{}, nil
}

// In order for the localizingFS to work as intended when working with PTGs we must update the baseExecutionState.
// The original baseExecutionState passed into this method contains a plan, currentPoses, and currentInputs.
// We update the plan object such that the trajectory which is of form referenceframe.Input also houses information
// about where we are in the world. This changes the plans trajectory from being of length 4 into being of length
// 11. This is because we add 7 values corresponding the X, Y, Z, OX, OY, OZ, Theta(radians).
// We update the currentInputs information so that when the wrapperFrame transforms off its inputs, the resulting
// pose is its position in world and we do not need to compose the resultant value with another pose.
// We update the currentPoses so that it is now in the name of the wrapperFrame which shares its name with
// mr.kinematicBase.Kinematics().
func (mr *moveRequest) augmentBaseExecutionState(
	baseExecutionState motionplan.ExecutionState,
) (motionplan.ExecutionState, error) {
	// update plan
	existingPlan := baseExecutionState.Plan()
	newTrajectory := make(motionplan.Trajectory, 0, len(existingPlan.Trajectory()))
	for idx, currTraj := range existingPlan.Trajectory() {
		// Suppose we have some plan of the following form:
		// Path = [s, p1, p2, g, g]
		// Traj = [  [0,0,0,0], [i1, a1, ds1, de1],
		// 						[i2, a2, ds2, de2],
		//						[i3, a3, ds3, de3], [0,0,0,0] ]
		// To properly interpolate across segments we will create in CheckPlan
		// we want the inputs of the pose frame to be the start position of that arc,
		// so we must look at the prior path step for the starting point.
		// To put this into an example:
		// [0,0,0,0] is tied to s, here we start at s and end at s
		// [i1, a1, ds1, de1] is tied to s, here we start at s and end at p1
		// [i2, a2, ds2, de2] is tied to p1, here we start at p1 and end at p2
		// [i3, a3, ds3, de3] is tied to p2, here we start at p2 and end at g
		// [0,0,0,0] is tied to g, here we start at g and end at g.

		// To accomplish this, we use PoseBetweenInverse on the transform of the
		// trajectory and the path pose for our current index to calculate our path
		// pose in the previous step.

		// The exception to this is if we are at the index we are currently executing, then
		// we will use the base's reported current position.

		currFrameSystemPoses := existingPlan.Path()[idx]
		kbTraj := currTraj[mr.kinematicBase.Name().Name]

		// determine which pose should be used as the origin of a ptg input
		var prevPathPose spatialmath.Pose
		if idx == baseExecutionState.Index() {
			prevPathPose = baseExecutionState.CurrentPoses()[mr.kinematicBase.LocalizationFrame().Name()].Pose()
		} else {
			kbPose := currFrameSystemPoses[mr.kinematicBase.Kinematics().Name()]
			trajPose, err := mr.kinematicBase.Kinematics().Transform(kbTraj)
			if err != nil {
				return baseExecutionState, err
			}
			prevPathPose = spatialmath.PoseBetweenInverse(trajPose, kbPose.Pose())
		}

		updatedTraj := kbTraj
		updatedTraj = append(updatedTraj, referenceframe.PoseToInputs(prevPathPose)...)
		newTrajectory = append(
			newTrajectory, referenceframe.FrameSystemInputs{mr.kinematicBase.Kinematics().Name(): updatedTraj},
		)
	}
	augmentedPlan := motionplan.NewSimplePlan(existingPlan.Path(), newTrajectory)

	// update currentInputs
	allCurrentInputsFromBaseExecutionState := baseExecutionState.CurrentInputs()
	kinematicBaseCurrentInputs := allCurrentInputsFromBaseExecutionState[mr.kinematicBase.Kinematics().Name()]
	// The order of inputs here matters as we construct the inputs for our poseFrame.
	// The poseFrame has DoF = 11.
	// The first four inputs correspond to the executionFrame's (ptgFrame) inputs which are:
	// alpha, index, start distance, end distance
	// The last seven inputs correspond to the localizationFrame's inputs which are:
	// X, Y, Z, OX, OY, OZ, Theta (in radians)
	kinematicBaseCurrentInputs = append(
		kinematicBaseCurrentInputs,
		referenceframe.PoseToInputs(baseExecutionState.CurrentPoses()[mr.kinematicBase.LocalizationFrame().Name()].Pose())...,
	)
	allCurrentInputsFromBaseExecutionState[mr.kinematicBase.Kinematics().Name()] = kinematicBaseCurrentInputs

	// originally currenPoses are in the name of the localization frame
	// here to transfer them to be in the name of the kinematics frame
	// the kinematic frame's name is the same as the wrapper frame
	existingCurrentPoses := baseExecutionState.CurrentPoses()
	localizationFramePose := existingCurrentPoses[mr.kinematicBase.LocalizationFrame().Name()]
	existingCurrentPoses[mr.kinematicBase.Name().Name] = localizationFramePose
	delete(existingCurrentPoses, mr.kinematicBase.LocalizationFrame().Name())

	return motionplan.NewExecutionState(
		augmentedPlan, baseExecutionState.Index(), allCurrentInputsFromBaseExecutionState, existingCurrentPoses,
	)
}

func kbOptionsFromCfg(motionCfg *validatedMotionConfiguration, validatedExtra validatedExtra) kinematicbase.Options {
	kinematicsOptions := kinematicbase.NewKinematicBaseOptions()

	if motionCfg.linearMPerSec > 0 {
		kinematicsOptions.LinearVelocityMMPerSec = motionCfg.linearMPerSec * 1000
	}

	if motionCfg.angularDegsPerSec > 0 {
		kinematicsOptions.AngularVelocityDegsPerSec = motionCfg.angularDegsPerSec
	}

	if motionCfg.planDeviationMM > 0 {
		kinematicsOptions.PlanDeviationThresholdMM = motionCfg.planDeviationMM
	}

	if validatedExtra.motionProfile != "" {
		kinematicsOptions.PositionOnlyMode = validatedExtra.motionProfile == motionplan.PositionOnlyMotionProfile
	}

	kinematicsOptions.GoalRadiusMM = motionCfg.planDeviationMM
	kinematicsOptions.HeadingThresholdDegrees = 8
	return kinematicsOptions
}

func validateNotNan(f float64, name string) error {
	if math.IsNaN(f) {
		return errors.Errorf("%s may not be NaN", name)
	}
	return nil
}

func validateNotNeg(f float64, name string) error {
	if f < 0 {
		return errors.Errorf("%s may not be negative", name)
	}
	return nil
}

func validateNotNegNorNaN(f float64, name string) error {
	if err := validateNotNan(f, name); err != nil {
		return err
	}
	return validateNotNeg(f, name)
}

func newValidatedMotionCfg(motionCfg *motion.MotionConfiguration, reqType requestType) (*validatedMotionConfiguration, error) {
	empty := &validatedMotionConfiguration{}
	vmc := &validatedMotionConfiguration{
		angularDegsPerSec:     defaultAngularDegsPerSec,
		linearMPerSec:         defaultLinearMPerSec,
		obstaclePollingFreqHz: defaultObstaclePollingHz,
		positionPollingFreqHz: defaultPositionPollingHz,
		obstacleDetectors:     []motion.ObstacleDetectorName{},
	}

	switch reqType {
	case requestTypeMoveOnGlobe:
		vmc.planDeviationMM = defaultGlobePlanDeviationM * 1e3
	case requestTypeMoveOnMap:
		vmc.planDeviationMM = defaultSlamPlanDeviationM * 1e3
	case requestTypeUnspecified:
		fallthrough
	default:
		return empty, fmt.Errorf("invalid moveRequest.requestType: %d", reqType)
	}

	if motionCfg == nil {
		return vmc, nil
	}

	if err := validateNotNegNorNaN(motionCfg.LinearMPerSec, "LinearMPerSec"); err != nil {
		return empty, err
	}

	if err := validateNotNegNorNaN(motionCfg.AngularDegsPerSec, "AngularDegsPerSec"); err != nil {
		return empty, err
	}

	if err := validateNotNegNorNaN(motionCfg.PlanDeviationMM, "PlanDeviationMM"); err != nil {
		return empty, err
	}

	if motionCfg.LinearMPerSec != 0 {
		vmc.linearMPerSec = motionCfg.LinearMPerSec
	}

	if motionCfg.AngularDegsPerSec != 0 {
		vmc.angularDegsPerSec = motionCfg.AngularDegsPerSec
	}

	if motionCfg.PlanDeviationMM != 0 {
		vmc.planDeviationMM = motionCfg.PlanDeviationMM
	}

	if motionCfg.ObstaclePollingFreqHz != nil {
		if err := validateNotNegNorNaN(*motionCfg.ObstaclePollingFreqHz, "ObstaclePollingFreqHz"); err != nil {
			return empty, err
		}
		vmc.obstaclePollingFreqHz = *motionCfg.ObstaclePollingFreqHz
	}

	if motionCfg.PositionPollingFreqHz != nil {
		if err := validateNotNegNorNaN(*motionCfg.PositionPollingFreqHz, "PositionPollingFreqHz"); err != nil {
			return empty, err
		}
		vmc.positionPollingFreqHz = *motionCfg.PositionPollingFreqHz
	}

	if motionCfg.ObstacleDetectors != nil {
		vmc.obstacleDetectors = motionCfg.ObstacleDetectors
	}

	return vmc, nil
}

func (ms *builtIn) newMoveOnGlobeRequest(
	ctx context.Context,
	req motion.MoveOnGlobeReq,
	seedPlan motionplan.Plan,
	replanCount int,
) (state.PlannerExecutor, error) {
	valExtra, err := newValidatedExtra(req.Extra)
	if err != nil {
		return nil, err
	}

	if valExtra.maxReplans >= 0 {
		if replanCount > valExtra.maxReplans {
			return nil, fmt.Errorf("exceeded maximum number of replans: %d", valExtra.maxReplans)
		}
	}

	motionCfg, err := newValidatedMotionCfg(req.MotionCfg, requestTypeMoveOnGlobe)
	if err != nil {
		return nil, err
	}
	// ensure arguments are well behaved
	obstacles := req.Obstacles
	if obstacles == nil {
		obstacles = []*spatialmath.GeoGeometry{}
	}
	if req.Destination == nil {
		return nil, errors.New("destination cannot be nil")
	}

	if math.IsNaN(req.Destination.Lat()) || math.IsNaN(req.Destination.Lng()) {
		return nil, errors.New("destination may not contain NaN")
	}

	// build kinematic options
	kinematicsOptions := kbOptionsFromCfg(motionCfg, valExtra)

	// build the localizer from the movement sensor
	movementSensor, ok := ms.movementSensors[req.MovementSensorName]
	if !ok {
		return nil, resource.DependencyNotFoundError(req.MovementSensorName)
	}

	origin, _, err := movementSensor.Position(ctx, nil)
	if err != nil {
		return nil, err
	}

	heading, err := movementSensor.CompassHeading(ctx, nil)
	if err != nil {
		return nil, err
	}

	// add an offset between the movement sensor and the base if it is applicable
	baseOrigin := referenceframe.NewPoseInFrame(req.ComponentName.ShortName(), spatialmath.NewZeroPose())
	movementSensorToBase, err := ms.fsService.TransformPose(ctx, baseOrigin, movementSensor.Name().ShortName(), nil)
	if err != nil {
		// here we make the assumption the movement sensor is coincident with the base
		movementSensorToBase = baseOrigin
	}
	// Create a localizer from the movement sensor, and collapse reported orientations to 2d
	localizer := motion.TwoDLocalizer(motion.NewMovementSensorLocalizer(movementSensor, origin, movementSensorToBase.Pose()))

	// create a KinematicBase from the componentName
	baseComponent, ok := ms.components[req.ComponentName]
	if !ok {
		return nil, resource.NewNotFoundError(req.ComponentName)
	}
	b, ok := baseComponent.(base.Base)
	if !ok {
		return nil, fmt.Errorf("cannot move component of type %T because it is not a Base", baseComponent)
	}

	fs, err := ms.fsService.FrameSystem(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Important: GeoPointToPose will create a pose such that incrementing latitude towards north increments +Y, and incrementing
	// longitude towards east increments +X. Heading is not taken into account. This pose must therefore be transformed based on the
	// orientation of the base such that it is a pose relative to the base's current location.
	goalPoseRaw := spatialmath.NewPoseFromPoint(spatialmath.GeoPointToPoint(req.Destination, origin))
	// construct limits
	straightlineDistance := goalPoseRaw.Point().Norm()
	if straightlineDistance > maxTravelDistanceMM {
		return nil, fmt.Errorf("cannot move more than %d kilometers", int(maxTravelDistanceMM*1e-6))
	}

	// Set the limits for a base if we are using diffential drive.
	// If we are using PTG kineamtics these limits will be ignored.
	limits := []referenceframe.Limit{
		{Min: -straightlineDistance * 3, Max: straightlineDistance * 3},
		{Min: -straightlineDistance * 3, Max: straightlineDistance * 3},
		{Min: -2 * math.Pi, Max: 2 * math.Pi},
	} // Note: this is only for diff drive, not used for PTGs
	kb, err := kinematicbase.WrapWithKinematics(ctx, b, ms.logger, localizer, limits, kinematicsOptions)
	if err != nil {
		return nil, err
	}

	// convert obstacles of type []GeoGeometry into []Geometry
	geomsRaw := spatialmath.GeoGeometriesToGeometries(obstacles, origin)

	// convert bounding regions which are GeoGeometries into Geometries
	boundingRegions := spatialmath.GeoGeometriesToGeometries(req.BoundingRegions, origin)

	mr, err := ms.createBaseMoveRequest(
		ctx,
		motionCfg,
		ms.logger,
		kb,
		goalPoseRaw,
		fs,
		geomsRaw,
		valExtra,
	)
	if err != nil {
		return nil, err
	}
	mr.seedPlan = seedPlan
	mr.replanCostFactor = valExtra.replanCostFactor
	mr.requestType = requestTypeMoveOnGlobe
	mr.geoPoseOrigin = spatialmath.NewGeoPose(origin, heading)
	mr.planRequest.BoundingRegions = boundingRegions
	return mr, nil
}

// newMoveOnMapRequest instantiates a moveRequest intended to be used in the context of a MoveOnMap call.
func (ms *builtIn) newMoveOnMapRequest(
	ctx context.Context,
	req motion.MoveOnMapReq,
	seedPlan motionplan.Plan,
	replanCount int,
) (state.PlannerExecutor, error) {
	valExtra, err := newValidatedExtra(req.Extra)
	if err != nil {
		return nil, err
	}

	if valExtra.maxReplans >= 0 {
		if replanCount > valExtra.maxReplans {
			return nil, fmt.Errorf("exceeded maximum number of replans: %d", valExtra.maxReplans)
		}
	}

	motionCfg, err := newValidatedMotionCfg(req.MotionCfg, requestTypeMoveOnMap)
	if err != nil {
		return nil, err
	}

	if req.Destination == nil {
		return nil, errors.New("destination cannot be nil")
	}

	// get the SLAM Service from the slamName
	slamSvc, ok := ms.slamServices[req.SlamName]
	if !ok {
		return nil, resource.DependencyNotFoundError(req.SlamName)
	}

	// verify slam is in localization mode
	slamProps, err := slamSvc.Properties(ctx)
	if err != nil {
		return nil, err
	}
	if slamProps.MappingMode != slam.MappingModeLocalizationOnly {
		return nil, fmt.Errorf("expected SLAM to be in localization only mode, got %v", slamProps.MappingMode)
	}

	// gets the extents of the SLAM map
	limits, err := slam.Limits(ctx, slamSvc, true)
	if err != nil {
		return nil, err
	}
	limits = append(limits, referenceframe.Limit{Min: -2 * math.Pi, Max: 2 * math.Pi})

	// create a KinematicBase from the componentName
	component, ok := ms.components[req.ComponentName]
	if !ok {
		return nil, resource.DependencyNotFoundError(req.ComponentName)
	}
	b, ok := component.(base.Base)
	if !ok {
		return nil, fmt.Errorf("cannot move component of type %T because it is not a Base", component)
	}

	// build kinematic options
	kinematicsOptions := kbOptionsFromCfg(motionCfg, valExtra)

	fs, err := ms.fsService.FrameSystem(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Create a localizer from the movement sensor, and collapse reported orientations to 2d
	localizer := motion.TwoDLocalizer(motion.NewSLAMLocalizer(slamSvc))
	kb, err := kinematicbase.WrapWithKinematics(ctx, b, ms.logger, localizer, limits, kinematicsOptions)
	if err != nil {
		return nil, err
	}

	goalPoseAdj := spatialmath.Compose(req.Destination, motion.SLAMOrientationAdjustment)

	// get point cloud data in the form of bytes from pcd
	pointCloudData, err := slam.PointCloudMapFull(ctx, slamSvc, true)
	if err != nil {
		return nil, err
	}
	// store slam point cloud data  in the form of a recursive octree for collision checking
	octree, err := pointcloud.ReadPCDToBasicOctree(bytes.NewReader(pointCloudData))
	if err != nil {
		return nil, err
	}

	req.Obstacles = append(req.Obstacles, octree)

	mr, err := ms.createBaseMoveRequest(
		ctx,
		motionCfg,
		ms.logger,
		kb,
		goalPoseAdj,
		fs,
		req.Obstacles,
		valExtra,
	)
	if err != nil {
		return nil, err
	}
	mr.requestType = requestTypeMoveOnMap
	return mr, nil
}

func (ms *builtIn) createBaseMoveRequest(
	ctx context.Context,
	motionCfg *validatedMotionConfiguration,
	logger logging.Logger,
	kb kinematicbase.KinematicBase,
	goalPoseInWorld spatialmath.Pose,
	fs referenceframe.FrameSystem,
	worldObstacles []spatialmath.Geometry,
	valExtra validatedExtra,
) (*moveRequest, error) {
	startPoseIF, err := kb.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	startPose := startPoseIF.Pose()

	// replace original base frame with one that knows how to move itself and allow planning for
	kinematicFrame := kb.Kinematics()
	if err := fs.ReplaceFrame(kinematicFrame); err != nil {
		// If the base frame is not in the frame system, add it to world. This will result in planning for a frame system containing
		// only world and the base after the FrameSystemSubset.
		err = fs.AddFrame(kinematicFrame, fs.Frame(referenceframe.World))
		if err != nil {
			return nil, err
		}
	}
	// We want to disregard anything in the FS whose eventual parent is not the base, because we don't know where it is.
	baseOnlyFS, err := fs.FrameSystemSubset(kinematicFrame)
	if err != nil {
		return nil, err
	}

	planningFS := baseOnlyFS
	_, ok := kinematicFrame.(tpspace.PTGProvider)
	if ok {
		// If we are not working with a differential drive base then we need to create a separate framesystem
		// used for planning.
		// This is because we rely on our starting configuration to place ourselves in our absolute position in the world.
		// If we are not working with a differential drive base then our base frame
		// is relative aka a PTG frame and therefore is not able to accurately place itself in its absolute position.
		// For this reason, we add a static transform which allows us to place the base in its absolute position in the
		// world frame.
		planningFS = referenceframe.NewEmptyFrameSystem("store-starting point")
		transformFrame, err := referenceframe.NewStaticFrame(kb.LocalizationFrame().Name(), startPose)
		if err != nil {
			return nil, err
		}
		err = planningFS.AddFrame(transformFrame, planningFS.World())
		if err != nil {
			return nil, err
		}
		err = planningFS.MergeFrameSystem(baseOnlyFS, transformFrame)
		if err != nil {
			return nil, err
		}
	}

	// We create a wrapperFrame so that we may place observed transient obstacles in
	// their absolute position in the world frame. The collision framesystem is used
	// by CheckPlan so that we may solely rely on referenceframe.Input information
	// to position ourselves correctly in the world.
	executionFrame := kb.Kinematics()
	localizationFrame := kb.LocalizationFrame()
	wf, err := newWrapperFrame(localizationFrame, executionFrame)
	if err != nil {
		return nil, err
	}
	collisionFS := baseOnlyFS
	err = collisionFS.ReplaceFrame(wf)
	if err != nil {
		return nil, err
	}

	goal := referenceframe.NewPoseInFrame(referenceframe.World, goalPoseInWorld)

	gif := referenceframe.NewGeometriesInFrame(referenceframe.World, worldObstacles)
	worldState, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{gif}, nil)
	if err != nil {
		return nil, err
	}

	obstacleDetectors := make(map[vision.Service][]resource.Name)
	for _, obstacleDetectorNamePair := range motionCfg.obstacleDetectors {
		// get vision service
		visionServiceName := obstacleDetectorNamePair.VisionServiceName
		visionSvc, ok := ms.visionServices[visionServiceName]
		if !ok {
			return nil, resource.DependencyNotFoundError(visionServiceName)
		}

		// add camera to vision service map
		camList, ok := obstacleDetectors[visionSvc]
		if !ok {
			obstacleDetectors[visionSvc] = []resource.Name{obstacleDetectorNamePair.CameraName}
		} else {
			camList = append(camList, obstacleDetectorNamePair.CameraName)
			obstacleDetectors[visionSvc] = camList
		}
	}

	currentInputs, _, err := ms.fsService.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}

	// effectively don't poll if the PositionPollingFreqHz is not provided
	positionPollingFreq := time.Duration(math.MaxInt64)
	if motionCfg.positionPollingFreqHz > 0 {
		positionPollingFreq = time.Duration(1000/motionCfg.positionPollingFreqHz) * time.Millisecond
	}

	// effectively don't poll if the ObstaclePollingFreqHz is not provided
	obstaclePollingFreq := time.Duration(math.MaxInt64)
	if motionCfg.obstaclePollingFreqHz > 0 {
		obstaclePollingFreq = time.Duration(1000/motionCfg.obstaclePollingFreqHz) * time.Millisecond
	}

	// TODO(RSDK-8683): move this check into the motionplan package
	atGoalCheck := func(basePose spatialmath.Pose) bool {
		if valExtra.motionProfile == motionplan.PositionOnlyMotionProfile {
			return spatialmath.PoseAlmostCoincidentEps(goal.Pose(), basePose, motionCfg.planDeviationMM)
		}
		return spatialmath.OrientationAlmostEqualEps(goal.Pose().Orientation(), basePose.Orientation(), 5) &&
			spatialmath.PoseAlmostCoincidentEps(goal.Pose(), basePose, motionCfg.planDeviationMM)
	}

	var backgroundWorkers sync.WaitGroup
	startState := motionplan.NewPlanState(
		referenceframe.FrameSystemPoses{kinematicFrame.Name(): referenceframe.NewPoseInFrame(referenceframe.World, startPose)},
		currentInputs,
	)
	goals := []*motionplan.PlanState{motionplan.NewPlanState(referenceframe.FrameSystemPoses{kinematicFrame.Name(): goal}, nil)}

	mr := &moveRequest{
		config: motionCfg,
		logger: ms.logger,
		planRequest: &motionplan.PlanRequest{
			Logger:      logger,
			Goals:       goals,
			FrameSystem: planningFS,
			StartState:  startState,
			WorldState:  worldState,
			Options:     valExtra.extra,
		},
		kinematicBase:     kb,
		replanCostFactor:  valExtra.replanCostFactor,
		atGoalCheck:       atGoalCheck,
		obstacleDetectors: obstacleDetectors,
		fsService:         ms.fsService,
		localizingFS:      collisionFS,

		executeBackgroundWorkers: &backgroundWorkers,

		responseChan: make(chan moveResponse, 1),
	}

	// TODO: Change deviatedFromPlan to just query positionPollingFreq on the struct & the same for the obstaclesIntersectPlan
	mr.position = newReplanner(positionPollingFreq, mr.deviatedFromPlan)
	mr.obstacle = newReplanner(obstaclePollingFreq, mr.obstaclesIntersectPlan)
	return mr, nil
}

type moveResponse struct {
	err             error
	executeResponse state.ExecuteResponse
}

func (mr moveResponse) String() string {
	return fmt.Sprintf("builtin.moveResponse{executeResponse: %#v, err: %v}", mr.executeResponse, mr.err)
}

func (mr *moveRequest) start(ctx context.Context, plan motionplan.Plan) {
	if ctx.Err() != nil {
		return
	}
	mr.executeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		mr.position.startPolling(ctx, plan)
	}, mr.executeBackgroundWorkers.Done)

	mr.executeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		mr.obstacle.startPolling(ctx, plan)
	}, mr.executeBackgroundWorkers.Done)

	// spawn function to execute the plan on the robot
	mr.executeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		executeResp, err := mr.execute(ctx, plan)
		resp := moveResponse{executeResponse: executeResp, err: err}
		mr.responseChan <- resp
	}, mr.executeBackgroundWorkers.Done)
}

func (mr *moveRequest) listen(ctx context.Context) (state.ExecuteResponse, error) {
	select {
	case <-ctx.Done():
		mr.logger.CDebugf(ctx, "context err: %s", ctx.Err())
		return state.ExecuteResponse{}, ctx.Err()

	case resp := <-mr.responseChan:
		mr.logger.CDebugf(ctx, "execution response: %s", resp)
		return resp.executeResponse, resp.err

	case resp := <-mr.position.responseChan:
		mr.logger.CDebugf(ctx, "position response: %s", resp)
		return resp.executeResponse, resp.err

	case resp := <-mr.obstacle.responseChan:
		mr.logger.CDebugf(ctx, "obstacle response: %s", resp)
		return resp.executeResponse, resp.err
	}
}

func (mr *moveRequest) stop() error {
	stopCtx, cancelFn := context.WithTimeout(context.Background(), baseStopTimeout)
	defer cancelFn()
	if stopErr := mr.kinematicBase.Stop(stopCtx, nil); stopErr != nil {
		mr.logger.Errorf("kinematicBase.Stop returned error %s", stopErr)
		return stopErr
	}
	return nil
}
