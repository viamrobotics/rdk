package builtin

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
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
// motion servicl.
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
	geoPoseOrigin spatialmath.GeoPose
	// poseOrigin is only set if requestType == requestTypeMoveOnMap
	poseOrigin        spatialmath.Pose
	logger            logging.Logger
	config            *validatedMotionConfiguration
	planRequest       *motionplan.PlanRequest
	seedPlan          motionplan.Plan
	kinematicBase     kinematicbase.KinematicBase
	obstacleDetectors map[vision.Service][]resource.Name
	replanCostFactor  float64
	fsService         framesystem.Service

	executeBackgroundWorkers *sync.WaitGroup
	responseChan             chan moveResponse
	// replanners for the move request
	// if we ever have to add additional instances we should figure out how to make this more scalable
	position, obstacle *replanner
	// waypointIndex tracks the waypoint we are currently executing on
	waypointIndex *atomic.Int32
}

// plan creates a plan using the currentInputs of the robot and the moveRequest's planRequest.
func (mr *moveRequest) Plan(ctx context.Context) (state.PlanResponse, error) {
	inputs, err := mr.kinematicBase.CurrentInputs(ctx)
	if err != nil {
		return state.PlanResponse{}, err
	}
	// TODO: this is really hacky and we should figure out a better place to store this information
	if len(mr.kinematicBase.Kinematics().DoF()) == 2 {
		inputs = inputs[:2]
	}
	mr.planRequest.StartConfiguration = map[string][]referenceframe.Input{mr.kinematicBase.Kinematics().Name(): inputs}

	existingGifs, err := mr.planRequest.WorldState.ObstaclesInWorldFrame(
		mr.planRequest.FrameSystem,
		referenceframe.StartPositions(mr.planRequest.FrameSystem),
	)
	if err != nil {
		return state.PlanResponse{}, err
	}

	// get transient detections
	gifs := []*referenceframe.GeometriesInFrame{}
	for visSrvc, cameraNames := range mr.obstacleDetectors {
		for _, camName := range cameraNames {
			currentPosition, err := mr.kinematicBase.CurrentPosition(ctx)
			if err != nil {
				return state.PlanResponse{}, err
			}
			_, relativeGIFs, err := mr.getTransientDetections(ctx, visSrvc, camName, currentPosition)
			if err != nil {
				return state.PlanResponse{}, err
			}
			gifs = append(gifs, relativeGIFs...)
		}
	}
	gifs = append(gifs, existingGifs)
	// update worldstate to include transient detections
	mr.planRequest.WorldState, err = referenceframe.NewWorldState(gifs, nil)
	if err != nil {
		return state.PlanResponse{}, err
	}

	// TODO(RSDK-5634): this should pass in mr.seedplan and the appropriate replanCostFactor once this bug is found and fixed.
	plan, err := motionplan.Replan(ctx, mr.planRequest, nil, 0)
	if err != nil {
		return state.PlanResponse{}, err
	}
	mr.logger.Debugf("plan: %v\n", plan)
	mr.logger.Debugf("sleeping now!")
	time.Sleep(time.Second * 3)

	waypoints, err := plan.GetFrameSteps(mr.kinematicBase.Kinematics().Name())
	if err != nil {
		return state.PlanResponse{}, err
	}

	switch mr.requestType {
	case requestTypeMoveOnMap:
		planSteps, err := motionplan.PlanToPlanSteps(plan, mr.kinematicBase.Name(), *mr.planRequest, mr.poseOrigin)
		if err != nil {
			return state.PlanResponse{}, err
		}
		mr.logger.Debug("PRINTING THE PLANSTEPS HERE")
		for _, step := range planSteps {
			asMap := map[resource.Name]spatialmath.Pose(step)
			for n, p := range asMap {
				mr.logger.Debugf("%s - pose: %v", n.Name, spatialmath.PoseToProtobuf(p))
			}
		}

		return state.PlanResponse{
			Waypoints:        waypoints,
			Motionplan:       plan,
			PosesByComponent: planSteps,
		}, nil
	case requestTypeMoveOnGlobe:
		// safe to use mr.poseOrigin since it is nil for requestTypeMoveOnGlobe
		planSteps, err := motionplan.PlanToPlanSteps(plan, mr.kinematicBase.Name(), *mr.planRequest, mr.poseOrigin)
		if err != nil {
			return state.PlanResponse{}, err
		}
		geoPoses := motionplan.PlanStepsToGeoPoses(planSteps, mr.kinematicBase.Name(), mr.geoPoseOrigin)

		// NOTE: Here we are smuggling GeoPoses into Poses by component
		planSteps, err = toGeoPosePlanSteps(planSteps, geoPoses)
		if err != nil {
			return state.PlanResponse{}, err
		}

		return state.PlanResponse{
			Waypoints:        waypoints,
			Motionplan:       plan,
			PosesByComponent: planSteps,
		}, nil
	case requestTypeUnspecified:
		fallthrough
	default:
		return state.PlanResponse{}, fmt.Errorf("invalid moveRequest.requestType: %d", mr.requestType)
	}
}

