package armplanning

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"time"

	"go.viam.com/utils/trace"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

const (
	maxPlanIter = 5000

	// Maximum number of iterations that constrainedExtend will run before exiting.
	maxExtendIter = 5000

	// When we generate solutions, if a new solution is within this level of similarity to an existing one, discard it as a duplicate.
	// This prevents seeding the solution tree with 50 copies of essentially the same configuration.
	defaultSimScore = 0.05
)

var debugConstrainNear = false

// cBiRRTMotionPlanner an object able to solve constrained paths around obstacles to some goal for a given referenceframe.
// It uses the Constrained Bidirctional Rapidly-expanding Random Tree algorithm, Berenson et al 2009
// https://ieeexplore.ieee.org/document/5152399/
type cBiRRTMotionPlanner struct {
	pc     *PlanContext
	psc    *PlanSegmentContext
	logger logging.Logger

	fastGradDescent *ik.NloptIK

	// rnd drives sampling. Defaults to the shared PlanContext rand; racing
	// attempts (raceCBiRRT) each get their own stream so they explore
	// different regions and can run concurrently.
	rnd *rand.Rand

	// maxIter caps rrtRunner's iterations when positive; raceCBiRRT uses short
	// caps so failed attempts restart with a fresh RNG stream instead of
	// grinding a bad trajectory to the 5000-iteration default.
	maxIter int
}

