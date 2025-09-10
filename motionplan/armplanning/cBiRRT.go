package armplanning

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"go.viam.com/utils"

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
	checker                   *motionplan.ConstraintChecker
	fs                        *referenceframe.FrameSystem
	lfs                       *linearizedFrameSystem
	solver                    ik.Solver
	logger                    logging.Logger
	randseed                  *rand.Rand
	start                     time.Time
	configurationDistanceFunc motionplan.SegmentFSMetric
	planOpts                  *PlannerOptions
	motionChains              *motionChains

	fastGradDescent ik.Solver
	qstep           map[string][]float64
}

// newCBiRRTMotionPlannerWithSeed creates a cBiRRTMotionPlanner object with a user specified random seed.
func newCBiRRTMotionPlanner(
	fs *referenceframe.FrameSystem,
	seed *rand.Rand,
	logger logging.Logger,
	opt *PlannerOptions,
	constraintHandler *motionplan.ConstraintChecker,
	chains *motionChains,
) (*cBiRRTMotionPlanner, error) {
	if opt == nil {
		return nil, errNoPlannerOptions
	}

	lfs, err := newLinearizedFrameSystem(fs)
	if err != nil {
		return nil, err
	}

	if constraintHandler == nil {
		constraintHandler = motionplan.NewEmptyConstraintChecker()
	}
	if chains == nil {
		chains = &motionChains{}
	}

	solver, err := ik.CreateCombinedIKSolver(lfs.dof, logger, opt.NumThreads, opt.GoalThreshold)
	if err != nil {
		return nil, err
	}

	// nlopt should try only once
	nlopt, err := ik.CreateNloptSolver(lfs.dof, logger, 1, true, true)
	if err != nil {
		return nil, err
	}

	return &cBiRRTMotionPlanner{
		checker:                   constraintHandler,
		solver:                    solver,
		fs:                        fs,
		lfs:                       lfs,
		logger:                    logger,
		randseed:                  seed,
		planOpts:                  opt,
		configurationDistanceFunc: motionplan.GetConfigurationDistanceFunc(opt.ConfigurationDistanceMetric),
		motionChains:              chains,
		fastGradDescent:           nlopt,
		qstep:                     getFrameSteps(lfs, defaultFrameStep),
	}, nil
}

// only used for testin.
func (mp *cBiRRTMotionPlanner) planForTest(ctx context.Context, seed, goal *PlanState) ([]node, error) {
	solutionChan := make(chan *rrtSolution, 1)
	initMaps := initRRTSolutions(ctx, atomicWaypoint{mp: mp, startState: seed, goalState: goal})
	if initMaps.err != nil {
		return nil, initMaps.err
	}
	if initMaps.steps != nil {
		return initMaps.steps, nil
	}
	utils.PanicCapturingGo(func() {
		mp.rrtBackgroundRunner(ctx, &rrtParallelPlannerShared{initMaps.maps, nil, solutionChan})
	})
	solution := <-solutionChan
	if solution.err != nil {
		return nil, solution.err
	}
	return solution.steps, nil
}