// execute attempts to follow a given Plan starting from the index percribed by waypointIndex.
// Note that waypointIndex is an atomic int that is incremented in this function after each waypoint has been successfully reached.
func (mr *moveRequest) execute(ctx context.Context, waypoints state.Waypoints, waypointIndex *atomic.Int32) (state.ExecuteResponse, error) {
	// Iterate through the list of waypoints and issue a command to move to each
	for i := int(waypointIndex.Load()); i < len(waypoints); i++ {
		select {
		case <-ctx.Done():
			mr.logger.CDebugf(ctx, "calling kinematicBase.Stop due to %s\n", ctx.Err())
			if stopErr := mr.stop(); stopErr != nil {
				return state.ExecuteResponse{}, errors.Wrap(ctx.Err(), stopErr.Error())
			}
			return state.ExecuteResponse{}, nil
		default:
			mr.planRequest.Logger.CInfo(ctx, waypoints[i])
			if err := mr.kinematicBase.GoToInputs(ctx, waypoints[i]); err != nil {
				// If there is an error on GoToInputs, stop the component if possible before returning the error
				mr.logger.CDebugf(ctx, "calling kinematicBase.Stop due to %s\n", err)
				if stopErr := mr.stop(); stopErr != nil {
					return state.ExecuteResponse{}, errors.Wrap(err, stopErr.Error())
				}
				return state.ExecuteResponse{}, err
			}
			if i < len(waypoints)-1 {
				waypointIndex.Add(1)
			}
		}
	}
	// the plan has been fully executed so check to see if where we are at is close enough to the goal.
	return mr.deviatedFromPlan(ctx, waypoints, len(waypoints)-1)
}

// deviatedFromPlan takes a list of waypoints and an index of a waypoint on that Plan and returns whether or not it is still
// following the plan as described by the PlanDeviation specified for the moveRequest.
func (mr *moveRequest) deviatedFromPlan(ctx context.Context, waypoints state.Waypoints, waypointIndex int) (state.ExecuteResponse, error) {
	errorState, err := mr.kinematicBase.ErrorState(ctx, waypoints, waypointIndex)
	if err != nil {
		return state.ExecuteResponse{}, err
	}
	if errorState.Point().Norm() > mr.config.planDeviationMM {
		msg := "error state exceeds planDeviationMM; planDeviationMM: %f, errorstate.Point().Norm(): %f, errorstate.Point(): %#v "
		reason := fmt.Sprintf(msg, mr.config.planDeviationMM, errorState.Point().Norm(), errorState.Point())
		return state.ExecuteResponse{Replan: true, ReplanReason: reason}, nil
	}
	return state.ExecuteResponse{}, nil
}

