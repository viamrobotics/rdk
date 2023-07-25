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

	// Parameters common to all RRT implementations
	*rrtOptions
}

// newRRTStarConnectOptions creates a struct controlling the running of a single invocation of the algorithm.
// All values are pre-set to reasonable defaults, but can be tweaked if needed.
func newRRTStarConnectOptions(planOpts *plannerOptions, frame referenceframe.Frame) (*rrtStarConnectOptions, error) {
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

	rrtOptions := newRRTOptions()
	rrtOptions.qstep = getFrameSteps(frame, rrtOptions.FrameStep)
	algOpts.rrtOptions = rrtOptions

	return algOpts, nil
}

// rrtStarConnectMotionPlanner is an object able to asymptotically optimally path around obstacles to some goal for a given referenceframe.
// It uses the RRT*-Connect algorithm, Klemm et al 2015
// https://ieeexplore.ieee.org/document/7419012
type rrtStarConnectMotionPlanner struct {
	*planner
	fastGradDescent *NloptIK
	algOpts         *rrtStarConnectOptions
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
	nlopt, err := CreateNloptIKSolver(frame, logger, 1, opt.GoalThreshold)
	if err != nil {
		return nil, err
	}
	algOpts, err := newRRTStarConnectOptions(opt, frame)
	if err != nil {
		return nil, err
	}
	return &rrtStarConnectMotionPlanner{mp, nlopt, algOpts}, nil
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

	nm1 := &neighborManager{nCPU: mp.planOpts.NumThreads}
	nm2 := &neighborManager{nCPU: mp.planOpts.NumThreads}
	nmContext, cancel := context.WithCancel(ctx)
	defer cancel()
	mp.start = time.Now()

	if rrt.maps == nil || len(rrt.maps.goalMap) == 0 {
		planSeed := initRRTSolutions(ctx, mp, seed)
		if planSeed.planerr != nil || planSeed.steps != nil {
			rrt.solutionChan <- planSeed
			return
		}
		rrt.maps = planSeed.maps
	}

	target := referenceframe.InterpolateInputs(seed, rrt.maps.optNode.Q(), 0.5)
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

		tryExtend := func(target []referenceframe.Input) (node, node, error) {
			// attempt to extend maps 1 and 2 towards the target
			utils.PanicCapturingGo(func() {
				m1chan <- nm1.nearestNeighbor(nmContext, mp.planOpts, newConfigurationNode(target), map1)
			})
			utils.PanicCapturingGo(func() {
				m2chan <- nm2.nearestNeighbor(nmContext, mp.planOpts, newConfigurationNode(target), map2)
			})
			nearest1 := <-m1chan
			nearest2 := <-m2chan
			// If ctx is done, nearest neighbors will be invalid and we want to return immediately
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			default:
			}

			//nolint: gosec
			rseed1 := rand.New(rand.NewSource(int64(mp.randseed.Int())))
			//nolint: gosec
			rseed2 := rand.New(rand.NewSource(int64(mp.randseed.Int())))

			utils.PanicCapturingGo(func() {
				mp.constrainedExtend(ctx, rseed1, map1, nearest1, newConfigurationNode(target), m1chan)
			})
			utils.PanicCapturingGo(func() {
				mp.constrainedExtend(ctx, rseed2, map2, nearest2, newConfigurationNode(target), m2chan)
			})
			map1reached := <-m1chan
			map2reached := <-m2chan

			return map1reached, map2reached, nil
		}

		map1reached, map2reached, err := tryExtend(target)
		if err != nil {
			rrt.solutionChan <- &rrtPlanReturn{planerr: err, maps: rrt.maps}
			return
		}

		reachedDelta := mp.planOpts.DistanceFunc(&Segment{StartConfiguration: map1reached.Q(), EndConfiguration: map2reached.Q()})

		// Second iteration; extend maps 1 and 2 towards the halfway point between where they reached
		if reachedDelta > mp.algOpts.JointSolveDist {
			target = referenceframe.InterpolateInputs(map1reached.Q(), map2reached.Q(), 0.5)
			map1reached, map2reached, err = tryExtend(target)
			if err != nil {
				rrt.solutionChan <- &rrtPlanReturn{planerr: err, maps: rrt.maps}
				return
			}
			reachedDelta = mp.planOpts.DistanceFunc(&Segment{StartConfiguration: map1reached.Q(), EndConfiguration: map2reached.Q()})
		}

		// Solved
		if reachedDelta <= mp.algOpts.JointSolveDist {
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
		target, err = mp.sample(map1reached, i)
		if err != nil {
			rrt.solutionChan <- &rrtPlanReturn{planerr: err, maps: rrt.maps}
			return
		}
		map1, map2 = map2, map1
	}
	mp.logger.Debug("RRT* exceeded max iter")
	rrt.solutionChan <- shortestPath(rrt.maps, shared)
}

func (mp *rrtStarConnectMotionPlanner) sample(rSeed node, sampleNum int) ([]referenceframe.Input, error) {
	// If we have done more than 50 iterations, start seeding off completely random positions 2 at a time
	// The 2 at a time is to ensure random seeds are added onto both the seed and goal maps.
	if rSeed == nil || (sampleNum >= mp.algOpts.IterBeforeRand && sampleNum%4 >= 2) {
		return referenceframe.RandomFrameInputs(mp.frame, mp.randseed), nil
	}
	// Seeding nearby to valid points results in much faster convergence in less constrained space
	return referenceframe.RestrictedRandomFrameInputs(mp.frame, mp.randseed, 0.1, rSeed.Q())
}

