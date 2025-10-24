package armplanning

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.opencensus.io/trace"
	"go.viam.com/utils"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

// fixedStepInterpolation returns inputs at qstep distance along the path from start to target.
func fixedStepInterpolation(start, target *node, qstep map[string][]float64) referenceframe.FrameSystemInputs {
	newNear := make(referenceframe.FrameSystemInputs)

	for frameName, startInputs := range start.inputs {
		// As this is constructed in-algorithm from already-near nodes, this is guaranteed to always exist
		targetInputs := target.inputs[frameName]
		frameSteps := make([]referenceframe.Input, len(startInputs))

		qframe, ok := qstep[frameName]
		for j, nearInput := range startInputs {
			v1, v2 := nearInput, targetInputs[j]

			step := 0.0
			if ok {
				step = qframe[j]
			}
			if step > math.Abs(v2-v1) {
				frameSteps[j] = v2
			} else if v1 < v2 {
				frameSteps[j] = nearInput + step
			} else {
				frameSteps[j] = nearInput - step
			}
		}
		newNear[frameName] = frameSteps
	}
	return newNear
}

type node struct {
	inputs referenceframe.FrameSystemInputs
	// Dan: What is a corner?
	corner bool
	// cost of moving from seed to this inputs
	cost float64
	// checkPath is true when the path has been checked and was determined to meet constraints
	checkPath bool
}

func newConfigurationNode(q referenceframe.FrameSystemInputs) *node {
	return &node{
		inputs: q,
		corner: false,
	}
}

// nodePair groups together nodes in a tuple
// TODO(rb): in the future we might think about making this into a list of nodes.
type nodePair struct{ a, b *node }

func extractPath(startMap, goalMap rrtMap, pair *nodePair, matched bool) []referenceframe.FrameSystemInputs {
	// need to figure out which of the two nodes is in the start map
	var startReached, goalReached *node
	if _, ok := startMap[pair.a]; ok {
		startReached, goalReached = pair.a, pair.b
	} else {
		startReached, goalReached = pair.b, pair.a
	}

	// extract the path to the seed
	path := []referenceframe.FrameSystemInputs{}
	for startReached != nil {
		path = append(path, startReached.inputs)
		startReached = startMap[startReached]
	}

	// reverse the slice
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	if goalReached != nil {
		if matched {
			// skip goalReached node and go directly to its parent in order to not repeat this node
			goalReached = goalMap[goalReached]
		}

		// extract the path to the goal
		for goalReached != nil {
			path = append(path, goalReached.inputs)
			goalReached = goalMap[goalReached]
		}
	}
	return path
}

type solutionSolvingState struct {
	psc          *planSegmentContext
	maxSolutions int

	seeds       []referenceframe.FrameSystemInputs
	linearSeeds [][]float64

	moving, nonmoving []string

	ratios   []float64
	goodCost float64

	processCalls int
	failures     *IkConstraintError

	solutions         []*node
	startTime         time.Time
	firstSolutionTime time.Duration

	bestScoreWithProblem float64
	bestScoreNoProblem   float64
}