// getTransientDetections returns a list of geometries in their relative position (with respect to the base)
// and another list of geometries in their absolute positions (with respect to the world) as observed by camName.
// Relative position geometries are used for constructing a plan, since the base plans in its own frame.
// Absolute position geometries are used for checking a plan for collisions.
func (mr *moveRequest) getTransientDetections(
	ctx context.Context,
	visSrvc vision.Service,
	camName resource.Name,
	localizerCurrentPosition *referenceframe.PoseInFrame,
) ([]*referenceframe.GeometriesInFrame, []*referenceframe.GeometriesInFrame, error) {
	mr.logger.CDebugf(ctx,
		"proceeding to get detections from vision service: %s with camera: %s",
		visSrvc.Name().ShortName(),
		camName.ShortName(),
	)
	// any obstacles specified by the worldstate of the moveRequest will also re-detected here.
	// get detections from vision service
	detections, err := visSrvc.GetObjectPointClouds(ctx, camName.Name, nil)
	if err != nil {
		return nil, nil, err
	}
	mr.logger.CDebugf(ctx, "got %d detections", len(detections))

	// Here we make the assumption that the localizer's current position/orientation
	// is shared with the observing camera?? or base??
	// TODO: ENSURE THIS ASSUMPTION HOLDS
	currentPositionInWorld, err := mr.fsService.TransformPose(ctx, localizerCurrentPosition, "world", nil)
	if err != nil {
		currentPositionInWorld = localizerCurrentPosition
	}
	mr.logger.CDebugf(ctx, "currentPositionInWorld: %v", spatialmath.PoseToProtobuf(currentPositionInWorld.Pose()))

	// determine transform of camera to world
	cameraOrigin := referenceframe.NewPoseInFrame(camName.ShortName(), spatialmath.NewZeroPose())
	cameraToWorld, err := mr.fsService.TransformPose(ctx, cameraOrigin, "world", nil)
	if err != nil {
		// here we make the assumption the camera is coincident with the world
		mr.logger.CDebugf(ctx,
			"we assume the world is coincident with the camera named: %s due to err: %v",
			camName.ShortName(), err.Error(),
		)
		cameraToWorld = cameraOrigin
	}
	mr.logger.CDebugf(ctx, "cameraToWorld transform: %v", spatialmath.PoseToProtobuf(cameraToWorld.Pose()))

	// determine transform of camera to base
	cameraToBase, err := mr.fsService.TransformPose(ctx, cameraOrigin, mr.kinematicBase.Name().ShortName(), nil)
	if err != nil {
		// here we make the assumption the camera is coincident with the world
		mr.logger.CDebugf(ctx,
			"we assume the base named: %s is coincident with the camera named: %s due to err: %v",
			mr.kinematicBase.Name().ShortName(), camName.ShortName(), err.Error(),
		)
		cameraToBase = cameraOrigin
	}
	mr.logger.CDebugf(ctx, "cameraToBase transform: %v", spatialmath.PoseToProtobuf(cameraToBase.Pose()))

	// detections in the world frame used for CheckPlan
	absoluteGeoms := []spatialmath.Geometry{}

	// detection in the base frame used for Replan
	relativeGeoms := []spatialmath.Geometry{}

	for i, detection := range detections {
		geometry := detection.Geometry
		// this is here so that we don't deal with junk on this specific hardware test
		// this real version should not have this conditional.
		if geometry.Pose().Point().Z > 1000 {
			continue
		}

		label := camName.Name + "_transientObstacle_" + strconv.Itoa(i)
		if geometry.Label() != "" {
			label += "_" + geometry.Label()
		}
		geometry.SetLabel(label)
		mr.logger.CDebugf(ctx, "detection %d observed from the camera frame coordinate system: %s - %s",
			i, camName.ShortName(), geometry.String(),
		)

		// transform the geometry to be relative to the base frame which is +Y forwards
		relativeGeom := geometry
		switch mr.requestType {
		case requestTypeMoveOnMap:
			// the base's orientation is be default rotated -90 RH degrees, here we fix that.
			// this assumes base and cam are co-incident
			relativeGeom = relativeGeom.Transform(cameraToBase.Pose())
			relativeGeom = relativeGeom.Transform(spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 90}))
			// this is incorrectly placing geoms im pretty sure
		case requestTypeMoveOnGlobe:
			relativeGeom = relativeGeom.Transform(cameraToBase.Pose())
		case requestTypeUnspecified:
			fallthrough
		default:
			return nil, nil, fmt.Errorf("invalid moveRequest.requestType: %d", mr.requestType)
		}
		mr.logger.CDebugf(ctx, "detection %d observed from the camera in the base frame coordinate system has pose: %v",
			i, spatialmath.PoseToProtobuf(relativeGeom.Pose()),
		)
		relativeGeoms = append(relativeGeoms, relativeGeom)

		// transform the geometry into the world frame coordinate system
		geometry = geometry.Transform(cameraToWorld.Pose())
		switch mr.requestType {
		case requestTypeMoveOnMap:
			baseTheta := currentPositionInWorld.Pose().Orientation().OrientationVectorDegrees().Theta
			geometry = geometry.Transform(
				spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: baseTheta + 90}),
			)
		}
		mr.logger.CDebugf(ctx, "detection %d observed from the camera in the world frame coordinate system has pose: %v",
			i, spatialmath.PoseToProtobuf(geometry.Pose()),
		)

		// transform the geometry into it's absolute coordinates, i.e. into the world frame
		desiredPose := spatialmath.NewPose(
			geometry.Pose().Point().Add(currentPositionInWorld.Pose().Point()),
			geometry.Pose().Orientation(),
		)
		transformBy := spatialmath.PoseBetweenInverse(geometry.Pose(), desiredPose)
		geometry = geometry.Transform(transformBy)
		mr.logger.CDebugf(ctx, "detection %d observed from world frame has pose: %v",
			i, spatialmath.PoseToProtobuf(geometry.Pose()),
		)

		absoluteGeoms = append(absoluteGeoms, geometry)
	}

	absoluteGIFs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, absoluteGeoms)}
	relativeGIFs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, relativeGeoms)}

	return absoluteGIFs, relativeGIFs, nil
}

