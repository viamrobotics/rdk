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
)

type rrtStarConnectOptions struct {
	// The number of nearest neighbors to consider when adding a new sample to the tree
	NeighborhoodSize int `json:"neighborhood_size"`

	// Parameters common to all RRT implementations
	*rrtOptions
}

// newRRTStarConnectOptions creates a struct controlling the running of a single invocation of the algorithm.
// All values are pre-set to reasonable defaults, but can be tweaked if needed.
func newRRTStarConnectOptions(planOpts *plannerOptions) (*rrtStarConnectOptions, error) {
	algOpts := &rrtStarConnectOptions{
		NeighborhoodSize: defaultNeighborhoodSize,
		rrtOptions:       newRRTOptions(planOpts),
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
type rrtStarConnectMotionPlanner struct{
	*planner
	algOpts *rrtStarConnectOptions
}

// NewRRTStarConnectMotionPlannerWithSeed creates a rrtStarConnectMotionPlanner object with a user specified random seed.
func newRRTStarConnectMotionPlanner(
	frame referenceframe.Frame,
	nCPU int,
	seed *rand.Rand,
	logger golog.Logger,
	opt *plannerOptions,
) (motionPlanner, error) {
	mp, err := newPlanner(frame, nCPU, seed, logger, opt)
	if err != nil {
		return nil, err
	}
	algOpts, err := newRRTStarConnectOptions(opt)
	if err != nil {
		return nil, err
	}
	return &rrtStarConnectMotionPlanner{mp, algOpts}, nil
}

func (mp *rrtStarConnectMotionPlanner) Plan(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
) ([][]referenceframe.Input, error) {
	if mp.planOpts == nil {
		mp.planOpts = newBasicPlannerOptions()
	}
	solutionChan := make(chan *rrtPlanReturn, 1)
	utils.PanicCapturingGo(func() {
		mp.rrtBackgroundRunner(ctx, goal, seed, &rrtParallelPlannerShared{initRRTMaps(), nil, solutionChan})
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case plan := <-solutionChan:
		return plan.toInputs(), plan.err()
	}
}

// rrtBackgroundRunner will execute the plan. Plan() will call rrtBackgroundRunner in a separate thread and wait for results.
// Separating this allows other things to call rrtBackgroundRunner in parallel allowing the thread-agnostic Plan to be accessible.
func (mp *rrtStarConnectMotionPlanner) rrtBackgroundRunner(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
	rrt *rrtParallelPlannerShared,
) {
	mp.logger.Debug("Starting RRT*")
	defer close(rrt.solutionChan)

	// setup planner options
	if mp.planOpts == nil {
		mp.planOpts = newBasicPlannerOptions()
	}
	algOpts, err := newRRTStarConnectOptions(mp.planOpts)
	if err != nil {
		rrt.solutionChan <- &rrtPlanReturn{planerr: err}
		return
	}

	mp.start = time.Now()

	// get many potential end goals from IK solver
	solutions, err := getSolutions(ctx, mp.planOpts, mp.solver, goal, seed, mp.Frame(), mp.randseed.Int())
	mp.logger.Debugf("RRT* found %d IK solutions", len(solutions))
	if err != nil {
		rrt.solutionChan <- &rrtPlanReturn{planerr: err}
		return
	}

	// publish endpoint of plan if it is known
	if mp.planOpts.MaxSolutions == 1 && rrt.endpointPreview != nil {
		mp.logger.Debug("RRT* found early final solution")
		rrt.endpointPreview <- solutions[0]
	}

	// the smallest interpolated distance between the start and end input represents a lower bound on cost
	_, optimalCost := mp.planOpts.DistanceFunc(&ConstraintInput{StartInput: seed, EndInput: solutions[0].Q()})

	// initialize maps
	for i, solution := range solutions {
		if i == 0 && mp.checkPath(seed, solution.Q()) {
			rrt.solutionChan <- &rrtPlanReturn{steps: []node{&basicNode{q: seed}, solution}}
			mp.logger.Debug("RRT* could interpolate directly to goal")
			return
		}
		rrt.rm.goalMap[newCostNode(solution.Q(), 0)] = nil
	}
	mp.logger.Debugf("RRT* failed to directly interpolate from %v to %v", seed, solutions[0].Q())
	rrt.rm.startMap[newCostNode(seed, 0)] = nil

	target := referenceframe.RandomFrameInputs(mp.frame, mp.randseed)

	// Keep a list of the node pairs that have the same inputs
	shared := make([]*nodePair, 0)

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
				mp.logger.Debugf("RRT* timed out after %d iterations, returning best path", i)
				rrt.solutionChan <- shortestPath(rrt.rm, shared, optimalCost)
			} else {
				mp.logger.Debugf("RRT* timed out after %d iterations, no path found", i)
				rrt.solutionChan <- &rrtPlanReturn{planerr: ctx.Err(), rm: rrt.rm}
			}
			return
		default:
		}

		// try to connect the target to map 1
		utils.PanicCapturingGo(func() {
			mp.extend(rrt.rm.startMap, target, m1chan)
		})
		utils.PanicCapturingGo(func() {
			mp.extend(rrt.rm.goalMap, target, m2chan)
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
	}
	mp.logger.Debug("RRT* exceeded max iter")
	rrt.solutionChan <- shortestPath(rrt.rm, shared, optimalCost)
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
	neighbors := kNearestNeighbors(mp.algOpts.plannerOptions, tree, target, mp.algOpts.NeighborhoodSize)
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
		_, connectionCost := mp.algOpts.DistanceFunc(&ConstraintInput{
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