// rrtBackgroundRunner will execute the plan. Plan() will call rrtBackgroundRunner in a separate thread and wait for results.
// Separating this allows other things to call rrtBackgroundRunner in parallel allowing the thread-agnostic Plan to be accessible.
func (mp *cBiRRTMotionPlanner) rrtBackgroundRunner(
	ctx context.Context,
	rrt *rrtParallelPlannerShared,
) {
	defer close(rrt.solutionChan)
	mp.logger.CDebugf(ctx, "starting cbirrt with start map len %d and goal map len %d\n", len(rrt.maps.startMap), len(rrt.maps.goalMap))

	// setup planner options
	if mp.planOpts == nil {
		rrt.solutionChan <- &rrtSolution{err: errNoPlannerOptions}
		return
	}

	_, cancel := context.WithCancel(ctx)
	defer cancel()
	mp.start = time.Now()

	var seed referenceframe.FrameSystemInputs

	// initialize maps
	// Pick a random (first in map) seed node to create the first interp node
	for sNode, parent := range rrt.maps.startMap {
		if parent == nil {
			seed = sNode.Q()
			break
		}
	}
	mp.logger.CDebugf(ctx, "goal node: %v\n", rrt.maps.optNode.Q())
	for n := range rrt.maps.startMap {
		mp.logger.CDebugf(ctx, "start node: %v\n", n.Q())
		break
	}
	mp.logger.Debug("DOF", mp.lfs.dof)
	interpConfig, err := referenceframe.InterpolateFS(mp.fs, seed, rrt.maps.optNode.Q(), 0.5)
	if err != nil {
		rrt.solutionChan <- &rrtSolution{err: err}
		return
	}
	target := newConfigurationNode(interpConfig)

	map1, map2 := rrt.maps.startMap, rrt.maps.goalMap

	m1chan := make(chan node, 1)
	m2chan := make(chan node, 1)
	defer close(m1chan)
	defer close(m2chan)

	for i := 0; i < mp.planOpts.PlanIter; i++ {
		select {
		case <-ctx.Done():
			mp.logger.CDebugf(ctx, "CBiRRT timed out after %d iterations", i)
			rrt.solutionChan <- &rrtSolution{err: fmt.Errorf("cbirrt timeout %w", ctx.Err()), maps: rrt.maps}
			return
		default:
		}
		if i > 0 && i%100 == 0 {
			mp.logger.CDebugf(ctx, "CBiRRT planner iteration %d", i)
		}

		tryExtend := func(target node) (node, node, error) {
			// attempt to extend maps 1 and 2 towards the target
			utils.PanicCapturingGo(func() {
				m1chan <- nearestNeighbor(target, map1, nodeConfigurationDistanceFunc)
			})
			utils.PanicCapturingGo(func() {
				m2chan <- nearestNeighbor(target, map2, nodeConfigurationDistanceFunc)
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
				mp.constrainedExtend(ctx, rseed1, map1, nearest1, target, m1chan)
			})
			utils.PanicCapturingGo(func() {
				mp.constrainedExtend(ctx, rseed2, map2, nearest2, target, m2chan)
			})
			map1reached := <-m1chan
			map2reached := <-m2chan

			map1reached.SetCorner(true)
			map2reached.SetCorner(true)

			return map1reached, map2reached, nil
		}

		map1reached, map2reached, err := tryExtend(target)
		if err != nil {
			rrt.solutionChan <- &rrtSolution{err: err, maps: rrt.maps}
			return
		}

		reachedDelta := mp.configurationDistanceFunc(
			&motionplan.SegmentFS{
				StartConfiguration: map1reached.Q(),
				EndConfiguration:   map2reached.Q(),
			},
		)

		// Second iteration; extend maps 1 and 2 towards the halfway point between where they reached
		if reachedDelta > mp.planOpts.InputIdentDist {
			targetConf, err := referenceframe.InterpolateFS(mp.fs, map1reached.Q(), map2reached.Q(), 0.5)
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
			reachedDelta = mp.configurationDistanceFunc(&motionplan.SegmentFS{
				StartConfiguration: map1reached.Q(),
				EndConfiguration:   map2reached.Q(),
			})
		}

		// Solved!
		if reachedDelta <= mp.planOpts.InputIdentDist {
			mp.logger.CDebugf(ctx, "CBiRRT found solution after %d iterations", i)
			cancel()
			path := extractPath(rrt.maps.startMap, rrt.maps.goalMap, &nodePair{map1reached, map2reached}, true)
			rrt.solutionChan <- &rrtSolution{steps: path, maps: rrt.maps}
			return
		}

		// sample near map 1 and switch which map is which to keep adding to them even
		target, err = mp.sample(map1reached, i)
		if err != nil {
			rrt.solutionChan <- &rrtSolution{err: err, maps: rrt.maps}
			return
		}
		map1, map2 = map2, map1
	}
	rrt.solutionChan <- &rrtSolution{err: errPlannerFailed, maps: rrt.maps}
}

