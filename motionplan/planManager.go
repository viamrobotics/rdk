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

	pb "go.viam.com/api/service/motion/v1"
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
	defaultTPspaceOrientationScale = 30.
)

// planManager is intended to be the single entry point to motion planners, wrapping all others, dealing with fallbacks, etc.
// Intended information flow should be:
// motionplan.PlanMotion() -> SolvableFrameSystem.SolveWaypointsWithOptions() -> planManager.planSingleWaypoint().
type planManager struct {
	*planner
	frame                   *solverFrame
	fs                      referenceframe.FrameSystem
	activeBackgroundWorkers sync.WaitGroup

	useTPspace bool
}

func newPlanManager(
	frame *solverFrame,
	fs referenceframe.FrameSystem,
	logger logging.Logger,
	seed int,
) (*planManager, error) {
	anyPTG := false     // Whether PTG frames have been observed
	anyNonzero := false // Whether non-PTG frames
	for _, movingFrame := range frame.frames {
		if ptgFrame, isPTGframe := movingFrame.(tpspace.PTGProvider); isPTGframe {
			if anyPTG {
				return nil, errors.New("only one PTG frame can be planned for at a time")
			}
			anyPTG = true
			frame.ptgs = ptgFrame.PTGSolvers()
		} else if len(movingFrame.DoF()) > 0 {
			anyNonzero = true
		}
		if anyNonzero && anyPTG {
			return nil, errors.New("cannot combine ptg with other nonzero DOF frames in a single planning call")
		}
	}

	//nolint: gosec
	p, err := newPlanner(frame, rand.New(rand.NewSource(int64(seed))), logger, newBasicPlannerOptions(frame))
	if err != nil {
		return nil, err
	}
	return &planManager{planner: p, frame: frame, fs: fs, useTPspace: anyPTG}, nil
}

