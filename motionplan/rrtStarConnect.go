package motionplan

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	// If a solution is found that is within this percentage of the optimal unconstrained solution, exit early.
	defaultOptimalityThreshold = .05

	// Period of iterations after which a new solution is calculated and updated.
	defaultSolutionCalculationPeriod = 150

	// The number of nearest neighbors to consider when adding a new sample to the tree.
	defaultNeighborhoodSize = 10
)

type rrtStarConnectOptions struct {
	// If a solution is found that is within this percentage of the optimal unconstrained solution, exit early
	OptimalityThreshold float64 `json:"optimality_threshold"`

	// The number of nearest neighbors to consider when adding a new sample to the tree
	NeighborhoodSize int `json:"neighborhood_size"`

	// Parameters common to all RRT implementations
	*rrtOptions
}

// newRRTStarConnectOptions creates a struct controlling the running of a single invocation of the algorithm.
// All values are pre-set to reasonable defaults, but can be tweaked if needed.
func newRRTStarConnectOptions(planOpts *plannerOptions) (*rrtStarConnectOptions, error) {
	algOpts := &rrtStarConnectOptions{
		OptimalityThreshold: defaultOptimalityThreshold,
		NeighborhoodSize:    defaultNeighborhoodSize,
		rrtOptions:          newRRTOptions(planOpts),
	}
	// convert map to json
	jsonString, err := json.Marshal(planOpts.extra)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonString, algOpts)
	if err != nil {
		return nil, err
	}
	return algOpts, nil
}

// rrtStarConnectMotionPlanner is an object able to asymptotically optimally path around obstacles to some goal for a given referenceframe.
// It uses the RRT*-Connect algorithm, Klemm et al 2015
// https://ieeexplore.ieee.org/document/7419012
type rrtStarConnectMotionPlanner struct{ *planner }

// NewRRTStarConnectMotionPlannerWithSeed creates a rrtStarConnectMotionPlanner object with a user specified random seed.
func newRRTStarConnectMotionPlanner(
	frame referenceframe.Frame,
	nCPU int,
	seed *rand.Rand,
	logger golog.Logger,
) (motionPlanner, error) {
	planner, err := newPlanner(frame, nCPU, seed, logger)
	if err != nil {
		return nil, err
	}
	return &rrtStarConnectMotionPlanner{planner}, nil
}

