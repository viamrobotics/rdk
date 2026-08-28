package armplanning

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go.viam.com/utils/trace"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// planManager is intended to be the single entry point to motion planners.
type planManager struct {
	pc      *PlanContext
	request *PlanRequest
	logger  logging.Logger
}

func newPlanManager(ctx context.Context, logger logging.Logger, request *PlanRequest, meta *PlanMeta) (*planManager, error) {
	pc, err := NewPlanContext(ctx, logger, request, meta)
	if err != nil {
		return nil, err
	}
	return &planManager{
		pc:      pc,
		logger:  logger,
		request: request,
	}, nil
}

// planMultiWaypoint plans a motion through multiple waypoints, using identical constraints for each
// Any constraints, etc, will be held for the entire motion.
// return trajector (always, even with error), which goal we got to, error.
func (pm *planManager) planMultiWaypoint(ctx context.Context) ([]*referenceframe.LinearInputs, int, error) {
	ctx, span := trace.StartSpan(ctx, "planMultiWaypoint")
	defer span.End()

	// set timeout for entire planning process if specified
	var cancel func()
	if pm.request.PlannerOptions.Timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, pm.request.PlannerOptions.timeoutDuration())
	}
	if cancel != nil {
		defer cancel()
	}

	linearTraj := []*referenceframe.LinearInputs{pm.request.StartState.LinearConfiguration()}
	start, err := pm.request.StartState.ComputePoses(ctx, pm.request.FrameSystem)
	if err != nil {
		return nil, 0, err
	}

	pm.pc.planMeta.SubgoalsPerGoal = make([]int, len(pm.request.Goals))
	for i, g := range pm.request.Goals {
		if ctx.Err() != nil {
			return linearTraj, i, err // note: here and below, we return traj because of ReturnPartialPlan
		}

		to, err := g.ComputePoses(ctx, pm.request.FrameSystem)
		if err != nil {
			return linearTraj, i, err
		}

		if i > 0 {
			pm.logger.Infof("planning step %d of %d, current linearTraj size: %d",
				i, len(pm.request.Goals), len(linearTraj))
		}

		for k, v := range to {
			pm.logger.Debug(k, v)
		}

		if len(g.Configuration()) > 0 {
			newTraj, err := pm.planToDirectJoints(ctx, linearTraj[len(linearTraj)-1], g)
			if err != nil {
				return linearTraj, i, err
			}
			linearTraj = append(linearTraj, newTraj...)
		} else {
			subGoals, cbirrtAllowed, err := pm.generateWaypoints(ctx, start, to)
			if err != nil {
				return linearTraj, i, err
			}
			pm.pc.planMeta.SubgoalsPerGoal[i] = len(subGoals)

			if len(subGoals) > 1 {
				pm.logger.Debugf("\t generateWaypoint turned into %d subGoals cbirrtAllowed: %v", len(subGoals), cbirrtAllowed)
				pm.logger.Debugf("\t start: %v\n", start)
				pm.logger.Debugf("\t to   : %v\n", to)
				for _, sg := range subGoals {
					pm.logger.Debugf("\t\t sg: %v", sg)
				}
			}

			for subGoalIdx, sg := range subGoals {
				singleGoalStart := time.Now()
				newTraj, err := pm.planSingleGoal(ctx, linearTraj[len(linearTraj)-1], sg, cbirrtAllowed)
				if err != nil {
					pm.logger.Infof("\t subgoal %d failed after %v with: %v", subGoalIdx, time.Since(singleGoalStart), err)
					return linearTraj, i, err
				}
				pm.logger.Debugf("\t subgoal %d took %v", subGoalIdx, time.Since(singleGoalStart))
				linearTraj = append(linearTraj, newTraj...)
			}
		}
		start = to
	}

	return linearTraj, len(pm.request.Goals), nil
}