// PlanSingleWaypoint will solve the solver frame to one individual pose. If you have multiple waypoints to hit, call this multiple times.
// Any constraints, etc, will be held for the entire motion.
func (pm *planManager) PlanSingleWaypoint(ctx context.Context,
	seedMap map[string][]referenceframe.Input,
	goalPos spatialmath.Pose,
	worldState *referenceframe.WorldState,
	constraintSpec *pb.Constraints,
	seedPlan Plan,
	motionConfig map[string]interface{},
) ([][]referenceframe.Input, error) {
	seed, err := pm.frame.mapToSlice(seedMap)
	if err != nil {
		return nil, err
	}
	seedPos, err := pm.frame.Transform(seed)
	if err != nil {
		return nil, err
	}

	var cancel func()

	// set timeout for entire planning process if specified
	if timeout, ok := motionConfig["timeout"].(float64); ok {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout*float64(time.Second)))
	}
	if cancel != nil {
		defer cancel()
	}

	// If we are world rooted, translate the goal pose into the world frame
	if pm.frame.worldRooted {
		tf, err := pm.frame.fss.Transform(seedMap, referenceframe.NewPoseInFrame(pm.frame.goalFrame.Name(), goalPos), referenceframe.World)
		if err != nil {
			return nil, err
		}
		goalPos = tf.(*referenceframe.PoseInFrame).Pose()
	}

	var goals []spatialmath.Pose
	var opts []*plannerOptions

	subWaypoints := false

	// linear motion profile has known intermediate points, so solving can be broken up and sped up
	if profile, ok := motionConfig["motion_profile"]; ok && profile == LinearMotionProfile {
		subWaypoints = true
	}

	if len(constraintSpec.GetLinearConstraint()) > 0 {
		subWaypoints = true
	}

	// If we are seeding off of a pre-existing plan, we don't need the speedup of subwaypoints
	if seedPlan != nil {
		subWaypoints = false
	}

	if subWaypoints {
		pathStepSize, ok := motionConfig["path_step_size"].(float64)
		if !ok {
			pathStepSize = defaultPathStepSize
		}
		numSteps := PathStepCount(seedPos, goalPos, pathStepSize)

		from := seedPos
		for i := 1; i < numSteps; i++ {
			by := float64(i) / float64(numSteps)
			to := spatialmath.Interpolate(seedPos, goalPos, by)
			goals = append(goals, to)
			opt, err := pm.plannerSetupFromMoveRequest(from, to, seedMap, worldState, constraintSpec, motionConfig)
			if err != nil {
				return nil, err
			}
			opt.SetGoal(to)
			opts = append(opts, opt)

			from = to
		}
		seedPos = from
	}
	goals = append(goals, goalPos)
	opt, err := pm.plannerSetupFromMoveRequest(seedPos, goalPos, seedMap, worldState, constraintSpec, motionConfig)
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

	resultSlices, err := pm.planAtomicWaypoints(ctx, goals, seed, planners, seedPlan)
	pm.activeBackgroundWorkers.Wait()
	if err != nil {
		if len(goals) > 1 {
			err = fmt.Errorf("failed to plan path for valid goal: %w", err)
		}
		return nil, err
	}
	return resultSlices, nil
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
) ([][]referenceframe.Input, error) {
	var err error
	// A resultPromise can be queried in the future and will eventually yield either a set of planner waypoints, or an error.
	// Each atomic waypoint produces one result promise, all of which are resolved at the end, allowing multiple to be solved in parallel.
	resultPromises := []*resultPromise{}

	// try to solve each goal, one at a time
	for i, goal := range goals {
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

	resultSlices := [][]referenceframe.Input{}

	// All goals have been submitted for solving. Reconstruct in order
	for _, future := range resultPromises {
		steps, err := future.result(ctx)
		if err != nil {
			return nil, err
		}
		resultSlices = append(resultSlices, steps...)
	}

	return resultSlices, nil
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
		solutionChan := make(chan *rrtPlanReturn, 1)
		pm.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer pm.activeBackgroundWorkers.Done()
			pm.planParallelRRTMotion(ctx, goal, seed, parPlan, endpointPreview, solutionChan, maps)
		})
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		select {
		case nextSeed := <-endpointPreview:
			return nextSeed.Q(), &resultPromise{future: solutionChan}, nil
		case planReturn := <-solutionChan:
			if planReturn.planerr != nil {
				return nil, nil, planReturn.planerr
			}
			steps := nodesToInputs(planReturn.steps)
			return steps[len(steps)-1], &resultPromise{steps: steps}, nil
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	} else {
		// This ctx is used exclusively for the running of the new planner and timing it out.
		plannerctx, cancel := context.WithTimeout(ctx, time.Duration(pathPlanner.opt().Timeout*float64(time.Second)))
		defer cancel()
		steps, err := pathPlanner.plan(plannerctx, goal, seed)
		if err != nil {
			return nil, nil, err
		}

		smoothedPath := nodesToInputs(pathPlanner.smoothPath(ctx, steps))

		// Update seed for the next waypoint to be the final configuration of this waypoint
		seed = smoothedPath[len(smoothedPath)-1]
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
	solutionChan chan *rrtPlanReturn,
	maps *rrtMaps,
) {
	var err error
	// If we don't pass in pre-made maps, initialize and seed with IK solutions here
	if !pm.useTPspace {
		if maps == nil {
			planSeed := initRRTSolutions(ctx, pathPlanner, seed)
			if planSeed.planerr != nil || planSeed.steps != nil {
				solutionChan <- planSeed
				return
			}
			maps = planSeed.maps
		}
	} else {
		if maps == nil {
			startNode := &basicNode{q: make([]referenceframe.Input, len(pm.frame.DoF())), cost: 0, pose: spatialmath.NewZeroPose()}
			goalNode := &basicNode{q: make([]referenceframe.Input, len(pm.frame.DoF())), cost: 0, pose: goal, corner: false}
			maps = &rrtMaps{
				startMap: map[node]node{startNode: nil},
				goalMap:  map[node]node{goalNode: nil},
			}
		}
	}

	// publish endpoint of plan if it is known
	var nextSeed node
	if maps != nil && len(maps.goalMap) == 1 {
		pm.logger.Debug("only one IK solution, returning endpoint preview")
		for key := range maps.goalMap {
			nextSeed = key
		}
		if endpointPreview != nil {
			endpointPreview <- nextSeed
			endpointPreview = nil
		}
	}

	// This ctx is used exclusively for the running of the new planner and timing it out.
	plannerctx, cancel := context.WithTimeout(ctx, time.Duration(pathPlanner.opt().Timeout*float64(time.Second)))
	defer cancel()

	plannerChan := make(chan *rrtPlanReturn, 1)

	// start the planner
	pm.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer pm.activeBackgroundWorkers.Done()
		pathPlanner.rrtBackgroundRunner(plannerctx, seed, &rrtParallelPlannerShared{maps, endpointPreview, plannerChan})
	})

	// Wait for results from the planner. This will also handle calling the fallback if needed, and will ultimately return the best path
	select {
	case <-ctx.Done():
		// Error will be caught by monitoring loop
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
		if finalSteps.err() == nil {
			if fallbackPlanner != nil {
				if ok, score := pm.goodPlan(finalSteps, pm.opt()); ok {
					pm.logger.Debugf("got path with score %f, close enough to optimal %f", score, maps.optNode.Cost())
					fallbackPlanner = nil
				} else {
					pm.logger.Debugf("path with score %f not close enough to optimal %f, falling back", score, maps.optNode.Cost())

					// If we have a connected but bad path, we recreate new IK solutions and start from scratch
					// rather than seeding with a completed, known-bad tree
					mapSeed = nil
				}
			}
		}

		// Start smoothing before initializing the fallback plan. This allows both to run simultaneously.
		smoothChan := make(chan []node, 1)
		pm.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer pm.activeBackgroundWorkers.Done()
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
			score = pm.frame.inputsToPlan(nodesToInputs(finalSteps.steps)).Evaluate(pm.opt().ScoreFunc)
		}

		// If we ran a fallback, retrieve the result and compare to the smoothed path
		if alternateFuture != nil {
			alternate, err := alternateFuture.result(ctx)
			if err == nil {
				// If the fallback successfully found a path, check if it is better than our smoothed previous path.
				// The fallback should emerge pre-smoothed, so that should be a non-issue
				altCost := pm.frame.inputsToPlan(alternate).Evaluate(pm.opt().ScoreFunc)
				if altCost < score {
					pm.logger.Debugf("replacing path with score %f with better score %f", score, altCost)
					finalSteps = &rrtPlanReturn{steps: stepsToNodes(alternate)}
				} else {
					pm.logger.Debugf("fallback path with score %f worse than original score %f; using original", altCost, score)
				}
			}
		}

		solutionChan <- finalSteps
		return

	case <-ctx.Done():
		return
	}
}

