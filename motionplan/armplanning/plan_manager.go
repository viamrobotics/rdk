package armplanning

import (
	"context"
	"fmt"
	"time"

	"go.viam.com/utils/trace"

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
func (pm *planManager) planMultiWaypoint(ctx context.Context) ([]*referenceframe.LinearInputs, error) {
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
		return nil, err
	}

	for i, g := range pm.request.Goals {
		if ctx.Err() != nil {
			return linearTraj, err // note: here and below, we return traj because of ReturnPartialPlan
		}

		to, err := g.ComputePoses(ctx, pm.request.FrameSystem)
		if err != nil {
			return linearTraj, err
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
				return linearTraj, err
			}
			linearTraj = append(linearTraj, newTraj...)
		} else {
			subGoals, cbirrtAllowed, err := pm.generateWaypoints(ctx, start, to)
			if err != nil {
				return linearTraj, err
			}

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
					return linearTraj, err
				}
				pm.logger.Debugf("\t subgoal %d took %v", subGoalIdx, time.Since(singleGoalStart))
				linearTraj = append(linearTraj, newTraj...)
			}
		}
		start = to
	}

	return linearTraj, nil
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

	err = psc.checkPath(ctx, start, fullConfig, false)
	if err == nil {
		return []*referenceframe.LinearInputs{fullConfig}, nil
	}

	pm.logger.Debugf("want to go to specific joint positions, but path is blocked: %v", err)
	_, err = psc.checker.CheckStateFSConstraints(ctx, &motionplan.StateFS{
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
	finalSteps.steps = smoothPath(ctx, psc, finalSteps.steps)
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

	psc, err := newPlanSegmentContext(ctx, pm.pc, start, goal)
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
		return planSeed.steps, nil
	}

	if !cbirrtAllowed {
		return nil, fmt.Errorf("linear with cbirrt not allowed and no direct solutions found")
	}

	pm.logger.Debugf("initRRTSolutions goalMap size: %d", len(planSeed.maps.goalMap))
	pathPlanner, err := newCBiRRTMotionPlanner(ctx, pm.pc, psc, pm.logger.Sublogger("cbirrt"))
	if err != nil {
		return nil, err
	}

	finalSteps, err := pathPlanner.rrtRunner(ctx, planSeed.maps)
	if err != nil {
		return nil, err
	}

	finalSteps.steps = smoothPath(ctx, psc, finalSteps.steps)
	return finalSteps.steps, nil
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
			to[frameName] = referenceframe.NewPoseInFrame(pif.Parent(), toPose)
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
func initRRTSolutions(ctx context.Context, psc *planSegmentContext, logger logging.Logger) (*rrtSolution, error) {
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
	goalNodes, err := getSolutions(ctx, psc, logger)
	if err != nil {
		return rrt, err
	}

	rrt.maps.optNode = goalNodes[0]
	logger.Debugf("optNode cost: %v", rrt.maps.optNode.cost)

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
			return rrt, nil
		}
		rrt.maps.goalMap[&node{inputs: solution.inputs}] = nil
	}
	rrt.maps.startMap[&node{inputs: seed.inputs}] = nil

	return rrt, nil
}
