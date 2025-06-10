package motionplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"go.viam.com/utils"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	defaultOptimalityMultiple      = 2.0
	defaultFallbackTimeout         = 1.5
	defaultTPspaceOrientationScale = 500.

	cbirrtName  = "cbirrt"
	rrtstarName = "rrtstar"
)

// planManager is intended to be the single entry point to motion planners, wrapping all others, dealing with fallbacks, etc.
// Intended information flow should be:
// motionplan.PlanMotion() -> SolvableFrameSystem.SolveWaypointsWithOptions() -> planManager.planSingleWaypoint().
type planManager struct {
	*planner                // TODO: This should probably be removed
	entireFS                referenceframe.FrameSystem
	entirePlanState         *PlanState
	activeBackgroundWorkers sync.WaitGroup
}

type atomicWaypoint struct {
	mp         motionPlanner
	startState *PlanState // A list of starting states, any of which would be valid to start from
	goalState  *PlanState // A list of goal states, any of which would be valid to arrive at

	// If partial plans are requested, we return up to the last explicit waypoint solved.
	// We want to distinguish between actual user-requested waypoints and automatically-generated intermediate waypoints, and only
	// consider the former when returning partial plans.
	origWP int
}

// planMultiWaypoint plans a motion through multiple waypoints, using identical constraints for each
// Any constraints, etc, will be held for the entire motion.
func (pm *planManager) planMultiWaypoint(ctx context.Context, request *PlanRequest, seedPlan Plan) (Plan, error) {
	request.Logger.Info("planMultiWaypoint start")
	defer request.Logger.Info("planMultiWaypoint end")

	opt, err := pm.plannerSetupFromMoveRequest(request, 0)
	if err != nil {
		return nil, err
	}
	pm.planOpts = opt
	if opt.useTPspace {
		return pm.planRelativeWaypoint(ctx, request, seedPlan, opt)
	}
	// Theoretically, a plan could be made between two poses, by running IK on both the start and end poses to create sets of seed and
	// goal configurations. However, the blocker here is the lack of a "known good" configuration used to determine which obstacles
	// are allowed to collide with one another.
	if request.StartState.configuration == nil {
		return nil, errors.New("must populate start state configuration if not planning for 2d base/tpspace")
	}

	// set timeout for entire planning process if specified
	var cancel func()
	if timeout, ok := request.Options["timeout"].(float64); ok {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout*float64(time.Second)))
	}
	if cancel != nil {
		defer cancel()
	}

	waypoints := []atomicWaypoint{}

	for i := range request.Goals {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		// Solving highly constrained motions by breaking apart into small pieces is much more performant
		goalWaypoints, err := pm.generateWaypoints(request, seedPlan, i)
		if err != nil {
			return nil, err
		}
		waypoints = append(waypoints, goalWaypoints...)
	}

	plan, err := pm.planAtomicWaypoints(ctx, waypoints, seedPlan)
	pm.activeBackgroundWorkers.Wait()
	if err != nil {
		if len(waypoints) > 1 {
			err = fmt.Errorf("failed to plan path for valid goal: %w", err)
		}
		return nil, err
	}
	return plan, nil
}