func newSolutionSolvingState(psc *planSegmentContext) (*solutionSolvingState, error) {
	var err error

	sss := &solutionSolvingState{
		psc:                  psc,
		seeds:                []referenceframe.FrameSystemInputs{psc.start},
		solutions:            []*node{},
		failures:             newIkConstraintError(psc.pc.fs, psc.checker),
		firstSolutionTime:    time.Hour,
		bestScoreNoProblem:   10000000,
		bestScoreWithProblem: 10000000,
		maxSolutions:         psc.pc.planOpts.MaxSolutions,
	}

	if sss.maxSolutions <= 0 {
		sss.maxSolutions = defaultSolutionsToSeed
	}

	ls, err := psc.pc.lfs.mapToSlice(psc.start)
	if err != nil {
		return nil, err
	}
	sss.linearSeeds = [][]float64{ls}

	{
		ssc, err := smartSeed(psc.pc.fs, psc.pc.logger)
		if err != nil {
			return nil, fmt.Errorf("cannot create smartSeeder: %w", err)
		}

		altSeeds, err := ssc.findSeeds(psc.goal, psc.start, psc.pc.logger)
		if err != nil {
			psc.pc.logger.Warnf("findSeeds failed, ignoring: %v", err)
		}
		psc.pc.logger.Debugf("got %d altSeeds", len(altSeeds))
		for _, s := range altSeeds {
			ls, err := psc.pc.lfs.mapToSlice(s)
			if err != nil {
				psc.pc.logger.Warnf("mapToSlice failed? %v", err)
				continue
			}
			sss.seeds = append(sss.seeds, s)
			sss.linearSeeds = append(sss.linearSeeds, ls)
		}
	}

	sss.moving, sss.nonmoving = sss.psc.motionChains.framesFilteredByMovingAndNonmoving()

	err = sss.computeGoodCost(psc.goal)
	if err != nil {
		return nil, err
	}

	sss.startTime = time.Now() // do this after we check the cache, etc.

	return sss, nil
}

func (sss *solutionSolvingState) computeGoodCost(goal referenceframe.FrameSystemPoses) error {
	sss.ratios = sss.psc.pc.lfs.inputChangeRatio(sss.psc.motionChains, sss.seeds[0], /* maybe use the best one? */
		sss.psc.pc.planOpts.getGoalMetric(goal), sss.psc.pc.logger)

	adjusted := []float64{}
	for idx, r := range sss.ratios {
		adjusted = append(adjusted, sss.psc.pc.lfs.jog(idx, sss.linearSeeds[0][idx] /* match above when we change */, r))
	}
	step, err := sss.psc.pc.lfs.sliceToMap(adjusted)
	if err != nil {
		return err
	}
	stepArc := &motionplan.SegmentFS{
		StartConfiguration: sss.psc.start,
		EndConfiguration:   step,
		FS:                 sss.psc.pc.fs,
	}
	sss.goodCost = sss.psc.pc.configurationDistanceFunc(stepArc)
	sss.psc.pc.logger.Debugf("goodCost: %v", sss.goodCost)
	return nil
}

// The purpose of this function is to allow solves that require the movement of components not in a motion chain, while preventing wild or
// random motion of these components unnecessarily. A classic example would be a scene with two arms. One arm is given a goal in World
// which it could reach, but the other arm is in the way. Randomly seeded IK will produce a valid configuration for the moving arm, and a
// random configuration for the other. This function attempts to replace that random configuration with the seed configuration, if valid,
// and if invalid will interpolate the solved random configuration towards the seed and set its configuration to the closest valid
// configuration to the seed.
func (sss *solutionSolvingState) nonchainMinimize(ctx context.Context,
	seed, step referenceframe.FrameSystemInputs,
) referenceframe.FrameSystemInputs {
	// Create a map with nonmoving configurations replaced with their seed values
	alteredStep := referenceframe.FrameSystemInputs{}
	for _, frame := range sss.moving {
		alteredStep[frame] = step[frame]
	}
	for _, frame := range sss.nonmoving {
		alteredStep[frame] = seed[frame]
	}
	if sss.psc.checkInputs(ctx, alteredStep) {
		return alteredStep
	}

	// Failing constraints with nonmoving frames at seed. Find the closest passing configuration to seed.

	//nolint:errcheck
	lastGood, _ := sss.psc.checker.CheckStateConstraintsAcrossSegmentFS(
		ctx,
		&motionplan.SegmentFS{
			StartConfiguration: step,
			EndConfiguration:   alteredStep,
			FS:                 sss.psc.pc.fs,
		}, sss.psc.pc.planOpts.Resolution,
	)
	if lastGood != nil {
		return lastGood.EndConfiguration
	}
	return nil
}