func (pm *planManager) planToDirectJoints(
	ctx context.Context,
	start *referenceframe.LinearInputs,
	goal *PlanState,
) ([]*referenceframe.LinearInputs, error) {
	ctx, span := trace.StartSpan(ctx, "planToDirectJoints")
	defer span.End()
	fullConfig := referenceframe.NewLinearInputs()
	for k, v := range goal.Configuration() {
		fullConfig.Put(k, v)
	}

	for k, v := range start.Items() {
		if len(fullConfig.Get(k)) == 0 {
			fullConfig.Put(k, v)
		}
	}

	goalPoses, err := goal.ComputePoses(ctx, pm.pc.fs)
	if err != nil {
		return nil, err
	}

	psc, err := NewPlanSegmentContext(ctx, pm.pc, start, goalPoses)
	if err != nil {
		return nil, err
	}

	err = psc.CheckPath(ctx, start, fullConfig, false, nil)
	if err == nil {
		return []*referenceframe.LinearInputs{fullConfig}, nil
	}

	pm.logger.Debugf("want to go to specific joint positions, but path is blocked: %v", err)
	_, err = psc.Checker.CheckStateFSConstraints(ctx, &motionplan.StateFS{
		Configuration: fullConfig,
		FS:            psc.pc.fs,
	})
	if err != nil {
		return nil, fmt.Errorf("want to go to specific joint config but it is invalid: %w", err)
	}

	pathPlanner, err := newCBiRRTMotionPlanner(ctx, pm.pc, psc, pm.logger.Sublogger("cbirrt"))
	if err != nil {
		return nil, err
	}

	maps := rrtMaps{}
	maps.startMap = rrtMap{&node{inputs: start}: nil}
	maps.goalMap = rrtMap{&node{inputs: fullConfig}: nil}
	maps.optNode = &node{inputs: fullConfig}

	finalSteps, err := pathPlanner.rrtRunner(ctx, &maps)
	if err != nil {
		return nil, err
	}
	finalSteps.steps, _, err = smoothPath(ctx, psc, finalSteps.steps)
	if err != nil {
		return nil, err
	}

	return finalSteps.steps, nil
}

