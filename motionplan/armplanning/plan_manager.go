package armplanning

import (
	"context"
	"errors"
	"fmt"
	"time"

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

func newPlanManager(logger logging.Logger, request *PlanRequest, meta *PlanMeta) (*planManager, error) {
	pc, err := newPlanContext(logger, request, meta)
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
func (pm *planManager) planMultiWaypoint(ctx context.Context) (motionplan.Trajectory, int, error) {
	defer pm.pc.planMeta.DeferTiming("planMultiWaypoint", time.Now())
	// Theoretically, a plan could be made between two poses, by running IK on both the start and
	// end poses to create sets of seed and goal configurations. However, the blocker here is the
	// lack of a "known good" configuration used to determine which obstacles are allowed to collide
	// with one another.
	if pm.request.StartState.configuration == nil {
		return nil, 0, errors.New("must populate start state configuration if not planning for 2d base/tpspace")
	}

	// set timeout for entire planning process if specified
	var cancel func()
	if pm.request.PlannerOptions.Timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(pm.request.PlannerOptions.Timeout*float64(time.Second)))
	}
	if cancel != nil {
		defer cancel()
	}

	traj := motionplan.Trajectory{pm.request.StartState.Configuration()}
	now := time.Now()
	start, err := pm.request.StartState.ComputePoses(pm.request.FrameSystem)
	pm.pc.planMeta.AddTiming("ComputePoses", time.Since(now))
	if err != nil {
		return nil, 0, err
	}

	for i, g := range pm.request.Goals {
		if ctx.Err() != nil {
			return traj, i, err // note: here and below, we return traj because of ReturnPartialPlan
		}

		now = time.Now()
		to, err := g.ComputePoses(pm.request.FrameSystem)
		pm.pc.planMeta.AddTiming("ComputePoses", time.Since(now))
		if err != nil {
			return traj, i, err
		}

		pm.logger.Info("planning step", i, "of", len(pm.request.Goals))
		for k, v := range to {
			pm.logger.Debug(k, v)
		}

		if len(g.configuration) > 0 {
			newTraj, err := pm.planToDirectJoints(ctx, traj[len(traj)-1], g)
			if err != nil {
				return traj, i, err
			}
			traj = append(traj, newTraj...)
		} else {
			subGoals, err := pm.generateWaypoints(start, to)
			if err != nil {
				return traj, i, err
			}

			if len(subGoals) > 1 {
				pm.logger.Infof("\t generateWaypoint turned into %d subGoals", len(subGoals))
			}

			for subGoalIdx, sg := range subGoals {
				singleGoalStart := time.Now()
				newTraj, err := pm.planSingleGoal(ctx, traj[len(traj)-1], sg)
				if err != nil {
					return traj, i, err
				}
				pm.logger.Debug("\t subgoal %d took %v", subGoalIdx, time.Since(singleGoalStart))
				traj = append(traj, newTraj...)
			}
		}
		start = to
	}

	return traj, len(pm.request.Goals), nil
}

func (pm *planManager) planToDirectJoints(
	ctx context.Context,
	start referenceframe.FrameSystemInputs,
	goal *PlanState,
) ([]referenceframe.FrameSystemInputs, error) {
	defer pm.pc.planMeta.DeferTiming("planToDirectJoints", time.Now())
	fullConfig := referenceframe.FrameSystemInputs{}
	for k, v := range goal.configuration {
		fullConfig[k] = v
	}

	for k, v := range start {
		if len(fullConfig[k]) == 0 {
			fullConfig[k] = v
		}
	}

	now := time.Now()
	goalPoses, err := goal.ComputePoses(pm.pc.fs)
	pm.pc.planMeta.AddTiming("ComputePoses", time.Since(now))
	if err != nil {
		return nil, err
	}

	psc, err := newPlanSegmentContext(pm.pc, start, goalPoses)
	if err != nil {
		return nil, err
	}

	err = psc.checkPath(start, fullConfig)
	if err == nil {
		return []referenceframe.FrameSystemInputs{fullConfig}, nil
	}

	err = psc.checker.CheckStateFSConstraints(&motionplan.StateFS{
		Configuration: fullConfig,
		FS:            psc.pc.fs,
	})
	if err != nil {
		return nil, fmt.Errorf("want to go to specific joint config but it is invalid: %w", err)
	}

	pm.logger.Debugf("want to go to specific joint positions, but path is blocked: %v", err)

	pathPlanner, err := newCBiRRTMotionPlanner(pm.pc, psc)
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
	start referenceframe.FrameSystemInputs,
	goal referenceframe.FrameSystemPoses,
) ([]referenceframe.FrameSystemInputs, error) {
	defer pm.pc.planMeta.DeferTiming("planSingleGoal", time.Now())
	pm.logger.Debug("start configuration", start)
	pm.logger.Debug("going to", goal)

	psc, err := newPlanSegmentContext(pm.pc, start, goal)
	if err != nil {
		return nil, err
	}

	planSeed, err := initRRTSolutions(ctx, psc)
	if err != nil {
		return nil, err
	}

	pm.logger.Debugf("initRRTSolutions goalMap size: %d", len(planSeed.maps.goalMap))

	if planSeed.steps != nil {
		pm.logger.Debugf("found an ideal ik solution")
		return planSeed.steps, nil
	}

	pathPlanner, err := newCBiRRTMotionPlanner(pm.pc, psc)
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
func (pm *planManager) generateWaypoints(start, goal referenceframe.FrameSystemPoses) ([]referenceframe.FrameSystemPoses, error) {
	defer pm.pc.planMeta.DeferTiming("generateWaypoints", time.Now())
	if len(pm.request.Constraints.GetLinearConstraint()) == 0 {
		return []referenceframe.FrameSystemPoses{goal}, nil
	}

	stepSize := pm.request.PlannerOptions.PathStepSize

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
	steps []referenceframe.FrameSystemInputs
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
func initRRTSolutions(ctx context.Context, psc *planSegmentContext) (*rrtSolution, error) {
	defer psc.pc.planMeta.DeferTiming("initRRTSolutions", time.Now())
	rrt := &rrtSolution{
		maps: &rrtMaps{
			startMap: rrtMap{},
			goalMap:  rrtMap{},
		},
	}

	seed := newConfigurationNode(psc.start)
	// goalNodes are sorted from lowest cost to highest.
	goalNodes, err := getSolutions(ctx, psc)
	if err != nil {
		return rrt, err
	}

	rrt.maps.optNode = goalNodes[0]

	psc.pc.logger.Debugf("optNode cost: %v", rrt.maps.optNode.cost)

	// `defaultOptimalityMultiple` is > 1.0
	reasonableCost := goalNodes[0].cost * defaultOptimalityMultiple
	for _, solution := range goalNodes {
		if solution.checkPath && solution.cost < reasonableCost {
			// If we've already checked the path of a solution that is "reasonable", we can just
			// return now. Otherwise, continue to initialize goal map with keys.
			rrt.steps = []referenceframe.FrameSystemInputs{solution.inputs}
			return rrt, nil
		}

		rrt.maps.goalMap[&node{inputs: solution.inputs}] = nil
	}
	rrt.maps.startMap[&node{inputs: seed.inputs}] = nil

	return rrt, nil
}
