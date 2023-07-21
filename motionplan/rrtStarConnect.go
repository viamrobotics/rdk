package motionplan

import (
	"context"
	"encoding/json"
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

	// Parameters common to all RRT implementations
	*rrtOptions
}

// newRRTStarConnectOptions creates a struct controlling the running of a single invocation of the algorithm.
// All values are pre-set to reasonable defaults, but can be tweaked if needed.
func newRRTStarConnectOptions(planOpts *plannerOptions) (*rrtStarConnectOptions, error) {
	algOpts := &rrtStarConnectOptions{
		NeighborhoodSize: defaultNeighborhoodSize,
		rrtOptions:       newRRTOptions(),
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
	*planner
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
		return nil, errNoPlannerOptions
	}
	mp, err := newPlanner(frame, seed, logger, opt)
	if err != nil {
		return nil, err
	}
	algOpts, err := newRRTStarConnectOptions(opt)
	if err != nil {
		return nil, err
	}
	return &rrtStarConnectMotionPlanner{mp, algOpts}, nil
}

func (mp *rrtStarConnectMotionPlanner) plan(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
) ([]node, error) {
	solutionChan := make(chan *rrtPlanReturn, 1)
	utils.PanicCapturingGo(func() {
		mp.rrtBackgroundRunner(ctx, seed, &rrtParallelPlannerShared{nil, nil, solutionChan})
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case plan := <-solutionChan:
		return plan.steps, plan.err()
	}
}

// rrtBackgroundRunner will execute the plan. Plan() will call rrtBackgroundRunner in a separate thread and wait for results.
// Separating this allows other things to call rrtBackgroundRunner in parallel allowing the thread-agnostic Plan to be accessible.
func (mp *rrtStarConnectMotionPlanner) rrtBackgroundRunner(ctx context.Context,
	seed []referenceframe.Input,
	rrt *rrtParallelPlannerShared,
) {
	mp.logger.Debug("Starting RRT*")
	defer close(rrt.solutionChan)

	// setup planner options
	if mp.planOpts == nil {
		rrt.solutionChan <- &rrtPlanReturn{planerr: errNoPlannerOptions}
		return
	}
	
	mp.start = time.Now()

	if rrt.maps == nil || len(rrt.maps.goalMap) == 0 {
		planSeed := initRRTSolutions(ctx, mp, seed)
		if planSeed.planerr != nil || planSeed.steps != nil {
			rrt.solutionChan <- planSeed
			return
		}
		rrt.maps = planSeed.maps
	}

	mid := referenceframe.InterpolateInputs(seed, rrt.maps.optNode.Q(), 0.5)
	target := mid
	map1, map2 := rrt.maps.startMap, rrt.maps.goalMap

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
				rrt.solutionChan <- shortestPath(rrt.maps, shared)
			} else {
				mp.logger.Debugf("RRT* timed out after %d iterations, no path found", i)
				rrt.solutionChan <- &rrtPlanReturn{planerr: ctx.Err(), maps: rrt.maps}
			}
			return
		default:
		}

		// try to connect the target to map 1
		utils.PanicCapturingGo(func() {
			mp.extend(map1, target, m1chan)
		})
		utils.PanicCapturingGo(func() {
			mp.extend(map2, target, m2chan)
		})
		map1reached := <-m1chan
		map2reached := <-m2chan

		if map1reached == nil || map2reached == nil {
			target, _ = referenceframe.RestrictedRandomFrameInputs(mp.frame, mp.randseed, 0.5, mid)
			continue
		}

		reachedDelta := mp.planOpts.DistanceFunc(&Segment{StartConfiguration: map1reached.Q(), EndConfiguration: map2reached.Q()})

		// Second iteration; extend maps 1 and 2 towards the halfway point between where they reached
		// if reachedDelta > 0.0001 {
		// 	target = referenceframe.InterpolateInputs(map1reached.Q(), map2reached.Q(), 0.5)
		// 	utils.PanicCapturingGo(func() {
		// 		mp.extend(rrt.maps.startMap, target, m1chan)
		// 	})
		// 	utils.PanicCapturingGo(func() {
		// 		mp.extend(rrt.maps.goalMap, target, m2chan)
		// 	})
		// 	map1reached := <-m1chan
		// 	map2reached := <-m2chan
			
		// 	reachedDelta = mp.planOpts.DistanceFunc(&Segment{StartConfiguration: map1reached.Q(), EndConfiguration: map2reached.Q()})
		// }

		// Solved
		if reachedDelta <= 0.0001 {
			// target was added to both map
			shared = append(shared, &nodePair{map1reached, map2reached})

			// Check if we can return
			if nSolved%defaultOptimalityCheckIter == 0 {
				solution := shortestPath(rrt.maps, shared)
				solutionCost := EvaluatePlan(nodesToInputs(solution.steps), mp.planOpts.DistanceFunc)
				if solutionCost-rrt.maps.optNode.Cost() < defaultOptimalityThreshold*rrt.maps.optNode.Cost() {
					mp.logger.Debug("RRT* progress: sufficiently optimal path found, exiting")
					rrt.solutionChan <- solution
					return
				}
			}

			nSolved++
		}

		// get next sample, switch map pointers
		target, _ = referenceframe.RestrictedRandomFrameInputs(mp.frame, mp.randseed, 0.5, mid)
		map1, map2 = map2, map1
	}
	mp.logger.Debug("RRT* exceeded max iter")
	rrt.solutionChan <- shortestPath(rrt.maps, shared)
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

	// find nearest neighbor and cost to connect the target node to the tree
	neighbors := kNearestNeighbors(mp.planOpts, tree, &basicNode{q: target}, 1)
	var targetNode node

	ok, validSeg := mp.planOpts.CheckSegmentAndStateValidity(
		&Segment{
			StartConfiguration: neighbors[0].node.Q(),
			EndConfiguration:   target,
			Frame:              mp.frame,
		},
		mp.planOpts.Resolution,
	)
	if !ok {
		if validSeg == nil {
			mchan <- nil
			return
		}
		targetNode = &basicNode{q: []referenceframe.Input{validSeg.EndConfiguration[0],
			validSeg.EndConfiguration[1],
			validSeg.EndConfiguration[2]},
			cost: neighbors[0].node.Cost() + neighbors[0].dist}
	} else {
		targetNode = &basicNode{q: target, cost: neighbors[0].node.Cost() + neighbors[0].dist}
	}

	tree[targetNode] = neighbors[0].node
	mchan <- targetNode
}