// planAtomicWaypoints will plan a single motion, which may be composed of one or more waypoints. Waypoints are here used to begin planning
// the next motion as soon as its starting point is known. This is responsible for repeatedly calling planSingleAtomicWaypoint for each
// intermediate waypoint. Waypoints here refer to points that the software has generated to.
func (pm *planManager) planAtomicWaypoints(
	ctx context.Context,
	waypoints []atomicWaypoint,
	seedPlan Plan,
) (Plan, error) {
	var err error
	// A resultPromise can be queried in the future and will eventually yield either a set of planner waypoints, or an error.
	// Each atomic waypoint produces one result promise, all of which are resolved at the end, allowing multiple to be solved in parallel.
	resultPromises := []*resultPromise{}

	var seed referenceframe.FrameSystemInputs
	var returnPartial bool
	var done bool

	// try to solve each goal, one at a time
	for i, wp := range waypoints {
		// Check if ctx is done between each waypoint
		select {
		case <-ctx.Done():
			if wp.mp.opt().ReturnPartialPlan {
				returnPartial = true
				done = true
				break // breaks out of select, then the `done` conditional below breaks the loop
			}
			return nil, ctx.Err()
		default:
		}
		if done {
			// breaks in the `select` above wil not break out of the loop, so we need to exit the select then break the loop if appropriate
			break
		}
		pm.logger.Info("planning step", i, "of", len(waypoints), ":", wp.goalState)
		for k, v := range wp.goalState.Poses() {
			pm.logger.Info(k, v)
		}

		var maps *rrtMaps
		if seedPlan != nil {
			maps, err = pm.planToRRTGoalMap(seedPlan, wp)
			if err != nil {
				if wp.mp.opt().ReturnPartialPlan {
					returnPartial = true
					break
				}
				return nil, err
			}
		}
		// If we don't pass in pre-made maps, initialize and seed with IK solutions here
		// TPspace should fill in its maps in planRelativeWaypoint and then call planSingleAtomicWaypoint directly so no need to
		// deal with that here.
		// TODO: Once TPspace also supports multiple waypoints, this needs to be updated.
		if !wp.mp.opt().useTPspace && maps == nil {
			if seed != nil {
				// If we have a seed, we are linking multiple waypoints, so the next one MUST start at the ending configuration of the last
				wp.startState = &PlanState{configuration: seed}
			}
			planSeed := initRRTSolutions(ctx, wp)
			if planSeed.err != nil {
				if wp.mp.opt().ReturnPartialPlan {
					returnPartial = true
					break
				}
				return nil, planSeed.err
			}
			if planSeed.steps != nil {
				resultPromises = append(resultPromises, &resultPromise{steps: planSeed.steps})
				seed = planSeed.steps[len(planSeed.steps)-1].Q()
				continue
			}
			maps = planSeed.maps
		}
		// Plan the single waypoint, and accumulate objects which will be used to constrauct the plan after all planning has finished
		newseed, future, err := pm.planSingleAtomicWaypoint(ctx, wp, maps)
		if err != nil {
			// Error getting the next seed. If we can, return the partial path if requested.
			if wp.mp.opt().ReturnPartialPlan {
				returnPartial = true
				break
			}
			return nil, err
		}
		seed = newseed
		resultPromises = append(resultPromises, future)
	}

	// All goals have been submitted for solving. Reconstruct in order
	resultSlices := []node{}
	partialSlices := []node{}

	// Keep track of which user-requested waypoints were solved for
	lastOrig := 0

	for i, future := range resultPromises {
		steps, err := future.result()
		if err != nil {
			if pm.opt().ReturnPartialPlan && lastOrig > 0 {
				returnPartial = true
				break
			}
			return nil, err
		}
		pm.logger.Debugf("completed planning for subwaypoint %d", i)
		if i > 0 {
			// Prevent doubled steps. The first step of each plan is the last step of the prior plan.
			partialSlices = append(partialSlices, steps[1:]...)
		} else {
			partialSlices = append(partialSlices, steps...)
		}
		if waypoints[i].origWP >= 0 {
			lastOrig = waypoints[i].origWP
			resultSlices = append(resultSlices, partialSlices...)
			partialSlices = []node{}
		}
	}
	if !waypoints[len(waypoints)-1].mp.opt().ReturnPartialPlan && len(partialSlices) > 0 {
		resultSlices = append(resultSlices, partialSlices...)
	}
	if returnPartial {
		pm.logger.Infof("returning partial plan up to waypoint %d", lastOrig)
	}

	// TODO: Once TPspace also supports multiple waypoints, this needs to be updated. For now it can be false.
	return newRRTPlan(resultSlices, pm.fs, false, nil)
}

// planSingleAtomicWaypoint attempts to plan a single waypoint. It may optionally be pre-seeded with rrt maps; these will be passed to the
// planner if supported, or ignored if not.
func (pm *planManager) planSingleAtomicWaypoint(
	ctx context.Context,
	wp atomicWaypoint,
	maps *rrtMaps,
) (referenceframe.FrameSystemInputs, *resultPromise, error) {
	fromPoses, err := wp.startState.ComputePoses(pm.fs)
	if err != nil {
		return nil, nil, err
	}
	toPoses, err := wp.goalState.ComputePoses(pm.fs)
	if err != nil {
		return nil, nil, err
	}
	pm.logger.Debug("start configuration", wp.startState.Configuration())
	pm.logger.Debug("start planning from\n", fromPoses, "\nto\n", toPoses)

	if _, ok := wp.mp.(rrtParallelPlanner); ok {
		// rrtParallelPlanner supports solution look-ahead for parallel waypoint solving
		// This will set that up, and if we get a result on `endpointPreview`, then the next iteration will be started, and the steps
		// for this solve will be rectified at the end.

		endpointPreview := make(chan node, 1)
		solutionChan := make(chan *rrtSolution, 1)
		pm.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer pm.activeBackgroundWorkers.Done()
			pm.planParallelRRTMotion(ctx, wp, endpointPreview, solutionChan, maps)
		})
		// We don't want to check context here; context cancellation will be handled by planParallelRRTMotion.
		// Instead, if a timeout occurs while we are smoothing, we want to return the best plan we have so far, rather than nothing at all.
		// This matches the behavior of a non-rrtParallelPlanner
		select {
		case nextSeed := <-endpointPreview:
			return nextSeed.Q(), &resultPromise{future: solutionChan}, nil
		case planReturn := <-solutionChan:
			if planReturn.err != nil {
				return nil, nil, planReturn.err
			}
			seed := planReturn.steps[len(planReturn.steps)-1].Q()
			return seed, &resultPromise{steps: planReturn.steps}, nil
		}
	} else {
		// This ctx is used exclusively for the running of the new planner and timing it out. It may be different from the main `ctx`
		// timeout due to planner fallbacks.
		plannerctx, cancel := context.WithTimeout(ctx, time.Duration(wp.mp.opt().Timeout*float64(time.Second)))
		defer cancel()
		plan, err := wp.mp.plan(plannerctx, wp.startState, wp.goalState)
		if err != nil {
			return nil, nil, err
		}

		smoothedPath := wp.mp.smoothPath(ctx, plan)

		// Update seed for the next waypoint to be the final configuration of this waypoint
		seed := smoothedPath[len(smoothedPath)-1].Q()
		return seed, &resultPromise{steps: smoothedPath}, nil
	}
}

