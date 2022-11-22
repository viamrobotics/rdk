package motionplan

import (
	"context"
	"math/rand"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// rrtConnectMotionPlanner is an object able to quickly solve for valid paths around obstacles to some goal for a given referenceframe.
// It uses the RRT-Connect algorithm, Kuffner & LaValle 2000
// https://ieeexplore.ieee.org/document/844730
type rrtConnectMotionPlanner struct{ *planner }

// NewRRTConnectMotionPlanner creates a rrtConnectMotionPlanner object.
func NewRRTConnectMotionPlanner(frame referenceframe.Frame, nCPU int, logger golog.Logger) (MotionPlanner, error) {
	//nolint:gosec
	return NewRRTConnectMotionPlannerWithSeed(frame, nCPU, rand.New(rand.NewSource(1)), logger)
}

// NewRRTConnectMotionPlannerWithSeed creates a rrtConnectMotionPlanner object with a user specified random seed.
func NewRRTConnectMotionPlannerWithSeed(frame referenceframe.Frame, nCPU int, seed *rand.Rand, logger golog.Logger) (MotionPlanner, error) {
	planner, err := newPlanner(frame, nCPU, seed, logger)
	if err != nil {
		return nil, err
	}
	return &rrtConnectMotionPlanner{planner}, nil
}

func (mp *rrtConnectMotionPlanner) Plan(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
	planOpts *PlannerOptions,
) ([][]referenceframe.Input, error) {
	if planOpts == nil {
		planOpts = NewBasicPlannerOptions()
	}
	solutionChan := make(chan *planReturn, 1)
	utils.PanicCapturingGo(func() {
		mp.planRunner(ctx, goal, seed, planOpts, nil, solutionChan)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case plan := <-solutionChan:
		return plan.toInputs(), plan.err
	}
}

// planRunner will execute the plan. When Plan() is called, it will call planRunner in a separate thread and wait for the results.
// Separating this allows other things to call planRunner in parallel while also enabling the thread-agnostic Plan to be accessible.
func (mp *rrtConnectMotionPlanner) planRunner(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
	planOpts *PlannerOptions,
	endpointPreview chan node,
	solutionChan chan *planReturn,
) {
	defer close(solutionChan)

	// setup planner options
	if planOpts == nil {
		solutionChan <- &planReturn{err: errNoPlannerOptions}
		return
	}
	algOpts := newRRTOptions(planOpts)

	// get many potential end goals from IK solver
	solutions, err := getSolutions(ctx, planOpts, mp.solver, goal, seed, mp.Frame())
	if err != nil {
		solutionChan <- &planReturn{err: err}
		return
	}

	// publish endpoint of plan if it is known
	if planOpts.MaxSolutions == 1 && endpointPreview != nil {
		endpointPreview <- solutions[0]
	}

	// initialize maps
	goalMap := make(map[node]node, len(solutions))
	for _, solution := range solutions {
		goalMap[solution] = nil
	}
	startMap := make(map[node]node)
	startMap[&basicNode{q: seed}] = nil

	// TODO(rb) package neighborManager better
	nm := &neighborManager{nCPU: mp.nCPU}
	nmContext, cancel := context.WithCancel(ctx)
	defer cancel()

	// for the first iteration, we try the 0.5 interpolation between seed and goal[0]
	target := referenceframe.InterpolateInputs(seed, solutions[0].Q(), 0.5)

	// Create a reference to the two maps so that we can alternate which one is grown
	map1, map2 := startMap, goalMap

	for i := 0; i < algOpts.PlanIter; i++ {
		select {
		case <-ctx.Done():
			solutionChan <- &planReturn{err: ctx.Err()}
			return
		default:
		}

		// for each map get the nearest neighbor to the target
		nearest1 := nm.nearestNeighbor(nmContext, planOpts, target, map1)
		nearest2 := nm.nearestNeighbor(nmContext, planOpts, target, map2)

		// attempt to extend the map to connect the target to map 1, then try to connect the maps together
		map1reached := mp.checkPath(planOpts, nearest1.Q(), target)
		targetNode := &basicNode{q: target}
		if map1reached {
			map1[targetNode] = nearest1
		}
		map2reached := mp.checkPath(planOpts, nearest2.Q(), target)
		if map2reached {
			map2[targetNode] = nearest2
		}

		if map1reached && map2reached {
			cancel()
			solutionChan <- &planReturn{steps: extractPath(startMap, goalMap, &nodePair{targetNode, targetNode})}
			return
		}

		// get next sample, switch map pointers
		target = referenceframe.RandomFrameInputs(mp.frame, mp.randseed)
		map1, map2 = map2, map1
	}

	solutionChan <- &planReturn{err: errPlannerFailed}
}
