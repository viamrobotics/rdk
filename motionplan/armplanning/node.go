package armplanning

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opencensus.io/trace"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

// fixedStepInterpolation returns inputs at qstep distance along the path from start to target.
func fixedStepInterpolation(start, target *node, qstep map[string][]float64) *referenceframe.LinearInputs {
	newNear := referenceframe.NewLinearInputs()

	for frameName, startInputs := range start.inputs.Items() {
		// As this is constructed in-algorithm from already-near nodes, this is guaranteed to always exist
		targetInputs := target.inputs.Get(frameName)
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

		newNear.Put(frameName, frameSteps)
	}
	return newNear
}

type node struct {
	name     int
	goalNode bool

	inputs *referenceframe.LinearInputs
	// Dan: What is a corner?
	corner bool
	// cost of moving from seed to this inputs
	cost float64
	// checkPath is true when the path has been checked and was determined to meet constraints
	checkPath bool

	liveSolution bool
}

var nodeNameCounter atomic.Int64

func newConfigurationNode(q *referenceframe.LinearInputs) *node {
	return &node{
		name:   int(nodeNameCounter.Add(1)),
		inputs: q,
		corner: false,
	}
}

// nodePair groups together nodes in a tuple
// TODO(rb): in the future we might think about making this into a list of nodes.
type nodePair struct{ a, b *node }

func extractPath(startMap, goalMap rrtMap, pair *nodePair, matched bool) []*referenceframe.LinearInputs {
	// need to figure out which of the two nodes is in the start map
	var startReached, goalReached *node
	if _, ok := startMap[pair.a]; ok {
		startReached, goalReached = pair.a, pair.b
	} else {
		startReached, goalReached = pair.b, pair.a
	}

	// extract the path to the seed
	path := []*referenceframe.LinearInputs{}
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

	linearSeeds [][]float64
	seedLimits  [][]referenceframe.Limit

	moving, nonmoving []string

	goodCost float64

	processCalls int
	failures     *IkConstraintError

	solutions         []*node
	startTime         time.Time
	firstSolutionTime time.Duration

	bestScoreWithProblem float64
	bestScoreNoProblem   float64

	fatal error
}