// planParallelRRTMotion will handle planning a single atomic waypoint using a parallel-enabled RRT solver. It will handle fallbacks
// as necessary.
func (pm *planManager) planParallelRRTMotion(
	ctx context.Context,
	wp atomicWaypoint,
	endpointPreview chan node,
	solutionChan chan *rrtSolution,
	maps *rrtMaps,
) {
	pathPlanner := wp.mp.(rrtParallelPlanner)
	var rrtBackground sync.WaitGroup
	var err error
	if maps == nil {
		solutionChan <- &rrtSolution{err: errors.New("nil maps")}
		return
	}

	// publish endpoint of plan if it is known
	var nextSeed node
	if len(maps.goalMap) == 1 {
		for key := range maps.goalMap {
			nextSeed = key
		}
		if endpointPreview != nil {
			endpointPreview <- nextSeed
			endpointPreview = nil
		}
	}

	// This ctx is used exclusively for the running of the new planner and timing it out. It may be different from the main `ctx` timeout
	// due to planner fallbacks.
	plannerctx, cancel := context.WithTimeout(ctx, time.Duration(wp.mp.opt().Timeout*float64(time.Second)))
	defer cancel()

	plannerChan := make(chan *rrtSolution, 1)

	// start the planner
	rrtBackground.Add(1)
	utils.PanicCapturingGo(func() {
		defer rrtBackground.Done()
		pathPlanner.rrtBackgroundRunner(plannerctx, &rrtParallelPlannerShared{maps, endpointPreview, plannerChan})
	})

	// Wait for results from the planner. This will also handle calling the fallback if needed, and will ultimately return the best path
	select {
	case <-ctx.Done():
		// Error will be caught by monitoring loop
		rrtBackground.Wait()
		solutionChan <- &rrtSolution{err: ctx.Err()}
		return
	default:
	}

	select {
	case finalSteps := <-plannerChan:
		// We didn't get a solution preview (possible error), so we get and process the full step set and error.

		mapSeed := finalSteps.maps

		// Create fallback planner
		var fallbackPlanner motionPlanner
		if pathPlanner.opt().Fallback != nil {
			//nolint: gosec
			fallbackPlanner, err = pathPlanner.opt().Fallback.PlannerConstructor(
				pm.fs,
				rand.New(rand.NewSource(int64(pm.randseed.Int()))),
				pm.logger,
				pathPlanner.opt().Fallback,
			)
			if err != nil {
				fallbackPlanner = nil
			}
		}

		// If there was no error, check path quality. If sufficiently good, move on.
		// If there *was* an error, then either the fallback will not error and will replace it, or the error will be returned
		if finalSteps.err == nil {
			if fallbackPlanner != nil {
				if ok, score := pm.goodPlan(finalSteps, pathPlanner.opt()); ok {
					pm.logger.CDebugf(ctx, "got path with score %f, close enough to optimal %f", score, maps.optNode.Cost())
					fallbackPlanner = nil
				} else {
					pm.logger.CDebugf(ctx, "path with score %f not close enough to optimal %f, falling back", score, maps.optNode.Cost())

					// If we have a connected but bad path, we recreate new IK solutions and start from scratch
					// rather than seeding with a completed, known-bad tree
					mapSeed = nil
				}
			}
		}

		// Start smoothing before initializing the fallback plan. This allows both to run simultaneously.
		smoothChan := make(chan []node, 1)
		rrtBackground.Add(1)
		utils.PanicCapturingGo(func() {
			defer rrtBackground.Done()
			smoothChan <- pathPlanner.smoothPath(ctx, finalSteps.steps)
		})
		var alternateFuture *resultPromise

		// Run fallback only if we don't have a very good path
		if fallbackPlanner != nil {
			fallbackWP := wp
			fallbackWP.mp = fallbackPlanner
			_, alternateFuture, err = pm.planSingleAtomicWaypoint(ctx, fallbackWP, mapSeed)
			if err != nil {
				alternateFuture = nil
			}
		}

		// Receive the newly smoothed path from our original solve, and score it
		finalSteps.steps = <-smoothChan
		score := math.Inf(1)

		if finalSteps.steps != nil {
			score = nodesToTrajectory(finalSteps.steps).EvaluateCost(pathPlanner.opt().scoreFunc)
		}

		// If we ran a fallback, retrieve the result and compare to the smoothed path
		if alternateFuture != nil {
			alternate, err := alternateFuture.result()
			if err == nil {
				// If the fallback successfully found a path, check if it is better than our smoothed previous path.
				// The fallback should emerge pre-smoothed, so that should be a non-issue
				altCost := nodesToTrajectory(alternate).EvaluateCost(pathPlanner.opt().scoreFunc)
				if altCost < score {
					pm.logger.CDebugf(ctx, "replacing path with score %f with better score %f", score, altCost)
					finalSteps = &rrtSolution{steps: alternate}
				} else {
					pm.logger.CDebugf(ctx, "fallback path with score %f worse than original score %f; using original", altCost, score)
				}
			}
		}

		solutionChan <- finalSteps
		return

	case <-ctx.Done():
		rrtBackground.Wait()
		solutionChan <- &rrtSolution{err: ctx.Err()}
		return
	}
}

