//go:build !no_cgo

package motionplan

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"time"

	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
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

	// This is how far rrtStarConnect will try to extend the map towards a goal per-step
	qstep map[string][]float64
}

// newRRTStarConnectOptions creates a struct controlling the running of a single invocation of the algorithm.
// All values are pre-set to reasonable defaults, but can be tweaked if needed.
func newRRTStarConnectOptions(planOpts *plannerOptions, lfs *linearizedFrameSystem) (*rrtStarConnectOptions, error) {
	algOpts := &rrtStarConnectOptions{
		NeighborhoodSize: defaultNeighborhoodSize,
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

	// Initialize qstep using the same helper function as CBIRRT
	algOpts.qstep = getFrameSteps(lfs, defaultFrameStep)

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
	fs referenceframe.FrameSystem,
	seed *rand.Rand,
	logger logging.Logger,
	opt *plannerOptions,
) (motionPlanner, error) {
	if opt == nil {
		return nil, errNoPlannerOptions
	}
	mp, err := newPlanner(fs, seed, logger, opt)
	if err != nil {
		return nil, err
	}
	algOpts, err := newRRTStarConnectOptions(opt, mp.lfs)
	if err != nil {
		return nil, err
	}
	return &rrtStarConnectMotionPlanner{mp, algOpts}, nil
}

func (mp *rrtStarConnectMotionPlanner) plan(ctx context.Context, seed, goal *PlanState) ([]node, error) {
	solutionChan := make(chan *rrtSolution, 1)
	initMaps := initRRTSolutions(ctx, atomicWaypoint{mp: mp, startState: seed, goalState: goal})
	if initMaps.err != nil {
		return nil, initMaps.err
	}
	if initMaps.steps != nil {
		return initMaps.steps, nil
	}
	utils.PanicCapturingGo(func() {
		mp.rrtBackgroundRunner(ctx, &rrtParallelPlannerShared{
			initMaps.maps,
			nil,
			solutionChan,
		})
	})
	solution := <-solutionChan
	if solution.err != nil {
		return nil, solution.err
	}
	return solution.steps, nil
}

// rrtBackgroundRunner will execute the plan. Plan() will call rrtBackgroundRunner in a separate thread and wait for results.
// Separating this allows other things to call rrtBackgroundRunner in parallel allowing the thread-agnostic Plan to be accessible.
func (mp *rrtStarConnectMotionPlanner) rrtBackgroundRunner(ctx context.Context,
	rrt *rrtParallelPlannerShared,
) {
	mp.logger.CDebug(ctx, "Starting RRT*")
	defer close(rrt.solutionChan)

	// setup planner options
	if mp.planOpts == nil {
		rrt.solutionChan <- &rrtSolution{err: errNoPlannerOptions}
		return
	}

	mp.start = time.Now()

	if rrt.maps == nil || len(rrt.maps.goalMap) == 0 || len(rrt.maps.startMap) == 0 {
		rrt.solutionChan <- &rrtSolution{err: errors.New("cannot run RRT background runner without prepopulated maps")}
	}
	var seed referenceframe.FrameSystemInputs
	// Pick a random seed node to create the first interp node
	for sNode, parent := range rrt.maps.startMap {
		if parent == nil {
			seed = sNode.Q()
		}
	}
	targetConf, err := referenceframe.InterpolateFS(mp.fs, seed, rrt.maps.optNode.Q(), 0.5)
	if err != nil {
		rrt.solutionChan <- &rrtSolution{err: err}
		return
	}
	target := newConfigurationNode(targetConf)
	map1, map2 := rrt.maps.startMap, rrt.maps.goalMap

	// Keep a list of the node pairs that have the same inputs
	shared := make([]*nodePair, 0)

	m1chan := make(chan node, 1)
	m2chan := make(chan node, 1)
	defer close(m1chan)
	defer close(m2chan)

	nSolved := 0

	for i := 0; i < mp.planOpts.PlanIter; i++ {
		select {
		case <-ctx.Done():
			// stop and return best path
			if nSolved > 0 {
				mp.logger.CDebugf(ctx, "RRT* timed out after %d iterations, returning best path", i)
				rrt.solutionChan <- shortestPath(rrt.maps, shared)
			} else {
				mp.logger.CDebugf(ctx, "RRT* timed out after %d iterations, no path found", i)
				rrt.solutionChan <- &rrtSolution{err: ctx.Err(), maps: rrt.maps}
			}
			return
		default:
		}

		tryExtend := func(target node) (node, node, error) {
			// attempt to extend maps 1 and 2 towards the target
			// If ctx is done, nearest neighbors will be invalid and we want to return immediately
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			default:
			}

			utils.PanicCapturingGo(func() {
				mp.extend(ctx, map1, target, m1chan)
			})
			utils.PanicCapturingGo(func() {
				mp.extend(ctx, map2, target, m2chan)
			})
			map1reached := <-m1chan
			map2reached := <-m2chan

			return map1reached, map2reached, nil
		}

		map1reached, map2reached, err := tryExtend(target)
		if err != nil {
			rrt.solutionChan <- &rrtSolution{err: err, maps: rrt.maps}
			return
		}

		reachedDelta := mp.planOpts.configurationDistanceFunc(&ik.SegmentFS{
			StartConfiguration: map1reached.Q(),
			EndConfiguration:   map2reached.Q(),
		})

		// Second iteration; extend maps 1 and 2 towards the halfway point between where they reached
		if reachedDelta > mp.planOpts.InputIdentDist {
			targetConf, err = referenceframe.InterpolateFS(mp.fs, map1reached.Q(), map2reached.Q(), 0.5)
			if err != nil {
				rrt.solutionChan <- &rrtSolution{err: err, maps: rrt.maps}
				return
			}
			target = newConfigurationNode(targetConf)
			map1reached, map2reached, err = tryExtend(target)
			if err != nil {
				rrt.solutionChan <- &rrtSolution{err: err, maps: rrt.maps}
				return
			}
			reachedDelta = mp.planOpts.configurationDistanceFunc(&ik.SegmentFS{
				StartConfiguration: map1reached.Q(),
				EndConfiguration:   map2reached.Q(),
			})
		}

		// Solved
		if reachedDelta <= mp.planOpts.InputIdentDist {
			// target was added to both map
			shared = append(shared, &nodePair{map1reached, map2reached})

			// Check if we can return
			if nSolved%defaultOptimalityCheckIter == 0 {
				solution := shortestPath(rrt.maps, shared)
				traj := nodesToTrajectory(solution.steps)
				// if cost of trajectory is sufficiently small, exit early
				solutionCost := traj.EvaluateCost(mp.planOpts.scoreFunc)
				if solutionCost-rrt.maps.optNode.Cost() < defaultOptimalityThreshold*rrt.maps.optNode.Cost() {
					mp.logger.CDebug(ctx, "RRT* progress: sufficiently optimal path found, exiting")
					rrt.solutionChan <- solution
					return
				}
			}

			nSolved++
		}

		// get next sample, switch map pointers
		target, err = mp.sample(map1reached, i)
		if err != nil {
			rrt.solutionChan <- &rrtSolution{err: err, maps: rrt.maps}
			return
		}
		map1, map2 = map2, map1
	}
	mp.logger.CDebug(ctx, "RRT* exceeded max iter")
	rrt.solutionChan <- shortestPath(rrt.maps, shared)
}