func (pm *planManager) planSingleGoal(
	ctx context.Context,
	start *referenceframe.LinearInputs,
	goal referenceframe.FrameSystemPoses,
	cbirrtAllowed bool,
) ([]*referenceframe.LinearInputs, error) {
	ctx, span := trace.StartSpan(ctx, "planSingleGoal")
	defer span.End()
	pm.logger.Debug("start configuration", logging.FloatArrayFormat{"", start.GetLinearizedInputs()})
	pm.logger.Debug("going to", goal)

	psc, err := NewPlanSegmentContext(ctx, pm.pc, start, goal)
	if err != nil {
		return nil, err
	}

	for x := range goal {
		pm.logger.Debugf("start (%s) from %v", x, psc.startPoses[x])
	}

	planSeed, err := initRRTSolutions(ctx, psc, pm.logger.Sublogger("solve"))
	if err != nil {
		return nil, err
	}

	if planSeed.steps != nil {
		pm.logger.Debugf("found an ideal ik solution")
		pm.harvestPlan(psc, append([]*referenceframe.LinearInputs{psc.start}, planSeed.steps...), pm.logger)
		return planSeed.steps, nil
	}

	if !cbirrtAllowed {
		return nil, fmt.Errorf("linear with cbirrt not allowed and no direct solutions found")
	}

	pm.logger.Debugf("initRRTSolutions goalMap size: %d", len(planSeed.maps.goalMap))

	// Before paying for a full bidirectional RRT search, try repairing the
	// straight-line path with small nudges off whatever it grazes — the common
	// case for barely-blocked paths.
	if nudged := tryNudgedStraightLine(ctx, psc, planSeed.maps.goalMap, pm.pc.randseed, pm.logger.Sublogger("nudge")); nudged != nil {
		steps, compact, err := smoothPath(ctx, psc, nudged)
		if err == nil {
			pm.logger.Debugf("solved with nudged straight line: %d -> %d waypoints", len(nudged), len(steps))
			pm.pc.planMeta.GoalsNudgeSolved++
			pm.harvestPlan(psc, compact, pm.logger)
			return steps, nil
		}
		pm.logger.Debugf("nudged path failed to smooth, falling back to cbirrt: %v", err)
	}

	// Roadmap: reusable configuration-space graph with per-scene lazily
	// validated edges - including workspace bridge edges between joint
	// families. First query in a scene pays edge validation; later queries in
	// the same scene reuse the verdicts.
	goalRoots := make([]*node, 0, len(planSeed.maps.goalMap))
	for n, parent := range planSeed.maps.goalMap {
		if parent == nil {
			goalRoots = append(goalRoots, n)
		}
	}
	if path := pm.tryRoadmap(ctx, psc, goalRoots, pm.logger.Sublogger("roadmap")); path != nil {
		rm := getRoadmap(psc, pm.logger)
		sceneKey := uint64(0)
		if rm != nil {
			sceneKey = pm.roadmapSceneKey(psc, rm)
			// Replayed corridor in an unchanged scene: the smoothed and
			// close-obstacle-expanded trajectory is deterministic and was
			// validated when first computed - skip recomputing it.
			if cached := rm.cachedSmoothed(psc, sceneKey, path); cached != nil {
				pm.logger.Debugf("solved via roadmap (cached smooth): %d waypoints", len(cached))
				pm.pc.planMeta.GoalsRoadmapSolved++
				return cached, nil
			}
		}
		smoothed, compact, err := smoothPath(ctx, psc, path)
		if err == nil {
			pm.logger.Debugf("solved via roadmap: %d -> %d waypoints", len(path), len(smoothed))
			pm.pc.planMeta.GoalsRoadmapSolved++
			if rm != nil {
				rm.storeSmoothed(sceneKey, path, smoothed)
			}
			pm.harvestPlan(psc, compact, pm.logger)
			return smoothed, nil
		}
		pm.logger.Debugf("roadmap path failed to smooth, falling back: %v", err)
	}

	// Reconfiguration-vs-constraint detection: when every reachable goal
	// configuration is a large joint-space move while the end effector barely
	// moves AND a linear constraint pins the path to a narrow tube, the
	// reconfiguration swing almost certainly cannot stay inside the tube.
	// Search still gets one full racing round (the heuristic is not a proof),
	// but relaunching draws against a wall like that only burns the budget -
	// and the warning tells the caller what to actually fix.
	allowRelaunch := true
	if lc := tightestLinearConstraintMM(pm.request.Constraints); lc > 0 && planSeed.maps.optNode != nil {
		eeDelta := maxGoalTranslationMM(psc)
		if eeDelta >= 0 && eeDelta < 50 && lc <= 50 && planSeed.maps.optNode.cost > 1.5 {
			pm.logger.Warnf("goal is %0.1fmm of end-effector motion but the nearest valid goal configuration is "+
				"%0.2f rad of joint motion; a linear constraint of %0.0fmm likely cannot accommodate that "+
				"reconfiguration. Planning one search round only. Consider relaxing the constraint for this step "+
				"or approaching in a different joint configuration.",
				eeDelta, planSeed.maps.optNode.cost, lc)
			allowRelaunch = false
		}
	}

	rawSteps, err := pm.raceCBiRRT(ctx, psc, planSeed.maps, allowRelaunch)
	if err != nil {
		return nil, err
	}

	steps, compact, err := smoothPath(ctx, psc, rawSteps)
	if err != nil {
		return nil, err
	}

	pm.pc.planMeta.GoalsCBIRRTSolved++
	pm.harvestPlan(psc, compact, pm.logger)
	return steps, nil
}