// This is where the map[string]interface{} passed in via `extra` is used to decide how planning happens.
func (pm *planManager) plannerSetupFromMoveRequest(req *PlanRequest, goalIndex int) (*plannerOptions, error) {
	var err error
	if req.Constraints == nil {
		// Constraints may be nil, but if a motion profile is set in planningOpts we need it to be a valid pointer to an empty struct.
		req.Constraints = &Constraints{}
	}
	planAlg := ""

	// Start with normal options
	opt := newBasicPlannerOptions()
	opt.extra = req.Options

	startPoses, err := req.StartState.ComputePoses(req.FrameSystem)
	if err != nil {
		return nil, err
	}
	to := req.Goals[goalIndex]
	goalPoses, err := to.ComputePoses(req.FrameSystem)
	if err != nil {
		return nil, err
	}

	if partial, ok := req.Options["return_partial_plan"]; ok {
		if use, ok := partial.(bool); ok && use {
			opt.ReturnPartialPlan = true
		}
	}

	collisionBufferMM := defaultCollisionBufferMM
	collisionBufferMMRaw, ok := req.Options["collision_buffer_mm"]
	if ok {
		collisionBufferMM, ok = collisionBufferMMRaw.(float64)
		if !ok {
			return nil, errors.New("could not interpret collision_buffer_mm field as float64")
		}
		if collisionBufferMM < 0 {
			return nil, errors.New("collision_buffer_mm can't be negative")
		}
	}

	err = opt.fillMotionChains(req.FrameSystem, to)
	if err != nil {
		return nil, err
	}
	if len(opt.motionChains) < 1 {
		return nil, errors.New("must have at least one motion chain")
	}
	// create motion chains for each goal, and error check for PTG frames
	// TODO: currently, if any motion chain has a PTG frame, that must be the only motion chain and that frame must be the only
	// frame in the chain with nonzero DoF. Eventually this need not be the case.
	for _, chain := range opt.motionChains {
		for _, movingFrame := range chain.frames {
			if _, isPTGframe := movingFrame.(tpspace.PTGProvider); isPTGframe {
				if opt.useTPspace {
					return nil, errors.New("only one PTG frame can be planned for at a time")
				}
				if len(opt.motionChains) > 1 {
					return nil, errMixedFrameTypes
				}
				opt.useTPspace = true
				opt.ptgFrameName = movingFrame.Name()
				chain.worldRooted = true
			} else if len(movingFrame.DoF()) > 0 {
				if opt.useTPspace {
					return nil, errMixedFrameTypes
				}
			}
		}
	}
	if opt.useTPspace {
		opt.Resolution = defaultPTGCollisionResolution
	}

	movingRobotGeometries := []spatialmath.Geometry{}

	// find all geometries that are not moving but are in the frame system
	staticRobotGeometries := []spatialmath.Geometry{}
	frameSystemGeometries, err := referenceframe.FrameSystemGeometries(pm.entireFS, pm.entirePlanState.configuration)
	if err != nil {
		return nil, err
	}
	for name, geometries := range frameSystemGeometries {
		moving := false
		for _, chain := range opt.motionChains {
			if chain.movingFS.Frame(name) != nil {
				moving = true
				movingRobotGeometries = append(movingRobotGeometries, geometries.Geometries()...)
				break
			}
		}
		if !moving {
			staticRobotGeometries = append(staticRobotGeometries, geometries.Geometries()...)
		}
	}

	rseed := defaultRandomSeed
	if seed, ok := req.Options["rseed"].(int); ok {
		rseed = seed
	}
	planningFS := req.FrameSystem
	if len(opt.motionChains) == 1 {
		// if there is only one chain no need to use the whole frame system as the planning frame
		planningFS = opt.motionChains[0].movingFS
		// need to update the start state to only contain what we care about
		req.StartState = req.StartState.Subset(planningFS.FrameNames())
	}

	//nolint: gosec
	planner, err := newPlanner(planningFS, rand.New(rand.NewSource(int64(rseed))), req.Logger, nil)
	if err != nil {
		return nil, err
	}
	pm.planner = planner

	// Note that all obstacles in worldState are assumed to be static so it is ok to transform them into the world frame
	// TODO(rb) it is bad practice to assume that the current inputs of the robot correspond to the passed in world state
	// the state that observed the worldState should ultimately be included as part of the worldState message
	obstaclesInFrame, err := req.WorldState.ObstaclesInWorldFrame(pm.fs, req.StartState.configuration)
	if err != nil {
		return nil, err
	}
	worldGeometries := obstaclesInFrame.Geometries()

	frameNames := map[string]bool{}
	for _, fName := range pm.fs.FrameNames() {
		frameNames[fName] = true
	}

	allowedCollisions, err := collisionSpecifications(
		req.Constraints.GetCollisionSpecification(),
		frameSystemGeometries,
		frameNames,
		req.WorldState.ObstacleNames(),
	)
	if err != nil {
		return nil, err
	}

	// If we are planning on a SLAM map we want to not allow a collision with the pointcloud to start our move call
	// Typically starting collisions are whitelisted,
	// TODO: This is not the most robust way to deal with this but is better than driving through walls.
	if opt.useTPspace {
		var zeroCG *collisionGraph
		for _, geom := range worldGeometries {
			if octree, ok := geom.(*pointcloud.BasicOctree); ok {
				if zeroCG == nil {
					zeroCG, err = setupZeroCG(movingRobotGeometries, worldGeometries, allowedCollisions, false, collisionBufferMM)
					if err != nil {
						return nil, err
					}
				}
				// Check if a moving geometry is in collision with a pointcloud. If so, error.
				for _, collision := range zeroCG.collisions(collisionBufferMM) {
					if collision.name1 == octree.Label() {
						return nil, fmt.Errorf("starting collision between SLAM map and %s, cannot move", collision.name2)
					} else if collision.name2 == octree.Label() {
						return nil, fmt.Errorf("starting collision between SLAM map and %s, cannot move", collision.name1)
					}
				}
			}
		}
	}

	// add collision constraints
	fsCollisionConstraints, stateCollisionConstraints, err := createAllCollisionConstraints(
		movingRobotGeometries,
		staticRobotGeometries,
		worldGeometries,
		req.BoundingRegions,
		allowedCollisions,
		collisionBufferMM,
	)
	if err != nil {
		return nil, err
	}
	// For TPspace
	for name, constraint := range stateCollisionConstraints {
		opt.AddStateConstraint(name, constraint)
	}
	for name, constraint := range fsCollisionConstraints {
		opt.AddStateFSConstraint(name, constraint)
	}

	// error handling around extracting motion_profile information from map[string]interface{}
	var motionProfile string
	profile, ok := req.Options["motion_profile"]
	if ok {
		motionProfile, ok = profile.(string)
		if !ok {
			return nil, errors.New("could not interpret motion_profile field as string")
		}
	}

	opt.profile = FreeMotionProfile
	switch motionProfile {
	case LinearMotionProfile:
		opt.profile = LinearMotionProfile
		// Linear constraints
		linTol, ok := req.Options["line_tolerance"].(float64)
		if !ok {
			// Default
			linTol = defaultLinearDeviation
		}
		orientTol, ok := req.Options["orient_tolerance"].(float64)
		if !ok {
			// Default
			orientTol = defaultOrientationDeviation
		}
		req.Constraints.AddLinearConstraint(LinearConstraint{linTol, orientTol})
	case PseudolinearMotionProfile:
		opt.profile = PseudolinearMotionProfile
		tolerance, ok := req.Options["tolerance"].(float64)
		if !ok {
			// Default
			tolerance = defaultPseudolinearTolerance
		}
		req.Constraints.AddPseudolinearConstraint(PseudolinearConstraint{tolerance, tolerance})
	case OrientationMotionProfile:
		opt.profile = OrientationMotionProfile
		tolerance, ok := req.Options["tolerance"].(float64)
		if !ok {
			// Default
			tolerance = defaultOrientationDeviation
		}
		req.Constraints.AddOrientationConstraint(OrientationConstraint{tolerance})
	case PositionOnlyMotionProfile:
		opt.profile = PositionOnlyMotionProfile
		if !opt.useTPspace || opt.PositionSeeds <= 0 {
			opt.goalMetricConstructor = ik.NewPositionOnlyMetric
		}
	}

	hasTopoConstraint, err := opt.addTopoConstraints(pm.fs, req.StartState.configuration, startPoses, goalPoses, req.Constraints)
	if err != nil {
		return nil, err
	}
	// convert map to json, then to a struct, overwriting present defaults
	jsonString, err := json.Marshal(req.Options)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonString, opt)
	if err != nil {
		return nil, err
	}

	alg, ok := req.Options["planning_alg"]
	if ok {
		planAlg, ok = alg.(string)
		if !ok {
			return nil, errors.New("could not interpret planning_alg field as string")
		}
	}
	if opt.useTPspace && planAlg != "" {
		return nil, fmt.Errorf("cannot specify a planning_alg when planning for a TP-space frame. alg specified was %s", planAlg)
	}
	if hasTopoConstraint {
		if planAlg != "" && planAlg != cbirrtName {
			return nil, fmt.Errorf("cannot specify a planning alg other than cbirrt with topo constraints. alg specified was %s", planAlg)
		}
		planAlg = cbirrtName
	}
	switch planAlg {
	case cbirrtName:
		opt.PlannerConstructor = newCBiRRTMotionPlanner
	case rrtstarName:
		opt.PlannerConstructor = newRRTStarConnectMotionPlanner
	default:
		// use default, already set
	}
	if opt.useTPspace {
		// overwrite default with TP space
		opt.PlannerConstructor = newTPSpaceMotionPlanner

		// Distances are computed in cartesian space rather than configuration space.
		opt.poseDistanceFunc = ik.NewSquaredNormSegmentMetric(defaultTPspaceOrientationScale)
		// If we have PTGs, then we calculate distances using the PTG-specific distance function.
		// Otherwise we just use squared norm on inputs.
		opt.scoreFunc = tpspace.NewPTGDistanceMetric([]string{opt.ptgFrameName})

		planAlg = "tpspace"
	}

	if opt.profile == FreeMotionProfile || opt.profile == PositionOnlyMotionProfile {
		if planAlg == "" {
			// set up deep copy for fallback
			try1 := deepAtomicCopyMap(req.Options)
			// No need to generate tons more IK solutions when the first alg will do it

			// time to run the first planning attempt before falling back
			try1["timeout"] = defaultFallbackTimeout
			try1["planning_alg"] = "rrtstar"
			try1Opt, err := pm.plannerSetupFromMoveRequest(&PlanRequest{
				Logger:          req.Logger,
				Goals:           []*PlanState{to},
				FrameSystem:     pm.fs,
				StartState:      req.StartState,
				WorldState:      req.WorldState,
				BoundingRegions: req.BoundingRegions,
				Constraints:     req.Constraints,
				Options:         try1,
			}, 0)
			if err != nil {
				return nil, err
			}

			try1Opt.Fallback = opt
			opt = try1Opt
		}
	}

	return opt, nil
}

