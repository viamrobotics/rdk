package motionplan

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	defaultOptimalityMultiple      = 2.0
	defaultFallbackTimeout         = 1.5
	defaultTPspaceOrientationScale = 500.
)

// planManager is intended to be the single entry point to motion planners, wrapping all others, dealing with fallbacks, etc.
// Intended information flow should be:
// motionplan.PlanMotion() -> SolvableFrameSystem.SolveWaypointsWithOptions() -> planManager.planSingleWaypoint().
type planManager struct {
	*planner // TODO: This should probably be removed
	// We store the request because we want to be able to inspect the original state of the plan
	// that was requested at any point during the process of creating multiple planners
	// for waypoints and such.
	request                 *PlanRequest
	activeBackgroundWorkers sync.WaitGroup
}

func newPlanManager(logger logging.Logger, fs referenceframe.FrameSystem, request *PlanRequest) (*planManager, error) {
	p, err := newPlannerFromPlanRequest(logger, fs, request)
	if err != nil {
		return nil, err
	}
	request.PlannerOptions = p.planOpts

	return &planManager{
		planner: p,
		request: request,
	}, nil
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
func (pm *planManager) planMultiWaypoint(ctx context.Context, seedPlan Plan) (Plan, error) {
	if pm.motionChains.useTPspace {
		return pm.planRelativeWaypoint(ctx, seedPlan)
	}
	// Theoretically, a plan could be made between two poses, by running IK on both the start and end poses to create sets of seed and
	// goal configurations. However, the blocker here is the lack of a "known good" configuration used to determine which obstacles
	// are allowed to collide with one another.
	if pm.request.StartState.configuration == nil {
		return nil, errors.New("must populate start state configuration if not planning for 2d base/tpspace")
	}

	// set timeout for entire planning process if specified
	var cancel func()
	if pm.planOpts.Timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(pm.planOpts.Timeout*float64(time.Second)))
	}
	if cancel != nil {
		defer cancel()
	}

	waypoints := []atomicWaypoint{}

	for i := range pm.request.Goals {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		// Solving highly constrained motions by breaking apart into small pieces is much more performant
		goalWaypoints, err := pm.generateWaypoints(seedPlan, i)
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
		if !pm.motionChains.useTPspace && maps == nil {
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
			fallbackPlanner, err = newMotionPlanner(
				pm.fs,
				rand.New(rand.NewSource(int64(pm.randseed.Int()))),
				pm.logger,
				pathPlanner.opt().Fallback,
				pm.ConstraintHandler,
				pm.motionChains,
			)
			if err != nil {
				fallbackPlanner = nil
			}
		}

		// If there was no error, check path quality. If sufficiently good, move on.
		// If there *was* an error, then either the fallback will not error and will replace it, or the error will be returned
		if finalSteps.err == nil {
			if fallbackPlanner != nil {
				if ok, score := pm.goodPlan(finalSteps); ok {
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
			score = nodesToTrajectory(finalSteps.steps).EvaluateCost(wp.mp.getScoringFunction())
		}

		// If we ran a fallback, retrieve the result and compare to the smoothed path
		if alternateFuture != nil {
			alternate, err := alternateFuture.result()
			if err == nil {
				// If the fallback successfully found a path, check if it is better than our smoothed previous path.
				// The fallback should emerge pre-smoothed, so that should be a non-issue
				altCost := nodesToTrajectory(alternate).EvaluateCost(wp.mp.getScoringFunction())
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

// generateWaypoints will return the list of atomic waypoints that correspond to a specific goal in a plan request.
func (pm *planManager) generateWaypoints(seedPlan Plan, wpi int) ([]atomicWaypoint, error) {
	wpGoals := pm.request.Goals[wpi]
	startState := pm.request.StartState
	if wpi > 0 {
		startState = pm.request.Goals[wpi-1]
	}

	startPoses, err := startState.ComputePoses(pm.fs)
	if err != nil {
		return nil, err
	}
	goalPoses, err := wpGoals.ComputePoses(pm.fs)
	if err != nil {
		return nil, err
	}

	subWaypoints := pm.useSubWaypoints(seedPlan, wpi)

	motionChains, err := motionChainsFromPlanState(pm.fs, wpGoals)
	if err != nil {
		return nil, err
	}

	opt, err := updateOptionsForPlanning(pm.request.PlannerOptions, motionChains.useTPspace)
	if err != nil {
		return nil, err
	}

	constraintHandler, err := newConstraintHandler(
		opt,
		pm.request.Constraints,
		startState,
		wpGoals,
		pm.fs,
		motionChains,
		pm.request.StartState.configuration,
		pm.request.WorldState,
		pm.boundingRegions,
	)
	if err != nil {
		return nil, err
	}

	if wpGoals.poses != nil {
		// Transform goal poses into world frame if needed. This is used for e.g. when a component's goal is given in terms of itself.
		alteredGoals, err := motionChains.translateGoalsToWorldPosition(pm.fs, pm.request.StartState.configuration, wpGoals)
		if err != nil {
			return nil, err
		}
		wpGoals = alteredGoals

		motionChains, err = motionChainsFromPlanState(pm.fs, wpGoals)
		if err != nil {
			return nil, err
		}

		// Regenerate opts since our metrics will have changed
		opt, err = updateOptionsForPlanning(opt, motionChains.useTPspace)
		if err != nil {
			return nil, err
		}

		constraintHandler, err = newConstraintHandler(
			opt,
			pm.request.Constraints,
			startState,
			wpGoals,
			pm.fs,
			motionChains,
			pm.request.StartState.configuration,
			pm.request.WorldState,
			pm.boundingRegions,
		)
		if err != nil {
			return nil, err
		}

		pm.motionChains = motionChains
	}
	pm.ConstraintHandler = constraintHandler

	// TPspace should never use subwaypoints
	if !subWaypoints || motionChains.useTPspace {
		//nolint: gosec
		pathPlanner, err := newMotionPlanner(
			pm.fs,
			rand.New(rand.NewSource(int64(pm.randseed.Int()))),
			pm.logger,
			opt,
			constraintHandler,
			pm.motionChains,
		)
		if err != nil {
			return nil, err
		}
		return []atomicWaypoint{{mp: pathPlanner, startState: pm.request.StartState, goalState: wpGoals, origWP: wpi}}, nil
	}

	stepSize := pm.planOpts.PathStepSize

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
		wpChains, err := motionChainsFromPlanState(pm.fs, to)
		if err != nil {
			return nil, err
		}

		wpOpt, err := updateOptionsForPlanning(pm.request.PlannerOptions, wpChains.useTPspace)
		if err != nil {
			return nil, err
		}

		wpConstraintHandler, err := newConstraintHandler(
			wpOpt,
			pm.request.Constraints,
			from,
			to,
			pm.fs,
			wpChains,
			pm.request.StartState.configuration,
			pm.request.WorldState,
			pm.boundingRegions,
		)
		if err != nil {
			return nil, err
		}

		//nolint: gosec
		pathPlanner, err := newMotionPlanner(
			pm.fs,
			rand.New(rand.NewSource(int64(pm.randseed.Int()))),
			pm.logger,
			wpOpt,
			wpConstraintHandler,
			pm.motionChains,
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
func (pm *planManager) goodPlan(pr *rrtSolution) (bool, float64) {
	solutionCost := math.Inf(1)
	if pr.steps != nil {
		if pr.maps.optNode.Cost() <= 0 {
			return true, solutionCost
		}
		solutionCost = nodesToTrajectory(pr.steps).EvaluateCost(pm.scoringFunction)
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

	if pm.motionChains.useTPspace {
		// Fill in positions from the old origin to where the goal was during the last run
		planNodesOld, err := rectifyTPspacePath(planNodes, pm.fs.Frame(pm.motionChains.ptgFrameName), spatialmath.NewZeroPose())
		if err != nil {
			return nil, err
		}

		// Figure out where our new starting point is relative to our last one, and re-rectify using the new adjusted location
		oldGoal := planNodesOld[len(planNodesOld)-1].Poses()[pm.motionChains.ptgFrameName].Pose()
		pathDiff := spatialmath.PoseBetween(oldGoal, goal.goalState.poses[pm.motionChains.ptgFrameName].Pose())
		planNodes, err = rectifyTPspacePath(planNodes, pm.fs.Frame(pm.motionChains.ptgFrameName), pathDiff)
		if err != nil {
			return nil, err
		}
	}

	var lastNode node
	goalMap := map[node]node{}
	for i := len(planNodes) - 1; i >= 0; i-- {
		if i != 0 {
			// Fill in costs
			cost := pm.configurationDistanceFunc(&ik.SegmentFS{
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
func (pm *planManager) planRelativeWaypoint(ctx context.Context, seedPlan Plan) (Plan, error) {
	if pm.request.StartState.poses == nil {
		return nil, errors.New("must provide a startPose if solving for PTGs")
	}
	if len(pm.request.Goals) != 1 {
		return nil, errors.New("can only provide one goal if solving for PTGs")
	}
	startPose := pm.request.StartState.poses[pm.motionChains.ptgFrameName].Pose()
	goalPif := pm.request.Goals[0].poses[pm.motionChains.ptgFrameName]

	pm.logger.CInfof(ctx,
		"planning relative motion for frame %s\nGoal: %v\nstartPose %v\n, worldstate: %v\n",
		pm.motionChains.ptgFrameName,
		referenceframe.PoseInFrameToProtobuf(goalPif),
		startPose,
		pm.request.WorldState.String(),
	)

	if pathdebug {
		pm.logger.Debug("$type,X,Y")
		pm.logger.Debugf("$SG,%f,%f", startPose.Point().X, startPose.Point().Y)
		pm.logger.Debugf("$SG,%f,%f", goalPif.Pose().Point().X, goalPif.Pose().Point().Y)
		gifs, err := pm.request.WorldState.ObstaclesInWorldFrame(pm.fs, pm.request.StartState.configuration)
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
	if pm.planOpts.Timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(pm.planOpts.Timeout*float64(time.Second)))
	}
	if cancel != nil {
		defer cancel()
	}

	// Create frame system subset using frame name
	relativeOnlyFS, err := pm.fs.FrameSystemSubset(pm.fs.Frame(pm.motionChains.ptgFrameName))
	if err != nil {
		return nil, err
	}
	pm.fs = relativeOnlyFS

	wps, err := pm.generateWaypoints(seedPlan, 0)
	if err != nil {
		return nil, err
	}
	// Should never happen, but checking to guard against future changes breaking this
	if len(wps) != 1 {
		return nil, fmt.Errorf("tpspace should only generate exactly one atomic waypoint, but got %d", len(wps))
	}
	wp := wps[0]

	zeroInputs := referenceframe.FrameSystemInputs{}
	zeroInputs[pm.motionChains.ptgFrameName] = make([]referenceframe.Input, len(pm.fs.Frame(pm.motionChains.ptgFrameName).DoF()))
	maps := &rrtMaps{}
	if seedPlan != nil {
		// TODO: This probably needs to be flipped? Check if these paths are ever used.
		maps, err = pm.planToRRTGoalMap(seedPlan, wp)
		if err != nil {
			return nil, err
		}
	}
	if pm.planOpts.PositionSeeds > 0 && pm.planOpts.MotionProfile == PositionOnlyMotionProfile {
		err = maps.fillPosOnlyGoal(wp.goalState.poses, pm.planOpts.PositionSeeds)
		if err != nil {
			return nil, err
		}
	} else {
		goalPose := wp.goalState.poses[pm.motionChains.ptgFrameName].Pose()
		goalMapFlip := map[string]*referenceframe.PoseInFrame{
			pm.motionChains.ptgFrameName: referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.Compose(goalPose, flipPose)),
		}
		goalNode := &basicNode{q: zeroInputs, poses: goalMapFlip}
		maps.goalMap = map[node]node{goalNode: nil}
	}
	startNode := &basicNode{q: zeroInputs, poses: pm.request.StartState.poses}
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

	return newRRTPlan(steps, pm.fs, pm.motionChains.useTPspace, startPose)
}

func nodesToTrajectory(nodes []node) Trajectory {
	traj := make(Trajectory, 0, len(nodes))
	for _, n := range nodes {
		traj = append(traj, n.Q())
	}
	return traj
}

// Determines whether to break a motion down into sub-waypoints if all intermediate points are known.
func (pm *planManager) useSubWaypoints(seedPlan Plan, wpi int) bool {
	// If we are seeding off of a pre-existing plan, we don't need the speedup of subwaypoints
	if seedPlan != nil {
		return false
	}
	// If goal has a configuration, do not use subwaypoints *unless* the start state is also a configuration.
	// We can interpolate from a pose or configuration to a pose, or a configuration to a configuration, but not from a pose to a
	// configuration.
	// TODO: If we run planning backwards, we could remove this restriction.
	if pm.request.Goals[wpi].configuration != nil {
		startState := pm.request.StartState
		if wpi > 0 {
			startState = pm.request.Goals[wpi-1]
		}
		if startState.configuration == nil {
			return false
		}
	}

	// linear motion profile has known intermediate points, so solving can be broken up and sped up
	if pm.planOpts.MotionProfile == LinearMotionProfile {
		return true
	}

	if len(pm.request.Constraints.GetLinearConstraint()) > 0 {
		return true
	}
	return false
}
