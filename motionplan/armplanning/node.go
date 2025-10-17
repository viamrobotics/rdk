package armplanning

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"go.opencensus.io/trace"
	"go.viam.com/utils"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

const ikTimeMultipleStart = 70

// fixedStepInterpolation returns inputs at qstep distance along the path from start to target.
func fixedStepInterpolation(start, target *node, qstep map[string][]float64) referenceframe.FrameSystemInputs {
	newNear := make(referenceframe.FrameSystemInputs)

	for frameName, startInputs := range start.inputs {
		// As this is constructed in-algorithm from already-near nodes, this is guaranteed to always exist
		targetInputs := target.inputs[frameName]
		frameSteps := make([]referenceframe.Input, len(startInputs))

		qframe, ok := qstep[frameName]
		for j, nearInput := range startInputs {
			v1, v2 := nearInput.Value, targetInputs[j].Value

			step := 0.0
			if ok {
				step = qframe[j]
			}
			if step > math.Abs(v2-v1) {
				frameSteps[j] = referenceframe.Input{Value: v2}
			} else if v1 < v2 {
				frameSteps[j] = referenceframe.Input{Value: nearInput.Value + step}
			} else {
				frameSteps[j] = referenceframe.Input{Value: nearInput.Value - step}
			}
		}
		newNear[frameName] = frameSteps
	}
	return newNear
}

type node struct {
	name     int
	goalNode bool

	inputs referenceframe.FrameSystemInputs
	// Dan: What is a corner?
	corner bool
	// cost of moving from seed to this inputs
	cost float64
	// checkPath is true when the path has been checked and was determined to meet constraints
	checkPath bool

	liveSolution bool
}

var nodeNameCounter atomic.Int64

func newConfigurationNode(q referenceframe.FrameSystemInputs) *node {
	return &node{
		name:   int(nodeNameCounter.Add(1)),
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
			if goalReached.goalNode {
				fmt.Println("Solution node:", goalReached.name, "Live?", goalReached.liveSolution)
			}
			goalReached = goalMap[goalReached]
		}
	}

	return path
}

type solutionSolvingState struct {
	psc          *planSegmentContext
	maxSolutions int

	linearSeed        []float64
	moving, nonmoving []string

	ratios   []float64
	goodCost float64

	processCalls int
	failures     *IkConstraintError

	solutions         []*node
	startTime         time.Time
	bestScore         float64
	ikTimeMultiple    int
	firstSolutionTime time.Duration
}

func newSolutionSolvingState(psc *planSegmentContext) (*solutionSolvingState, error) {
	var err error

	sss := &solutionSolvingState{
		psc:               psc,
		solutions:         []*node{},
		failures:          newIkConstraintError(psc.pc.fs, psc.checker),
		startTime:         time.Now(),
		firstSolutionTime: time.Hour,
		bestScore:         10000000,
		maxSolutions:      psc.pc.planOpts.MaxSolutions,
		ikTimeMultiple:    ikTimeMultipleStart, // look for a while, unless we find good things
	}

	if sss.maxSolutions <= 0 {
		sss.maxSolutions = defaultSolutionsToSeed
	}

	sss.linearSeed, err = psc.pc.lfs.mapToSlice(psc.start)
	if err != nil {
		return nil, err
	}

	sss.moving, sss.nonmoving = sss.psc.motionChains.framesFilteredByMovingAndNonmoving()

	err = sss.computeGoodCost(psc.goal)
	if err != nil {
		return nil, err
	}

	return sss, nil
}