// generateWaypoints will return the list of atomic waypoints that correspond to a specific goal in a plan request.
func (pm *planManager) generateWaypoints(request *PlanRequest, seedPlan Plan, wpi int) ([]atomicWaypoint, error) {
	wpGoals := request.Goals[wpi]
	startState := request.StartState
	if wpi > 0 {
		startState = request.Goals[wpi-1]
	}

	startPoses, err := startState.ComputePoses(pm.fs)
	if err != nil {
		return nil, err
	}
	goalPoses, err := wpGoals.ComputePoses(pm.fs)
	if err != nil {
		return nil, err
	}

	subWaypoints := useSubWaypoints(request, seedPlan, wpi)
	opt, err := pm.plannerSetupFromMoveRequest(&PlanRequest{
		Logger:          request.Logger,
		Goals:           []*PlanState{wpGoals},
		FrameSystem:     pm.fs,
		StartState:      request.StartState,
		WorldState:      request.WorldState,
		BoundingRegions: request.BoundingRegions,
		Constraints:     request.Constraints,
		Options:         request.Options,
	}, 0)

	if err != nil {
		return nil, err
	}
	if wpGoals.poses != nil {
		// Transform goal poses into world frame if needed if a component's goal is given in terms of itself.
		for fName, pf := range wpGoals.poses {
			if fName == pf.Parent() {
				alteredGoals, err := alterGoals(opt.motionChains, pm.fs, request.StartState.configuration, wpGoals)
				if err != nil {
					return nil, err
				}
				wpGoals = alteredGoals
				break
			}
		}

		// Regenerate opts since our metrics will have changed
		opt, err = pm.plannerSetupFromMoveRequest(&PlanRequest{
			Logger:          request.Logger,
			Goals:           []*PlanState{wpGoals},
			FrameSystem:     pm.fs,
			StartState:      request.StartState,
			WorldState:      request.WorldState,
			BoundingRegions: request.BoundingRegions,
			Constraints:     request.Constraints,
			Options:         request.Options,
		}, 0)
		if err != nil {
			return nil, err
		}
	}

	// TPspace should never use subwaypoints
	if !subWaypoints || opt.useTPspace {
		//nolint: gosec
		pathPlanner, err := opt.PlannerConstructor(
			pm.fs,
			rand.New(rand.NewSource(int64(pm.randseed.Int()))),
			pm.logger,
			opt,
		)
		if err != nil {
			return nil, err
		}
		return []atomicWaypoint{{mp: pathPlanner, startState: request.StartState, goalState: wpGoals, origWP: wpi}}, nil
	}

	stepSize, ok := request.Options["path_step_size"].(float64)
	if !ok {
		stepSize = defaultStepSizeMM
	}

	numSteps := 0
	for frame, pif := range goalPoses {
		// Calculate steps needed for this frame
		steps := CalculateStepCount(startPoses[frame].Pose(), pif.Pose(), stepSize)
		if steps > numSteps {
			numSteps = steps
		}
	}

	from := startState
	waypoints := []atomicWaypoint{}
	for i := 1; i <= numSteps; i++ {
		by := float64(i) / float64(numSteps)
		to := &PlanState{referenceframe.FrameSystemPoses{}, referenceframe.FrameSystemInputs{}}
		if wpGoals.poses != nil {
			for frameName, pif := range wpGoals.poses {
				toPose := spatialmath.Interpolate(startPoses[frameName].Pose(), pif.Pose(), by)
				to.poses[frameName] = referenceframe.NewPoseInFrame(pif.Parent(), toPose)
			}
		}
		if wpGoals.configuration != nil {
			for frameName, inputs := range wpGoals.configuration {
				frame := pm.fs.Frame(frameName)
				// If subWaypoints was true, then StartState had a configuration, and if our goal does, so will `from`
				toInputs, err := frame.Interpolate(from.configuration[frameName], inputs, by)
				if err != nil {
					return nil, err
				}
				to.configuration[frameName] = toInputs
			}
		}
		wpOpt, err := pm.plannerSetupFromMoveRequest(&PlanRequest{
			Logger:          request.Logger,
			Goals:           []*PlanState{to},
			FrameSystem:     pm.fs,
			StartState:      request.StartState,
			WorldState:      request.WorldState,
			BoundingRegions: request.BoundingRegions,
			Constraints:     request.Constraints,
			Options:         request.Options,
		}, 0)
		if err != nil {
			return nil, err
		}
		//nolint: gosec
		pathPlanner, err := wpOpt.PlannerConstructor(
			pm.fs,
			rand.New(rand.NewSource(int64(pm.randseed.Int()))),
			pm.logger,
			wpOpt,
		)
		if err != nil {
			return nil, err
		}
		waypoints = append(waypoints, atomicWaypoint{mp: pathPlanner, startState: from, goalState: to, origWP: -1})

		from = to
	}
	waypoints[len(waypoints)-1].origWP = wpi

	return waypoints, nil
}

