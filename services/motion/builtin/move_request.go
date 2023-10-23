package builtin

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"sync/atomic"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

const (
	defaultReplanCostFactor = 1.0
	defaultMaxReplans       = -1 // Values below zero will replan infinitely
)

// moveRequest is a structure that contains all the information necessary for to make a move call.
type moveRequest struct {
	config           *motion.MotionConfiguration
	planRequest      *motionplan.PlanRequest
	seedPlan         motionplan.Plan
	kinematicBase    kinematicbase.KinematicBase
	replanCostFactor float64
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
	plan, err := motionplan.Replan(ctx, mr.planRequest, mr.seedPlan, mr.replanCostFactor)
	if err != nil {
		return nil, err
	}
	mr.seedPlan = plan
	return mr.seedPlan.GetFrameSteps(mr.kinematicBase.Kinematics().Name())
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

	// the plan has been fully executed so check to see if where we are at is close enough to the goal.
	deviated, err := mr.deviatedFromPlan(ctx, waypoints, len(waypoints)-1)
	if err != nil {
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
	mr.planRequest.Logger.Debug("deviation from plan: %v", errorState.Point())
	return errorState.Point().Norm() > mr.config.PlanDeviationMM, nil
}

func (mr *moveRequest) obstaclesIntersectPlan(ctx context.Context, waypoints [][]referenceframe.Input, waypointIndex int) (bool, error) {
	// TODO(RSDK-4507): implement this function
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
	seedPlan motionplan.Plan,
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

	// Important: GeoPointToPose will create a pose such that incrementing latitude towards north increments +Y, and incrementing
	// longitude towards east increments +X. Heading is not taken into account. This pose must therefore be transformed based on the
	// orientation of the base such that it is a pose relative to the base's current location.
	goalPoseRaw := spatialmath.GeoPointToPose(destination, origin)
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
	ms.logger.Debugf("base limits: %v", limits)

	replanCostFactor := defaultReplanCostFactor
	if extra != nil {
		if profile, ok := extra["motion_profile"]; ok {
			motionProfile, ok := profile.(string)
			if !ok {
				return nil, errors.New("could not interpret motion_profile field as string")
			}
			kinematicsOptions.PositionOnlyMode = motionProfile == motionplan.PositionOnlyMotionProfile
		}
		if costFactorRaw, ok := extra["replan_cost_factor"]; ok {
			costFactor, ok := costFactorRaw.(float64)
			if !ok {
				return nil, errors.New("could not interpret replan_cost_factor field as float")
			}
			replanCostFactor = costFactor
		}
	}

	kb, err := kinematicbase.WrapWithKinematics(ctx, b, ms.logger, localizer, limits, kinematicsOptions)
	if err != nil {
		return nil, err
	}

	geomsRaw := spatialmath.GeoObstaclesToGeometries(obstacles, origin)

	mr, err := relativeMoveRequestFromAbsolute(
		ctx,
		motionCfg,
		ms.logger,
		kb,
		goalPoseRaw,
		fs,
		geomsRaw,
		extra,
	)
	if err != nil {
		return nil, err
	}
	mr.seedPlan = seedPlan
	mr.replanCostFactor = replanCostFactor
	return mr, nil
}

// newMoveOnMapRequest instantiates a moveRequest intended to be used in the context of a MoveOnMap call.
func (ms *builtIn) newMoveOnMapRequest(
	ctx context.Context,
	componentName resource.Name,
	goalPoseRaw spatialmath.Pose,
	slamName resource.Name,
	kinematicsOptions kinematicbase.Options,
	extra map[string]interface{},
) (*moveRequest, error) {
	// get the SLAM Service from the slamName
	slamSvc, ok := ms.slamServices[slamName]
	if !ok {
		return nil, resource.DependencyNotFoundError(slamName)
	}

	// gets the extents of the SLAM map
	limits, err := slam.Limits(ctx, slamSvc)
	if err != nil {
		return nil, err
	}
	limits = append(limits, referenceframe.Limit{Min: -2 * math.Pi, Max: 2 * math.Pi})

	// create a KinematicBase from the componentName
	component, ok := ms.components[componentName]
	if !ok {
		return nil, resource.DependencyNotFoundError(componentName)
	}
	b, ok := component.(base.Base)
	if !ok {
		return nil, fmt.Errorf("cannot move component of type %T because it is not a Base", component)
	}

	if extra != nil {
		if profile, ok := extra["motion_profile"]; ok {
			motionProfile, ok := profile.(string)
			if !ok {
				return nil, errors.New("could not interpret motion_profile field as string")
			}
			kinematicsOptions.PositionOnlyMode = motionProfile == motionplan.PositionOnlyMotionProfile
		}
	}

	fs, err := ms.fsService.FrameSystem(ctx, nil)
	if err != nil {
		return nil, err
	}

	kb, err := kinematicbase.WrapWithKinematics(ctx, b, ms.logger, motion.NewSLAMLocalizer(slamSvc), limits, kinematicsOptions)
	if err != nil {
		return nil, err
	}

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

	mr, err := relativeMoveRequestFromAbsolute(
		ctx,
		nil,
		ms.logger,
		kb,
		goalPoseRaw,
		fs,
		[]spatialmath.Geometry{octree},
		extra,
	)
	return mr, err
}

func relativeMoveRequestFromAbsolute(
	ctx context.Context,
	motionCfg *motion.MotionConfiguration,
	logger golog.Logger,
	kb kinematicbase.KinematicBase,
	goalPoseInWorld spatialmath.Pose,
	fs referenceframe.FrameSystem,
	worldObstacles []spatialmath.Geometry,
	extra map[string]interface{},
) (*moveRequest, error) {
	// replace original base frame with one that knows how to move itself and allow planning for
	kinematicFrame := kb.Kinematics()
	if err := fs.ReplaceFrame(kinematicFrame); err != nil {
		return nil, err
	}
	// We want to disregard anything in the FS whose eventual parent is not the base, because we don't know where it is.
	baseOnlyFS, err := fs.FrameSystemSubset(kinematicFrame)
	if err != nil {
		return nil, err
	}

	startPoseInWorld, err := kb.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	startPoseToWorld := spatialmath.PoseInverse(startPoseInWorld.Pose())

	goal := referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.PoseBetween(startPoseInWorld.Pose(), goalPoseInWorld))

	// convert GeoObstacles into GeometriesInFrame with respect to the base's starting point

	geoms := make([]spatialmath.Geometry, 0, len(worldObstacles))
	for _, geom := range worldObstacles {
		geoms = append(geoms, geom.Transform(startPoseToWorld))
	}

	gif := referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)
	worldState, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{gif}, nil)
	if err != nil {
		return nil, err
	}

	if motionCfg == nil {
		motionCfg = &motion.MotionConfiguration{}
	}

	mr := &moveRequest{
		config: motionCfg,
		planRequest: &motionplan.PlanRequest{
			Logger:             logger,
			Goal:               goal,
			Frame:              kinematicFrame,
			FrameSystem:        baseOnlyFS,
			StartConfiguration: referenceframe.StartPositions(baseOnlyFS),
			WorldState:         worldState,
			Options:            extra,
		},
		kinematicBase: kb,
	}
	return mr, nil
}