func (mp *rrtStarConnectMotionPlanner) Plan(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
	planOpts *plannerOptions,
) ([][]referenceframe.Input, error) {
	if planOpts == nil {
		planOpts = newBasicPlannerOptions()
	}
	solutionChan := make(chan *rrtPlanReturn, 1)
	utils.PanicCapturingGo(func() {
		mp.rrtBackgroundRunner(ctx, goal, seed, planOpts, initRRTMaps(), nil, solutionChan)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case plan := <-solutionChan:
		return plan.ToInputs(), plan.err
	}
}

// rrtBackgroundRunner will execute the plan. Plan() will call rrtBackgroundRunner in a separate thread and wait for results.
// Separating this allows other things to call rrtBackgroundRunner in parallel allowing the thread-agnostic Plan to be accessible.
func (mp *rrtStarConnectMotionPlanner) rrtBackgroundRunner(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
	planOpts *plannerOptions,
	rm *rrtMaps,
	endpointPreview chan node,
	solutionChan chan *rrtPlanReturn,
) {
	defer close(solutionChan)

	// setup planner options
	if planOpts == nil {
		planOpts = newBasicPlannerOptions()
	}
	algOpts, err := newRRTStarConnectOptions(planOpts)
	if err != nil {
		solutionChan <- &rrtPlanReturn{err: err}
		return
	}

	mp.start = time.Now()

	// get many potential end goals from IK solver
	solutions, err := getSolutions(ctx, planOpts, mp.solver, goal, seed, mp.Frame(), mp.randseed.Int())
	if err != nil {
		solutionChan <- &rrtPlanReturn{err: err}
		return
	}

	// publish endpoint of plan if it is known
	if planOpts.MaxSolutions == 1 && endpointPreview != nil {
		endpointPreview <- solutions[0]
	}

	// the smallest interpolated distance between the start and end input represents a lower bound on cost
	optimalCost := solutions[0].cost

	// initialize maps
	for i, solution := range solutions {
		if i == 0 && mp.checkPath(planOpts, seed, solution.Q()) {
			solutionChan <- &rrtPlanReturn{steps: []node{&basicNode{q: seed}, solution}}
			return
		}
		rm.goalMap[newCostNode(solution.Q(), 0)] = nil
	}
	rm.startMap[newCostNode(seed, 0)] = nil

	// for the first iteration, we try the 0.5 interpolation between seed and goal[0]
	target := referenceframe.InterpolateInputs(seed, solutions[0].Q(), 0.5)

	// Create a reference to the two maps so that we can alternate which one is grown
	map1, map2 := rm.startMap, rm.goalMap

	// Keep a list of the node pairs that have the same inputs
	shared := make([]*nodePair, 0)

	// sample until the max number of iterations is reached
	var solutionCost float64

	m1chan := make(chan node, 1)
	m2chan := make(chan node, 1)
	defer close(m1chan)
	defer close(m2chan)

	solved := false

	for i := 0; i < algOpts.PlanIter; i++ {
		select {
		case <-ctx.Done():
			// stop and return best path
			if solved {
				solutionChan <- shortestPath(rm, shared)
			} else {
				solutionChan <- &rrtPlanReturn{err: ctx.Err(), rm: rm}
			}
			return
		default:
		}

		// try to connect the target to map 1
		utils.PanicCapturingGo(func() {
			mp.extend(algOpts, map1, target, m1chan)
		})
		utils.PanicCapturingGo(func() {
			mp.extend(algOpts, map2, target, m2chan)
		})
		map1reached := <-m1chan
		map2reached := <-m2chan

		if map1reached != nil && map2reached != nil {
			// target was added to both map
			solved = true
			shared = append(shared, &nodePair{map1reached, map2reached})
		}

		// get next sample, switch map pointers
		target = referenceframe.RandomFrameInputs(mp.frame, mp.randseed)
		map1, map2 = map2, map1

		// calculate the solution and log status of planner
		if i%defaultSolutionCalculationPeriod == 0 {
			solution := shortestPath(rm, shared)
			solutionCost = EvaluatePlan(solution, planOpts)
			mp.logger.Warnf("RRT* progress: %d%%\tpath cost: %.3f", 100*i/algOpts.PlanIter, solutionCost)
			// check if an early exit is possible
			if solutionCost-optimalCost < algOpts.OptimalityThreshold*optimalCost {
				mp.logger.Warn("RRT* progress: sufficiently optimal path found, exiting")
				solutionChan <- solution
				return
			}
		}
	}

	solutionChan <- shortestPath(rm, shared)
}

func (mp *rrtStarConnectMotionPlanner) extend(
	algOpts *rrtStarConnectOptions,
	tree map[node]node,
	target []referenceframe.Input,
	mchan chan node,
) {
	if validTarget := mp.checkInputs(algOpts.planOpts, target); !validTarget {
		mchan <- nil
		return
	}
	// iterate over the k nearest neighbors and find the minimum cost to connect the target node to the tree
	neighbors := kNearestNeighbors(algOpts.planOpts, tree, target, algOpts.NeighborhoodSize)
	minCost := math.Inf(1)
	minIndex := -1
	for i, neighbor := range neighbors {
		neighborNode := neighbor.node.(*costNode)
		cost := neighborNode.cost + neighbor.dist
		if cost < minCost && mp.checkPath(algOpts.planOpts, neighborNode.Q(), target) {
			minIndex = i
			minCost = cost
			break
		}
	}

	// add new node to tree as a child of the minimum cost neighbor node if it was reachable
	if minIndex == -1 {
		mchan <- nil
		return
	}
	targetNode := newCostNode(target, minCost)
	tree[targetNode] = neighbors[minIndex].node

	// rewire the tree
	for i, neighbor := range neighbors {
		// dont need to try to rewire minIndex, so skip it
		if i == minIndex {
			continue
		}

		// check to see if a shortcut is possible, and rewire the node if it is
		neighborNode := neighbor.node.(*costNode)
		_, connectionCost := algOpts.planOpts.DistanceFunc(&ConstraintInput{
			StartInput: neighborNode.Q(),
			EndInput:   targetNode.Q(),
		})
		cost := connectionCost + targetNode.cost
		if cost < neighborNode.cost && mp.checkPath(algOpts.planOpts, target, neighborNode.Q()) {
			neighborNode.cost = cost
			tree[neighborNode] = targetNode
		}
	}
	mchan <- targetNode
}