// generateWaypoints will return the list of atomic waypoints that correspond to a specific goal in a plan request.
// bool is if cbirrt is allowed
func (pm *planManager) generateWaypoints(ctx context.Context, start, goal referenceframe.FrameSystemPoses,
) ([]referenceframe.FrameSystemPoses, bool, error) {
	_, span := trace.StartSpan(ctx, "generateWaypoints")
	defer span.End()
	if len(pm.request.Constraints.LinearConstraint) == 0 {
		return []referenceframe.FrameSystemPoses{goal}, true, nil
	}

	tighestConstraint := 10.0

	for _, lc := range pm.request.Constraints.LinearConstraint {
		tighestConstraint = min(tighestConstraint, lc.LineToleranceMm)
		tighestConstraint = min(tighestConstraint, lc.OrientationToleranceDegs)
	}

	tighestConstraint = max(tighestConstraint, 0)

	stepSize := defaultStepSizeMM / max(1, ((10-tighestConstraint)/2))
	pm.logger.Debugf("stepSize: %0.2f tighestConstraint: %0.2f", stepSize, tighestConstraint)

	numSteps := 0
	for frame, pif := range goal {
		startPIF, ok := start[frame]
		if !ok {
			return nil, true, fmt.Errorf("frame system broken?? %v and %v aren't connected?", frame, pif.Parent())
		}
		steps := motionplan.CalculateStepCount(startPIF.Pose(), pif.Pose(), stepSize)
		if steps > numSteps {
			numSteps = steps
		}
	}

	pm.logger.Debugf("numSteps: %d", numSteps)

	waypoints := []referenceframe.FrameSystemPoses{}

	for i := 1; i <= numSteps; i++ {
		by := float64(i) / float64(numSteps)
		to := referenceframe.FrameSystemPoses{}

		for frameName, pif := range goal {
			if start[frameName].Parent() != pif.Parent() {
				return nil, false, fmt.Errorf("frame mismatch %v %v", start[frameName].Parent(), pif.Parent())
			}
			toPose := spatialmath.Interpolate(start[frameName].Pose(), pif.Pose(), by)

			// Dan: We copy the PoseCloud over to each intermediate (and final goal) waypoint. We do
			// this thinking that if the PoseCloud describes a set of desirable orientations to make
			// IK easier, we would like all of the intermediate waypoints to also be more easily
			// solved for.
			to[frameName] = referenceframe.NewPoseInFrameWithGoalCloud(pif.Parent(), toPose, pif.GoalCloud)
		}

		waypoints = append(waypoints, to)
	}

	return waypoints, tighestConstraint >= 10, nil
}

type rrtMap map[*node]*node

type rrtSolution struct {
	steps []*referenceframe.LinearInputs
	maps  *rrtMaps
}

type rrtMaps struct {
	startMap rrtMap
	goalMap  rrtMap
	optNode  *node // The highest quality IK solution
}

