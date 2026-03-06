package builtin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.viam.com/rdk/components/arm/sim"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
	"golang.org/x/sync/errgroup"
	"gorgonia.org/tensor"
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
	for ctx.Err() == nil {
		// build maps of relevant components and inputs from initial inputs
		fsInputs, err := ms.fsService.CurrentInputs(ctx)
		if err != nil {
			return nil, err
		}

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
		ms.logger.CDebugf(ctx,
			"Replannable motion planning request. Start: %v Goal: %v NumObstacles: %v NumPlannedWaypoints: %v Err: %v",
			fsInputs[moveRequest.ComponentName], moveRequest.Destination, len(worldState.Obstacles()),
			plan.Trajectory(), err)
		if err != nil {
			return nil, err
		}

		trajTensors := map[string]*tensor.Dense{
			"waypoints_rads": plan.Trajectory().ToTrajectoryWaypointTensor("arm"),
			"velocity_limits_rads_per_sec": tensor.New(
				tensor.Of(tensor.Float64),
				tensor.WithShape(6),
				tensor.WithBacking([]float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2}),
			),
			"acceleration_limits_rads_per_sec2": tensor.New(
				tensor.Of(tensor.Float64),
				tensor.WithShape(6),
				// tensor.WithBacking([]float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2}),
				tensor.WithBacking([]float64{1, 1, 1, 1, 1, 1}),
			),
			"waypoint_deduplication_tolerance_rads": tensor.New(
				tensor.Of(tensor.Float64),
				tensor.WithShape(1),
				tensor.WithBacking([]float64{0}),
			),
			"path_tolerance_delta_rads": tensor.New(
				tensor.Of(tensor.Float64),
				tensor.WithShape(1),
				tensor.WithBacking([]float64{0}),
			),
			"path_colinearization_ratio": tensor.New(
				tensor.Of(tensor.Float64),
				tensor.WithShape(1),
				tensor.WithBacking([]float64{0}),
			),
			"trajectory_sampling_freq_hz": tensor.New(
				tensor.Of(tensor.Int64),
				tensor.WithShape(1),
				tensor.WithBacking([]int64{10}),
			),
		}

		trajRaw, err := ms.trajexService.Infer(ctx, trajTensors)
		if err != nil {
			ms.logger.Error("Panicing:", err)
			panic(err)
		}
		// ms.logger.Info("Map:", trajRaw)

		nSamples := trajRaw["sample_times_sec"].Shape()[0]
		times := trajRaw["sample_times_sec"].Data().([]float64)
		configsFlat := trajRaw["configurations_rads"].Data().([]float64)
		configsHydrated := make([][]referenceframe.Input, nSamples)
		for idx := range nSamples {
			step := make([]referenceframe.Input, 6)
			for jointIdx := range 6 {
				step[jointIdx] = configsFlat[idx*6+jointIdx]
			}
			configsHydrated[idx] = step
		}

		modifiedTrajectory := motionplan.Trajectory([]referenceframe.FrameSystemInputs{})
		for _, traj := range configsHydrated {
			modifiedTrajectory = append(modifiedTrajectory, referenceframe.FrameSystemInputs{
				"arm": traj,
			})
		}

		// ms.logger.Info("Samples:", nSamples, "Times:", times)
		// ms.logger.Info("Inputs:", configsHydrated)

		// While executing, the world state might change (an obstacle later comes into existence
		// that must be avoided). If an `obstacleVisionService` exists, set up a background
		// goroutine for polling the vision service for new obstacles. And validate if the executing
		// plan continues to be safe.
		//
		// If the plan is no longer safe, the `executeCtx` will be canceled. Stopping the
		// arm/actuator and allowing us to replan with the new information.
		obstacleAvoidance, executeCtx := errgroup.WithContext(ctx)
		// Dan: Calling `errgroup.Wait` will first wait on goroutines and _then_ cancel the
		// `executeCtx`. Which feels backwards. I would expect this use-case, the happy path for
		// some background goroutines are determined by context cancelation.
		executeCtx, executeDone := context.WithCancel(executeCtx)

		// Deferring this wait inside an outer loop looks wrong, but it's argued to be safe here. In
		// the error case where the loop tries again from the top, the errgroup will have already
		// been emptied. If the execution succeeds without a problem, the code will return, in which
		// case this `Wait` becomes meaningful.
		defer func() {
			executeDone()
			obstacleAvoidance.Wait()
		}()

		executeStart := time.Now()
		if obstacleVisionService != nil {
			obstacleAvoidance.Go(func() error {
				for executeCtx.Err() == nil {
					// TODO: Vision service objects are returned from the camera reference frame.
					geomsInFrame, err := getExtraObstacles(ctx, obstacleVisionService)
					if err != nil {
						ms.logger.Warnw("Getting extra obstacles error", "err", err)
						return err
					}

					motionPlanRequest.WorldState = baseWorldState.Merge(geomsInFrame)
					// Dan: Is it safe to overwrite the world state field on the motion plan
					// request? My expectation is that this goroutine must exit before the parent
					// goroutine will re-invoke `PlanMotion`. Can always make some shallow-ish copy,
					// but would prefer to avoid as that would be brittle.
					//
					// If the `motionPlanRequest` used `PlannerOptions.MeshesAsOctrees`, make sure
					// validation uses an octree representation. With the exception of new obstacles
					// found in the `mergedWorldState`.
					validateErr := armplanning.ValidatePlan(
						ctx, plan, time.Since(executeStart), times, configsHydrated, motionPlanRequest, ms.logger)
					if validateErr != nil {
						ms.logger.Infow("Validate plan returned error. Canceling execution.", "err", validateErr)
						return validateErr
					}

					time.Sleep(10 * time.Millisecond)
				}

				return nil
			})
		}

	executeLoop:
		for trajIdx, traj := range plan.Trajectory() {
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
				var moveErr error
				if simArm, ok := ie.(*sim.SimulatedArm); ok {
					if trajIdx == 0 {
						moveErr = simArm.OptimizedTrajectory(executeCtx, times, configsHydrated)
					} else {
						ms.logger.Info("Dan hack, already did full movement.")
					}
				} else {
					moveErr = ie.GoToInputs(executeCtx, inputs)
				}

				if moveErr != nil {
					if errors.Is(moveErr, context.Canceled) {
						break executeLoop
					} else {
						return nil, moveErr
					}
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

	// If the `executeCtx` was canceled because `ctx` was canceled. We return with an error.
	return nil, ctx.Err()
}
