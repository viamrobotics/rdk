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
	// The number of nearest neighbors to consider when adding a new sample to the tree.
	defaultNeighborhoodSize = 10

	defaultOptimalityThreshold = 1.05

	defaultOptimalityCheckIter = 10
)

type rrtStarConnectOptions struct {
	// The number of nearest neighbors to consider when adding a new sample to the tree
	NeighborhoodSize int `json:"neighborhood_size"`

	OptimalityThreshold float64 `json:"optimality_threshold"`

	OptimalityCheckIterations int `json:"optimality_check_iterations"`

	// Parameters common to all RRT implementations
	*rrtOptions
}

// newRRTStarConnectOptions creates a struct controlling the running of a single invocation of the algorithm.
// All values are pre-set to reasonable defaults, but can be tweaked if needed.
func newRRTStarConnectOptions(planOpts *plannerOptions) (*rrtStarConnectOptions, error) {
	algOpts := &rrtStarConnectOptions{
		NeighborhoodSize:          defaultNeighborhoodSize,
		OptimalityThreshold:       defaultOptimalityThreshold,
		OptimalityCheckIterations: defaultOptimalityCheckIter,
		rrtOptions:                newRRTOptions(),
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
type rrtStarConnectMotionPlanner struct {
	*rrtPlanner
	algOpts *rrtStarConnectOptions
}

// NewRRTStarConnectMotionPlannerWithSeed creates a rrtStarConnectMotionPlanner object with a user specified random seed.
func newRRTStarConnectMotionPlanner(
	frame referenceframe.Frame,
	seed *rand.Rand,
	logger golog.Logger,
	opt *plannerOptions,
) (motionPlanner, error) {
	if opt == nil {
		opt = newBasicPlannerOptions()
	}
	rrt, err := newRRTPlanner(frame, seed, logger, opt)
	if err != nil {
		return nil, err
	}
	algOpts, err := newRRTStarConnectOptions(opt)
	if err != nil {
		return nil, err
	}
	return &rrtStarConnectMotionPlanner{rrt, algOpts}, nil
}

func (mp *rrtStarConnectMotionPlanner) plan(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
) ([][]referenceframe.Input, error) {
	solutionChan := make(chan *rrtPlanReturn, 1)
	utils.PanicCapturingGo(func() {
		mp.planParallel(ctx, goal, seed, solutionChan)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case plan := <-solutionChan:
		return plan.toInputs(), plan.err()
	}
}

// planParallel will execute the plan. Plan() will call planParallel in a separate thread and wait for results.
// Separating this allows other things to call planParallel allowing the thread-agnostic Plan to be accessible.
func (mp *rrtStarConnectMotionPlanner) planParallel(
	ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
	solutionChan chan<- *rrtPlanReturn,
) {
	mp.logger.Debug("Starting RRT*")
	defer close(solutionChan)

	// setup planner options
	if mp.planOpts == nil {
		mp.planOpts = newBasicPlannerOptions()
	}

	mp.start = time.Now()

	if mp.maps == nil || len(mp.maps.goalMap) == 0 {
		planSeed := mp.initRRTSolutions(ctx, goal, seed)
		if planSeed.planerr != nil || planSeed.steps != nil {
			solutionChan <- planSeed
			return
		}
	}
	target := referenceframe.InterpolateInputs(seed, mp.maps.optNode.Q(), 0.5)

	// Keep a list of the node pairs that have the same inputs
	shared := make([]*nodePair, 0)

	m1chan := make(chan node, 1)
	m2chan := make(chan node, 1)
	defer close(m1chan)
	defer close(m2chan)

	nSolved := 0

	for i := 0; i < mp.algOpts.PlanIter; i++ {
		select {
		case <-ctx.Done():
			// stop and return best path
			if nSolved > 0 {
				mp.logger.Debugf("RRT* timed out after %d iterations, returning best path", i)
				solutionChan <- shortestPath(mp.maps, shared)
			} else {
				mp.logger.Debugf("RRT* timed out after %d iterations, no path found", i)
				solutionChan <- &rrtPlanReturn{planerr: ctx.Err(), maps: mp.maps}
			}
			return
		default:
		}

		// try to connect the target to map 1
		utils.PanicCapturingGo(func() {
			mp.extend(mp.maps.startMap, target, m1chan)
		})
		utils.PanicCapturingGo(func() {
			mp.extend(mp.maps.goalMap, target, m2chan)
		})
		map1reached := <-m1chan
		map2reached := <-m2chan

		if map1reached != nil && map2reached != nil {
			// target was added to both map
			shared = append(shared, &nodePair{map1reached, map2reached})

			// Check if we can return
			if nSolved%mp.algOpts.OptimalityCheckIterations == 0 {
				solution := shortestPath(mp.maps, shared)
				solutionCost := EvaluatePlan(solution.toInputs(), mp.planOpts.DistanceFunc)
				if solutionCost-mp.maps.optNode.cost < mp.algOpts.OptimalityThreshold*mp.maps.optNode.cost {
					mp.logger.Debug("RRT* progress: sufficiently optimal path found, exiting")
					solutionChan <- solution
					return
				}
			}

			nSolved++
		}

		// get next sample, switch map pointers
		target = referenceframe.RandomFrameInputs(mp.frame, mp.randseed)
	}
	mp.logger.Debug("RRT* exceeded max iter")
	solutionChan <- shortestPath(mp.maps, shared)
}

func (mp *rrtStarConnectMotionPlanner) extend(
	tree rrtMap,
	target []referenceframe.Input,
	mchan chan node,
) {
	if validTarget := mp.checkInputs(target); !validTarget {
		mchan <- nil
		return
	}
	// iterate over the k nearest neighbors and find the minimum cost to connect the target node to the tree
	neighbors := kNearestNeighbors(mp.planOpts, tree, target, mp.algOpts.NeighborhoodSize)
	minCost := math.Inf(1)
	minIndex := -1
	for i, neighbor := range neighbors {
		neighborNode := neighbor.node.(*costNode)
		cost := neighborNode.cost + neighbor.dist
		if mp.checkPath(neighborNode.Q(), target) {
			minIndex = i
			minCost = cost
			// Neighbors are returned ordered by their costs. The first valid one we find is best, so break here.
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
		_, connectionCost := mp.planOpts.DistanceFunc(&ConstraintInput{
			StartInput: neighborNode.Q(),
			EndInput:   targetNode.Q(),
		})
		cost := connectionCost + targetNode.cost
		if cost < neighborNode.cost && mp.checkPath(target, neighborNode.Q()) {
			neighborNode.cost = cost
			tree[neighborNode] = targetNode
		}
	}
	mchan <- targetNode
}