// return bool is if we should stop because we're done.
func (sss *solutionSolvingState) process(ctx context.Context, stepSolution *ik.Solution) bool {
	ctx, span := trace.StartSpan(ctx, "process")
	defer span.End()
	sss.processCalls++

	step, err := sss.psc.pc.lfs.sliceToMap(stepSolution.Configuration)
	if err != nil {
		sss.psc.pc.logger.Warnf("bad stepSolution.Configuration %v %v", stepSolution.Configuration, err)
		return false
	}

	alteredStep := sss.nonchainMinimize(ctx, sss.psc.start, step)
	if alteredStep != nil {
		// if nil, step is guaranteed to fail the below check, but we want to do it anyway to capture the failure reason
		step = alteredStep
	}
	// Ensure the end state is a valid one
	err = sss.psc.checker.CheckStateFSConstraints(ctx, &motionplan.StateFS{
		Configuration: step,
		FS:            sss.psc.pc.fs,
	})
	if err != nil {
		sss.failures.add(step, err)
		return false
	}

	stepArc := &motionplan.SegmentFS{
		StartConfiguration: sss.psc.start,
		EndConfiguration:   step,
		FS:                 sss.psc.pc.fs,
	}
	err = sss.psc.checker.CheckSegmentFSConstraints(stepArc)
	if err != nil {
		sss.failures.add(step, err)
		return false
	}

	for _, oldSol := range sss.solutions {
		similarity := &motionplan.SegmentFS{
			StartConfiguration: oldSol.inputs,
			EndConfiguration:   step,
			FS:                 sss.psc.pc.fs,
		}
		simscore := sss.psc.pc.configurationDistanceFunc(similarity)
		if simscore < defaultSimScore {
			return false
		}
	}

	if len(sss.solutions) == 0 {
		sss.firstSolutionTime = time.Since(sss.startTime)
	}

	myNode := &node{inputs: step, cost: sss.psc.pc.configurationDistanceFunc(stepArc)}
	sss.solutions = append(sss.solutions, myNode)

	if myNode.cost < sss.bestScoreWithProblem {
		sss.bestScoreWithProblem = max(1, myNode.cost)
	}

	if myNode.cost <= min(sss.goodCost, sss.bestScoreWithProblem*defaultOptimalityMultiple) {
		whyNot := sss.psc.checkPath(ctx, sss.psc.start, step)
		sss.psc.pc.logger.Debugf("got score %0.4f @ %v - %s - result: %v", myNode.cost, time.Since(sss.startTime), stepSolution.Meta, whyNot)
		myNode.checkPath = whyNot == nil

		if whyNot == nil && myNode.cost < sss.bestScoreNoProblem {
			sss.bestScoreNoProblem = myNode.cost
		}
	}

	return sss.shouldStopEarly()
}

// return bool is if we should stop because we're done.
func (sss *solutionSolvingState) shouldStopEarly() bool {
	elapsed := time.Since(sss.startTime)

	if len(sss.solutions) >= sss.maxSolutions {
		sss.psc.pc.logger.Debugf("stopping with %d solutions after: %v", len(sss.solutions), elapsed)
		return true
	}

	if sss.bestScoreNoProblem < .2 {
		sss.psc.pc.logger.Debugf("stopping early with amazing %0.2f after: %v", sss.bestScoreNoProblem, elapsed)
		return true
	}

	multiple := 100.0
	minMillis := 250

	if sss.bestScoreNoProblem < sss.goodCost/20 {
		multiple = 0
		minMillis = 10
	} else if sss.bestScoreNoProblem < sss.goodCost/15 {
		multiple = 1
		minMillis = 15
	} else if sss.bestScoreNoProblem < sss.goodCost/10 {
		multiple = 0
		minMillis = 20
	} else if sss.bestScoreNoProblem < sss.goodCost/5 {
		multiple = 2
		minMillis = 20
	} else if sss.bestScoreNoProblem < sss.goodCost/2 {
		multiple = 20
		minMillis = 100
	} else if sss.bestScoreNoProblem < sss.goodCost {
		multiple = 50
	} else if sss.bestScoreWithProblem < sss.goodCost {
		// we're going to have to do cbirrt, so look a little less, but still look
		multiple = 75
	}

	if elapsed > max(sss.firstSolutionTime*time.Duration(multiple), time.Duration(minMillis)*time.Millisecond) {
		sss.psc.pc.logger.Debugf("stopping early with bestScore %0.2f (%0.3f)/ %0.2f (%0.3f) after: %v",
			sss.bestScoreNoProblem, sss.bestScoreNoProblem/sss.goodCost,
			sss.bestScoreWithProblem, sss.bestScoreWithProblem/sss.goodCost,
			elapsed)
		return true
	}

	return false
}

