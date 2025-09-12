package armplanning

import (
	"context"
	"errors"
	"fmt"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
)

const (
	// Number of planner iterations before giving up.
	defaultPlanIter = 1500

	// The maximum percent of a joints range of motion to allow per step.
	defaultFrameStep = 0.01

	// If the dot product between two sets of configurations is less than this, consider them identical.
	defaultInputIdentDist = 0.0001

	// Number of iterations to run before beginning to accept randomly seeded locations.
	defaultIterBeforeRand = 50
)

type rrtParallelPlannerShared struct {
	maps            *rrtMaps
	endpointPreview chan node
	solutionChan    chan *rrtSolution
}

type rrtMap map[node]node

type rrtSolution struct {
	steps []node
	err   error
	maps  *rrtMaps
}

type rrtMaps struct {
	startMap rrtMap
	goalMap  rrtMap
	optNode  node // The highest quality IK solution
}

// initRRTsolutions will create the maps to be used by a RRT-based algorithm. It will generate IK solutions to pre-populate the goal
// map, and will check if any of those goals are able to be directly interpolated to.
// If the waypoint specifies poses for start or goal, IK will be run to create configurations.
func initRRTSolutions(ctx context.Context, wp atomicWaypoint) *rrtSolution {
	rrt := &rrtSolution{
		maps: &rrtMaps{
			startMap: map[node]node{},
			goalMap:  map[node]node{},
		},
	}

	if len(wp.startState.configuration) == 0 {
		rrt.err = fmt.Errorf("no configurations")
		return rrt
	}
	seed := newConfigurationNode(wp.startState.configuration)

	goalNodes, err := generateNodeListForPlanState(ctx, wp.mp, wp.goalState, wp.startState.configuration)
	if err != nil {
		rrt.err = err
		return rrt
	}

	configDistMetric := motionplan.GetConfigurationDistanceFunc(wp.mp.planOpts.ConfigurationDistanceMetric)

	// the smallest interpolated distance between the start and end input represents a lower bound on cost
	optimalCost := configDistMetric(&motionplan.SegmentFS{
		StartConfiguration: seed.Q(),
		EndConfiguration:   goalNodes[0].Q(),
	})
	rrt.maps.optNode = &basicNode{q: goalNodes[0].Q()}

	// Check for direct interpolation for the subset of IK solutions within some multiple of optimal
	// Since solutions are returned ordered, we check until one is out of bounds, then skip remaining checks
	canInterp := true

	// initialize maps and check whether direct interpolation is an option
	for _, solution := range goalNodes {
		if canInterp {
			cost := configDistMetric(
				&motionplan.SegmentFS{StartConfiguration: seed.Q(), EndConfiguration: solution.Q()},
			)
			if cost < optimalCost*defaultOptimalityMultiple {
				if wp.mp.checkPath(seed.Q(), solution.Q()) {
					wp.mp.logger.Debugf("found an ideal ik solution with cost %v < %v", cost, optimalCost*defaultOptimalityMultiple)
					rrt.steps = []node{seed, solution}
					return rrt
				}
			} else {
				canInterp = false
			}
		}
		rrt.maps.goalMap[&basicNode{q: solution.Q()}] = nil
	}
	rrt.maps.startMap[&basicNode{q: seed.Q()}] = nil

	return rrt
}

func newRRTPlan(solution []node, fs *referenceframe.FrameSystem) (motionplan.Plan, error) {
	if len(solution) == 0 {
		return nil, errors.New("cannot create plan, no solution was found")
	}

	if len(solution) == 1 {
		// started at the goal, nothing to do except make a trivial plan
		// do plans have to have 2 things ??
		solution = append(solution, solution[0])
	}

	traj := motionplan.Trajectory{}
	for _, n := range solution {
		traj = append(traj, n.Q())
	}

	return motionplan.NewSimplePlanFromTrajectory(traj, fs)
}