// initRRTsolutions will create the maps to be used by a RRT-based algorithm. It will generate IK
// solutions to pre-populate the goal map, and will check if any of those goals are able to be
// directly interpolated to.
func initRRTSolutions(ctx context.Context, psc *PlanSegmentContext, logger logging.Logger) (*rrtSolution, error) {
	ctx, span := trace.StartSpan(ctx, "initRRTSolutions")
	defer span.End()
	rrt := &rrtSolution{
		maps: &rrtMaps{
			startMap: rrtMap{},
			goalMap:  rrtMap{},
		},
	}

	if psc.pc.planMeta.CollectSolutionDiagnostics {
		psc.pc.planMeta.PerGoal = append(psc.pc.planMeta.PerGoal, PerGoalMeta{})
	}

	seed := newConfigurationNode(psc.start)
	// goalNodes are sorted from lowest cost to highest.
	goalNodes, err := getSolutions(ctx, psc, logger)
	if err != nil {
		return rrt, err
	}

	rrt.maps.optNode = goalNodes[0]
	logger.Debugf("optNode cost: %v", rrt.maps.optNode.cost)
	// `defaultOptimalityMultiple` is > 1.0
	reasonableCost := max(.01, goalNodes[0].cost) * defaultOptimalityMultiple

	if psc.pc.planMeta.CollectSolutionDiagnostics {
		perGoal := &psc.pc.planMeta.PerGoal[len(psc.pc.planMeta.PerGoal)-1]
		perGoal.StartConfiguration = psc.start
		perGoal.GoalPoses = psc.goal
		perGoal.ReasonableCost = reasonableCost

		for _, goalNode := range goalNodes {
			perGoal.SolutionNodes = append(perGoal.SolutionNodes, SolutionNodeInfo{
				Score:          goalNode.cost,
				CheckPathError: goalNode.checkPathError,
				Inputs:         goalNode.inputs,
				LastGoodInputs: goalNode.checkPathFeedback.LastGoodInputs,
			})
		}
	}

	// Near-duplicate goal solutions (IK polishing the same joint family from
	// several seeds) add no reachability but multiply the work of everything
	// downstream - nudge target attempts, the roadmap's multi-goal A* edge
	// validations, cBiRRT goal trees. goalNodes is cost-sorted, so keeping
	// the first of each cluster keeps the best.
	const goalRootDedupSq = 0.25 // squared L2 rad; IK near-dupes are far under, families far over
	const maxGoalRoots = 16
	kept := make([][]float64, 0, maxGoalRoots)
	for _, solution := range goalNodes {
		if solution.cost > reasonableCost {
			// if it's this bad, we don't want for cbirrt or going straight
			continue
		}

		if solution.checkPath {
			// If we've already checked the path of a solution that is "reasonable", we can just
			// return now. Otherwise, continue to initialize goal map with keys.
			rrt.steps = []*referenceframe.LinearInputs{solution.inputs}
			return rrt, nil
		}
		if len(kept) >= maxGoalRoots {
			break
		}
		flat := solution.inputs.GetLinearizedInputs()
		dup := false
		for _, k := range kept {
			if flatL2Sq(k, flat) < goalRootDedupSq {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		kept = append(kept, flat)
		rrt.maps.goalMap[&node{inputs: solution.inputs, cost: solution.cost}] = nil
	}
	rrt.maps.startMap[&node{inputs: seed.inputs}] = nil

	return rrt, nil
}

// cbirrtRaceAttempts is how many independently-seeded cBiRRT searches run
// concurrently when the search phase is reached. Constrained searches have
// heavy-tailed run-to-run variance (samples on one captured scene span 18s to
// 139s under identical inputs); racing takes roughly the minimum of that
// distribution and turns the tail into the exception instead of the ruin of a
// plan. The searches share the plan's collision caches (concurrent-safe, and
// one racer's discoveries speed up the others) but use private RNG streams
// and tree maps.
var cbirrtRaceAttempts = utils.GetenvInt("CBIRRT_RACE_ATTEMPTS", 4)

// cloneRRTMaps gives one racing attempt its own tree maps. Nodes are shared
// (they are never mutated after insertion); only the map structure - tree
// membership and parentage - must be private per attempt.
func cloneRRTMaps(maps *rrtMaps) *rrtMaps {
	c := &rrtMaps{
		startMap: make(rrtMap, len(maps.startMap)),
		goalMap:  make(rrtMap, len(maps.goalMap)),
		optNode:  maps.optNode,
	}
	for k, v := range maps.startMap {
		c.startMap[k] = v
	}
	for k, v := range maps.goalMap {
		c.goalMap[k] = v
	}
	return c
}

// raceCBiRRT runs cbirrtRaceAttempts independent cBiRRT searches concurrently
// and returns the first solution found, cancelling the rest.
func (pm *planManager) raceCBiRRT(
	ctx context.Context,
	psc *PlanSegmentContext,
	maps *rrtMaps,
	allowRelaunch bool,
) ([]*referenceframe.LinearInputs, error) {
	attempts := max(1, cbirrtRaceAttempts)

	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type raceResult struct {
		steps   []*referenceframe.LinearInputs
		err     error
		attempt int
	}
	results := make(chan raceResult, attempts)

	// Seed the goal retreat corridors once on the master maps; every attempt's
	// clone inherits them, so restarts don't repeat the corridor IK.
	if seeder, err := newCBiRRTMotionPlanner(raceCtx, pm.pc, psc, pm.logger.Sublogger("corridors")); err == nil {
		seeder.seedGoalRetreatCorridors(raceCtx, maps)
	}
	// NOTE: pruning goal roots without corridors was tried here and reverted:
	// corridor presence is not connectability, and on the captured hard scene
	// it sometimes removed every root the search could actually reach,
	// converting a solvable plan into a timeout. The corridor-aiming already
	// biases racers toward corridor-bearing roots without excluding the rest.

	launch := func(a int) {
		go func(a int) {
			logger := pm.logger.Sublogger(fmt.Sprintf("cbirrt%d", a))
			planner, err := newCBiRRTMotionPlanner(raceCtx, pm.pc, psc, logger)
			if err != nil {
				results <- raceResult{nil, err, a}
				return
			}
			// Each attempt is a continuous full-budget search (restart-CHOPPING
			// - truncating attempts below the full iteration budget - was tried
			// and hurt: the trees are the asset, and successful searches
			// historically need several hundred iterations). Diversity comes
			// from the attempts' distinct RNG streams.
			//nolint:gosec
			planner.rnd = rand.New(rand.NewSource(int64(pm.pc.planOpts.RandomSeed) + int64(a)*7919))
			sol, err := planner.rrtRunner(raceCtx, cloneRRTMaps(maps))
			if err != nil {
				results <- raceResult{nil, err, a}
				return
			}
			results <- raceResult{sol.steps, nil, a}
		}(a)
	}
	for a := 0; a < attempts; a++ {
		launch(a)
	}

	// Search time is heavy-tailed: a full-budget attempt that exhausts its
	// iterations was simply a bad draw, and with request budget remaining the
	// slot is worth relaunching with a fresh stream and fresh tree clones.
	// Fresh trees also keep the linear nearest-neighbor scan fast - letting
	// one tree grow without bound makes each extend progressively slower.
	// Before this, racers could exhaust 5000 iterations in 20s of a 300s
	// request and fail the plan with 280s unused. maxTotalAttempts is a
	// backstop against tight failure loops, not a budget.
	const maxTotalAttempts = 64
	totalLaunched := attempts
	var firstErr error
	for inFlight := attempts; inFlight > 0; {
		r := <-results
		if r.err == nil {
			pm.logger.Debugf("cbirrt race: attempt %d finished first (%d raw nodes)", r.attempt, len(r.steps))
			return r.steps, nil
		}
		inFlight--
		if allowRelaunch && raceCtx.Err() == nil && totalLaunched < maxTotalAttempts {
			pm.logger.Debugf("cbirrt race: attempt %d exhausted (%v); relaunching as attempt %d", r.attempt, r.err, totalLaunched)
			launch(totalLaunched)
			totalLaunched++
			inFlight++
			continue
		}
		// Context cancellation of the losers is expected once someone wins;
		// remember only the first real failure.
		if firstErr == nil {
			firstErr = r.err
		}
	}
	return nil, firstErr
}

// tightestLinearConstraintMM returns the smallest line tolerance among the
// request's linear constraints, or 0 when none are set.
func tightestLinearConstraintMM(c *motionplan.Constraints) float64 {
	if c == nil {
		return 0
	}
	out := 0.0
	for _, lc := range c.LinearConstraint {
		if lc.LineToleranceMm > 0 && (out == 0 || lc.LineToleranceMm < out) {
			out = lc.LineToleranceMm
		}
	}
	return out
}

// maxGoalTranslationMM returns the largest straight-line translation any goal
// frame must cover for this segment, or -1 when it cannot be computed.
func maxGoalTranslationMM(psc *PlanSegmentContext) float64 {
	out := -1.0
	for f, gp := range psc.goal {
		sp, ok := psc.startPoses[f]
		if !ok {
			return -1
		}
		d := gp.Pose().Point().Sub(sp.Pose().Point()).Norm()
		if d > out {
			out = d
		}
	}
	return out
}
