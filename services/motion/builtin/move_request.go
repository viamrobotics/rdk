package builtin

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"

	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
)

// moveRequest is a structure that contains all the information necessary for to make a move call.
type moveRequest struct {
	config        *motion.MotionConfiguration
	planRequest   *motionplan.PlanRequest
	kinematicBase kinematicbase.KinematicBase
	visionSvcs    []vision.Service
	cameraNames   []resource.Name
}

// plan creates a plan using the currentInputs of the robot and the moveRequest's planRequest.
func (mr *moveRequest) plan(ctx context.Context) ([][]referenceframe.Input, error) {
	inputs, err := mr.kinematicBase.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	// TODO: this is really hacky and we should figure out a better place to store this information
	if len(mr.kinematicBase.Kinematics().DoF()) == 2 {
		inputs = inputs[:2]
	}
	mr.planRequest.StartConfiguration = map[string][]referenceframe.Input{mr.kinematicBase.Kinematics().Name(): inputs}
	plan, err := motionplan.PlanMotion(ctx, mr.planRequest)
	if err != nil {
		return nil, err
	}
	return plan.GetFrameSteps(mr.kinematicBase.Kinematics().Name())
}

// execute attempts to follow a given Plan starting from the index percribed by waypointIndex.
// Note that waypointIndex is an atomic int that is incremented in this function after each waypoint has been successfully reached.
func (mr *moveRequest) execute(ctx context.Context, waypoints [][]referenceframe.Input, waypointIndex *atomic.Int32) moveResponse {
	// Iterate through the list of waypoints and issue a command to move to each
	for i := int(waypointIndex.Load()); i < len(waypoints); i++ {
		select {
		case <-ctx.Done():
			return moveResponse{}
		default:
			mr.planRequest.Logger.Info(waypoints[i])
			if err := mr.kinematicBase.GoToInputs(ctx, waypoints[i]); err != nil {
				// If there is an error on GoToInputs, stop the component if possible before returning the error
				if stopErr := mr.kinematicBase.Stop(ctx, nil); stopErr != nil {
					return moveResponse{err: errors.Wrap(err, stopErr.Error())}
				}
				// If the error was simply a cancellation of context return without erroring out
				if errors.Is(err, context.Canceled) {
					return moveResponse{}
				}
				return moveResponse{err: err}
			}
			if i < len(waypoints)-1 {
				waypointIndex.Add(1)
			}
		}
	}

	// the plan has been fully executed so check to see if the GeoPoint we are at is close enough to the goal.
	deviated, err := mr.deviatedFromPlan(ctx, waypoints, len(waypoints)-1)
	if errors.Is(err, context.Canceled) {
		return moveResponse{}
	} else if err != nil {
		return moveResponse{err: err}
	}
	return moveResponse{success: !deviated}
}

// deviatedFromPlan takes a list of waypoints and an index of a waypoint on that Plan and returns whether or not it is still
// following the plan as described by the PlanDeviation specified for the moveRequest.
func (mr *moveRequest) deviatedFromPlan(ctx context.Context, waypoints [][]referenceframe.Input, waypointIndex int) (bool, error) {
	errorState, err := mr.kinematicBase.ErrorState(ctx, waypoints, waypointIndex)
	if err != nil {
		return false, err
	}
	mr.planRequest.Logger.Debugf("deviation from plan: %v", errorState.Point())
	return errorState.Point().Norm() > mr.config.PlanDeviationMM, nil
}

func (mr *moveRequest) obstaclesIntersectPlan(ctx context.Context, waypoints [][]referenceframe.Input, waypointIndex int) (bool, error) {
	// convert waypoints to type of motionplan.Plan
	var plan []map[string][]referenceframe.Input
	for i, inputs := range waypoints {
		// We only care to check against waypoints we have not reached yet.
		if i >= waypointIndex {
			input := make(map[string][]referenceframe.Input)
			input[mr.kinematicBase.Name().Name] = inputs
			plan = append(plan, input)
		}
	}

	// iterate through vision services
	for _, srvc := range mr.visionSvcs {
		// iterate through all cameras on the robot
		// any camera may be used by any vision service
		for _, camName := range mr.cameraNames {
			// get detections from vision service
			detections, err := srvc.GetObjectPointClouds(ctx, camName.Name, nil)
			if err != nil {
				return false, err
			}
			// Any obstacles specified by the worldstate of the moveRequest will also re-detected here.
			// There is no need to append the new detections to the existing worldstate.
			// We can safely build from scratch without excluding any valuable information.
			geoms := []spatialmath.Geometry{}
			for _, detection := range detections {
				detection.Geometry.SetLabel("transient" + detection.Geometry.Label())
				geoms = append(geoms, detection.Geometry)
			}
			gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(camName.Name, geoms)}
			worldState, err := referenceframe.NewWorldState(gifs, nil)
			if err != nil {
				return false, err
			}

			currentPosition, err := mr.kinematicBase.CurrentPosition(ctx)
			if err != nil {
				return false, err
			}
			currentInputs, err := mr.kinematicBase.CurrentInputs(ctx)
			if err != nil {
				return false, err
			}

			// get the pose difference between where the robot is versus where it ought to be.
			errorState, err := mr.kinematicBase.ErrorState(ctx, waypoints, waypointIndex)
			if err != nil {
				return false, err
			}

			if err := motionplan.CheckPlan(
				mr.kinematicBase.Kinematics(), // frame we wish to check for collisions
				plan,                          // remainder of plan we wish to check against
				worldState,                    // detected obstacles by this instance of camera + service
				mr.planRequest.FrameSystem,
				currentPosition.Pose(), // currentPosition of robot accounts for errorState
				currentInputs,
				errorState, // deviation of robot from plan
				mr.planRequest.Logger,
			); err != nil {
				mr.planRequest.Logger.Info("found collision with detected obstacle")
				return true, err
			}
		}
	}
	return false, nil
}

