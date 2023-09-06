package builtin

import (
	"context"
	"fmt"
	"math"
	"time"

	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

// moveRequest is a structure that contains all the information necessary for to make a move call.
type moveRequest struct {
	planRequest        *motionplan.PlanRequest
	actuator           kinematicbase.KinematicBase
	execute            func(context.Context, motionplan.Plan) moveResponse
	position, obstacle *replanner
}

// plan creates a plan using the currentInputs of the robot and the moveRequest's planRequest
func (mr *moveRequest) plan(ctx context.Context) (motionplan.Plan, error) {
	inputs, err := mr.actuator.CurrentInputs(ctx)
	if err != nil {
		return make(motionplan.Plan, 0), err
	}
	// TODO: this is really hacky and we should figure out a better place to store this information
	if len(mr.planRequest.Frame.DoF()) == 2 {
		inputs = inputs[:2]
	}
	mr.planRequest.StartConfiguration = map[string][]referenceframe.Input{mr.actuator.Kinematics().Name(): inputs}
	return motionplan.PlanMotion(ctx, mr.planRequest)
}

// newMoveOnGlobeRequest instantiates a moveRequest intended to be used in the context of a MoveOnGlobe call
func (ms *builtIn) newMoveOnGlobeRequest(
	ctx context.Context,
	componentName resource.Name,
	destination *geo.Point,
	movementSensorName resource.Name,
	obstacles []*spatialmath.GeoObstacle,
	motionCfg *motion.MotionConfiguration,
	extra map[string]interface{},
) (*moveRequest, error) {
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
	if straightlineDistance > maxTravelDistance {
		return nil, fmt.Errorf("cannot move more than %d kilometers", int(maxTravelDistance*1e-6))
	}
	limits := []referenceframe.Limit{
		{Min: -straightlineDistance * 3, Max: straightlineDistance * 3},
		{Min: -straightlineDistance * 3, Max: straightlineDistance * 3},
		{Min: -2 * math.Pi, Max: 2 * math.Pi},
	}
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

	kb, err := kinematicbase.WrapWithKinematics(ctx, b, ms.logger, localizer, limits, kinematicsOptions)
	if err != nil {
		return nil, err
	}

	// create a new empty framesystem which we add the kinematic base to
	fs := referenceframe.NewEmptyFrameSystem("")
	kbf := kb.Kinematics()
	if err := fs.AddFrame(kbf, fs.World()); err != nil {
		return nil, err
	}

	// TODO(RSDK-3407): this does not adequately account for geometries right now since it is a transformation after the fact.
	// This is probably acceptable for the time being, but long term the construction of the frame system for the kinematic base should
	// be moved under the purview of the kinematic base wrapper instead of being done here.
	offsetFrame, err := referenceframe.NewStaticFrame("offset", movementSensorToBase.Pose())
	if err != nil {
		return nil, err
	}
	if err := fs.AddFrame(offsetFrame, kbf); err != nil {
		return nil, err
	}

	return &moveRequest{
		planRequest: &motionplan.PlanRequest{
			Logger:             ms.logger,
			Goal:               referenceframe.NewPoseInFrame(referenceframe.World, goal),
			Frame:              offsetFrame,
			FrameSystem:        fs,
			StartConfiguration: referenceframe.StartPositions(fs),
			WorldState:         worldState,
			Options:            extra,
		},
		actuator: kb,
		execute: func(ctx context.Context, plan motionplan.Plan) moveResponse {
			waypoints, err := plan.GetFrameSteps(kb.Name().Name)
			if err != nil {
				return moveResponse{err: err}
			}

			// Iterate through the list of waypoints and issue a command to move to each
			for i := 1; i < len(waypoints); i++ {
				select {
				case <-ctx.Done():
					return moveResponse{}
				default:
					ms.logger.Info(waypoints[i])
					if err := kb.GoToInputs(ctx, waypoints[i]); err != nil {
						// If there is an error on GoToInputs, stop the component if possible before returning the error
						if stopErr := kb.Stop(ctx, nil); stopErr != nil {
							return moveResponse{err: errors.Wrap(err, stopErr.Error())}
						}
						// If the error was simply a cancellation of context return without erroring out
						if errors.Is(err, context.Canceled) {
							return moveResponse{}
						}
						return moveResponse{err: err}
					}
				}
			}

			// the plan has been fully executed so check to see if the GeoPoint we are at is close enough to the goal.
			position, _, err := movementSensor.Position(ctx, nil)
			if err != nil {
				return moveResponse{err: err}
			}
			if spatialmath.GeoPointToPose(position, destination).Point().Norm() <= motionCfg.PlanDeviationMM {
				return moveResponse{success: true}
			}
			return moveResponse{err: errors.New("reached end of plan but not at goal")}
		},
		position: &replanner{
			period:       time.Duration(1000/motionCfg.PositionPollingFreqHz) * time.Millisecond,
			responseChan: make(chan replanResponse),
			fnToPoll: func(ctx context.Context) replanResponse {
				return replanResponse{}
			},
		},
		obstacle: &replanner{
			period:       time.Duration(1000/motionCfg.ObstaclePollingFreqHz) * time.Millisecond,
			responseChan: make(chan replanResponse),
			fnToPoll: func(ctx context.Context) replanResponse {
				return replanResponse{}
			},
		},
	}, nil
}