// check whether the solution is within some amount of the optimal.
func (pm *planManager) goodPlan(pr *rrtSolution, opt *plannerOptions) (bool, float64) {
	solutionCost := math.Inf(1)
	if pr.steps != nil {
		if pr.maps.optNode.Cost() <= 0 {
			return true, solutionCost
		}
		solutionCost = nodesToTrajectory(pr.steps).EvaluateCost(opt.scoreFunc)
		if solutionCost < pr.maps.optNode.Cost()*defaultOptimalityMultiple {
			return true, solutionCost
		}
	}

	return false, solutionCost
}

func (pm *planManager) planToRRTGoalMap(plan Plan, goal atomicWaypoint) (*rrtMaps, error) {
	traj := plan.Trajectory()
	path := plan.Path()
	if len(traj) != len(path) {
		return nil, errors.New("plan trajectory and path should be the same length")
	}
	planNodes := make([]node, 0, len(traj))
	for i, tConf := range traj {
		planNodes = append(planNodes, &basicNode{q: tConf, poses: path[i]})
	}

	if goal.mp.opt().useTPspace {
		// Fill in positions from the old origin to where the goal was during the last run
		planNodesOld, err := rectifyTPspacePath(planNodes, pm.fs.Frame(goal.mp.opt().ptgFrameName), spatialmath.NewZeroPose())
		if err != nil {
			return nil, err
		}

		// Figure out where our new starting point is relative to our last one, and re-rectify using the new adjusted location
		oldGoal := planNodesOld[len(planNodesOld)-1].Poses()[goal.mp.opt().ptgFrameName].Pose()
		pathDiff := spatialmath.PoseBetween(oldGoal, goal.goalState.poses[goal.mp.opt().ptgFrameName].Pose())
		planNodes, err = rectifyTPspacePath(planNodes, pm.fs.Frame(goal.mp.opt().ptgFrameName), pathDiff)
		if err != nil {
			return nil, err
		}
	}

	var lastNode node
	goalMap := map[node]node{}
	for i := len(planNodes) - 1; i >= 0; i-- {
		if i != 0 {
			// Fill in costs
			cost := goal.mp.opt().configurationDistanceFunc(&ik.SegmentFS{
				StartConfiguration: planNodes[i-1].Q(),
				EndConfiguration:   planNodes[i].Q(),
				FS:                 pm.fs,
			})
			planNodes[i].SetCost(cost)
		}
		goalMap[planNodes[i]] = lastNode
		lastNode = planNodes[i]
	}

	maps := &rrtMaps{
		startMap: map[node]node{},
		goalMap:  goalMap,
	}

	return maps, nil
}

