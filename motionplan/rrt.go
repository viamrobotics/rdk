//go:build !no_cgo

package motionplan

import (
	"context"
	"errors"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	// Number of planner iterations before giving up.
	defaultPlanIter = 3000

	// The maximum percent of a joints range of motion to allow per step.
	defaultFrameStep = 0.01

	// If the dot product between two sets of configurations is less than this, consider them identical.
	defaultInputIdentDist = 0.0001

	// Number of iterations to run before beginning to accept randomly seeded locations.
	defaultIterBeforeRand = 50
)

type rrtParallelPlanner interface {
	motionPlanner
	rrtBackgroundRunner(context.Context, *rrtParallelPlannerShared)
}

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

func (maps *rrtMaps) fillPosOnlyGoal(goal referenceframe.FrameSystemPoses, posSeeds int) error {
	thetaStep := 360. / float64(posSeeds)
	if maps == nil {
		return errors.New("cannot call method fillPosOnlyGoal on nil maps")
	}
	if maps.goalMap == nil {
		maps.goalMap = map[node]node{}
	}
	for i := 0; i < posSeeds; i++ {
		newMap := referenceframe.FrameSystemPoses{}
		for frame, goal := range goal {
			newMap[frame] = referenceframe.NewPoseInFrame(
				frame,
				spatialmath.NewPose(goal.Pose().Point(), &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: float64(i) * thetaStep}),
			)
		}

		goalNode := &basicNode{
			q:     make(referenceframe.FrameSystemInputs),
			poses: newMap,
		}
		maps.goalMap[goalNode] = nil
	}
	return nil
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

	startNodes, err := generateNodeListForPlanState(ctx, wp.mp, wp.startState, wp.goalState.configuration)
	if err != nil {
		rrt.err = err
		return rrt
	}
	goalNodes, err := generateNodeListForPlanState(ctx, wp.mp, wp.goalState, wp.startState.configuration)
	if err != nil {
		rrt.err = err
		return rrt
	}

	// the smallest interpolated distance between the start and end input represents a lower bound on cost
	optimalCost := wp.mp.opt().configurationDistanceFunc(&ik.SegmentFS{
		StartConfiguration: startNodes[0].Q(),
		EndConfiguration:   goalNodes[0].Q(),
	})
	rrt.maps.optNode = &basicNode{q: goalNodes[0].Q(), cost: optimalCost}

	// Check for direct interpolation for the subset of IK solutions within some multiple of optimal
	// Since solutions are returned ordered, we check until one is out of bounds, then skip remaining checks
	canInterp := true
	// initialize maps and check whether direct interpolation is an option
	for _, seed := range startNodes {
		for _, solution := range goalNodes {
			if canInterp {
				cost := wp.mp.opt().configurationDistanceFunc(&ik.SegmentFS{StartConfiguration: seed.Q(), EndConfiguration: solution.Q()})
				if cost < optimalCost*defaultOptimalityMultiple {
					if wp.mp.checkPath(seed.Q(), solution.Q()) {
						rrt.steps = []node{seed, solution}
						return rrt
					}
				} else {
					canInterp = false
				}
			}
			rrt.maps.goalMap[&basicNode{q: solution.Q(), cost: 0}] = nil
		}
		rrt.maps.startMap[&basicNode{q: seed.Q(), cost: 0}] = nil
	}
	return rrt
}

func shortestPath(maps *rrtMaps, nodePairs []*nodePair) *rrtSolution {
	if len(nodePairs) == 0 {
		return &rrtSolution{err: errPlannerFailed, maps: maps}
	}
	minIdx := 0
	minDist := nodePairs[0].sumCosts()
	for i := 1; i < len(nodePairs); i++ {
		if dist := nodePairs[i].sumCosts(); dist < minDist {
			minDist = dist
			minIdx = i
		}
	}
	return &rrtSolution{steps: extractPath(maps.startMap, maps.goalMap, nodePairs[minIdx], true), maps: maps}
}

type rrtPlan struct {
	SimplePlan

	// nodes corresponding to inputs can be cached with the Plan for easy conversion back into a form usable by RRT
	// depending on how the trajectory is constructed these may be nil and should be computed before usage
	nodes []node
}

func newRRTPlan(solution []node, fs referenceframe.FrameSystem, relative bool, offsetPose spatialmath.Pose) (Plan, error) {
	if len(solution) < 2 {
		if len(solution) == 1 {
			// Started at the goal, nothing to do
			solution = append(solution, solution[0])
		} else {
			return nil, errors.New("cannot construct a Plan using fewer than two nodes")
		}
	}
	traj := nodesToTrajectory(solution)
	path, err := newPath(solution, fs)
	if err != nil {
		return nil, err
	}
	if relative {
		path, err = newPathFromRelativePath(path)
		if err != nil {
			return nil, err
		}
	}
	var plan Plan
	plan = &rrtPlan{SimplePlan: *NewSimplePlan(path, traj), nodes: solution}
	if relative {
		plan = OffsetPlan(plan, offsetPose)
	}
	return plan, nil
}
