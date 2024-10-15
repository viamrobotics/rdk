//go:build !no_cgo

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

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/motionplan/tpspace"
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
	*planner
	frame                   *solverFrame
	activeBackgroundWorkers sync.WaitGroup

	useTPspace bool
}

func newPlanManager(
	frame *solverFrame,
	logger logging.Logger,
	seed int,
) (*planManager, error) {
	//nolint: gosec
	p, err := newPlanner(frame, rand.New(rand.NewSource(int64(seed))), logger, newBasicPlannerOptions(frame))
	if err != nil {
		return nil, err
	}
	return &planManager{planner: p, frame: frame, useTPspace: len(frame.PTGSolvers()) > 0}, nil
}

// PlanMultiWaypoint plans a motion through multiple waypoints, using identical constraints for each
// Unlike PlanSingleWaypoint, this does not break up individual.
func (pm *planManager) PlanMultiWaypoint(ctx context.Context, request *PlanRequest, goals []spatialmath.Pose) (Plan, error) {
	if pm.useTPspace {
		return nil, errors.New("TPspace does not support multi-waypoint planning")
	}

	if request.StartPose != nil {
		request.Logger.Warn("plan request passed a start pose, but non-relative plans will use the pose from transforming StartConfiguration")
	}
	seed, err := pm.frame.mapToSlice(request.StartConfiguration)
	if err != nil {
		return nil, err
	}
	startPose, err := pm.frame.Transform(seed)
	if err != nil {
		return nil, err
	}

	var cancel func()

	request.Logger.CInfof(ctx,
		"planning motion for frame %s\nGoal: %v\nStarting seed map %v\n, startPose %v\n, worldstate: %v\n",
		request.Frame.Name(),
		referenceframe.PoseInFrameToProtobuf(request.Goal),
		request.StartConfiguration,
		spatialmath.PoseToProtobuf(startPose),
		request.WorldState.String(),
	)

	// set timeout for entire planning process if specified
	if timeout, ok := request.Options["timeout"].(float64); ok {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout*float64(time.Second)))
	}
	if cancel != nil {
		defer cancel()
	}

	goalPos := request.Goal.Pose()
	// If we are world rooted, translate the goal pose into the world frame
	if pm.frame.worldRooted {
		tf, err := pm.frame.fss.Transform(request.StartConfiguration, request.Goal, referenceframe.World)
		if err != nil {
			return nil, err
		}
		goalPos = tf.(*referenceframe.PoseInFrame).Pose()
	}
	goals = append(goals, goalPos)

	planners := make([]motionPlanner, len(goals))
	for i, goal := range goals {
		opt, err := pm.plannerSetupFromMoveRequest(
			startPose,
			goal,
			request.StartConfiguration,
			request.WorldState,
			request.BoundingRegions,
			request.Constraints,
			request.Options,
		)
		if err != nil {
			return nil, err
		}
		opt.SetGoal(goal)

		//nolint: gosec
		pathPlanner, err := opt.PlannerConstructor(
			pm.frame,
			rand.New(rand.NewSource(int64(pm.randseed.Int()))),
			pm.logger,
			opt,
		)
		if err != nil {
			return nil, err
		}
		planners[i] = pathPlanner

		startPose = goal // Update startPose for the next iteration
	}

	plan, err := pm.planAtomicWaypoints(ctx, goals, seed, planners, nil)
	pm.activeBackgroundWorkers.Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to plan path for valid goals: %w", err)
	}

	return plan, nil
}