// newMoveOnGlobeRequest instantiates a moveRequest intended to be used in the context of a MoveOnGlobe call.
func (ms *builtIn) newMoveOnGlobeRequest(
	ctx context.Context,
	componentName resource.Name,
	destination *geo.Point,
	movementSensorName resource.Name,
	obstacles []*spatialmath.GeoObstacle,
	motionCfg *motion.MotionConfiguration,
	extra map[string]interface{},
) (*moveRequest, error) {
	// build kinematic options
	kinematicsOptions := kinematicbase.NewKinematicBaseOptions()
	if motionCfg.LinearMPerSec != 0 {
		kinematicsOptions.LinearVelocityMMPerSec = motionCfg.LinearMPerSec * 1000
	}
	if motionCfg.AngularDegsPerSec != 0 {
		kinematicsOptions.AngularVelocityDegsPerSec = motionCfg.AngularDegsPerSec
	}
	if motionCfg.PlanDeviationMM != 0 {
		kinematicsOptions.PlanDeviationThresholdMM = motionCfg.PlanDeviationMM
	}
	kinematicsOptions.GoalRadiusMM = motionCfg.PlanDeviationMM
	kinematicsOptions.HeadingThresholdDegrees = 8

	// build the localizer from the movement sensor
	movementSensor, ok := ms.movementSensors[movementSensorName]
	if !ok {
		return nil, resource.DependencyNotFoundError(movementSensorName)
	}
	origin, _, err := movementSensor.Position(ctx, nil)
	if err != nil {
		return nil, err
	}

	// add an offset between the movement sensor and the base if it is applicable
	baseOrigin := referenceframe.NewPoseInFrame(componentName.ShortName(), spatialmath.NewZeroPose())
	movementSensorToBase, err := ms.fsService.TransformPose(ctx, baseOrigin, movementSensor.Name().ShortName(), nil)
	if err != nil {
		// here we make the assumption the movement sensor is coincident with the base
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
		return nil, err
	}

	// construct limits
	straightlineDistance := goal.Point().Norm()
	if straightlineDistance > maxTravelDistanceMM {
		return nil, fmt.Errorf("cannot move more than %d kilometers", int(maxTravelDistanceMM*1e-6))
	}
	limits := []referenceframe.Limit{
		{Min: -straightlineDistance * 3, Max: straightlineDistance * 3},
		{Min: -straightlineDistance * 3, Max: straightlineDistance * 3},
		{Min: -2 * math.Pi, Max: 2 * math.Pi},
	} // Note: this is only for diff drive, not used for PTGs
	ms.logger.Debugf("base limits: %v", limits)

	if extra != nil {
		if profile, ok := extra["motion_profile"]; ok {
			motionProfile, ok := profile.(string)
			if !ok {
				return nil, errors.New("could not interpret motion_profile field as string")
			}
			kinematicsOptions.PositionOnlyMode = motionProfile == motionplan.PositionOnlyMotionProfile
		}
	}

	// create a KinematicBase from the componentName
	baseComponent, ok := ms.components[componentName]
	if !ok {
		return nil, resource.NewNotFoundError(componentName)
	}
	b, ok := baseComponent.(base.Base)
	if !ok {
		return nil, fmt.Errorf("cannot move component of type %T because it is not a Base", baseComponent)
	}

	fs, err := ms.fsService.FrameSystem(ctx, nil)
	if err != nil {
		return nil, err
	}

	kb, err := kinematicbase.WrapWithKinematics(ctx, b, ms.logger, localizer, limits, kinematicsOptions)
	if err != nil {
		return nil, err
	}

	// replace original base frame with one that knows how to move itself and allow planning for
	kinematicFrame := kb.Kinematics()
	if err = fs.ReplaceFrame(kinematicFrame); err != nil {
		return nil, err
	}
	// We want to disregard anything in the FS whose eventual parent is not the base, because we don't know where it is.
	baseOnlyFS, err := fs.FrameSystemSubset(kinematicFrame)
	if err != nil {
		return nil, err
	}
	// Place the base at the correct heading relative to the frame system
	currentPosition, err := kb.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	headingFrame, err := referenceframe.NewStaticFrame(
		kinematicFrame.Name()+"_origin",
		spatialmath.NewPoseFromOrientation(currentPosition.Pose().Orientation()),
	)
	if err != nil {
		return nil, err
	}
	baseFSWithOrient := referenceframe.NewEmptyFrameSystem("baseFS")
	err = baseFSWithOrient.AddFrame(headingFrame, baseFSWithOrient.World())
	if err != nil {
		return nil, err
	}
	err = baseFSWithOrient.MergeFrameSystem(baseOnlyFS, headingFrame)
	if err != nil {
		return nil, err
	}

	// add vision services to move request
	visionServices := []vision.Service{}
	for _, serviceName := range motionCfg.VisionServices {
		srvc, ok := ms.visionServices[serviceName]
		if !ok {
			return nil, resource.DependencyNotFoundError(serviceName)
		}
		visionServices = append(visionServices, srvc)
	}

	return &moveRequest{
		config: motionCfg,
		planRequest: &motionplan.PlanRequest{
			Logger:             ms.logger,
			Goal:               referenceframe.NewPoseInFrame(referenceframe.World, goal),
			Frame:              kb.Kinematics(),
			FrameSystem:        baseFSWithOrient,
			StartConfiguration: referenceframe.StartPositions(baseFSWithOrient),
			WorldState:         worldState,
			Options:            extra,
		},
		kinematicBase: kb,
		visionSvcs:    visionServices,
		cameraNames:   ms.cameras,
	}, nil
}