func (mp *rrtStarConnectMotionPlanner) extend(
	ctx context.Context,
	rrtMap map[node]node,
	target node,
	mchan chan node,
) {
	// This should iterate until one of the following conditions:
	// 1) we have reached the target
	// 2) the request is cancelled/times out
	// 3) we are no longer approaching the target and our "best" node is further away than the previous best
	// 4) further iterations change our best node by close-to-zero amounts
	// 5) we have iterated more than maxExtendIter times
	near := kNearestNeighbors(mp.planOpts, rrtMap, &basicNode{q: target.Q()}, mp.algOpts.NeighborhoodSize)[0].node
	oldNear := near
	for i := 0; i < maxExtendIter; i++ {
		select {
		case <-ctx.Done():
			mchan <- oldNear
			return
		default:
		}

		dist := mp.planOpts.configurationDistanceFunc(&ik.SegmentFS{StartConfiguration: near.Q(), EndConfiguration: target.Q()})
		if dist < mp.planOpts.InputIdentDist {
			mchan <- near
			return
		}

		oldNear = near
		newNear := fixedStepInterpolation(near, target, mp.algOpts.qstep)
		// Check whether oldNear -> newNear path is a valid segment, and if not then set to nil
		if !mp.checkPath(oldNear.Q(), newNear) {
			break
		}

		extendCost := mp.planOpts.configurationDistanceFunc(&ik.SegmentFS{
			StartConfiguration: oldNear.Q(),
			EndConfiguration:   near.Q(),
		})
		near = &basicNode{q: newNear, cost: oldNear.Cost() + extendCost}
		rrtMap[near] = oldNear

		// rewire the tree
		neighbors := kNearestNeighbors(mp.planOpts, rrtMap, &basicNode{q: newNear}, mp.algOpts.NeighborhoodSize)
		for i, thisNeighbor := range neighbors {
			// dont need to try to rewire nearest neighbor, so skip it
			if i == 0 {
				continue
			}

			// check to see if a shortcut is possible, and rewire the node if it is
			connectionCost := mp.planOpts.configurationDistanceFunc(&ik.SegmentFS{
				StartConfiguration: thisNeighbor.node.Q(),
				EndConfiguration:   near.Q(),
			})
			cost := connectionCost + near.Cost()

			// If 1) we have a lower cost, and 2) the putative updated path is valid
			if cost < thisNeighbor.node.Cost() && mp.checkPath(target.Q(), thisNeighbor.node.Q()) {
				// Alter the cost of the node
				// This needs to edit the existing node, rather than make a new one, as there are pointers in the tree
				thisNeighbor.node.SetCost(cost)
				rrtMap[thisNeighbor.node] = near
			}
		}
	}
	mchan <- oldNear
}