// constrainedExtend will try to extend the map towards the target while meeting constraints along the way. It will
// return the closest solution to the target that it reaches, which may or may not actually be the target.
func (mp *cBiRRTMotionPlanner) constrainedExtend(
	ctx context.Context,
	randseed *rand.Rand,
	rrtMap map[node]node,
	near, target node,
	mchan chan node,
) {
	// Allow qstep to be doubled as a means to escape from configurations which gradient descend to their seed
	deepCopyQstep := func() map[string][]float64 {
		qstep := map[string][]float64{}
		for fName, fStep := range mp.qstep {
			newStep := make([]float64, len(fStep))
			copy(newStep, fStep)
			qstep[fName] = newStep
		}
		return qstep
	}
	qstep := deepCopyQstep()
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
		configDistMetric := mp.configurationDistanceFunc
		dist := configDistMetric(
			&motionplan.SegmentFS{StartConfiguration: near.Q(), EndConfiguration: target.Q()})
		oldDist := configDistMetric(
			&motionplan.SegmentFS{StartConfiguration: oldNear.Q(), EndConfiguration: target.Q()})

		switch {
		case dist < mp.planOpts.InputIdentDist:
			mchan <- near
			return
		case dist > oldDist:
			mchan <- oldNear
			return
		}

		oldNear = near

		newNear := fixedStepInterpolation(near, target, mp.qstep)
		// Check whether newNear meets constraints, and if not, update it to a configuration that does meet constraints (or nil)
		newNear = mp.constrainNear(ctx, randseed, oldNear.Q(), newNear)

		if newNear != nil {
			nearDist := mp.configurationDistanceFunc(
				&motionplan.SegmentFS{StartConfiguration: oldNear.Q(), EndConfiguration: newNear})

			if nearDist < math.Pow(mp.planOpts.InputIdentDist, 3) {
				if !doubled {
					doubled = true
					// Check if doubling qstep will allow escape from the identical configuration
					// If not, we terminate and return.
					// If so, qstep will be reset to its original value after the rescue.
					for f, frameQ := range qstep {
						for i, q := range frameQ {
							qstep[f][i] = q * 2.0
						}
					}
					continue
				}
				// We've arrived back at very nearly the same configuration again; stop solving and send back oldNear.
				// Do not add the near-identical configuration to the RRT map
				mchan <- oldNear
				return
			}
			if doubled {
				qstep = deepCopyQstep()
				doubled = false
			}
			// constrainNear will ensure path between oldNear and newNear satisfies constraints along the way
			near = &basicNode{q: newNear}
			rrtMap[near] = oldNear
		} else {
			break
		}
	}
	mchan <- oldNear
}