// newCBiRRTMotionPlannerWithSeed creates a cBiRRTMotionPlanner object with a user specified random seed.
func newCBiRRTMotionPlanner(ctx context.Context, pc *PlanContext, psc *PlanSegmentContext, logger logging.Logger,
) (*cBiRRTMotionPlanner, error) {
	_, span := trace.StartSpan(ctx, "newCBiRRTMotionPlanner")
	defer span.End()
	c := &cBiRRTMotionPlanner{
		pc:     pc,
		psc:    psc,
		logger: logger,
		rnd:    pc.randseed,
	}

	var err error

	// nlopt should try only once
	c.fastGradDescent, err = ik.CreateNloptSolver(logger, 1, true, true, time.Second)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// only used for testin.
func (mp *cBiRRTMotionPlanner) planForTest(ctx context.Context) ([]*referenceframe.LinearInputs, error) {
	initMaps, err := initRRTSolutions(ctx, mp.psc, mp.psc.pc.logger.Sublogger("solve"))
	if err != nil {
		return nil, err
	}

	x := []*referenceframe.LinearInputs{mp.psc.start}

	if initMaps.steps != nil {
		x = append(x, initMaps.steps...)
		return x, nil
	}

	solution, err := mp.rrtRunner(ctx, initMaps.maps)
	if err != nil {
		return nil, err
	}

	x = append(x, solution.steps...)

	return x, nil
}

func (mp *cBiRRTMotionPlanner) rrtRunner(
	ctx context.Context,
	rrtMaps *rrtMaps,
) (*rrtSolution, error) {
	ctx, span := trace.StartSpan(ctx, "rrtRunner")
	defer span.End()

	mp.logger.CDebugf(ctx, "starting cbirrt with start map len %d and goal map len %d\n", len(rrtMaps.startMap), len(rrtMaps.goalMap))
	// setup planner options
	if mp.pc.planOpts == nil {
		return nil, errNoPlannerOptions
	}

	_, cancel := context.WithCancel(ctx)
	defer cancel()
	startTime := time.Now()

	corridorTops := mp.seedGoalRetreatCorridors(ctx, rrtMaps)

	// initialize maps
	// Pick a random (first in map) seed node to create the first interp node
	var seed *referenceframe.LinearInputs
	for sNode, parent := range rrtMaps.startMap {
		if parent == nil {
			seed = sNode.inputs
			break
		}
	}
	mp.logger.CDebugf(ctx, "goal node: %v\n", rrtMaps.optNode.inputs)
	mp.logger.CDebugf(ctx, "start node: %v\n", seed)
	mp.logger.Debug("DOF", mp.pc.lis.GetLimits())

	interpConfig, err := referenceframe.InterpolateFS(mp.pc.fs, seed, rrtMaps.optNode.inputs, 0.5)
	if err != nil {
		return nil, err
	}

	target := newConfigurationNode(interpConfig)

	iterCap := maxPlanIter
	if mp.maxIter > 0 && mp.maxIter < iterCap {
		iterCap = mp.maxIter
	}
	map1, map2 := rrtMaps.startMap, rrtMaps.goalMap
	for i := 0; i < iterCap; i++ {
		mp.logger.CDebugf(ctx, "iteration: %d target: %v", i, logging.FloatArrayFormat{"", target.inputs.GetLinearizedInputs()})
		if ctx.Err() != nil {
			mp.logger.CDebugf(ctx, "CBiRRT timed out after %d iterations", i)
			return &rrtSolution{maps: rrtMaps}, fmt.Errorf("cbirrt timeout %w", ctx.Err())
		}

		tryExtend := func(target *node) (*node, *node) {
			// attempt to extend maps 1 and 2 towards the target

			nearest1 := nearestNeighbor(target, map1, nodeConfigurationDistanceFunc)
			nearest2 := nearestNeighbor(target, map2, nodeConfigurationDistanceFunc)

			map1reached := mp.constrainedExtend(ctx, i, map1, nearest1, target)
			map2reached := mp.constrainedExtend(ctx, i, map2, nearest2, target)

			return map1reached, map2reached
		}

		map1reached, map2reached := tryExtend(target)

		reachedDelta := mp.pc.ConfigurationDistanceFunc(
			&motionplan.SegmentFS{
				StartConfiguration: map1reached.inputs,
				EndConfiguration:   map2reached.inputs,
			},
		)

		// Second iteration; extend maps 1 and 2 towards the halfway point between where they reached
		if reachedDelta > mp.pc.planOpts.InputIdentDist {
			targetConf, err := referenceframe.InterpolateFS(mp.pc.fs, map1reached.inputs, map2reached.inputs, 0.5)
			if err != nil {
				return &rrtSolution{maps: rrtMaps}, err
			}
			target = newConfigurationNode(targetConf)
			map1reached, map2reached = tryExtend(target)

			reachedDelta = mp.pc.ConfigurationDistanceFunc(&motionplan.SegmentFS{
				StartConfiguration: map1reached.inputs,
				EndConfiguration:   map2reached.inputs,
			})
		}

		// Solved!
		if reachedDelta <= mp.pc.planOpts.InputIdentDist {
			mp.logger.CDebugf(ctx, "CBiRRT found solution after %d iterations in %v", i, time.Since(startTime))
			cancel()
			path := extractPath(rrtMaps.startMap, rrtMaps.goalMap, &nodePair{map1reached, map2reached}, true)
			return &rrtSolution{steps: path, maps: rrtMaps}, nil
		}

		// Aim the early iterations at the goal corridors' open-space entrances:
		// they are already wired into the goal tree, so each such iteration is
		// a direct attempt to connect the start tree to a validated descent.
		// After that, revisit them occasionally as the trees grow.
		if len(corridorTops) > 0 && (i < 2*len(corridorTops) || i%8 == 0) {
			target = newConfigurationNode(corridorTops[i%len(corridorTops)].inputs)
		} else {
			// sample near map 1 and switch which map is which to keep adding to them even
			target, err = mp.sample(map1reached, i)
			if err != nil {
				return &rrtSolution{maps: rrtMaps}, err
			}
		}
		map1, map2 = map2, map1
	}

	return &rrtSolution{maps: rrtMaps}, errPlannerFailed
}

// constrainedExtend will try to extend the map towards the target while meeting constraints along the way. It will
// return the closest solution to the target that it reaches, which may or may not actually be the target.
func (mp *cBiRRTMotionPlanner) constrainedExtend(
	ctx context.Context,
	iterationNumber int,
	rrtMap map[*node]*node,
	near, target *node,
) *node {
	qstep := mp.getFrameSteps(defaultFrameStep, iterationNumber, false)

	// Allow qstep to be doubled as a means to escape from configurations which gradient descend to their seed
	doubled := false

	oldNear := near
	// This should iterate until one of the following conditions:
	// 1) we have reached the target
	// 2) the request is cancelled/times out
	// 3) we are no longer approaching the target and our "best" node is further away than the previous best
	// 4) further iterations change our best node by close-to-zero amounts
	// 5) we have iterated more than maxExtendIter times
	for i := 0; i < maxExtendIter; i++ {
		configDistMetric := mp.pc.ConfigurationDistanceFunc
		dist := configDistMetric(
			&motionplan.SegmentFS{StartConfiguration: near.inputs, EndConfiguration: target.inputs})
		oldDist := configDistMetric(
			&motionplan.SegmentFS{StartConfiguration: oldNear.inputs, EndConfiguration: target.inputs})

		switch {
		case dist < mp.pc.planOpts.InputIdentDist:
			return near
		case dist > oldDist:
			return oldNear
		}

		oldNear = near

		newNear := fixedStepInterpolation(near, target, qstep)
		// Check whether newNear meets constraints, and if not, update it to a configuration that does meet constraints (or nil)
		newNear = mp.constrainNear(ctx, oldNear.inputs, newNear)

		if newNear == nil {
			return oldNear
		}

		nearDist := mp.pc.ConfigurationDistanceFunc(
			&motionplan.SegmentFS{StartConfiguration: oldNear.inputs, EndConfiguration: newNear})

		if nearDist < math.Pow(mp.pc.planOpts.InputIdentDist, 3) {
			if !doubled {
				// Check if doubling qstep will allow escape from the identical configuration
				// If not, we terminate and return.
				// If so, qstep will be reset to its original value after the rescue.

				doubled = true
				qstep = mp.getFrameSteps(defaultFrameStep, iterationNumber, true)
				continue
			}
			// We've arrived back at very nearly the same configuration again; stop solving and send back oldNear.
			// Do not add the near-identical configuration to the RRT map
			return oldNear
		}
		if doubled {
			qstep = mp.getFrameSteps(defaultFrameStep, iterationNumber, false)
			doubled = false
		}
		// constrainNear will ensure path between oldNear and newNear satisfies constraints along the way
		near = &node{inputs: newNear}
		rrtMap[near] = oldNear
	}
	return oldNear
}

// constrainNear will do a IK gradient descent from seedInputs to target. If a gradient descent distance
// function has been specified, this will use that.
// This function will return either a valid configuration that meets constraints, or nil.
func (mp *cBiRRTMotionPlanner) constrainNear(
	ctx context.Context,
	seedInputs,
	target *referenceframe.LinearInputs,
) *referenceframe.LinearInputs {
	if debugConstrainNear {
		mp.logger.Infof("constrainNear called")
		mp.logger.Infof("\t start: %v end: %v",
			logging.FloatArrayFormat{"", seedInputs.GetLinearizedInputs()}, logging.FloatArrayFormat{"", target.GetLinearizedInputs()})
	}

	newArc := &motionplan.SegmentFS{
		StartConfiguration: seedInputs,
		EndConfiguration:   target,
		FS:                 mp.pc.fs,
	}

	// Check if the arc of "seedInputs" to "target" is valid
	_, err := mp.psc.Checker.CheckStateConstraintsAcrossSegmentFS(ctx, newArc, mp.pc.planOpts.Resolution, true)
	if debugConstrainNear {
		mp.logger.Infof("\t err %v", err)
	}
	if err == nil {
		return target
	}

	if metric := mp.psc.topoProjectionMetric(); metric != nil {
		projected := mp.psc.projectToOrientationBand(ctx, mp.fastGradDescent, metric, target)
		if projected == nil {
			mp.logger.Debugf("constrainNear: orientation projection failed")
			return nil
		}
		if debugConstrainNear {
			mp.logger.Infof("\t -> %v", logging.FloatArrayFormat{"", projected.GetLinearizedInputs()})
		}
		target = projected
	}

	failpos, err := mp.psc.Checker.CheckStateConstraintsAcrossSegmentFS(
		ctx,
		&motionplan.SegmentFS{
			StartConfiguration: seedInputs,
			EndConfiguration:   target,
			FS:                 mp.pc.fs,
		},
		mp.pc.planOpts.Resolution,
		true,
	)
	if debugConstrainNear {
		mp.logger.Infof("\t failpos: %v err: %v", failpos != nil, err)
	}
	if err == nil {
		return target
	}

	if failpos == nil {
		// no forward progress
		return nil
	}

	dist := mp.pc.ConfigurationDistanceFunc(&motionplan.SegmentFS{
		StartConfiguration: seedInputs,
		EndConfiguration:   failpos.EndConfiguration,
	})

	if dist < mp.pc.planOpts.InputIdentDist {
		// next position is no better, give up
		return nil
	}

	target = failpos.EndConfiguration
	if debugConstrainNear {
		mp.logger.Infof("\t new target %v", logging.FloatArrayFormat{"", target.GetLinearizedInputs()})
	}
	return target
}

// getFrameSteps will return a slice of positive values representing the largest amount a particular DOF of a frame should
// move in any given step. The second argument is a float describing the percentage of the total movement.
func (mp *cBiRRTMotionPlanner) getFrameSteps(percentTotalMovement float64, iterationNumber int, double bool) map[string][]float64 {
	moving, _ := mp.psc.motionChains.framesFilteredByMovingAndNonmoving()

	frameQstep := map[string][]float64{}
	for _, fName := range mp.pc.lis.FrameNamesInOrder() {
		f := mp.pc.fs.Frame(fName)
		if f == nil {
			continue
		}

		isMoving := slices.Contains(moving, f.Name())
		if !isMoving && !double {
			continue
		}

		dof := f.DoF()
		if len(dof) == 0 {
			continue
		}

		pos := make([]float64, len(dof))
		for i, lim := range dof {
			_, _, jRange := lim.GoodLimits()
			pos[i] = jRange * percentTotalMovement

			if isMoving {
				if iterationNumber > 20 {
					pos[i] *= 2
				}
				if double {
					pos[i] *= 2
				}
			} else { // nonmoving
				// we move non-moving frames just a little if we have to get them out of the way
				if iterationNumber > 50 {
					pos[i] *= .5
				} else if iterationNumber > 20 {
					pos[i] *= .25 // we move non-moving frames just a little if we have to get them out of the way
				} else {
					pos[i] = 0
				}
			}
		}
		frameQstep[f.Name()] = pos
	}
	return frameQstep
}

func (mp *cBiRRTMotionPlanner) sample(rSeed *node, sampleNum int) (*node, error) {
	// we look close first, and expand
	// we try to find a balance between not making wild motions for simple motions
	// while looking broadly for situations we have to make large movements to work around obstacles.

	percent := min(1, float64(sampleNum)/1000)

	newInputs := referenceframe.NewLinearInputs()
	for name, inputs := range rSeed.inputs.Items() {
		f := mp.pc.fs.Frame(name)
		if f != nil && len(f.DoF()) > 0 {
			q, err := referenceframe.RestrictedRandomFrameInputs(f, mp.rnd, percent, inputs)
			if err != nil {
				return nil, err
			}
			newInputs.Put(name, q)
		}
	}
	return newConfigurationNode(newInputs), nil
}

// seedGoalRetreatCorridors pre-populates the goal tree with a "descent
// corridor" per goal root: a chain of validated configurations jogging the
// end effector straight up into open space. The final approach to a goal is
// usually the most cluttered part of a scene, and random bidirectional
// sampling spends most of its iterations rediscovering how to descend into
// it; a deterministic retreat chain lets the search connect to the corridor
// high in open space instead, with the descent already validated.
func (mp *cBiRRTMotionPlanner) seedGoalRetreatCorridors(ctx context.Context, rrtMaps *rrtMaps) []*node {
	if len(mp.psc.goal) != 1 {
		return nil
	}
	var frame string
	for f := range mp.psc.goal {
		frame = f
	}

	roots := []*node{}
	isParent := map[*node]bool{}
	childCount := 0
	for n, parent := range rrtMaps.goalMap {
		if parent == nil {
			roots = append(roots, n)
		} else {
			childCount++
			isParent[parent] = true
		}
	}
	if childCount > 0 {
		// Corridors were already seeded (racing attempts share pre-seeded
		// maps); the corridor tops are the non-root leaves.
		tops := []*node{}
		for n, parent := range rrtMaps.goalMap {
			if parent != nil && !isParent[n] {
				tops = append(tops, n)
			}
		}
		return tops
	}

	added := 0
	tops := []*node{}
	for _, root := range roots {
		chain := retreatChain(ctx, mp.psc, mp.fastGradDescent, frame, root.inputs)
		cur := root
		for _, cfg := range chain[1:] {
			n := &node{inputs: cfg}
			rrtMaps.goalMap[n] = cur
			cur = n
			added++
		}
		if cur != root {
			tops = append(tops, cur)
		}
	}
	if added > 0 {
		mp.logger.CDebugf(ctx, "seeded %d goal retreat corridor nodes across %d roots (%d corridor tops)", added, len(roots), len(tops))
	}
	return tops
}