// planRelativeWaypoint will solve the PTG frame to one individual pose. This is used for frames whose inputs are relative, that
// is, the pose returned by `Transform` is a transformation rather than an absolute position.
func (pm *planManager) planRelativeWaypoint(ctx context.Context, request *PlanRequest, seedPlan Plan, opt *plannerOptions) (Plan, error) {
	if request.StartState.poses == nil {
		return nil, errors.New("must provide a startPose if solving for PTGs")
	}
	if len(request.Goals) != 1 {
		return nil, errors.New("can only provide one goal if solving for PTGs")
	}
	startPose := request.StartState.poses[opt.ptgFrameName].Pose()
	goalPif := request.Goals[0].poses[opt.ptgFrameName]

	request.Logger.CInfof(ctx,
		"planning relative motion for frame %s\nGoal: %v\nstartPose %v\n, worldstate: %v\n",
		opt.ptgFrameName,
		referenceframe.PoseInFrameToProtobuf(goalPif),
		startPose,
		request.WorldState.String(),
	)

	if pathdebug {
		pm.logger.Debug("$type,X,Y")
		pm.logger.Debugf("$SG,%f,%f", startPose.Point().X, startPose.Point().Y)
		pm.logger.Debugf("$SG,%f,%f", goalPif.Pose().Point().X, goalPif.Pose().Point().Y)
		gifs, err := request.WorldState.ObstaclesInWorldFrame(pm.fs, request.StartState.configuration)
		if err == nil {
			for _, geom := range gifs.Geometries() {
				pts := geom.ToPoints(1.)
				for _, pt := range pts {
					if math.Abs(pt.Z) < 0.1 {
						pm.logger.Debugf("$OBS,%f,%f", pt.X, pt.Y)
					}
				}
			}
		}
	}

	var cancel func()
	// set timeout for entire planning process if specified
	if timeout, ok := request.Options["timeout"].(float64); ok {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout*float64(time.Second)))
	}
	if cancel != nil {
		defer cancel()
	}

	// Create frame system subset using frame name
	relativeOnlyFS, err := pm.fs.FrameSystemSubset(pm.fs.Frame(opt.ptgFrameName))
	if err != nil {
		return nil, err
	}
	pm.fs = relativeOnlyFS

	wps, err := pm.generateWaypoints(request, seedPlan, 0)
	if err != nil {
		return nil, err
	}
	// Should never happen, but checking to guard against future changes breaking this
	if len(wps) != 1 {
		return nil, fmt.Errorf("tpspace should only generate exactly one atomic waypoint, but got %d", len(wps))
	}
	wp := wps[0]

	zeroInputs := referenceframe.FrameSystemInputs{}
	zeroInputs[opt.ptgFrameName] = make([]referenceframe.Input, len(pm.fs.Frame(opt.ptgFrameName).DoF()))
	maps := &rrtMaps{}
	if seedPlan != nil {
		// TODO: This probably needs to be flipped? Check if these paths are ever used.
		maps, err = pm.planToRRTGoalMap(seedPlan, wp)
		if err != nil {
			return nil, err
		}
	}
	if opt.PositionSeeds > 0 && opt.profile == PositionOnlyMotionProfile {
		err = maps.fillPosOnlyGoal(wp.goalState.poses, opt.PositionSeeds)
		if err != nil {
			return nil, err
		}
	} else {
		goalPose := wp.goalState.poses[opt.ptgFrameName].Pose()
		goalMapFlip := map[string]*referenceframe.PoseInFrame{
			opt.ptgFrameName: referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.Compose(goalPose, flipPose)),
		}
		goalNode := &basicNode{q: zeroInputs, poses: goalMapFlip}
		maps.goalMap = map[node]node{goalNode: nil}
	}
	startNode := &basicNode{q: zeroInputs, poses: request.StartState.poses}
	maps.startMap = map[node]node{startNode: nil}

	// Plan the single waypoint, and accumulate objects which will be used to constrauct the plan after all planning has finished
	_, future, err := pm.planSingleAtomicWaypoint(ctx, wp, maps)
	if err != nil {
		return nil, err
	}
	steps, err := future.result()
	if err != nil {
		return nil, err
	}

	return newRRTPlan(steps, pm.fs, opt.useTPspace, startPose)
}