func (mr *moveRequest) obstaclesIntersectPlan(
	ctx context.Context,
	waypoints state.Waypoints,
	waypointIndex int,
) (state.ExecuteResponse, error) {
	mr.logger.Debugf("waypointIndex: %d", waypointIndex)
	var plan motionplan.Plan
	// We only care to check against waypoints we have not reached yet.
	for _, inputs := range waypoints[waypointIndex:] {
		input := make(map[string][]referenceframe.Input)
		input[mr.kinematicBase.Name().Name] = inputs
		plan = append(plan, input)
	}

	for visSrvc, cameraNames := range mr.obstacleDetectors {
		for _, camName := range cameraNames {
			// get the current position of the base which we will use to transform the detection into world coordinates
			currentPosition, err := mr.kinematicBase.CurrentPosition(ctx)
			if err != nil {
				return state.ExecuteResponse{}, err
			}

			// Note: detections are initially observed from the camera frame but must be transformed to be in
			// world frame. We cannot use the inputs of the base to transform the detections since they are relative.
			absoluteGIFs, _, err := mr.getTransientDetections(ctx, visSrvc, camName, currentPosition)
			if err != nil {
				return state.ExecuteResponse{}, err
			}

			// get the already defined geometries of worldstate
			existingGifs, err := mr.planRequest.WorldState.ObstaclesInWorldFrame(
				mr.planRequest.FrameSystem,
				referenceframe.StartPositions(mr.planRequest.FrameSystem),
			)
			if err != nil {
				return state.ExecuteResponse{}, err
			}

			// existing geometries are in their relative position, i.e. with respect to the base's frame
			// here we transform them into their absolute positions. i.e. with respect to the world frame
			// Note: we remove any existing transient obstacle geometries as they will be re-detected
			existingGeomsAbs := []spatialmath.Geometry{}
			for _, g := range existingGifs.Geometries() {
				if strings.Contains(g.Label(), "_transientObstacle_") {
					continue
				}
				mr.logger.Debugf("g BEFORE TRANSFORM %v", spatialmath.PoseToProtobuf(g.Pose()))

				absolutePositionGeom := g.Transform(mr.poseOrigin)
				mr.logger.Debugf("AFTER FIRST TRANSFORM: %v", spatialmath.PoseToProtobuf(absolutePositionGeom.Pose()))

				existingGeomsAbs = append(existingGeomsAbs, absolutePositionGeom)
			}
			absExistingGifs := referenceframe.NewGeometriesInFrame(referenceframe.World, existingGeomsAbs)

			// append the existing elements of the worldstate in their absolute positions
			absoluteGIFs = append(absoluteGIFs, absExistingGifs)

			worldState, err := referenceframe.NewWorldState(absoluteGIFs, nil)
			if err != nil {
				return state.ExecuteResponse{}, err
			}

			mr.logger.Debugf("currentPosition: %v", spatialmath.PoseToProtobuf(currentPosition.Pose()))

			currentInputs, err := mr.kinematicBase.CurrentInputs(ctx)
			if err != nil {
				return state.ExecuteResponse{}, err
			}
			mr.logger.Debugf("currentInputs: %v", currentInputs)

			// get the pose difference between where the robot is versus where it ought to be.
			errorState, err := mr.kinematicBase.ErrorState(ctx, waypoints, waypointIndex)
			if err != nil {
				return state.ExecuteResponse{}, err
			}
			mr.logger.Debugf("errorState: %v", spatialmath.PoseToProtobuf(errorState))

			if err := motionplan.CheckPlan(
				mr.kinematicBase.Kinematics(), // frame we wish to check for collisions
				plan,                          // remainder of plan we wish to check against
				worldState,                    // detected obstacles by this instance of camera + service
				mr.planRequest.FrameSystem,
				currentPosition.Pose(), // currentPosition of robot accounts for errorState
				currentInputs,
				errorState, // deviation of robot from plan
				lookAheadDistanceMM,
				mr.planRequest.Logger,
			); err != nil {
				mr.logger.Debug("WE HAVE A COLLISION")
				mr.planRequest.Logger.CInfo(ctx, err.Error())
				return state.ExecuteResponse{Replan: true, ReplanReason: err.Error()}, nil
			}
		}
	}
	mr.logger.Debug("HUZZAH NO ERRORS")
	return state.ExecuteResponse{}, nil
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

func newValidatedMotionCfg(motionCfg *motion.MotionConfiguration) (*validatedMotionConfiguration, error) {
	empty := &validatedMotionConfiguration{}
	vmc := &validatedMotionConfiguration{
		angularDegsPerSec:     defaultAngularDegsPerSec,
		linearMPerSec:         defaultLinearMPerSec,
		obstaclePollingFreqHz: defaultObstaclePollingHz,
		positionPollingFreqHz: defaultPositionPollingHz,
		planDeviationMM:       defaultPlanDeviationM * 1e3,
		obstacleDetectors:     []motion.ObstacleDetectorName{},
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

	if err := validateNotNegNorNaN(motionCfg.ObstaclePollingFreqHz, "ObstaclePollingFreqHz"); err != nil {
		return empty, err
	}

	if err := validateNotNegNorNaN(motionCfg.PositionPollingFreqHz, "PositionPollingFreqHz"); err != nil {
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

	if motionCfg.ObstaclePollingFreqHz != 0 {
		vmc.obstaclePollingFreqHz = motionCfg.ObstaclePollingFreqHz
	}

	if motionCfg.PositionPollingFreqHz != 0 {
		vmc.positionPollingFreqHz = motionCfg.PositionPollingFreqHz
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

	motionCfg, err := newValidatedMotionCfg(req.MotionCfg)
	if err != nil {
		return nil, err
	}
	// ensure arguments are well behaved
	obstacles := req.Obstacles
	if obstacles == nil {
		obstacles = []*spatialmath.GeoObstacle{}
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
	localizer := motion.NewMovementSensorLocalizer(movementSensor, origin, movementSensorToBase.Pose())

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
	limits := []referenceframe.Limit{
		{Min: -straightlineDistance * 3, Max: straightlineDistance * 3},
		{Min: -straightlineDistance * 3, Max: straightlineDistance * 3},
		{Min: -2 * math.Pi, Max: 2 * math.Pi},
	} // Note: this is only for diff drive, not used for PTGs
	ms.logger.CDebugf(ctx, "base limits: %v", limits)

	kb, err := kinematicbase.WrapWithKinematics(ctx, b, ms.logger, localizer, limits, kinematicsOptions)
	if err != nil {
		return nil, err
	}

	geomsRaw := spatialmath.GeoObstaclesToGeometries(obstacles, origin)

	mr, err := ms.relativeMoveRequestFromAbsolute(
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
	mr.geoPoseOrigin = *spatialmath.NewGeoPose(origin, heading)
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

	motionCfg, err := newValidatedMotionCfg(req.MotionCfg)
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

	// gets the extents of the SLAM map
	limits, err := slam.Limits(ctx, slamSvc)
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

	kb, err := kinematicbase.WrapWithKinematics(ctx, b, ms.logger, motion.NewSLAMLocalizer(slamSvc), limits, kinematicsOptions)
	if err != nil {
		return nil, err
	}

	goalPoseAdj := spatialmath.Compose(req.Destination, motion.SLAMOrientationAdjustment)

	// get point cloud data in the form of bytes from pcd
	pointCloudData, err := slam.PointCloudMapFull(ctx, slamSvc)
	if err != nil {
		return nil, err
	}
	// store slam point cloud data  in the form of a recursive octree for collision checking
	octree, err := pointcloud.ReadPCDToBasicOctree(bytes.NewReader(pointCloudData))
	if err != nil {
		return nil, err
	}

	mr, err := ms.relativeMoveRequestFromAbsolute(
		ctx,
		motionCfg,
		ms.logger,
		kb,
		goalPoseAdj,
		fs,
		[]spatialmath.Geometry{octree},
		valExtra,
	)
	if err != nil {
		return nil, err
	}
	startPose, err := mr.kinematicBase.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	mr.poseOrigin = startPose.Pose()
	mr.requestType = requestTypeMoveOnMap
	return mr, nil
}

func (ms *builtIn) relativeMoveRequestFromAbsolute(
	ctx context.Context,
	motionCfg *validatedMotionConfiguration,
	logger logging.Logger,
	kb kinematicbase.KinematicBase,
	goalPoseInWorld spatialmath.Pose,
	fs referenceframe.FrameSystem,
	worldObstacles []spatialmath.Geometry,
	valExtra validatedExtra,
) (*moveRequest, error) {
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

	startPose, err := kb.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	ms.logger.Debugf("startPose: %v", spatialmath.PoseToProtobuf(startPose.Pose()))
	startPoseInv := spatialmath.PoseInverse(startPose.Pose())
	ms.logger.Debugf("startPoseInv: %v", spatialmath.PoseToProtobuf(startPoseInv))

	goal := referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.PoseBetween(startPose.Pose(), goalPoseInWorld))

	// convert GeoObstacles into GeometriesInFrame with respect to the base's starting point
	geoms := make([]spatialmath.Geometry, 0, len(worldObstacles))
	// TODO: understand this better and what to do when placing into abosolute position
	for _, geom := range worldObstacles {
		ms.logger.Debugf("WRLDST GEOM - BEFORE - TRANSFORM: %v", spatialmath.PoseToProtobuf(geom.Pose()))
		after := geom.Transform(startPoseInv)
		geoms = append(geoms, after)
		ms.logger.Debugf("WRLDST GEOM - AFTER - TRANSFORM: %v", spatialmath.PoseToProtobuf(after.Pose()))
	}

	gif := referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)
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

	var backgroundWorkers sync.WaitGroup

	var waypointIndex atomic.Int32
	waypointIndex.Store(1)

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

	mr := &moveRequest{
		config: motionCfg,
		logger: ms.logger,
		planRequest: &motionplan.PlanRequest{
			Logger:             logger,
			Goal:               goal,
			Frame:              kinematicFrame,
			FrameSystem:        baseOnlyFS,
			StartConfiguration: currentInputs,
			WorldState:         worldState,
			Options:            valExtra.extra,
		},
		kinematicBase:     kb,
		replanCostFactor:  valExtra.replanCostFactor,
		obstacleDetectors: obstacleDetectors,
		fsService:         ms.fsService,

		executeBackgroundWorkers: &backgroundWorkers,

		responseChan: make(chan moveResponse, 1),

		waypointIndex: &waypointIndex,
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

func (mr *moveRequest) start(ctx context.Context, waypoints [][]referenceframe.Input) {
	if ctx.Err() != nil {
		return
	}
	mr.executeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		mr.position.startPolling(ctx, waypoints, mr.waypointIndex)
	}, mr.executeBackgroundWorkers.Done)

	mr.executeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		mr.obstacle.startPolling(ctx, waypoints, mr.waypointIndex)
	}, mr.executeBackgroundWorkers.Done)

	// spawn function to execute the plan on the robot
	mr.executeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		executeResp, err := mr.execute(ctx, waypoints, mr.waypointIndex)
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

func (mr *moveRequest) Execute(ctx context.Context, waypoints state.Waypoints) (state.ExecuteResponse, error) {
	defer mr.executeBackgroundWorkers.Wait()
	cancelCtx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	mr.start(cancelCtx, waypoints)
	return mr.listen(cancelCtx)
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

func toGeoPosePlanSteps(posesByComponent []motionplan.PlanStep, geoPoses []spatialmath.GeoPose) ([]motionplan.PlanStep, error) {
	if len(geoPoses) != len(posesByComponent) {
		msg := "GeoPoses (len: %d) & PosesByComponent (len: %d) must have the same length"
		return nil, fmt.Errorf(msg, len(geoPoses), len(posesByComponent))
	}
	steps := make([]motionplan.PlanStep, 0, len(posesByComponent))
	for i, ps := range posesByComponent {
		if len(ps) == 0 {
			continue
		}

		if l := len(ps); l > 1 {
			return nil, fmt.Errorf("only single component or fewer plan steps supported, received plan step with %d componenents", l)
		}

		var resourceName resource.Name
		for k := range ps {
			resourceName = k
		}
		geoPose := geoPoses[i]
		heading := math.Mod(math.Abs(geoPose.Heading()-360), 360)
		o := &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: heading}
		poseContainingGeoPose := spatialmath.NewPose(r3.Vector{X: geoPose.Location().Lng(), Y: geoPose.Location().Lat()}, o)
		steps = append(steps, map[resource.Name]spatialmath.Pose{resourceName: poseContainingGeoPose})
	}
	return steps, nil
}