// This is where the map[string]interface{} passed in via `extra` is used to decide how planning happens.
func (pm *planManager) plannerSetupFromMoveRequest(
	from, to spatialmath.Pose,
	seedMap map[string][]referenceframe.Input,
	worldState *referenceframe.WorldState,
	constraints *pb.Constraints,
	planningOpts map[string]interface{},
) (*plannerOptions, error) {
	planAlg := ""

	// This will adjust the goal position to make movements more intuitive when using incrementation near poles
	to = fixOvIncrement(to, from)

	// Start with normal options
	opt := newBasicPlannerOptions(pm.frame)
	opt.extra = planningOpts

	// add collision constraints
	collisionConstraints, err := createAllCollisionConstraints(
		pm.frame,
		pm.fs,
		worldState,
		seedMap,
		constraints.GetCollisionSpecification(),
	)
	if err != nil {
		return nil, err
	}
	for name, constraint := range collisionConstraints {
		opt.AddStateConstraint(name, constraint)
	}

	hasTopoConstraint := opt.addPbTopoConstraints(from, to, constraints)
	if hasTopoConstraint {
		planAlg = "cbirrt"
	}

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
		if pm.useTPspace && planAlg != "" {
			return nil, fmt.Errorf("cannot specify a planning_alg when planning for a TP-space frame. alg specified was %s", planAlg)
		}
		switch planAlg {
		// TODO(pl): make these consts
		case "cbirrt":
			opt.PlannerConstructor = newCBiRRTMotionPlanner
		case "rrtstar":
			// no motion profiles for RRT*
			opt.PlannerConstructor = newRRTStarConnectMotionPlanner
			// TODO(pl): more logic for RRT*?
			return opt, nil
		default:
			// use default, already set
		}
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
		opt.goalMetricConstructor = ik.NewPositionOnlyMetric
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
			try1Opt, err := pm.plannerSetupFromMoveRequest(from, to, seedMap, worldState, constraints, try1)
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
func (pm *planManager) goodPlan(pr *rrtPlanReturn, opt *plannerOptions) (bool, float64) {
	solutionCost := math.Inf(1)
	if pr.steps != nil {
		if pr.maps.optNode.Cost() <= 0 {
			return true, solutionCost
		}
		solutionCost = pm.frame.inputsToPlan(nodesToInputs(pr.steps)).Evaluate(opt.ScoreFunc)
		if solutionCost < pr.maps.optNode.Cost()*defaultOptimalityMultiple {
			return true, solutionCost
		}
	}

	return false, solutionCost
}

func (pm *planManager) planToRRTGoalMap(plan Plan, goal spatialmath.Pose) (*rrtMaps, error) {
	planNodes := make([]node, 0, len(plan))
	// Build a list of nodes from the plan
	for _, planStep := range plan {
		conf, err := pm.frame.mapToSlice(planStep)
		if err != nil {
			return nil, err
		}
		planNodes = append(planNodes, newConfigurationNode(conf))
	}

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
	// Fill in costs
	for i := 1; i < len(planNodes); i++ {
		cost := pm.opt().DistanceFunc(&ik.Segment{
			StartConfiguration: planNodes[i-1].Q(),
			StartPosition:      planNodes[i-1].Pose(),
			EndConfiguration:   planNodes[i].Q(),
			EndPosition:        planNodes[i].Pose(),
			Frame:              pm.frame,
		})
		planNodes[i].SetCost(cost)
	}

	var lastNode node
	goalMap := map[node]node{}
	for i := len(planNodes) - 1; i >= 0; i-- {
		goalMap[planNodes[i]] = lastNode
		lastNode = planNodes[i]
	}

	startNode := &basicNode{q: make([]referenceframe.Input, len(pm.frame.DoF())), cost: 0, pose: spatialmath.NewZeroPose()}
	maps := &rrtMaps{
		startMap: map[node]node{startNode: nil},
		goalMap:  goalMap,
	}

	return maps, nil
}
// Copy any atomic values.
func deepAtomicCopyMap(opt map[string]interface{}) map[string]interface{} {
	optCopy := map[string]interface{}{}
	for k, v := range opt {
		optCopy[k] = v
	}
	return optCopy
}
