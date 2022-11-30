package motionplan

import (
	"context"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	// Number of planner iterations before giving up.
	defaultPlanIter = 20000
)

type rrtParallelPlanner interface {
	motionPlanner
	rrtBackgroundRunner(context.Context, spatialmath.Pose, []referenceframe.Input, *rrtParallelPlannerShared)
}

type rrtParallelPlannerShared struct {
	rm              *rrtMaps
	endpointPreview chan node
	solutionChan    chan *rrtPlanReturn
}

type rrtOptions struct {
	// Number of planner iterations before giving up.
	PlanIter int `json:"plan_iter"`

	// Number of CPU cores to use for RRT*
	Ncpu int `json:"ncpu"`

	// Contains constraints, IK solving params, etc
	*plannerOptions
}

func newRRTOptions(planOpts *plannerOptions) *rrtOptions {
	return &rrtOptions{
		PlanIter:       defaultPlanIter,
		plannerOptions: planOpts,
	}
}

type rrtMap map[node]node

type rrtPlanReturn struct {
	steps   []node
	planerr error
	rm      *rrtMaps
}

func (plan *rrtPlanReturn) toInputs() [][]referenceframe.Input {
	inputs := make([][]referenceframe.Input, 0, len(plan.steps))
	for _, step := range plan.steps {
		inputs = append(inputs, step.Q())
	}
	return inputs
}

func (plan *rrtPlanReturn) err() error {
	return plan.planerr
}

type rrtMaps struct {
	startMap rrtMap
	goalMap  rrtMap
	optNode  *costNode // The highest quality IK solution
}

func initRRTMaps(ctx context.Context, mp motionPlanner, goal spatialmath.Pose, seed []referenceframe.Input) *rrtPlanReturn {
	rrt := &rrtPlanReturn{
		rm: &rrtMaps{
			startMap: map[node]node{},
			goalMap:  map[node]node{},
		},
	}
	seedNode := newCostNode(seed, 0)
	rrt.rm.startMap[seedNode] = nil

	// get many potential end goals from IK solver
	solutions, err := mp.getSolutions(ctx, goal, seed)
	if err != nil {
		rrt.planerr = err
		return rrt
	}
	mp.golog().Debugf("found %d IK solutions", len(solutions))

	// the smallest interpolated distance between the start and end input represents a lower bound on cost
	_, optimalCost := mp.opt().DistanceFunc(&ConstraintInput{StartInput: seed, EndInput: solutions[0].Q()})
	rrt.rm.optNode = newCostNode(solutions[0].Q(), optimalCost)

	// Check for direct interpolation for the subset of IK solutions within some multiple of optimal
	// Since solutions are returned ordered, we check until one is out of bounds, then skip remaining checks
	canInterp := true
	// initialize maps and check whether direct interpolation is an option
	for _, solution := range solutions {
		if canInterp {
			_, cost := mp.opt().DistanceFunc(&ConstraintInput{StartInput: seed, EndInput: solution.Q()})
			if cost < optimalCost*defaultOptimalityMultiple {
				if mp.checkPath(seed, solution.Q()) {
					mp.golog().Debug("could interpolate directly to goal")
					rrt.steps = []node{seedNode, solution}
					return rrt
				}
			} else {
				canInterp = false
			}
		}
		rrt.rm.goalMap[newCostNode(solution.Q(), 0)] = nil
	}
	mp.golog().Debugf("failed to directly interpolate from %v to %v", seed, solutions[0].Q())
	return rrt
}

func shortestPath(rm *rrtMaps, nodePairs []*nodePair) *rrtPlanReturn {
	if len(nodePairs) == 0 {
		return &rrtPlanReturn{planerr: errPlannerFailed, rm: rm}
	}
	minIdx := 0
	minDist := nodePairs[0].sumCosts()
	for i := 1; i < len(nodePairs); i++ {
		if dist := nodePairs[i].sumCosts(); dist < minDist {
			minDist = dist
			minIdx = i
		}
	}
	return &rrtPlanReturn{steps: extractPath(rm.startMap, rm.goalMap, nodePairs[minIdx]), rm: rm}
}
