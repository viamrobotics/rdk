package armplanning

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

const (
	// Maximum number of iterations that constrainNear will run before exiting nil.
	// Typically it will solve in the first five iterations, or not at all.
	maxNearIter = 20

	// Maximum number of iterations that constrainedExtend will run before exiting.
	maxExtendIter = 5000

	// When we generate solutions, if a new solution is within this level of similarity to an existing one, discard it as a duplicate.
	// This prevents seeding the solution tree with 50 copies of essentially the same configuration.
	defaultSimScore = 0.05
)

// cBiRRTMotionPlanner an object able to solve constrained paths around obstacles to some goal for a given referenceframe.
// It uses the Constrained Bidirctional Rapidly-expanding Random Tree algorithm, Berenson et al 2009
// https://ieeexplore.ieee.org/document/5152399/
type cBiRRTMotionPlanner struct {
	pc *planContext
	psc *planSegmentContext
	
	fastGradDescent *ik.NloptIK
}

// newCBiRRTMotionPlannerWithSeed creates a cBiRRTMotionPlanner object with a user specified random seed.
func newCBiRRTMotionPlanner(pc *planContext, psc *planSegmentContext) (*cBiRRTMotionPlanner, error) {
	c := &cBiRRTMotionPlanner{
		pc: pc,
		psc: psc,
	}

	var err error
	
	// nlopt should try only once
	c.fastGradDescent, err = ik.CreateNloptSolver(pc.lfs.dof, pc.logger, 1, true, true)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// only used for testin.
func (mp *cBiRRTMotionPlanner) planForTest(ctx context.Context) ([]referenceframe.FrameSystemInputs, error) {
	initMaps, err := initRRTSolutions(ctx, mp.psc)
	if err != nil {
		return nil, err
	}

	if initMaps.steps != nil {
		return initMaps.steps, nil
	}
	solution, err := mp.rrtRunner(ctx, initMaps.maps)
	if err != nil {
		return nil, err
	}
	return solution.steps, nil
}

// rrtRunner will execute the plan. Plan() will call rrtRunner in a separate thread and wait for results.
// Separating this allows other things to call rrtRunner in parallel allowing the thread-agnostic Plan to be accessible.
func (mp *cBiRRTMotionPlanner) rrtRunner(
	ctx context.Context,
	rrtMaps *rrtMaps,
) (*rrtSolution, error) {
	mp.pc.logger.CDebugf(ctx, "starting cbirrt with start map len %d and goal map len %d\n", len(rrtMaps.startMap), len(rrtMaps.goalMap))

	// setup planner options
	if mp.pc.planOpts == nil {
		return nil, errNoPlannerOptions
	}

	_, cancel := context.WithCancel(ctx)
	defer cancel()
	startTime := time.Now()

	var seed referenceframe.FrameSystemInputs

	// initialize maps
	// Pick a random (first in map) seed node to create the first interp node
	for sNode, parent := range rrtMaps.startMap {
		if parent == nil {
			seed = sNode.inputs
			break
		}
	}
	mp.pc.logger.CDebugf(ctx, "goal node: %v\n", rrtMaps.optNode.inputs)
	mp.pc.logger.CDebugf(ctx, "start node: %v\n", seed)
	mp.pc.logger.Debug("DOF", mp.pc.lfs.dof)

	interpConfig, err := referenceframe.InterpolateFS(mp.pc.fs, seed, rrtMaps.optNode.inputs, 0.5)
	if err != nil {
		return nil, err
	}

	target := newConfigurationNode(interpConfig)

	map1, map2 := rrtMaps.startMap, rrtMaps.goalMap
	for i := 0; i < mp.pc.planOpts.PlanIter; i++ {
		mp.pc.logger.CDebugf(ctx, "iteration: %d target: %v\n", i, target.inputs)
		if ctx.Err() != nil {
			mp.pc.logger.CDebugf(ctx, "CBiRRT timed out after %d iterations", i)
			return &rrtSolution{maps: rrtMaps}, fmt.Errorf("cbirrt timeout %w", ctx.Err())
		}

		tryExtend := func(target *node) (*node, *node) {
			// attempt to extend maps 1 and 2 towards the target

			nearest1 := nearestNeighbor(target, map1, nodeConfigurationDistanceFunc)
			nearest2 := nearestNeighbor(target, map2, nodeConfigurationDistanceFunc)

			map1reached := mp.constrainedExtend(ctx, i, map1, nearest1, target)
			map2reached := mp.constrainedExtend(ctx, i, map2, nearest2, target)

			map1reached.corner = true
			map2reached.corner = true

			return map1reached, map2reached
		}

		map1reached, map2reached := tryExtend(target)

		reachedDelta := mp.pc.configurationDistanceFunc(
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

			reachedDelta = mp.pc.configurationDistanceFunc(&motionplan.SegmentFS{
				StartConfiguration: map1reached.inputs,
				EndConfiguration:   map2reached.inputs,
			})
		}

		// Solved!
		if reachedDelta <= mp.pc.planOpts.InputIdentDist {
			mp.pc.logger.CDebugf(ctx, "CBiRRT found solution after %d iterations in %v", i, time.Since(startTime))
			cancel()
			path := extractPath(rrtMaps.startMap, rrtMaps.goalMap, &nodePair{map1reached, map2reached}, true)
			return &rrtSolution{steps: path, maps: rrtMaps}, nil
		}

		// sample near map 1 and switch which map is which to keep adding to them even
		target, err = mp.sample(map1reached, i)
		if err != nil {
			return &rrtSolution{maps: rrtMaps}, err
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
		configDistMetric := mp.pc.configurationDistanceFunc
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

		nearDist := mp.pc.configurationDistanceFunc(
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
	target referenceframe.FrameSystemInputs,
) referenceframe.FrameSystemInputs {
	for i := 0; i < maxNearIter; i++ {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		newArc := &motionplan.SegmentFS{
			StartConfiguration: seedInputs,
			EndConfiguration:   target,
			FS:                 mp.pc.fs,
		}

		// Check if the arc of "seedInputs" to "target" is valid
		_, err := mp.psc.checker.CheckSegmentAndStateValidityFS(newArc, mp.pc.planOpts.Resolution)
		if err == nil {
			return target
		}

		linearSeed, err := mp.pc.lfs.mapToSlice(target)
		if err != nil {
			mp.pc.logger.Infof("constrainNear fail: %v", err)
			return nil
		}

		solutions, err := ik.DoSolve(ctx, mp.fastGradDescent, mp.pc.linearizeFSmetric(mp.psc.checker.PathMetric()), linearSeed)
		if err != nil {
			mp.pc.logger.Infof("constrainNear fail: %v", err)
			return nil
		}

		if len(solutions) == 0 {
			return nil
		}

		solutionMap, err := mp.pc.lfs.sliceToMap(solutions[0])
		if err != nil {
			mp.pc.logger.Infof("constrainNear fail: %v", err)
			return nil
		}

		failpos, err := mp.psc.checker.CheckSegmentAndStateValidityFS(
			&motionplan.SegmentFS{
				StartConfiguration: seedInputs,
				EndConfiguration:   solutionMap,
				FS:                 mp.pc.fs,
			},
			mp.pc.planOpts.Resolution,
		)
		if err == nil {
			return solutionMap
		}
		if failpos != nil {
			dist := mp.pc.configurationDistanceFunc(&motionplan.SegmentFS{
				StartConfiguration: target,
				EndConfiguration:   failpos.EndConfiguration,
			})
			if dist > mp.pc.planOpts.InputIdentDist {
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


// getFrameSteps will return a slice of positive values representing the largest amount a particular DOF of a frame should
// move in any given step. The second argument is a float describing the percentage of the total movement.
func (mp *cBiRRTMotionPlanner) getFrameSteps(percentTotalMovement float64, iterationNumber int, double bool) map[string][]float64 {
	moving, _ := mp.psc.motionChains.framesFilteredByMovingAndNonmoving()

	frameQstep := map[string][]float64{}
	for _, f := range mp.pc.lfs.frames {
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
			l, u := lim.Min, lim.Max

			// Default to [-999,999] as range if limits are infinite
			if l == math.Inf(-1) {
				l = -999
			}
			if u == math.Inf(1) {
				u = 999
			}

			jRange := math.Abs(u - l)
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
	// If we have done more than 50 iterations, start seeding off completely random positions 2 at a time
	// The 2 at a time is to ensure random seeds are added onto both the seed and gofsal maps.
	if sampleNum >= mp.pc.planOpts.IterBeforeRand && sampleNum%4 >= 2 {
		randomInputs := make(referenceframe.FrameSystemInputs)
		for _, name := range mp.pc.fs.FrameNames() {
			f := mp.pc.fs.Frame(name)
			if f != nil && len(f.DoF()) > 0 {
				randomInputs[name] = referenceframe.RandomFrameInputs(f, mp.pc.randseed)
			}
		}
		return newConfigurationNode(randomInputs), nil
	}

	// Seeding nearby to valid points results in much faster convergence in less constrained space
	newInputs := make(referenceframe.FrameSystemInputs)
	for name, inputs := range rSeed.inputs {
		f := mp.pc.fs.Frame(name)
		if f != nil && len(f.DoF()) > 0 {
			q, err := referenceframe.RestrictedRandomFrameInputs(f, mp.pc.randseed, 0.1, inputs)
			if err != nil {
				return nil, err
			}
			newInputs[name] = q
		}
	}
	return newConfigurationNode(newInputs), nil
}