// getSolutions will initiate an IK solver for the given position and seed, collect solutions, and
// score them by constraints.
//
// If maxSolutions is positive, once that many solutions have been collected, the solver will
// terminate and return that many solutions.
//
// If minScore is positive, if a solution scoring below that amount is found, the solver will
// terminate and return that one solution.
func getSolutions(ctx context.Context, psc *planSegmentContext) ([]*node, error) {
	if len(psc.start) == 0 {
		return nil, fmt.Errorf("getSolutions start can't be empty")
	}

	solvingState, err := newSolutionSolvingState(psc)
	if err != nil {
		return nil, err
	}

	// Spawn the IK solver to generate solutions until done
	minFunc := psc.pc.linearizeFSmetric(psc.pc.planOpts.getGoalMetric(psc.goal))

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	solutionGen := make(chan *ik.Solution, defaultNumThreads*20)
	defer func() {
		// In lieu of creating a separate WaitGroup to wait on before returning, we simply wait to
		// see the `solutionGen` channel get closed to know that the goroutine we spawned has
		// finished.
		for range solutionGen {
		}
	}()

	solver, err := ik.CreateCombinedIKSolver(psc.pc.lfs.dof, psc.pc.logger, defaultNumThreads, psc.pc.planOpts.GoalThreshold)
	if err != nil {
		return nil, err
	}

	var solveError error
	var solveErrorLock sync.Mutex

	// Spawn the IK solver to generate solutions until done
	utils.PanicCapturingGo(func() {
		// This channel close doubles as signaling that the goroutine has exited.
		defer close(solutionGen)
		_, err := solver.Solve(ctxWithCancel, solutionGen, solvingState.linearSeeds, solvingState.ratios, minFunc, psc.pc.randseed.Int())
		if err != nil {
			solveErrorLock.Lock()
			solveError = err
			solveErrorLock.Unlock()
		}
	})

solutionLoop:
	for {
		select {
		case <-ctx.Done():
			// We've been canceled. So have our workers. Can just return.
			return nil, ctx.Err()
		case stepSolution, ok := <-solutionGen:
			if !ok || solvingState.process(ctx, stepSolution) {
				// No longer using the generated solutions. Cancel the workers.
				cancel()
				break solutionLoop
			}
		}
	}

	solveErrorLock.Lock()
	defer solveErrorLock.Unlock()
	if solveError != nil {
		return nil, fmt.Errorf("solver had an error: %w", solveError)
	}

	if len(solvingState.solutions) == 0 {
		// We have failed to produce a usable IK solution. Let the user know if zero IK solutions
		// were produced, or if non-zero solutions were produced, which constraints were violated.
		if solvingState.failures.Count == 0 {
			return nil, errIKSolve
		}

		return nil, solvingState.failures
	}

	sort.Slice(solvingState.solutions, func(i, j int) bool {
		return solvingState.solutions[i].cost < solvingState.solutions[j].cost
	})

	return solvingState.solutions, nil
}
