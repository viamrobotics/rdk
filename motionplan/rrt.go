package motionplan

import(
	"context"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	// Number of planner iterations before giving up.
	defaultPlanIter = 2000
	
	defaultTimeout = 25.0
)

type RRTParallelPlanner interface{
	MotionPlanner
	RRTBackgroundRunner(context.Context, spatialmath.Pose,[]referenceframe.Input, *PlannerOptions, *rrtMaps, chan node, chan *rrtPlanReturn)
}

type rrtOptions struct {
	// Number of seconds before terminating planner
	Timeout float64 `json:"timeout"`
	
	// Number of planner iterations before giving up.
	PlanIter int `json:"plan_iter"`

	// Contains constraints, IK solving params, etc
	planOpts *PlannerOptions
}

func newRRTOptions(planOpts *PlannerOptions) *rrtOptions {
	return &rrtOptions{
		Timeout: defaultTimeout,
		PlanIter: defaultPlanIter,
		planOpts: planOpts,
	}
}

type rrtMap map[node]node

type rrtPlanner interface{
	
}

type rrtPlanReturn struct {
	steps []node
	err   error
	sp *rrtMaps
}

func (plan *rrtPlanReturn) ToInputs() [][]referenceframe.Input {
	inputs := make([][]referenceframe.Input, 0, len(plan.steps))
	for _, step := range plan.steps {
		inputs = append(inputs, step.Q())
	}
	return inputs
}

func (plan *rrtPlanReturn) Err() error {
	return plan.err
}

type rrtMaps struct {
	startMap map[node]node
	goalMap map[node]node
}

func shortestPath(sp *rrtMaps, nodePairs []*nodePair) *rrtPlanReturn {
	if len(nodePairs) == 0 {
		return &rrtPlanReturn{err: errPlannerFailed, sp: sp}
	}
	minIdx := 0
	minDist := nodePairs[0].sumCosts()
	for i := 1; i < len(nodePairs); i++ {
		if dist := nodePairs[i].sumCosts(); dist < minDist {
			minDist = dist
			minIdx = i
		}
	}
	return &rrtPlanReturn{steps: extractPath(sp.startMap, sp.goalMap, nodePairs[minIdx])}
}