// PlanSingleWaypoint will solve the solver frame to one individual pose. If you have multiple waypoints to hit, call this multiple times.
// Any constraints, etc, will be held for the entire motion.
func (pm *planManager) PlanSingleWaypoint(ctx context.Context, request *PlanRequest, seedPlan Plan) (Plan, error) {
	if pm.useTPspace {
		return pm.planRelativeWaypoint(ctx, request, seedPlan)
	}

	if request.StartPose != nil {
		request.Logger.Warn("plan request passed a start pose, but non-relative plans will use the pose from transforming StartConfiguration")
	}
	seed, err := pm.frame.mapToSlice(request.StartConfiguration)
	if err != nil {
		return nil, err
	}
	startPose, err := pm.frame.Transform(seed)
	if err != nil {
		return nil, err
	}
	goalPos := request.Goal.Pose()

	var cancel func()

	request.Logger.CInfof(ctx,
		"planning motion for frame %s\nGoal: %v\nStarting seed map %v\n, startPose %v\n, worldstate: %v\n",
		request.Frame.Name(),
		referenceframe.PoseInFrameToProtobuf(request.Goal),
		request.StartConfiguration,
		spatialmath.PoseToProtobuf(startPose),
		request.WorldState.String(),
	)

	// set timeout for entire planning process if specified
	if timeout, ok := request.Options["timeout"].(float64); ok {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout*float64(time.Second)))
	}
	if cancel != nil {
		defer cancel()
	}

	// If we are world rooted, translate the goal pose into the world frame
	if pm.frame.worldRooted {
		tf, err := pm.frame.fss.Transform(request.StartConfiguration, request.Goal, referenceframe.World)
		if err != nil {
			return nil, err
		}
		goalPos = tf.(*referenceframe.PoseInFrame).Pose()
	}

	var goals []spatialmath.Pose
	var opts []*plannerOptions

	subWaypoints := false

	// linear motion profile has known intermediate points, so solving can be broken up and sped up
	if profile, ok := request.Options["motion_profile"]; ok && profile == LinearMotionProfile {
		subWaypoints = true
	}

	if len(request.Constraints.GetLinearConstraint()) > 0 {
		subWaypoints = true
	}

	// If we are seeding off of a pre-existing plan, we don't need the speedup of subwaypoints
	if seedPlan != nil {
		subWaypoints = false
	}

	if subWaypoints {
		pathStepSize, ok := request.Options["path_step_size"].(float64)
		if !ok {
			pathStepSize = defaultPathStepSize
		}
		numSteps := PathStepCount(startPose, goalPos, pathStepSize)

		from := startPose
		for i := 1; i < numSteps; i++ {
			by := float64(i) / float64(numSteps)
			to := spatialmath.Interpolate(startPose, goalPos, by)
			goals = append(goals, to)
			opt, err := pm.plannerSetupFromMoveRequest(
				from,
				to,
				request.StartConfiguration,
				request.WorldState,
				request.BoundingRegions,
				request.Constraints,
				request.Options,
			)
			if err != nil {
				return nil, err
			}
			opt.SetGoal(to)
			opts = append(opts, opt)

			from = to
		}
		startPose = from
	}
	goals = append(goals, goalPos)
	opt, err := pm.plannerSetupFromMoveRequest(
		startPose,
		goalPos,
		request.StartConfiguration,
		request.WorldState,
		request.BoundingRegions,
		request.Constraints,
		request.Options,
	)
	if err != nil {
		return nil, err
	}
	pm.planOpts = opt
	opt.SetGoal(goalPos)
	opts = append(opts, opt)

	planners := make([]motionPlanner, 0, len(opts))
	// Set up planners for later execution
	for _, opt := range opts {
		// Build planner
		//nolint: gosec
		pathPlanner, err := opt.PlannerConstructor(
			pm.frame,
			rand.New(rand.NewSource(int64(pm.randseed.Int()))),
			pm.logger,
			opt,
		)
		if err != nil {
			return nil, err
		}
		planners = append(planners, pathPlanner)
	}

	// If we have multiple sub-waypoints, make sure the final goal is not unreachable.
	if len(goals) > 1 {
		// Viability check; ensure that the waypoint is not impossible to reach
		_, err = planners[0].getSolutions(ctx, seed)
		if err != nil {
			return nil, err
		}
	}

	plan, err := pm.planAtomicWaypoints(ctx, goals, seed, planners, seedPlan)
	pm.activeBackgroundWorkers.Wait()
	if err != nil {
		if len(goals) > 1 {
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
	goals []spatialmath.Pose,
	seed []referenceframe.Input,
	planners []motionPlanner,
	seedPlan Plan,
) (Plan, error) {
	var err error
	// A resultPromise can be queried in the future and will eventually yield either a set of planner waypoints, or an error.
	// Each atomic waypoint produces one result promise, all of which are resolved at the end, allowing multiple to be solved in parallel.
	resultPromises := []*resultPromise{}

	// try to solve each goal, one at a time
	for i, goal := range goals {
		pm.logger.Debug("start planning for ", spatialmath.PoseToProtobuf(goal))
		// Check if ctx is done between each waypoint
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		pathPlanner := planners[i]

		var maps *rrtMaps
		if seedPlan != nil {
			maps, err = pm.planToRRTGoalMap(seedPlan, goal)
			if err != nil {
				return nil, err
			}
		}
		// Plan the single waypoint, and accumulate objects which will be used to constrauct the plan after all planning has finished
		newseed, future, err := pm.planSingleAtomicWaypoint(ctx, goal, seed, pathPlanner, maps)
		if err != nil {
			return nil, err
		}
		seed = newseed
		resultPromises = append(resultPromises, future)
	}

	// All goals have been submitted for solving. Reconstruct in order
	resultSlices := []node{}
	for i, future := range resultPromises {
		steps, err := future.result()
		if err != nil {
			return nil, err
		}
		pm.logger.Debug("completed planning for ", spatialmath.PoseToProtobuf(goals[i]))
		resultSlices = append(resultSlices, steps...)
	}

	return newRRTPlan(resultSlices, pm.frame, pm.useTPspace)
}

// planSingleAtomicWaypoint attempts to plan a single waypoint. It may optionally be pre-seeded with rrt maps; these will be passed to the
// planner if supported, or ignored if not.
func (pm *planManager) planSingleAtomicWaypoint(
	ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
	pathPlanner motionPlanner,
	maps *rrtMaps,
) ([]referenceframe.Input, *resultPromise, error) {
	if parPlan, ok := pathPlanner.(rrtParallelPlanner); ok {
		// rrtParallelPlanner supports solution look-ahead for parallel waypoint solving
		// This will set that up, and if we get a result on `endpointPreview`, then the next iteration will be started, and the steps
		// for this solve will be rectified at the end.
		endpointPreview := make(chan node, 1)
		solutionChan := make(chan *rrtSolution, 1)
		pm.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer pm.activeBackgroundWorkers.Done()
			pm.planParallelRRTMotion(ctx, goal, seed, parPlan, endpointPreview, solutionChan, maps)
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
		plannerctx, cancel := context.WithTimeout(ctx, time.Duration(pathPlanner.opt().Timeout*float64(time.Second)))
		defer cancel()
		nodes, err := pathPlanner.plan(plannerctx, goal, seed)
		if err != nil {
			return nil, nil, err
		}

		smoothedPath := pathPlanner.smoothPath(ctx, nodes)

		// Update seed for the next waypoint to be the final configuration of this waypoint
		seed = smoothedPath[len(smoothedPath)-1].Q()
		return seed, &resultPromise{steps: smoothedPath}, nil
	}
}

// planParallelRRTMotion will handle planning a single atomic waypoint using a parallel-enabled RRT solver. It will handle fallbacks
// as necessary.
func (pm *planManager) planParallelRRTMotion(
	ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
	pathPlanner rrtParallelPlanner,
	endpointPreview chan node,
	solutionChan chan *rrtSolution,
	maps *rrtMaps,
) {
	var rrtBackground sync.WaitGroup
	var err error
	// If we don't pass in pre-made maps, initialize and seed with IK solutions here
	if !pm.useTPspace && maps == nil {
		planSeed := initRRTSolutions(ctx, pathPlanner, seed)
		if planSeed.err != nil || planSeed.steps != nil {
			solutionChan <- planSeed
			return
		}
		maps = planSeed.maps
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
	plannerctx, cancel := context.WithTimeout(ctx, time.Duration(pathPlanner.opt().Timeout*float64(time.Second)))
	defer cancel()

	plannerChan := make(chan *rrtSolution, 1)

	// start the planner
	rrtBackground.Add(1)
	utils.PanicCapturingGo(func() {
		defer rrtBackground.Done()
		pathPlanner.rrtBackgroundRunner(plannerctx, seed, &rrtParallelPlannerShared{maps, endpointPreview, plannerChan})
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
				pm.frame,
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
				if ok, score := pm.goodPlan(finalSteps, pm.opt()); ok {
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
			_, alternateFuture, err = pm.planSingleAtomicWaypoint(
				ctx,
				goal,
				seed,
				fallbackPlanner,
				mapSeed,
			)
			if err != nil {
				alternateFuture = nil
			}
		}

		// Receive the newly smoothed path from our original solve, and score it
		finalSteps.steps = <-smoothChan
		score := math.Inf(1)
		if finalSteps.steps != nil {
			score = pm.frame.nodesToTrajectory(finalSteps.steps).EvaluateCost(pm.opt().ScoreFunc)
		}

		// If we ran a fallback, retrieve the result and compare to the smoothed path
		if alternateFuture != nil {
			alternate, err := alternateFuture.result()
			if err == nil {
				// If the fallback successfully found a path, check if it is better than our smoothed previous path.
				// The fallback should emerge pre-smoothed, so that should be a non-issue
				altCost := pm.frame.nodesToTrajectory(alternate).EvaluateCost(pm.opt().ScoreFunc)
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
func (pm *planManager) plannerSetupFromMoveRequest(
	from, to spatialmath.Pose,
	seedMap map[string][]referenceframe.Input,
	worldState *referenceframe.WorldState,
	boundingRegions []spatialmath.Geometry,
	constraints *Constraints,
	planningOpts map[string]interface{},
) (*plannerOptions, error) {
	planAlg := ""

	// This will adjust the goal position to make movements more intuitive when using incrementation near poles
	to = fixOvIncrement(to, from)

	// Start with normal options
	opt := newBasicPlannerOptions(pm.frame)
	opt.extra = planningOpts
	opt.StartPose = from

	collisionBufferMM := defaultCollisionBufferMM
	collisionBufferMMRaw, ok := planningOpts["collision_buffer_mm"]
	if ok {
		collisionBufferMM, ok = collisionBufferMMRaw.(float64)
		if !ok {
			return nil, errors.New("could not interpret collision_buffer_mm field as float64")
		}
		if collisionBufferMM < 0 {
			return nil, errors.New("collision_buffer_mm can't be negative")
		}
	}

	// extract inputs corresponding to the frame
	frameInputs, err := pm.frame.mapToSlice(seedMap)
	if err != nil {
		return nil, err
	}

	// create robot collision entities
	movingGeometriesInFrame, err := pm.frame.Geometries(frameInputs)
	if err != nil {
		return nil, err // no geometries defined for frame
	}
	movingRobotGeometries := movingGeometriesInFrame.Geometries() // solver frame returns geoms in frame World

	// find all geometries that are not moving but are in the frame system
	staticRobotGeometries := make([]spatialmath.Geometry, 0)
	frameSystemGeometries, err := referenceframe.FrameSystemGeometries(pm.frame.fss, seedMap)
	if err != nil {
		return nil, err
	}
	for name, geometries := range frameSystemGeometries {
		if !pm.frame.movingFrame(name) {
			staticRobotGeometries = append(staticRobotGeometries, geometries.Geometries()...)
		}
	}

	// Note that all obstacles in worldState are assumed to be static so it is ok to transform them into the world frame
	// TODO(rb) it is bad practice to assume that the current inputs of the robot correspond to the passed in world state
	// the state that observed the worldState should ultimately be included as part of the worldState message
	worldGeometries, err := worldState.ObstaclesInWorldFrame(pm.frame.fss, seedMap)
	if err != nil {
		return nil, err
	}

	allowedCollisions, err := collisionSpecifications(constraints.GetCollisionSpecification(), frameSystemGeometries, worldState)
	if err != nil {
		return nil, err
	}

	// add collision constraints
	collisionConstraints, err := createAllCollisionConstraints(
		movingRobotGeometries,
		staticRobotGeometries,
		worldGeometries.Geometries(),
		boundingRegions,
		allowedCollisions,
		collisionBufferMM,
	)
	if err != nil {
		return nil, err
	}
	for name, constraint := range collisionConstraints {
		opt.AddStateConstraint(name, constraint)
	}

	hasTopoConstraint := opt.addPbTopoConstraints(from, to, constraints)

	// error handling around extracting motion_profile information from map[string]interface{}
	var motionProfile string
	profile, ok := planningOpts["motion_profile"]
	if ok {
		motionProfile, ok = profile.(string)
		if !ok {
			return nil, errors.New("could not interpret motion_profile field as string")
		}
	}

	// convert map to json, then to a struct, overwriting present defaults
	jsonString, err := json.Marshal(planningOpts)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonString, opt)
	if err != nil {
		return nil, err
	}

	alg, ok := planningOpts["planning_alg"]
	if ok {
		planAlg, ok = alg.(string)
		if !ok {
			return nil, errors.New("could not interpret planning_alg field as string")
		}
	}
	if pm.useTPspace && planAlg != "" {
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
		// no motion profiles for RRT*
		opt.PlannerConstructor = newRRTStarConnectMotionPlanner
		// TODO(pl): more logic for RRT*?
		return opt, nil
	default:
		// use default, already set
	}
	if pm.useTPspace {
		// overwrite default with TP space
		opt.PlannerConstructor = newTPSpaceMotionPlanner
		// Distances are computed in cartesian space rather than configuration space
		opt.DistanceFunc = ik.NewSquaredNormSegmentMetric(defaultTPspaceOrientationScale)
		// If we have PTGs, then we calculate distances using the PTG-specific distance function.
		// Otherwise we just use squared norm on inputs.
		opt.ScoreFunc = tpspace.PTGSegmentMetric

		planAlg = "tpspace"
		opt.relativeInputs = true
	}

	switch motionProfile {
	case LinearMotionProfile:
		opt.profile = LinearMotionProfile
		// Linear constraints
		linTol, ok := planningOpts["line_tolerance"].(float64)
		if !ok {
			// Default
			linTol = defaultLinearDeviation
		}
		orientTol, ok := planningOpts["orient_tolerance"].(float64)
		if !ok {
			// Default
			orientTol = defaultOrientationDeviation
		}
		constraint, pathMetric := NewAbsoluteLinearInterpolatingConstraint(from, to, linTol, orientTol)
		opt.AddStateConstraint(defaultLinearConstraintDesc, constraint)
		opt.pathMetric = pathMetric
	case PseudolinearMotionProfile:
		opt.profile = PseudolinearMotionProfile
		tolerance, ok := planningOpts["tolerance"].(float64)
		if !ok {
			// Default
			tolerance = defaultPseudolinearTolerance
		}
		constraint, pathMetric := NewProportionalLinearInterpolatingConstraint(from, to, tolerance)
		opt.AddStateConstraint(defaultPseudolinearConstraintDesc, constraint)
		opt.pathMetric = pathMetric
	case OrientationMotionProfile:
		opt.profile = OrientationMotionProfile
		tolerance, ok := planningOpts["tolerance"].(float64)
		if !ok {
			// Default
			tolerance = defaultOrientationDeviation
		}
		constraint, pathMetric := NewSlerpOrientationConstraint(from, to, tolerance)
		opt.AddStateConstraint(defaultOrientationConstraintDesc, constraint)
		opt.pathMetric = pathMetric
	case PositionOnlyMotionProfile:
		opt.profile = PositionOnlyMotionProfile
		if !pm.useTPspace || opt.PositionSeeds <= 0 {
			opt.goalMetricConstructor = ik.NewPositionOnlyMetric
		}
	case FreeMotionProfile:
		// No restrictions on motion
		fallthrough
	default:
		opt.profile = FreeMotionProfile
		if planAlg == "" {
			// set up deep copy for fallback
			try1 := deepAtomicCopyMap(planningOpts)
			// No need to generate tons more IK solutions when the first alg will do it

			// time to run the first planning attempt before falling back
			try1["timeout"] = defaultFallbackTimeout
			try1["planning_alg"] = "rrtstar"
			try1Opt, err := pm.plannerSetupFromMoveRequest(from, to, seedMap, worldState, boundingRegions, constraints, try1)
			if err != nil {
				return nil, err
			}

			try1Opt.Fallback = opt
			opt = try1Opt
		}
	}
	return opt, nil
}

// check whether the solution is within some amount of the optimal.
func (pm *planManager) goodPlan(pr *rrtSolution, opt *plannerOptions) (bool, float64) {
	solutionCost := math.Inf(1)
	if pr.steps != nil {
		if pr.maps.optNode.Cost() <= 0 {
			return true, solutionCost
		}
		solutionCost = pm.frame.nodesToTrajectory(pr.steps).EvaluateCost(opt.ScoreFunc)
		if solutionCost < pr.maps.optNode.Cost()*defaultOptimalityMultiple {
			return true, solutionCost
		}
	}

	return false, solutionCost
}

func (pm *planManager) planToRRTGoalMap(plan Plan, goal spatialmath.Pose) (*rrtMaps, error) {
	// TODO: make this work with any implementation of Plan
	var rrt *rrtPlan
	var ok bool
	if rrt, ok = plan.(*rrtPlan); !ok {
		return nil, errBadPlanImpl
	}
	planNodes := rrt.nodes

	if pm.useTPspace {
		// Fill in positions from the old origin to where the goal was during the last run
		planNodesOld, err := rectifyTPspacePath(planNodes, pm.frame, spatialmath.NewZeroPose())
		if err != nil {
			return nil, err
		}

		// Figure out where our new starting point is relative to our last one, and re-rectify using the new adjusted location
		oldGoal := planNodesOld[len(planNodesOld)-1].Pose()
		pathDiff := spatialmath.PoseBetween(oldGoal, goal)
		planNodes, err = rectifyTPspacePath(planNodes, pm.frame, pathDiff)
		if err != nil {
			return nil, err
		}
	}

	var lastNode node
	goalMap := map[node]node{}
	for i := len(planNodes) - 1; i >= 0; i-- {
		if i != 0 {
			// Fill in costs
			cost := pm.opt().DistanceFunc(&ik.Segment{
				StartConfiguration: planNodes[i-1].Q(),
				StartPosition:      planNodes[i-1].Pose(),
				EndConfiguration:   planNodes[i].Q(),
				EndPosition:        planNodes[i].Pose(),
				Frame:              pm.frame,
			})
			planNodes[i].SetCost(cost)
		}
		goalMap[planNodes[i]] = lastNode
		lastNode = planNodes[i]
	}

	startNode := &basicNode{q: make([]referenceframe.Input, len(pm.frame.DoF())), pose: spatialmath.NewZeroPose()}
	maps := &rrtMaps{
		startMap: map[node]node{startNode: nil},
		goalMap:  goalMap,
	}

	return maps, nil
}

// planRelativeWaypoint will solve the solver frame to one individual pose. This is used for solverframes whose inputs are relative, that
// is, the pose returned by `Transform` is a transformation rather than an absolute position.
func (pm *planManager) planRelativeWaypoint(ctx context.Context, request *PlanRequest, seedPlan Plan) (Plan, error) {
	anyNonzero := false // Whether non-PTG frames exist
	for _, movingFrame := range pm.frame.frames {
		if _, isPTGframe := movingFrame.(tpspace.PTGProvider); isPTGframe {
			continue
		} else if len(movingFrame.DoF()) > 0 {
			anyNonzero = true
		}
		if anyNonzero {
			return nil, errors.New("cannot combine ptg with other nonzero DOF frames in a single planning call")
		}
	}

	if request.StartPose == nil {
		return nil, errors.New("must provide a startPose if solving for PTGs")
	}

	startPose := request.StartPose

	request.Logger.CInfof(ctx,
		"planning relative motion for frame %s\nGoal: %v\nstartPose %v\n, worldstate: %v\n",
		request.Frame.Name(),
		referenceframe.PoseInFrameToProtobuf(request.Goal),
		spatialmath.PoseToProtobuf(startPose),
		request.WorldState.String(),
	)

	if pathdebug {
		pm.logger.Debug("$type,X,Y")
		pm.logger.Debugf("$SG,%f,%f", startPose.Point().X, startPose.Point().Y)
		pm.logger.Debugf("$SG,%f,%f", request.Goal.Pose().Point().X, request.Goal.Pose().Point().Y)
		gifs, err := request.WorldState.ObstaclesInWorldFrame(pm.frame.fss, request.StartConfiguration)
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

	tf, err := pm.frame.fss.Transform(request.StartConfiguration, request.Goal, referenceframe.World)
	if err != nil {
		return nil, err
	}
	goalPos := tf.(*referenceframe.PoseInFrame).Pose()
	opt, err := pm.plannerSetupFromMoveRequest(
		startPose, goalPos, request.StartConfiguration, request.WorldState, request.BoundingRegions, request.Constraints, request.Options,
	)
	if err != nil {
		return nil, err
	}

	if opt.profile == PositionOnlyMotionProfile {
		opt.AtGoalMetric = func(startPose, goalPose spatialmath.Pose) bool {
			return spatialmath.PoseAlmostCoincidentEps(startPose, goalPose, request.GoalThreshold)
		}
	} else {
		opt.AtGoalMetric = func(startPose, goalPose spatialmath.Pose) bool {
			return spatialmath.OrientationAlmostEqual(goalPose.Orientation(), startPose.Orientation()) &&
				spatialmath.PoseAlmostCoincidentEps(goalPose, startPose, request.GoalThreshold)
		}
	}

	// re-root the frame system on the relative frame
	relativeOnlyFS, err := pm.frame.fss.FrameSystemSubset(request.Frame)
	if err != nil {
		return nil, err
	}
	pm.frame.fss = relativeOnlyFS
	pm.planOpts = opt

	opt.SetGoal(goalPos)

	// Build planner
	//nolint: gosec
	pathPlanner, err := opt.PlannerConstructor(
		pm.frame,
		rand.New(rand.NewSource(int64(pm.randseed.Int()))),
		pm.logger,
		opt,
	)
	if err != nil {
		return nil, err
	}
	zeroInputs := make([]referenceframe.Input, len(pm.frame.DoF()))
	maps := &rrtMaps{}
	if seedPlan != nil {
		maps, err = pm.planToRRTGoalMap(seedPlan, goalPos)
		if err != nil {
			return nil, err
		}
	}
	if pm.opt().PositionSeeds > 0 && pm.opt().profile == PositionOnlyMotionProfile {
		err = maps.fillPosOnlyGoal(goalPos, pm.opt().PositionSeeds, len(pm.frame.DoF()))
		if err != nil {
			return nil, err
		}
	} else {
		goalNode := &basicNode{q: zeroInputs, pose: spatialmath.Compose(goalPos, flipPose)}
		maps.goalMap = map[node]node{goalNode: nil}
	}
	startNode := &basicNode{q: zeroInputs, pose: startPose}
	maps.startMap = map[node]node{startNode: nil}

	// Plan the single waypoint, and accumulate objects which will be used to constrauct the plan after all planning has finished
	_, future, err := pm.planSingleAtomicWaypoint(ctx, goalPos, zeroInputs, pathPlanner, maps)
	if err != nil {
		return nil, err
	}
	steps, err := future.result()
	if err != nil {
		return nil, err
	}

	return newRRTPlan(steps, pm.frame, pm.useTPspace)
}

// Copy any atomic values.
func deepAtomicCopyMap(opt map[string]interface{}) map[string]interface{} {
	optCopy := map[string]interface{}{}
	for k, v := range opt {
		optCopy[k] = v
	}
	return optCopy
}