// Copy any atomic values.
func deepAtomicCopyMap(opt map[string]interface{}) map[string]interface{} {
	optCopy := map[string]interface{}{}
	for k, v := range opt {
		optCopy[k] = v
	}
	return optCopy
}

func nodesToTrajectory(nodes []node) Trajectory {
	traj := make(Trajectory, 0, len(nodes))
	for _, n := range nodes {
		traj = append(traj, n.Q())
	}
	return traj
}

// Determines whether to break a motion down into sub-waypoints if all intermediate points are known.
func useSubWaypoints(request *PlanRequest, seedPlan Plan, wpi int) bool {
	// If we are seeding off of a pre-existing plan, we don't need the speedup of subwaypoints
	if seedPlan != nil {
		return false
	}
	// If goal has a configuration, do not use subwaypoints *unless* the start state is also a configuration.
	// We can interpolate from a pose or configuration to a pose, or a configuration to a configuration, but not from a pose to a
	// configuration.
	// TODO: If we run planning backwards, we could remove this restriction.
	if request.Goals[wpi].configuration != nil {
		startState := request.StartState
		if wpi > 0 {
			startState = request.Goals[wpi-1]
		}
		if startState.configuration == nil {
			return false
		}
	}

	// linear motion profile has known intermediate points, so solving can be broken up and sped up
	if profile, ok := request.Options["motion_profile"]; ok && profile == LinearMotionProfile {
		return true
	}

	if len(request.Constraints.GetLinearConstraint()) > 0 {
		return true
	}
	return false
}
