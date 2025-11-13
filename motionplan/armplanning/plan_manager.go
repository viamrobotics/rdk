package armplanning

import (
	"context"
	"fmt"
	"time"

	"go.opencensus.io/trace"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// planManager is intended to be the single entry point to motion planners.
type planManager struct {
	pc      *planContext
	request *PlanRequest
	logger  logging.Logger
}

func newPlanManager(ctx context.Context, logger logging.Logger, request *PlanRequest, meta *PlanMeta) (*planManager, error) {
	pc, err := newPlanContext(ctx, logger, request, meta)
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

	bgGens := make([]*backgroundGenerator, 0, len(pm.request.Goals))
	defer func() {
		for idx := range bgGens {
			bgGens[idx].StopAndWait()
		}
	}()

	for i, g := range pm.request.Goals {
		if ctx.Err() != nil {
			return linearTraj, i, err // note: here and below, we return traj because of ReturnPartialPlan
		}

		to, err := g.ComputePoses(ctx, pm.request.FrameSystem)
		if err != nil {
			return linearTraj, i, err
		}

		pm.logger.Info("planning step", i, "of", len(pm.request.Goals))
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
			subGoals, err := pm.generateWaypoints(ctx, start, to)
			if err != nil {
				return linearTraj, i, err
			}

			if len(subGoals) > 1 {
				pm.logger.Infof("\t generateWaypoint turned into %d subGoals", len(subGoals))
			}

			for subGoalIdx, sg := range subGoals {
				singleGoalStart := time.Now()
				newTraj, bgGen, err := pm.planSingleGoal(ctx, linearTraj[len(linearTraj)-1], sg)
				if err != nil {
					return linearTraj, i, err
				}

				pm.logger.Debugf("\t subgoal %d took %v", subGoalIdx, time.Since(singleGoalStart))
				linearTraj = append(linearTraj, newTraj...)
				bgGens = append(bgGens, bgGen)
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

	psc, err := newPlanSegmentContext(ctx, pm.pc, start, goalPoses)
	if err != nil {
		return nil, err
	}

	err = psc.checkPath(ctx, start, fullConfig)
	if err == nil {
		return []*referenceframe.LinearInputs{fullConfig}, nil
	}

	pm.logger.Debugf("want to go to specific joint positions, but path is blocked: %v", err)
	err = psc.checker.CheckStateFSConstraints(ctx, &motionplan.StateFS{
		Configuration: fullConfig,
		FS:            psc.pc.fs,
	})
	if err != nil {
		return nil, fmt.Errorf("want to go to specific joint config but it is invalid: %w", err)
	}

	if false { // true cartesian half
		// TODO(eliot): finish me
		startPoses, err := start.ComputePoses(pm.pc.fs)
		if err != nil {
			return nil, err
		}

		mid := interp(startPoses, goalPoses, .5)

		pm.logger.Infof("foo things\n\t%v\n\t%v\n\t%v", startPoses, mid, goalPoses)

		err = pm.foo(ctx, start, mid)
		if err != nil {
			pm.logger.Infof("foo failed: %v", err)
		} else {
			panic(2)
		}

		// panic(1)
	}

	pathPlanner, err := newCBiRRTMotionPlanner(ctx, pm.pc, psc)
	if err != nil {
		return nil, err
	}

	maps := rrtMaps{}
	maps.startMap = rrtMap{&node{name: -1, inputs: start}: nil}
	maps.goalMap = rrtMap{&node{name: -2, inputs: fullConfig}: nil}
	maps.optNode = &node{name: -3, inputs: fullConfig}

	// Pass in a non-nil `backgroundGenerator`. To avoid needing to nil check inside `rrtRunner`. It
	// will never require the backgroundGenerator channel to be useful.
	finalSteps, err := pathPlanner.rrtRunner(ctx, &maps, &backgroundGenerator{})
	if err != nil {
		return nil, err
	}
	finalSteps.steps = smoothPath(ctx, psc, finalSteps.steps)
	return finalSteps.steps, nil
}

func (pm *planManager) planSingleGoal(
	ctx context.Context,
	start *referenceframe.LinearInputs,
	goal referenceframe.FrameSystemPoses,
) ([]*referenceframe.LinearInputs, *backgroundGenerator, error) {
	ctx, span := trace.StartSpan(ctx, "planSingleGoal")
	defer span.End()
	pm.logger.Debug("start configuration", start)
	pm.logger.Debug("going to", goal)

	psc, err := newPlanSegmentContext(ctx, pm.pc, start, goal)
	if err != nil {
		return nil, nil, err
	}

	// For error cases, we'll wait for `bgGen` to drain before returning. Otherwise we'll `Stop` the
	// generator, but the caller is responsible for waiting.
	//
	// This optimization is useful for cases where we plan for dozens of goals, and each planning
	// each goal is very fast. Each "wait" can be up to ~2ms. Hence we'd rather do one big wait at
	// the end. See `data/wine-adjust.json`.
	planSeed, bgGen, err := initRRTSolutions(ctx, psc)
	if err != nil {
		bgGen.StopAndWait()
		return nil, nil, err
	}

	if planSeed.steps != nil {
		pm.logger.Debug("found an ideal ik solution")
		bgGen.Stop()
		return planSeed.steps, bgGen, nil
	}

	pm.logger.Debugf("initRRTSolutions goalMap size: %d", len(planSeed.maps.goalMap))
	pathPlanner, err := newCBiRRTMotionPlanner(ctx, pm.pc, psc)
	if err != nil {
		bgGen.StopAndWait()
		return nil, nil, err
	}

	finalSteps, err := pathPlanner.rrtRunner(ctx, planSeed.maps, bgGen)
	if err != nil {
		bgGen.StopAndWait()
		return nil, nil, err
	}

	// Nothing left for IK to do. Cancel it and let goroutines exit while we do smoothing.
	bgGen.Stop()

	finalSteps.steps = smoothPath(ctx, psc, finalSteps.steps)
	return finalSteps.steps, bgGen, nil
}

// generateWaypoints will return the list of atomic waypoints that correspond to a specific goal in a plan request.
func (pm *planManager) generateWaypoints(ctx context.Context, start, goal referenceframe.FrameSystemPoses,
) ([]referenceframe.FrameSystemPoses, error) {
	_, span := trace.StartSpan(ctx, "generateWaypoints")
	defer span.End()
	if len(pm.request.Constraints.GetLinearConstraint()) == 0 {
		return []referenceframe.FrameSystemPoses{goal}, nil
	}

	tighestConstraint := 10.0

	for _, lc := range pm.request.Constraints.GetLinearConstraint() {
		tighestConstraint = min(tighestConstraint, lc.LineToleranceMm)
		tighestConstraint = min(tighestConstraint, lc.OrientationToleranceDegs)
	}

	tighestConstraint = max(tighestConstraint, 0)

	stepSize := defaultStepSizeMM / max(1, ((10-tighestConstraint)/2))
	pm.logger.Debugf("stepSize: %0.2f", stepSize)

	numSteps := 0
	for frame, pif := range goal {
		steps := motionplan.CalculateStepCount(start[frame].Pose(), pif.Pose(), stepSize)
		if steps > numSteps {
			numSteps = steps
		}
	}

	pm.logger.Debugf("numSteps: %d", numSteps)

	waypoints := []referenceframe.FrameSystemPoses{}

	from := start

	for i := 1; i <= numSteps; i++ {
		by := float64(i) / float64(numSteps)
		to := referenceframe.FrameSystemPoses{}

		for frameName, pif := range goal {
			if from[frameName].Parent() != pif.Parent() {
				return nil, fmt.Errorf("frame mismatch %v %v", from[frameName].Parent(), pif.Parent())
			}
			toPose := spatialmath.Interpolate(from[frameName].Pose(), pif.Pose(), by)
			to[frameName] = referenceframe.NewPoseInFrame(pif.Parent(), toPose)
		}

		waypoints = append(waypoints, to)

		from = to
	}

	return waypoints, nil
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
func initRRTSolutions(ctx context.Context, psc *planSegmentContext) (*rrtSolution, *backgroundGenerator, error) {
	ctx, span := trace.StartSpan(ctx, "initRRTSolutions")
	defer span.End()
	rrt := &rrtSolution{
		maps: &rrtMaps{
			startMap: rrtMap{},
			goalMap:  rrtMap{},
		},
	}

	seed := newConfigurationNode(psc.start)
	// goalNodes are sorted from lowest cost to highest.
	goalNodes, goalNodeGenerator, err := getSolutions(ctx, psc)
	if err != nil {
		return rrt, nil, err
	}

	rrt.maps.optNode = goalNodes[0]

	psc.pc.logger.Debugf("optNode cost: %v", rrt.maps.optNode.cost)

	// `defaultOptimalityMultiple` is > 1.0
	reasonableCost := max(.01, goalNodes[0].cost) * defaultOptimalityMultiple
	for _, solution := range goalNodes {
		if solution.cost > reasonableCost {
			// if it's this bad, we don't want for cbirrt or going straight
			continue
		}

		if solution.checkPath {
			// If we've already checked the path of a solution that is "reasonable", we can just
			// return now. Otherwise, continue to initialize goal map with keys.
			rrt.steps = []*referenceframe.LinearInputs{solution.inputs}
			return rrt, goalNodeGenerator, nil
		}

		rrt.maps.goalMap[&node{name: solution.name, goalNode: solution.goalNode, inputs: solution.inputs}] = nil
	}
	rrt.maps.startMap[&node{name: -5, inputs: seed.inputs}] = nil

	return rrt, goalNodeGenerator, nil
}

func interp(start, end referenceframe.FrameSystemPoses, delta float64) referenceframe.FrameSystemPoses {
	mid := referenceframe.FrameSystemPoses{}

	for k, s := range start {
		e, ok := end[k]
		if !ok {
			mid[k] = s
			continue
		}
		if s.Parent() != e.Parent() {
			panic("eliottttt")
		}
		m := spatialmath.Interpolate(s.Pose(), e.Pose(), delta)
		mid[k] = referenceframe.NewPoseInFrame(s.Parent(), m)
	}
	return mid
}

func (pm *planManager) foo(ctx context.Context, start *referenceframe.LinearInputs, goal referenceframe.FrameSystemPoses) error {
	psc, err := newPlanSegmentContext(ctx, pm.pc, start, goal)
	if err != nil {
		return err
	}

	planSeed, bgGen, err := initRRTSolutions(ctx, psc)
	if err != nil {
		return err
	}
	bgGen.StopAndWait()

	if planSeed.steps == nil {
		return fmt.Errorf("no steps")
	}

	if len(planSeed.steps) != 1 {
		return fmt.Errorf("steps odd %d", len(planSeed.steps))
	}

	err = psc.checkPath(ctx, start, planSeed.steps[0])
	if err != nil {
		return err
	}

	panic(5)
}