func newSolutionSolvingState(ctx context.Context, psc *planSegmentContext) (*solutionSolvingState, error) {
	ctx, span := trace.StartSpan(ctx, "newSolutionSolvingState")
	defer span.End()

	var err error

	sss := &solutionSolvingState{
		psc:                  psc,
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

	sss.linearSeeds = [][]float64{psc.start.GetLinearizedInputs()}
	sss.seedLimits = [][]referenceframe.Limit{psc.pc.lis.GetLimits()}

	ratios, minRatio, err := sss.computeGoodCost(psc.goal)
	if err != nil {
		return nil, err
	}

	sss.linearSeeds = append(sss.linearSeeds, sss.linearSeeds[0])
	sss.seedLimits = append(sss.seedLimits, ik.ComputeAdjustLimitsArray(sss.linearSeeds[0], sss.seedLimits[0], ratios))

	sss.linearSeeds = append(sss.linearSeeds, sss.linearSeeds[0])
	sss.seedLimits = append(sss.seedLimits, ik.ComputeAdjustLimits(sss.linearSeeds[0], sss.seedLimits[0], .05))

	if sss.goodCost > 1 && minRatio > .05 {
		ssc, err := smartSeed(psc.pc.fs, psc.pc.logger)
		if err != nil {
			return nil, fmt.Errorf("cannot create smartSeeder: %w", err)
		}

		altSeeds, altLimitDivisors, err := ssc.findSeeds(ctx, psc.goal, psc.start, 5 /* TODO */, psc.pc.logger)
		if err != nil {
			psc.pc.logger.Warnf("findSeeds failed, ignoring: %v", err)
		}
		psc.pc.logger.Debugf("got %d altSeeds", len(altSeeds))

		for _, s := range altSeeds {
			si := s.GetLinearizedInputs()
			sss.linearSeeds = append(sss.linearSeeds, si)
			ll := ik.ComputeAdjustLimitsArray(si, sss.seedLimits[0], altLimitDivisors)
			sss.seedLimits = append(sss.seedLimits, ll)
			psc.pc.logger.Debugf("\t ss (%d): %v", len(sss.linearSeeds)-1, logging.FloatArrayFormat{"", si})
		}
	}

	sss.moving, sss.nonmoving = sss.psc.motionChains.framesFilteredByMovingAndNonmoving()

	sss.startTime = time.Now() // do this after we check the cache, etc.

	return sss, nil
}

func (sss *solutionSolvingState) computeGoodCost(goal referenceframe.FrameSystemPoses) ([]float64, float64, error) {
	ratios, err := inputChangeRatio(sss.psc.motionChains, sss.psc.start, sss.psc.pc.fs,
		sss.psc.pc.planOpts.getGoalMetric(goal), sss.psc.pc.logger)
	if err != nil {
		return nil, 1, err
	}

	minRatio := 1.0

	adjusted := []float64{}
	for idx, r := range ratios {
		adjusted = append(adjusted, sss.psc.pc.lis.Jog(idx, sss.linearSeeds[0][idx], r))
		minRatio = min(minRatio, r)
	}

	step, err := sss.psc.pc.lis.FloatsToInputs(adjusted)
	if err != nil {
		return nil, minRatio, err
	}

	stepArc := &motionplan.SegmentFS{
		StartConfiguration: sss.psc.start,
		EndConfiguration:   step,
		FS:                 sss.psc.pc.fs,
	}

	sss.goodCost = sss.psc.pc.configurationDistanceFunc(stepArc)
	sss.psc.pc.logger.Debugf("goodCost: %v", sss.goodCost)
	return ratios, minRatio, nil
}

// processCorrectness returns a non-nil SegmentFS if the step satisfies all constraints.
func (sss *solutionSolvingState) processCorrectness(ctx context.Context, step *referenceframe.LinearInputs,
) *motionplan.SegmentFS {
	ctx, span := trace.StartSpan(ctx, "processCorrectness")
	defer span.End()

	// Ensure the end state is a valid one
	err := sss.psc.checker.CheckStateFSConstraints(ctx, &motionplan.StateFS{
		Configuration: step,
		FS:            sss.psc.pc.fs,
	})
	if err != nil {
		// sss.psc.pc.logger.Debugf("bad solution a: %v %v", stepSolution, err)
		if len(sss.solutions) == 0 && sss.psc.pc.isFatalCollision(err) {
			sss.fatal = fmt.Errorf("fatal early collision: %w", err)
		}
		sss.failures.add(step, err)
		return nil
	}

	stepArc := &motionplan.SegmentFS{
		StartConfiguration: sss.psc.start,
		EndConfiguration:   step,
		FS:                 sss.psc.pc.fs,
	}
	err = sss.psc.checker.CheckSegmentFSConstraints(ctx, stepArc)
	if err != nil {
		// sss.psc.pc.logger.Debugf("bad solution b: %v %v", stepSolution, err)
		sss.failures.add(step, err)
		return nil
	}

	return stepArc
}

// processSimilarity returns a non-nil *node object if the solution is unique amongst the existing solutions
func (sss *solutionSolvingState) processSimilarity(
	_ context.Context,
	step *referenceframe.LinearInputs,
	stepArc *motionplan.SegmentFS,
) *node {
	myCost := sss.psc.pc.configurationDistanceFunc(stepArc)
	if myCost > sss.bestScoreNoProblem {
		sss.psc.pc.logger.Debugf("got score %0.4f worse than bestScoreNoProblem", myCost)
		return nil
	}

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

func (sss *solutionSolvingState) toInputs(_ context.Context, stepSolution *ik.Solution) *referenceframe.LinearInputs {
	step, err := sss.psc.pc.lis.FloatsToInputs(stepSolution.Configuration)
	if err != nil {
		sss.psc.pc.logger.Warnf("bad stepSolution.Configuration %v %v", stepSolution.Configuration, err)
		return nil
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
	now := time.Since(sss.startTime)
	if len(sss.solutions) == 0 {
		sss.firstSolutionTime = now
	}

	sss.solutions = append(sss.solutions, myNode)
	if myNode.cost < sss.bestScoreWithProblem {
		sss.bestScoreWithProblem = max(1, myNode.cost)
	}

	whyNot := sss.psc.checkPath(ctx, sss.psc.start, step)
	sss.psc.pc.logger.Debugf("got score %0.4f @ %v - %s - result: %v", myNode.cost, now, stepSolution.Meta, whyNot)
	myNode.checkPath = whyNot == nil

	if whyNot == nil && myNode.cost < sss.bestScoreNoProblem {
		sss.bestScoreNoProblem = myNode.cost
	}

	return sss.shouldStopEarly()
}

// return bool is if we should stop because we're done.
func (sss *solutionSolvingState) shouldStopEarly() bool {
	elapsed := time.Since(sss.startTime)
	if sss.fatal != nil {
		sss.psc.pc.logger.Debugf("stopping with fatal %v", sss.fatal)
		return true
	}

	if len(sss.solutions) >= sss.maxSolutions {
		sss.psc.pc.logger.Debugf("stopping with %d solutions after: %v", len(sss.solutions), elapsed)
		return true
	}

	if sss.bestScoreNoProblem < .2 {
		sss.psc.pc.logger.Debugf("stopping early with amazing %0.2f after: %v", sss.bestScoreNoProblem, elapsed)
		return true
	}

	multiple := 100.0
	minMillis := 10000

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
	} else if sss.bestScoreNoProblem < sss.goodCost/3.5 {
		multiple = 4
		minMillis = 30
	} else if sss.bestScoreNoProblem < sss.goodCost/2 {
		multiple = 20
		minMillis = 100
	} else if sss.bestScoreNoProblem < sss.goodCost {
		multiple = 50
		minMillis = 250
	} else if sss.bestScoreWithProblem < sss.goodCost {
		// we're going to have to do cbirrt, so look a little less, but still look
		multiple = 100
	}

	timeToSearch := max(sss.firstSolutionTime*time.Duration(multiple), time.Duration(minMillis)*time.Millisecond)

	if sss.psc.pc.planOpts.Timeout > 0 && len(sss.solutions) > 0 {
		timeToSearch = min(timeToSearch, sss.psc.pc.planOpts.timeoutDuration()/2)
	}

	if elapsed > timeToSearch {
		sss.psc.pc.logger.Debugf("stopping early bestScore %0.2f (%0.3f)/ %0.2f (%0.3f) after: %v \n\t timeToSearch: %v firstSolutionTime: %v",
			sss.bestScoreNoProblem, sss.bestScoreNoProblem/sss.goodCost,
			sss.bestScoreWithProblem, sss.bestScoreWithProblem/sss.goodCost,
			elapsed, timeToSearch, sss.firstSolutionTime)
		return true
	}

	if len(sss.solutions) == 0 && elapsed > (1000*time.Millisecond) {
		// if we found any solution, we want to look for better for a while
		// but if we've found 0, then probably never going to
		sss.psc.pc.logger.Debugf("stopping early after: %v because nothing has been found, probably won't", elapsed)
		return true
	}

	return false
}

type backgroundGenerator struct {
	newSolutionsCh chan *node
	cancel         func()
	wg             sync.WaitGroup
}

func (bgGen *backgroundGenerator) Stop() {
	if bgGen != nil {
		bgGen.cancel()
	}
}

func (bgGen *backgroundGenerator) Wait() {
	if bgGen != nil {
		bgGen.wg.Wait()
	}
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
	if psc.start.Len() == 0 {
		return nil, nil, fmt.Errorf("getSolutions start can't be empty")
	}

	solvingState, err := newSolutionSolvingState(ctx, psc)
	if err != nil {
		return nil, nil, err
	}

	// Spawn the IK solver to generate solutions until done
	minFunc := psc.pc.linearizeFSmetric(psc.pc.planOpts.getGoalMetric(psc.goal))

	solver, err := ik.CreateCombinedIKSolver(psc.pc.logger, defaultNumThreads, psc.pc.planOpts.GoalThreshold)
	if err != nil {
		return nil, nil, err
	}

	var solveError error
	var solveMeta []ik.SeedSolveMetaData
	var solveErrorLock sync.Mutex

	ctxWithCancel, cancel := context.WithCancel(ctx)
	goalNodeGenerator := &backgroundGenerator{
		newSolutionsCh: make(chan *node, 2),
		cancel:         cancel,
	}

	solutionGen := make(chan *ik.Solution, defaultNumThreads*20)
	// Spawn the IK solver to generate solutions until done
	utils.PanicCapturingGo(func() {
		// This channel close doubles as signaling that the goroutine has exited.
		defer close(solutionGen)
		nSol, m, err := solver.Solve(ctxWithCancel,
			solutionGen, solvingState.linearSeeds, solvingState.seedLimits, minFunc, psc.pc.randseed.Int())
		solvingState.psc.pc.logger.Debugf("Solver stopping. Solutions: %v Err? %v", nSol, err)

		solveErrorLock.Lock()
		solveError = err
		solveMeta = m
		solveErrorLock.Unlock()
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
			solvingState.process(ctx, stepSolution)
			if solvingState.shouldStopEarly() {
				cancel()
				// we don't exit the loop to get the last solutions so we don't waste them
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
		if solvingState.fatal != nil {
			return nil, nil, solvingState.fatal
		}

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

	// The above goroutine will continue to append to `solvingState.solutions` for similarity
	// checking. We make a copy to return for the caller to own.
	ret := make([]*node, len(solvingState.solutions))
	copy(ret, solvingState.solutions)

	goalNodeGenerator.wg.Add(1)
	utils.PanicCapturingGo(func() {
		ctx, span := trace.StartSpan(ctx, "backgroundIK")
		defer func() {
			close(goalNodeGenerator.newSolutionsCh)
			waitForWorkers()

			// The first `inputs` argument is known prior to starting this goroutine, but the
			// `solveMeta` argument is only guaranteed to be filled in after all of the solvers have
			// exited. This information was complete back when IK was finished before starting
			// cbirrt. But now that we can continue to generate IK solutions, it would be cumbersome
			// to pass a `solvingState` back up to the caller.
			solvingState.debugSeedInfoForWinner(solvingState.solutions[0].inputs, solveMeta)
			span.End()
			goalNodeGenerator.wg.Done()
		}()

		for ctxWithCancel.Err() == nil {
			select {
			case <-ctxWithCancel.Done():
				return
			case solution, more := <-solutionGen:
				if !more {
					return
				}

				if ctxWithCancel.Err() != nil {
					// If we've been canceled, avoid busy work.
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
					return
				}
			}
		}
	})

	return ret, goalNodeGenerator, nil
}

func (sss *solutionSolvingState) debugSeedInfoForWinner(winner *referenceframe.LinearInputs, solveMeta []ik.SeedSolveMetaData) {
	if sss.psc.pc.logger.GetLevel() != logging.DEBUG {
		return
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "\n")

	inValid := make([]bool, len(solveMeta))

	for _, frameName := range sss.psc.pc.fs.FrameNames() {
		f := sss.psc.pc.fs.Frame(frameName)
		dof := f.DoF()
		if len(dof) == 0 {
			continue
		}
		fmt.Fprintf(&builder, "frame: %s\n", frameName)

		inputs := winner.Get(frameName)

		for jointNumber, l := range dof {
			min, max, r := l.GoodLimits()
			winningValue := inputs[jointNumber]
			fmt.Fprintf(&builder, "\t joint %d min: %0.2f, max: %0.2f range: %0.2f\n", jointNumber, min, max, r)
			fmt.Fprintf(&builder, "\t\t winner: %0.2f\n", winningValue)

			for seedNumber, s := range sss.linearSeeds {
				step, err := sss.psc.pc.lis.FloatsToInputs(s)
				if err != nil {
					sss.psc.pc.logger.Debugw("Error generating debug output", "err", err)
					return
				}
				v := step.Get(frameName)[jointNumber]
				myLimit := sss.seedLimits[seedNumber][jointNumber]
				fmt.Fprintf(&builder, "\t\t  seed %d %0.2f delta: %0.2f valid: %v limits: %v\n",
					seedNumber, v, math.Abs(v-winningValue)/r, myLimit.IsValid(winningValue), myLimit)
				if !myLimit.IsValid(winningValue) {
					inValid[seedNumber] = true
				}
			}
		}
	}

	for idx, m := range solveMeta {
		fmt.Fprintf(&builder, "seed: %d %#v\n", idx, m)
		fmt.Fprintf(&builder, "\t %v\n", logging.FloatArrayFormat{"", sss.linearSeeds[idx]})
		fmt.Fprintf(&builder, "\t valid: %v\n", !inValid[idx])
	}

	sss.psc.pc.logger.Debugf(builder.String())
}