// constrainNear will do a IK gradient descent from seedInputs to target. If a gradient descent distance
// function has been specified, this will use that.
// This function will return either a valid configuration that meets constraints, or nil.
func (mp *cBiRRTMotionPlanner) constrainNear(
	ctx context.Context,
	randseed *rand.Rand,
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
			FS:                 mp.fs,
		}

		// Check if the arc of "seedInputs" to "target" is valid
		ok, _ := mp.checker.CheckSegmentAndStateValidityFS(newArc, mp.planOpts.Resolution)
		if ok {
			return target
		}
		solutionGen := make(chan *ik.Solution, 1)
		linearSeed, err := mp.lfs.mapToSlice(target)
		if err != nil {
			return nil
		}

		// Spawn the IK solver to generate solutions until done
		err = mp.fastGradDescent.Solve(ctx, solutionGen, linearSeed, 0, 0,
			mp.linearizeFSmetric(mp.checker.PathMetric()), randseed.Int())
		// We should have zero or one solutions
		var solved *ik.Solution
		select {
		case solved = <-solutionGen:
		default:
		}
		close(solutionGen)
		if err != nil || solved == nil {
			return nil
		}
		solutionMap, err := mp.lfs.sliceToMap(solved.Configuration)
		if err != nil {
			return nil
		}

		ok, failpos := mp.checker.CheckSegmentAndStateValidityFS(
			&motionplan.SegmentFS{
				StartConfiguration: seedInputs,
				EndConfiguration:   solutionMap,
				FS:                 mp.fs,
			},
			mp.planOpts.Resolution,
		)
		if ok {
			return solutionMap
		}
		if failpos != nil {
			dist := mp.configurationDistanceFunc(&motionplan.SegmentFS{
				StartConfiguration: target,
				EndConfiguration:   failpos.EndConfiguration,
			})
			if dist > mp.planOpts.InputIdentDist {
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

// smoothPath will pick two points at random along the path and attempt to do a fast gradient descent directly between
// them, which will cut off randomly-chosen points with odd joint angles into something that is a more intuitive motion.
func (mp *cBiRRTMotionPlanner) smoothPath(ctx context.Context, inputSteps []node) []node {
	toIter := int(math.Min(float64(len(inputSteps)*len(inputSteps)), float64(mp.planOpts.SmoothIter)))

	schan := make(chan node, 1)
	defer close(schan)

	for numCornersToPass := 2; numCornersToPass > 0; numCornersToPass-- {
		for iter := 0; iter < toIter/2 && len(inputSteps) > 3; iter++ {
			select {
			case <-ctx.Done():
				return inputSteps
			default:
			}
			// get start node of first edge. Cannot be either the last or second-to-last node.
			// Intn will return an int in the half-open interval [0,n)
			i := mp.randseed.Intn(len(inputSteps) - 2)
			j := i + 1
			cornersPassed := 0
			hitCorners := []node{}
			for (cornersPassed != numCornersToPass || !inputSteps[j].Corner()) && j < len(inputSteps)-1 {
				j++
				if cornersPassed < numCornersToPass && inputSteps[j].Corner() {
					cornersPassed++
					hitCorners = append(hitCorners, inputSteps[j])
				}
			}
			// no corners existed between i and end of inputSteps -> not good candidate for smoothing
			if len(hitCorners) == 0 {
				continue
			}

			shortcutGoal := make(map[node]node)

			iSol := inputSteps[i]
			jSol := inputSteps[j]
			shortcutGoal[jSol] = nil

			mp.constrainedExtend(ctx, mp.randseed, shortcutGoal, jSol, iSol, schan)
			reached := <-schan

			// Note this could technically replace paths with "longer" paths i.e. with more waypoints.
			// However, smoothed paths are invariably more intuitive and smooth, and lend themselves to future shortening,
			// so we allow elongation here.
			dist := mp.configurationDistanceFunc(&motionplan.SegmentFS{
				StartConfiguration: inputSteps[i].Q(),
				EndConfiguration:   reached.Q(),
			})
			if dist < mp.planOpts.InputIdentDist {
				for _, hitCorner := range hitCorners {
					hitCorner.SetCorner(false)
				}

				newInputSteps := append([]node{}, inputSteps[:i]...)
				for reached != nil {
					newInputSteps = append(newInputSteps, reached)
					reached = shortcutGoal[reached]
				}
				newInputSteps[i].SetCorner(true)
				newInputSteps[len(newInputSteps)-1].SetCorner(true)
				newInputSteps = append(newInputSteps, inputSteps[j+1:]...)
				inputSteps = newInputSteps
			}
		}
	}
	return inputSteps
}

// getFrameSteps will return a slice of positive values representing the largest amount a particular DOF of a frame should
// move in any given step. The second argument is a float describing the percentage of the total movement.
func getFrameSteps(lfs *linearizedFrameSystem, percentTotalMovement float64) map[string][]float64 {
	frameQstep := map[string][]float64{}
	for _, f := range lfs.frames {
		dof := f.DoF()
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
		}
		frameQstep[f.Name()] = pos
	}
	return frameQstep
}

func (mp *cBiRRTMotionPlanner) checkInputs(inputs referenceframe.FrameSystemInputs) bool {
	return mp.checker.CheckStateFSConstraints(&motionplan.StateFS{
		Configuration: inputs,
		FS:            mp.fs,
	}) == nil
}

func (mp *cBiRRTMotionPlanner) checkPath(seedInputs, target referenceframe.FrameSystemInputs) bool {
	ok, _ := mp.checker.CheckSegmentAndStateValidityFS(
		&motionplan.SegmentFS{
			StartConfiguration: seedInputs,
			EndConfiguration:   target,
			FS:                 mp.fs,
		},
		mp.planOpts.Resolution,
	)
	return ok
}

func (mp *cBiRRTMotionPlanner) sample(rSeed node, sampleNum int) (node, error) {
	// If we have done more than 50 iterations, start seeding off completely random positions 2 at a time
	// The 2 at a time is to ensure random seeds are added onto both the seed and gofsal maps.
	if sampleNum >= mp.planOpts.IterBeforeRand && sampleNum%4 >= 2 {
		randomInputs := make(referenceframe.FrameSystemInputs)
		for _, name := range mp.fs.FrameNames() {
			f := mp.fs.Frame(name)
			if f != nil && len(f.DoF()) > 0 {
				randomInputs[name] = referenceframe.RandomFrameInputs(f, mp.randseed)
			}
		}
		return newConfigurationNode(randomInputs), nil
	}

	// Seeding nearby to valid points results in much faster convergence in less constrained space
	newInputs := make(referenceframe.FrameSystemInputs)
	for name, inputs := range rSeed.Q() {
		f := mp.fs.Frame(name)
		if f != nil && len(f.DoF()) > 0 {
			q, err := referenceframe.RestrictedRandomFrameInputs(f, mp.randseed, 0.1, inputs)
			if err != nil {
				return nil, err
			}
			newInputs[name] = q
		}
	}
	return newConfigurationNode(newInputs), nil
}

type solutionSolvingState struct {
	solutions         map[float64]referenceframe.FrameSystemInputs
	failures          map[string]int // A map keeping track of which constraints fail
	constraintFailCnt int
	startTime         time.Time
	firstSolutionTime time.Duration
}

// return bool is if we should stop because we're done.
func (mp *cBiRRTMotionPlanner) process(sss *solutionSolvingState, seed referenceframe.FrameSystemInputs, stepSolution *ik.Solution) bool {
	step, err := mp.lfs.sliceToMap(stepSolution.Configuration)
	if err != nil {
		mp.logger.Warnf("bad stepSolution.Configuration %v %v", stepSolution.Configuration, err)
		return false
	}

	alteredStep := mp.nonchainMinimize(seed, step)
	if alteredStep != nil {
		// if nil, step is guaranteed to fail the below check, but we want to do it anyway to capture the failure reason
		step = alteredStep
	}
	// Ensure the end state is a valid one
	err = mp.checker.CheckStateFSConstraints(&motionplan.StateFS{
		Configuration: step,
		FS:            mp.fs,
	})
	if err != nil {
		sss.constraintFailCnt++
		sss.failures[err.Error()]++
		return false
	}

	stepArc := &motionplan.SegmentFS{
		StartConfiguration: seed,
		EndConfiguration:   step,
		FS:                 mp.fs,
	}
	err = mp.checker.CheckSegmentFSConstraints(stepArc)
	if err != nil {
		sss.constraintFailCnt++
		sss.failures[err.Error()]++
		return false
	}

	score := mp.configurationDistanceFunc(stepArc)

	if score < mp.planOpts.MinScore && mp.planOpts.MinScore > 0 {
		sss.solutions = map[float64]referenceframe.FrameSystemInputs{}
		sss.solutions[score] = step
		// good solution, stopping early
		return true
	}

	for _, oldSol := range sss.solutions {
		similarity := &motionplan.SegmentFS{
			StartConfiguration: oldSol,
			EndConfiguration:   step,
			FS:                 mp.fs,
		}
		simscore := mp.configurationDistanceFunc(similarity)
		if simscore < defaultSimScore {
			return false
		}
	}

	sss.solutions[score] = step
	if len(sss.solutions) >= mp.planOpts.MaxSolutions {
		// sufficient solutions found, stopping early
		return true
	}

	if len(sss.solutions) == 1 {
		sss.firstSolutionTime = time.Since(sss.startTime)
	} else {
		elapsed := time.Since(sss.startTime)
		if elapsed > (time.Duration(mp.planOpts.TimeMultipleAfterFindingFirstSolution) * sss.firstSolutionTime) {
			mp.logger.Infof("ending early because of time elapsed: %v firstSolutionTime: %v", elapsed, sss.firstSolutionTime)
			return true
		}
	}
	return false
}

// getSolutions will initiate an IK solver for the given position and seed, collect solutions, and score them by constraints.
// If maxSolutions is positive, once that many solutions have been collected, the solver will terminate and return that many solutions.
// If minScore is positive, if a solution scoring below that amount is found, the solver will terminate and return that one solution.
func (mp *cBiRRTMotionPlanner) getSolutions(
	ctx context.Context,
	seed referenceframe.FrameSystemInputs,
	metric motionplan.StateFSMetric,
) ([]node, error) {
	if mp.planOpts.MaxSolutions == 0 {
		mp.planOpts.MaxSolutions = defaultSolutionsToSeed
	}
	if len(seed) == 0 {
		seed = referenceframe.FrameSystemInputs{}
		// If no seed is passed, generate one randomly
		for _, frameName := range mp.fs.FrameNames() {
			seed[frameName] = referenceframe.RandomFrameInputs(mp.fs.Frame(frameName), mp.randseed)
		}
	}

	if metric == nil {
		return nil, errors.New("metric is nil")
	}

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	solutionGen := make(chan *ik.Solution, mp.planOpts.NumThreads*20)

	defer func() {
		// In the case that we have an error, we need to explicitly drain the channel before we return
		for len(solutionGen) > 0 {
			<-solutionGen
		}
	}()

	linearSeed, err := mp.lfs.mapToSlice(seed)
	if err != nil {
		return nil, err
	}

	minFunc := mp.linearizeFSmetric(metric)
	// Spawn the IK solver to generate solutions until done
	approxCartesianDist := math.Sqrt(minFunc(linearSeed))

	var activeSolvers sync.WaitGroup
	defer activeSolvers.Wait()
	activeSolvers.Add(1)
	var solverFinished atomic.Bool
	utils.PanicCapturingGo(func() {
		defer activeSolvers.Done()
		defer solverFinished.Store(true)
		err := mp.solver.Solve(ctxWithCancel, solutionGen, linearSeed, 0, approxCartesianDist, minFunc, mp.randseed.Int())
		if err != nil {
			if ctxWithCancel.Err() == nil {
				mp.logger.Warnf("solver had an error: %v", err)
			}
		}
	})

	solvingState := solutionSolvingState{
		solutions:         map[float64]referenceframe.FrameSystemInputs{},
		failures:          map[string]int{},
		startTime:         time.Now(),
		firstSolutionTime: time.Hour,
	}

	for !solverFinished.Load() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case stepSolution := <-solutionGen:
			if mp.process(&solvingState, seed, stepSolution) {
				cancel()
			}

		default:
			continue
		}
	}

	// Cancel any ongoing processing within the IK solvers if we're done receiving solutions
	cancel()

	activeSolvers.Wait()

	if len(solvingState.solutions) == 0 {
		// We have failed to produce a usable IK solution. Let the user know if zero IK solutions were produced, or if non-zero solutions
		// were produced, which constraints were failed
		if solvingState.constraintFailCnt == 0 {
			return nil, errIKSolve
		}

		return nil, newIKConstraintErr(solvingState.failures, solvingState.constraintFailCnt)
	}

	keys := make([]float64, 0, len(solvingState.solutions))
	for k := range solvingState.solutions {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	orderedSolutions := make([]node, 0)
	for _, key := range keys {
		orderedSolutions = append(orderedSolutions, &basicNode{q: solvingState.solutions[key]})
	}
	return orderedSolutions, nil
}

