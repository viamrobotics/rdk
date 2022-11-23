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
	planOpts        *plannerOptions
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
	planOpts *plannerOptions
}

func newRRTOptions(planOpts *plannerOptions) *rrtOptions {
	return &rrtOptions{
		PlanIter: defaultPlanIter,
		planOpts: planOpts,
	}
}

type rrtMap map[node]node

type rrtPlanReturn struct {
	steps   []node
	planerr error
	rm      *rrtMaps
	optimal float64
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
}

func initRRTMaps() *rrtMaps {
	return &rrtMaps{
		startMap: map[node]node{},
		goalMap:  map[node]node{},
	}
}

func shortestPath(rm *rrtMaps, nodePairs []*nodePair, optimalCost float64) *rrtPlanReturn {
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
	return &rrtPlanReturn{steps: extractPath(rm.startMap, rm.goalMap, nodePairs[minIdx]), rm: rm, optimal: optimalCost}
}