func (mp *rrtStarConnectMotionPlanner) constrainedExtend(
	ctx context.Context,
	randseed *rand.Rand,
	rrtMap map[node]node,
	near, target node,
	mchan chan node,
) {
	// Allow qstep to be doubled as a means to escape from configurations which gradient descend to their seed
	qstep := make([]float64, len(mp.algOpts.qstep))
	copy(qstep, mp.algOpts.qstep)
	doubled := false

	oldNear := near
	// This should iterate until one of the following conditions:
	// 1) we have reached the target
	// 2) the request is cancelled/times out
	// 3) we are no longer approaching the target and our "best" node is further away than the previous best
	// 4) further iterations change our best node by close-to-zero amounts
	// 5) we have iterated more than maxExtendIter times
	for i := 0; i < maxExtendIter; i++ {
		select {
		case <-ctx.Done():
			mchan <- oldNear
			return
		default:
		}

		// iterate over the k nearest neighbors and find the minimum cost to connect the target node to the tree
		neighbors := kNearestNeighbors(mp.planOpts, rrtMap, &basicNode{q: target.Q()}, mp.algOpts.NeighborhoodSize)

		dist := mp.planOpts.DistanceFunc(&Segment{StartConfiguration: near.Q(), EndConfiguration: target.Q()})
		oldDist := mp.planOpts.DistanceFunc(&Segment{StartConfiguration: oldNear.Q(), EndConfiguration: target.Q()})
		switch {
		case dist < mp.algOpts.JointSolveDist:
			mchan <- near
			return
		case dist > oldDist:
			mchan <- oldNear
			return
		}

		oldNear = near
		newNear := make([]referenceframe.Input, 0, len(near.Q()))

		// alter near to be closer to target
		for j, nearInput := range near.Q() {
			if nearInput.Value == target.Q()[j].Value {
				newNear = append(newNear, nearInput)
			} else {
				v1, v2 := nearInput.Value, target.Q()[j].Value
				newVal := math.Min(qstep[j], math.Abs(v2-v1))
				// get correct sign
				newVal *= (v2 - v1) / math.Abs(v2-v1)
				newNear = append(newNear, referenceframe.Input{nearInput.Value + newVal})
			}
		}
		// Check whether newNear meets constraints, and if not, update it to a configuration that does meet constraints (or nil)
		newNear = mp.constrainNear(ctx, randseed, oldNear.Q(), newNear)

		if newNear != nil {
			nearDist := mp.planOpts.DistanceFunc(&Segment{StartConfiguration: oldNear.Q(), EndConfiguration: newNear})
			if nearDist < math.Pow(mp.algOpts.JointSolveDist, 3) {
				if !doubled {
					doubled = true
					// Check if doubling qstep will allow escape from the identical configuration
					// If not, we terminate and return.
					// If so, qstep will be reset to its original value after the rescue.
					for i, q := range qstep {
						qstep[i] = q * 2.0
					}
					continue
				} else {
					// We've arrived back at very nearly the same configuration again; stop solving and send back oldNear.
					// Do not add the near-identical configuration to the RRT map
					mchan <- oldNear
					return
				}
			}
			if doubled {
				copy(qstep, mp.algOpts.qstep)
				doubled = false
			}
			// constrainNear will ensure path between oldNear and newNear satisfies constraints along the way
			near = &basicNode{q: newNear, cost: neighbors[0].node.Cost() + neighbors[0].dist}
			rrtMap[near] = oldNear

			// rewire the tree
			for i, thisNeighbor := range neighbors {
				// dont need to try to rewire nearest neighbor, so skip it
				if i == 0 {
					continue
				}

				// check to see if a shortcut is possible, and rewire the node if it is
				connectionCost := mp.planOpts.DistanceFunc(&Segment{
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
		} else {
			break
		}
	}
	mchan <- oldNear
}

func (mp *rrtStarConnectMotionPlanner) constrainNear(
	ctx context.Context,
	randseed *rand.Rand,
	seedInputs,
	target []referenceframe.Input,
) []referenceframe.Input {
	for i := 0; i < maxNearIter; i++ {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		seedPos, err := mp.frame.Transform(seedInputs)
		if err != nil {
			return nil
		}
		goalPos, err := mp.frame.Transform(target)
		if err != nil {
			return nil
		}

		newArc := &Segment{
			StartPosition:      seedPos,
			EndPosition:        goalPos,
			StartConfiguration: seedInputs,
			EndConfiguration:   target,
			Frame:              mp.frame,
		}

		// Check if the arc of "seedInputs" to "target" is valid
		ok, _ := mp.planOpts.CheckSegmentAndStateValidity(newArc, mp.planOpts.Resolution)
		if ok {
			return target
		}
		solutionGen := make(chan []referenceframe.Input, 1)
		// Spawn the IK solver to generate solutions until done
		err = mp.fastGradDescent.Solve(ctx, solutionGen, target, mp.planOpts.pathMetric, randseed.Int())
		// We should have zero or one solutions
		var solved []referenceframe.Input
		select {
		case solved = <-solutionGen:
		default:
		}
		close(solutionGen)
		if err != nil {
			return nil
		}

		ok, failpos := mp.planOpts.CheckSegmentAndStateValidity(
			&Segment{StartConfiguration: seedInputs, EndConfiguration: solved, Frame: mp.frame},
			mp.planOpts.Resolution,
		)
		if ok {
			return solved
		}
		if failpos != nil {
			dist := mp.planOpts.DistanceFunc(&Segment{StartConfiguration: target, EndConfiguration: failpos.EndConfiguration})
			if dist > mp.algOpts.JointSolveDist {
				// If we have a first failing position, and that target is updating (no infinite loop), then recurse
				seedInputs = failpos.StartConfiguration
				target = failpos.EndConfiguration
			}
		} else {
			return nil
		}
	}
	return nil
}
