package armplanning

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// planManager is intended to be the single entry point to motion planners.
type planManager struct {
	boundingRegions []spatialmath.Geometry

	randseed     *rand.Rand

	logger logging.Logger

	// We store the request because we want to be able to inspect the original state of the plan
	// that was requested at any point during the process of creating multiple planners
	// for waypoints and such.
	request                 *PlanRequest
}

type planContext struct {
	fs                        *referenceframe.FrameSystem
	lfs                       *linearizedFrameSystem

	configurationDistanceFunc motionplan.SegmentFSMetric
	planOpts                  *PlannerOptions
	
	randseed                  *rand.Rand
	
	logger                    logging.Logger

}

type planSegmentContext struct {
	start referenceframe.FrameSystemInputs
	goal referenceframe.FrameSystemPoses


	checker                   *motionplan.ConstraintChecker
	motionChains              *motionChains
}

func newPlanManager(logger logging.Logger, request *PlanRequest) (*planManager, error) {
	boundingRegions, err := referenceframe.NewGeometriesFromProto(request.BoundingRegions)
	if err != nil {
		return nil, err
	}

	return &planManager{
		boundingRegions: boundingRegions,
		randseed:     rand.New(rand.NewSource(int64(request.PlannerOptions.RandomSeed))), //nolint:gosec
		logger:       logger,
		request:      request,
	}, nil
}


// planMultiWaypoint plans a motion through multiple waypoints, using identical constraints for each
// Any constraints, etc, will be held for the entire motion.
func (pm *planManager) planMultiWaypoint(ctx context.Context) (motionplan.Plan, error) {
	// Theoretically, a plan could be made between two poses, by running IK on both the start and
	// end poses to create sets of seed and goal configurations. However, the blocker here is the
	// lack of a "known good" configuration used to determine which obstacles are allowed to collide
	// with one another.
	if pm.request.StartState.configuration == nil {
		return nil, errors.New("must populate start state configuration if not planning for 2d base/tpspace")
	}

	// set timeout for entire planning process if specified
	var cancel func()
	if pm.request.PlannerOptions.Timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(pm.request.PlannerOptions.Timeout*float64(time.Second)))
	}
	if cancel != nil {
		defer cancel()
	}

	goals := []referenceframe.FrameSystemPoses{}

	start, err := pm.request.StartState.ComputePoses(pm.request.FrameSystem)
	if err != nil {
		return nil, err
	}

	for _, g := range pm.request.Goals {
		to, err := g.ComputePoses(pm.request.FrameSystem)
		if err != nil {
			return nil, err
		}
		subGoals, err := pm.generateWaypoints(start, to)
		if err != nil {
			return nil, err
		}
		goals = append(goals, subGoals...)
		start = to
	}

	pm.logger.Debugf("planMultiWaypoint orig goals:%v total goals:%v\n", len(pm.request.Goals), len(goals))

	return pm.planAtomicWaypoints(ctx, goals)
}

// planAtomicWaypoints will plan a single motion, which may be composed of one or more waypoints. Waypoints are here used to begin planning
// the next motion as soon as its starting point is known. This is responsible for repeatedly calling planSingleAtomicWaypoint for each
// intermediate waypoint. Waypoints here refer to points that the software has generated to.
func (pm *planManager) planAtomicWaypoints(ctx context.Context, goals []referenceframe.FrameSystemPoses) (motionplan.Plan, error) {

	traj := motionplan.Trajectory{pm.request.StartState.Configuration()}

	var err error
	var newTraj []referenceframe.FrameSystemInputs
	
	for i, wp := range goals {
		if ctx.Err() != nil {
			err = ctx.Err()
			break
		}

		pm.logger.Info("planning step", i, "of", len(goals))
		for k, v := range wp {
			pm.logger.Info(k, v)
		}


		newTraj, err = pm.planSingleAtomicWaypoint(ctx, traj[len(traj)-1], wp)
		if err != nil {
			break
		}
		traj = append(traj, newTraj...)
	}

	if err != nil {
		if pm.request.PlannerOptions.ReturnPartialPlan {
			pm.logger.Infof("returning partial plan")
		} else {
			return nil, err
		}
	}

	return motionplan.NewSimplePlanFromTrajectory(traj, pm.request.FrameSystem)
}

// planSingleAtomicWaypoint attempts to plan a single waypoint. It may optionally be pre-seeded with rrt maps; these will be passed to the
// planner if supported, or ignored if not.
func (pm *planManager) planSingleAtomicWaypoint(
	ctx context.Context,
	start referenceframe.FrameSystemInputs,
	goal referenceframe.FrameSystemPoses,
) ([]referenceframe.FrameSystemInputs, error) {
	pm.logger.Debug("start configuration", start)
	pm.logger.Debug("going to",  goal)

	planSeed, err := initRRTSolutions(ctx, start, goal)
	if err != nil {
		return nil, err
	}
	
	pm.logger.Debugf("initRRTSolutions goalMap size: %d", len(planSeed.maps.goalMap))

	if planSeed.steps != nil {
		pm.logger.Debugf("found an ideal ik solution")
		return planSeed.steps, nil
	}

	motionChains, err := motionChainsFromPlanState(pm.request.FrameSystem, goal)
	if err != nil {
		return nil, err
	}

	startPoses, err := start.ComputePoses(pm.request.FrameSystem)
	if err != nil {
		return nil, err
	}
	
	constraintHandler, err := newConstraintChecker(
		pm.request.PlannerOptions,
		pm.request.Constraints,
		startPoses,
		goal,
		pm.request.FrameSystem,
		motionChains,
		pm.request.StartState.configuration,
		pm.request.WorldState,
		pm.boundingRegions,
	)
	if err != nil {
		return nil, err
	}
	
	pathPlanner, err := newCBiRRTMotionPlanner(
		pm.request.FrameSystem,
		rand.New(rand.NewSource(int64(pm.randseed.Int()))), //nolint: gosec
		pm.logger,
		pm.request.PlannerOptions,
		constraintHandler,
		motionChains,
	)

	finalSteps, err := pathPlanner.rrtRunner(ctx, planSeed.maps)
	if err != nil {
		return nil, err
	}
	finalSteps.steps = pathPlanner.smoothPath(ctx, finalSteps.steps)
	return finalSteps.steps, nil
}

// generateWaypoints will return the list of atomic waypoints that correspond to a specific goal in a plan request.
func (pm *planManager) generateWaypoints(start, goal referenceframe.FrameSystemPoses) ([]referenceframe.FrameSystemPoses, error) {
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
// directly interpolated to.  If the waypoint specifies poses for start or goal, IK will be run to
// create configurations.
func initRRTSolutions(ctx context.Context, start referenceframe.FrameSystemInputs, goal referenceframe.FrameSystemPoses) (*rrtSolution, error) {
	rrt := &rrtSolution{
		maps: &rrtMaps{
			startMap: rrtMap{},
			goalMap:  rrtMap{},
		},
	}

	seed := newConfigurationNode(start)
	goalNodes, err := generateNodeListForGoalState(ctx, wp.motionPlanner, goal, start)
	if err != nil {
		return rrt, err
	}

	rrt.maps.optNode = goalNodes[0]
	for _, solution := range goalNodes {
		if solution.checkPath && solution.cost < goalNodes[0].cost*defaultOptimalityMultiple {
			rrt.steps = []referenceframe.FrameSystemInputs{solution.inputs}
			return rrt, nil
		}

		rrt.maps.goalMap[&node{inputs: solution.inputs}] = nil
	}
	rrt.maps.startMap[&node{inputs: seed.inputs}] = nil

	return rrt, nil
}
