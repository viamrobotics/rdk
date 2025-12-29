package builtin

import (
	"context"
	"fmt"
	"time"

	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
	"golang.org/x/sync/errgroup"
)

func getExtraObstacles(ctx context.Context, vs vision.Service) (*referenceframe.GeometriesInFrame, error) {
	if vs == nil {
		return nil, nil
	}

	const hardCodedCameraNameFromTest = ""
	currObjects, err := vs.GetObjectPointClouds(ctx, hardCodedCameraNameFromTest, nil)
	if err != nil {
		return nil, err
	}

	geoms := make([]spatialmath.Geometry, len(currObjects))
	for idx, newObstacle := range currObjects {
		geoms[idx] = newObstacle.Geometry
	}

	// Dan: TODO: Test is hardcoded to return objects in the world frame.
	return referenceframe.NewGeometriesInFrame("world", geoms), nil
}

func (ms *builtIn) doReplannable(ctx context.Context, reqI any) (map[string]any, error) {
	moveRequest, err := stringToMoveRequest(reqI)
	if err != nil {
		return nil, err
	}

	frameSys, err := ms.getFrameSystem(ctx, moveRequest.WorldState.Transforms())
	if err != nil {
		return nil, err
	}

	var obstacleVisionService vision.Service
	obsVisSvcNameI, exists := moveRequest.Extra["obstacleVisionService"]
	if exists {
		obsVisSvcName, ok := obsVisSvcNameI.(string)
		if !ok {
			return nil, fmt.Errorf("MoveRequest `obstacleVisionService` param is not a string: %T", obsVisSvcNameI)
		}

		obstacleVisionService, exists = ms.visionServices[obsVisSvcName]
		if !exists {
			return nil, fmt.Errorf("MoveRequest `obstacleVisionService` does not exist. Name: %v", obsVisSvcName)
		}
	}

	baseWorldState := moveRequest.WorldState
	var extraObstacles *referenceframe.GeometriesInFrame
	for {
		// build maps of relevant components and inputs from initial inputs
		fsInputs, err := ms.fsService.CurrentInputs(ctx)
		if err != nil {
			return nil, err
		}
		ms.logger.CDebugf(ctx, "frame system start inputs: %v", fsInputs)

		extraObstacles, err = getExtraObstacles(ctx, obstacleVisionService)
		if err != nil {
			ms.logger.Warnw("Error querying for obstacles. Delaying motion.", "err", err)
			time.Sleep(10 * time.Millisecond)
			continue
		}

		worldState := baseWorldState.Merge(extraObstacles)
		// Generate a plan for execution.
		motionPlanRequest := &armplanning.PlanRequest{
			FrameSystem: frameSys,
			StartState:  armplanning.NewPlanState(nil, fsInputs),
			Goals: []*armplanning.PlanState{
				armplanning.NewPlanState(referenceframe.FrameSystemPoses{
					moveRequest.ComponentName: moveRequest.Destination,
				}, nil),
			},
			WorldState:     worldState,
			Constraints:    moveRequest.Constraints,
			PlannerOptions: nil,
		}

		plan, _, err := armplanning.PlanMotion(ctx, ms.logger, motionPlanRequest)
		if err != nil {
			return nil, err
		}

		// While executing, the world state might change (an obstacle later comes into existence
		// that must be avoided). If an `obstacleVisionService` exists, set up a background
		// goroutine for polling the vision service for new obstacles. And validate if the executing
		// plan continues to be safe.
		//
		// If the plan is no longer safe, the `executeCtx` will be canceled. Stopping the
		// arm/actuator and allowing us to replan with the new information.
		obstacleAvoidance, executeCtx := errgroup.WithContext(ctx)
		executeCtx, executeDone := context.WithCancel(executeCtx)
		defer func() {
			executeDone()
			obstacleAvoidance.Wait()
		}()

		if obstacleVisionService != nil {
			obstacleAvoidance.Go(func() error {
				for executeCtx.Err() == nil {
					// TODO: Vision service objects are returned from the camera reference frame.
					geomsInFrame, err := getExtraObstacles(ctx, obstacleVisionService)
					if err != nil {
						return err
					}

					mergedWorldState := moveRequest.WorldState.Merge(geomsInFrame)
					// Dan: `ValidatePlan` ignores `motionPlanRequest.WorldState`. Until I'm
					// convinced that only a single `obstacleAvoidance` loop needs to exist/modify
					// it.
					validateErr := armplanning.ValidatePlan(ctx, plan, motionPlanRequest, mergedWorldState, ms.logger)
					if validateErr != nil {
						return validateErr
					}

					time.Sleep(10 * time.Millisecond)
				}

				return nil
			})
		}

		for _, traj := range plan.Trajectory() {
			for actuatorName, inputs := range traj {
				if len(inputs) == 0 {
					continue
				}

				actuator, ok := ms.components[actuatorName]
				if !ok {
					return nil, fmt.Errorf("Actuator in plan to move does not exist. Name: %v", actuatorName)
				}

				ie, err := utils.AssertType[framesystem.InputEnabled](actuator)
				if err != nil {
					return nil, err
				}

				ms.logger.CDebugf(ctx, "Issuing GoToInputs. Actuator: %v Inputs: %v", actuatorName, inputs)
				if err = ie.GoToInputs(executeCtx, inputs); err != nil {
					return nil, err
				}
			}
		}

		if executeCtx.Err() == nil {
			// We've completed executing through `Trajectory` without being interrupted. We're at
			// our destination and can return success.
			return nil, nil
		}

		// If `executeCtx` had an error, the `obstacleAvoidance` background goroutine returned an
		// error. We do not need to explicitly `Wait` on it.
	}
}