func (sss *solutionSolvingState) computeGoodCost(goal referenceframe.FrameSystemPoses) error {
	sss.ratios = sss.psc.pc.lfs.inputChangeRatio(sss.psc.motionChains, sss.psc.start,
		sss.psc.pc.planOpts.getGoalMetric(goal), sss.psc.pc.logger)

	adjusted := []float64{}
	for idx, r := range sss.ratios {
		adjusted = append(adjusted, sss.psc.pc.lfs.jog(idx, sss.linearSeed[idx], r))
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

// processCorrectness returns a non-nil SegmentFS if the step satisfies all constraints.
func (sss *solutionSolvingState) processCorrectness(ctx context.Context, step referenceframe.FrameSystemInputs) *motionplan.SegmentFS {
	ctx, span := trace.StartSpan(ctx, "processCorrectness")
	defer span.End()

	// Ensure the end state is a valid one
	err := sss.psc.checker.CheckStateFSConstraints(ctx, &motionplan.StateFS{
		Configuration: step,
		FS:            sss.psc.pc.fs,
	})
	if err != nil {
		sss.failures.add(step, err)
		return nil
	}

	stepArc := &motionplan.SegmentFS{
		StartConfiguration: sss.psc.start,
		EndConfiguration:   step,
		FS:                 sss.psc.pc.fs,
	}
	err = sss.psc.checker.CheckSegmentFSConstraints(stepArc)
	if err != nil {
		sss.failures.add(step, err)
		return nil
	}

	return stepArc
}

// processSimilarity returns a non-nil *node object if the solution is unique amongst the existing solutions
func (sss *solutionSolvingState) processSimilarity(
	ctx context.Context,
	step referenceframe.FrameSystemInputs,
	stepArc *motionplan.SegmentFS,
) *node {
	for _, oldSol := range sss.solutions {
		similarity := &motionplan.SegmentFS{
			StartConfiguration: oldSol.inputs,
			EndConfiguration:   step,
			FS:                 sss.psc.pc.fs,
		}
		simscore := sss.psc.pc.configurationDistanceFunc(similarity)
		if simscore < defaultSimScore {
			return nil
		}
	}

	return &node{name: int(nodeNameCounter.Add(1)), inputs: step, cost: sss.psc.pc.configurationDistanceFunc(stepArc)}
}

func (sss *solutionSolvingState) toInputs(ctx context.Context, stepSolution *ik.Solution) referenceframe.FrameSystemInputs {
	step, err := sss.psc.pc.lfs.sliceToMap(stepSolution.Configuration)
	if err != nil {
		sss.psc.pc.logger.Warnf("bad stepSolution.Configuration %v %v", stepSolution.Configuration, err)
		return nil
	}

	alteredStep := sss.nonchainMinimize(ctx, sss.psc.start, step)
	if alteredStep != nil {
		// if nil, step is guaranteed to fail later checks, but we want to do it anyway to capture the failure reason
		return alteredStep
	}

	return step
}

// return bool is if we should stop because we're done.
func (sss *solutionSolvingState) process(ctx context.Context, stepSolution *ik.Solution,
) bool {
	ctx, span := trace.StartSpan(ctx, "process")
	defer span.End()
	sss.processCalls++

	step := sss.toInputs(ctx, stepSolution)
	if step == nil {
		return false
	}

	stepArc := sss.processCorrectness(ctx, step)
	if stepArc == nil {
		return false
	}

	myNode := sss.processSimilarity(ctx, step, stepArc)
	if myNode == nil {
		return false
	}

	myNode.goalNode = true
	sss.solutions = append(sss.solutions, myNode)

	// TODO: Reevaluate this constant when better quality IK solutions are being generated.
	const goodCostStopDivider = 4.0

	if myNode.cost < sss.goodCost || // this checks the absolute score of the plan
		// if we've got something sane, and it's really good, let's check
		myNode.cost < (sss.bestScore*defaultOptimalityMultiple) {
		whyNot := sss.psc.checkPath(ctx, sss.psc.start, step)
		sss.psc.pc.logger.Debugf("got score %0.4f and goodCost: %0.2f - result: %v", myNode.cost, sss.goodCost, whyNot)
		if whyNot == nil {
			myNode.checkPath = true
			if (myNode.cost < (sss.goodCost / goodCostStopDivider)) ||
				(myNode.cost < sss.psc.pc.planOpts.MinScore && sss.psc.pc.planOpts.MinScore > 0) {
				sss.psc.pc.logger.Debugf("\tscore %0.4f stopping early (%0.2f) processCalls: %d after %v",
					myNode.cost, sss.goodCost/goodCostStopDivider, sss.processCalls, time.Since(sss.startTime))
				return true // good solution, stopping early
			}

			if myNode.cost < (sss.goodCost / (.5 * goodCostStopDivider)) {
				// we find something very good, but not great
				// so we look at lot
				sss.ikTimeMultiple = min(sss.ikTimeMultiple, ikTimeMultipleStart/12)
			} else if myNode.cost < sss.goodCost {
				sss.ikTimeMultiple = min(sss.ikTimeMultiple, ikTimeMultipleStart/5)
			}
		}
	}

	if len(sss.solutions) >= sss.maxSolutions {
		return true
	}

	if myNode.cost < sss.bestScore {
		sss.bestScore = myNode.cost
	}

	if len(sss.solutions) == 1 {
		sss.firstSolutionTime = time.Since(sss.startTime)
	} else {
		elapsed := time.Since(sss.startTime)
		if elapsed > (time.Duration(sss.ikTimeMultiple) * sss.firstSolutionTime) {
			sss.psc.pc.logger.Infof("ending early because of time elapsed: %v firstSolutionTime: %v processCalls: %d",
				elapsed, sss.firstSolutionTime, sss.processCalls)
			return true
		}
	}
	return false
}

type backgroundGenerator struct {
	newSolutionsCh chan *node
	cancel         func()
	wg             sync.WaitGroup
}

func (bgGen *backgroundGenerator) StopAndWait() {
	if bgGen != nil {
		bgGen.cancel()
		bgGen.wg.Wait()
	}
}

// getSolutions will initiate an IK solver for the given position and seed, collect solutions, and
// score them by constraints.
//
// If maxSolutions is positive, once that many solutions have been collected, the solver will
// terminate and return that many solutions.
//
// If minScore is positive, if a solution scoring below that amount is found, the solver will
// terminate and return that one solution.
func getSolutions(ctx context.Context, psc *planSegmentContext) ([]*node, *backgroundGenerator, error) {
	if len(psc.start) == 0 {
		return nil, nil, fmt.Errorf("getSolutions start can't be empty")
	}

	solvingState, err := newSolutionSolvingState(psc)
	if err != nil {
		return nil, nil, err
	}

	// Spawn the IK solver to generate solutions until done
	minFunc := psc.pc.linearizeFSmetric(psc.pc.planOpts.getGoalMetric(psc.goal))

	psc.pc.logger.Debugf("seed: %v", psc.start)

	solver, err := ik.CreateCombinedIKSolver(psc.pc.lfs.dof, psc.pc.logger, psc.pc.planOpts.NumThreads, psc.pc.planOpts.GoalThreshold)
	if err != nil {
		return nil, nil, err
	}

	var solveError error
	var solveErrorLock sync.Mutex

	ctxWithCancel, cancel := context.WithCancel(ctx)
	goalNodeGenerator := &backgroundGenerator{
		newSolutionsCh: make(chan *node, 2),
		cancel:         cancel,
	}

	solutionGen := make(chan *ik.Solution, psc.pc.planOpts.NumThreads*20)
	// Spawn the IK solver to generate solutions until done
	utils.PanicCapturingGo(func() {
		// This channel close doubles as signaling that the goroutine has exited.
		defer close(solutionGen)
		_, err := solver.Solve(ctxWithCancel, solutionGen, solvingState.linearSeed, solvingState.ratios, minFunc, psc.pc.randseed.Int())
		if err != nil {
			solveErrorLock.Lock()
			solveError = err
			solveErrorLock.Unlock()
		}
	})

	// When `getSolutions` exits, we may or may not continue to generate IK solutions. In cases
	// where we are done generating solutions, `waitForWorkers` will be called before returning.
	//
	// Otherwise the background goroutine that hands off new solutions is responsible for cleaning
	// up.
	waitForWorkers := func() {
		// In lieu of creating a separate WaitGroup to wait on before returning, we simply wait to
		// see the `solutionGen` channel get closed to know that the goroutine we spawned has
		// finished.
		for range solutionGen {
		}
	}

solutionLoop:
	for {
		select {
		case <-ctx.Done():
			// We've been canceled. So have our workers. Can just return.
			waitForWorkers()
			return nil, nil, ctx.Err()
		case stepSolution, ok := <-solutionGen:
			if !ok || solvingState.process(ctx, stepSolution) {
				// We're done grabbing up-front solutions. But we'll continue to keep generating
				// solutions in the background.
				break solutionLoop
			}
		}
	}

	solveErrorLock.Lock()
	defer solveErrorLock.Unlock()
	if solveError != nil {
		waitForWorkers()
		return nil, nil, fmt.Errorf("solver had an error: %w", solveError)
	}

	if len(solvingState.solutions) == 0 {
		waitForWorkers()
		// We have failed to produce a usable IK solution. Let the user know if zero IK solutions
		// were produced, or if non-zero solutions were produced, which constraints were violated.
		if solvingState.failures.Count == 0 {
			return nil, nil, errIKSolve
		}

		return nil, nil, solvingState.failures
	}

	sort.Slice(solvingState.solutions, func(i, j int) bool {
		return solvingState.solutions[i].cost < solvingState.solutions[j].cost
	})

	goalNodeGenerator.wg.Add(1)
	utils.PanicCapturingGo(func() {
		defer goalNodeGenerator.wg.Done()
		for {
			solution, more := <-solutionGen
			if !more {
				return
			}

			step := solvingState.toInputs(ctx, solution)
			if step == nil {
				continue
			}

			stepArc := solvingState.processCorrectness(ctx, step)
			if stepArc == nil {
				continue
			}

			myNode := solvingState.processSimilarity(ctx, step, stepArc)
			if myNode == nil {
				continue
			}

			myNode.liveSolution = true
			myNode.goalNode = true
			select {
			case goalNodeGenerator.newSolutionsCh <- myNode:
				solvingState.solutions = append(solvingState.solutions, myNode)
			case <-ctxWithCancel.Done():
				waitForWorkers()
				return
			}
		}
	})

	// We assume the caller will only ever read the `solutions` elements between index [0,
	// len(solutions)). And it will never append to the `solutions` slice. Hence, we do not need to
	// make a copy. It's safe for the background goal node generator to read/append to the slice for
	// similarity checking.
	return solvingState.solutions, goalNodeGenerator, nil
}