// linearize the goal metric for use with solvers.
// Since our solvers operate on arrays of floats, there needs to be a way to map bidirectionally between the framesystem configuration
// of FrameSystemInputs and the []float64 that the solver expects. This is that mapping.
func (mp *cBiRRTMotionPlanner) linearizeFSmetric(metric motionplan.StateFSMetric) func([]float64) float64 {
	return func(query []float64) float64 {
		inputs, err := mp.lfs.sliceToMap(query)
		if err != nil {
			return math.Inf(1)
		}
		return metric(&motionplan.StateFS{Configuration: inputs, FS: mp.fs})
	}
}

// The purpose of this function is to allow solves that require the movement of components not in a motion chain, while preventing wild or
// random motion of these components unnecessarily. A classic example would be a scene with two arms. One arm is given a goal in World
// which it could reach, but the other arm is in the way. Randomly seeded IK will produce a valid configuration for the moving arm, and a
// random configuration for the other. This function attempts to replace that random configuration with the seed configuration, if valid,
// and if invalid will interpolate the solved random configuration towards the seed and set its configuration to the closest valid
// configuration to the seed.
func (mp *cBiRRTMotionPlanner) nonchainMinimize(seed, step referenceframe.FrameSystemInputs) referenceframe.FrameSystemInputs {
	moving, nonmoving := mp.motionChains.framesFilteredByMovingAndNonmoving(mp.fs)
	// Create a map with nonmoving configurations replaced with their seed values
	alteredStep := referenceframe.FrameSystemInputs{}
	for _, frame := range moving {
		alteredStep[frame] = step[frame]
	}
	for _, frame := range nonmoving {
		alteredStep[frame] = seed[frame]
	}
	if mp.checkInputs(alteredStep) {
		return alteredStep
	}
	// Failing constraints with nonmoving frames at seed. Find the closest passing configuration to seed.

	_, lastGood := mp.checker.CheckStateConstraintsAcrossSegmentFS(
		&motionplan.SegmentFS{
			StartConfiguration: step,
			EndConfiguration:   alteredStep,
			FS:                 mp.fs,
		}, mp.planOpts.Resolution,
	)
	if lastGood != nil {
		return lastGood.EndConfiguration
	}
	return nil
}
