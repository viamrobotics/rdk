package armplanning

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

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
	psc *planSegmentContext

	linearSeed []float64

	moving, nonmoving []string

	ratios   []float64
	goodCost float64

	solutions         []*node
	failures          *IkConstraintError
	startTime         time.Time
	firstSolutionTime time.Duration
	bestScore         float64
	ikTimeMultiple    int
}

func (sss *solutionSolvingState) setup(goal referenceframe.FrameSystemPoses) error {
	err := sss.computeGoodCost(goal)
	if err != nil {
		return err
	}

	sss.moving, sss.nonmoving = sss.psc.motionChains.framesFilteredByMovingAndNonmoving()

	return nil
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
func (sss *solutionSolvingState) nonchainMinimize(seed, step referenceframe.FrameSystemInputs) referenceframe.FrameSystemInputs {
	// Create a map with nonmoving configurations replaced with their seed values
	alteredStep := referenceframe.FrameSystemInputs{}
	for _, frame := range sss.moving {
		alteredStep[frame] = step[frame]
	}
	for _, frame := range sss.nonmoving {
		alteredStep[frame] = seed[frame]
	}
	if sss.psc.checkInputs(alteredStep) {
		return alteredStep
	}

	// Failing constraints with nonmoving frames at seed. Find the closest passing configuration to seed.

	//nolint:errcheck
	lastGood, _ := sss.psc.checker.CheckStateConstraintsAcrossSegmentFS(
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
func (sss *solutionSolvingState) process(stepSolution *ik.Solution,
) bool {
	step, err := sss.psc.pc.lfs.sliceToMap(stepSolution.Configuration)
	if err != nil {
		sss.psc.pc.logger.Warnf("bad stepSolution.Configuration %v %v", stepSolution.Configuration, err)
		return false
	}

	alteredStep := sss.nonchainMinimize(sss.psc.start, step)
	if alteredStep != nil {
		// if nil, step is guaranteed to fail the below check, but we want to do it anyway to capture the failure reason
		step = alteredStep
	}
	// Ensure the end state is a valid one
	err = sss.psc.checker.CheckStateFSConstraints(&motionplan.StateFS{
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

	myNode := &node{inputs: step, cost: sss.psc.pc.configurationDistanceFunc(stepArc)}
	sss.solutions = append(sss.solutions, myNode)

	const goodCostStopDivier = 3.0

	if myNode.cost < sss.goodCost || // this checks the absolute score of the plan
		// if we've got something sane, and it's really good, let's check
		(myNode.cost < (sss.bestScore*defaultOptimalityMultiple) && myNode.cost < sss.goodCost) {
		whyNot := sss.psc.checkPath(sss.psc.start, step)
		sss.psc.pc.logger.Debugf("got score %0.4f and goodCost: %0.2f - result: %v", myNode.cost, sss.goodCost, whyNot)
		if whyNot == nil {
			myNode.checkPath = true
			if (myNode.cost < (sss.goodCost / goodCostStopDivier)) ||
				(myNode.cost < sss.psc.pc.planOpts.MinScore && sss.psc.pc.planOpts.MinScore > 0) {
				sss.psc.pc.logger.Debugf("\tscore %0.4f stopping early (%0.2f)", myNode.cost, sss.goodCost/goodCostStopDivier)
				return true // good solution, stopping early
			} else if myNode.cost < (sss.goodCost / (.5 * goodCostStopDivier)) {
				sss.ikTimeMultiple = sss.psc.pc.planOpts.TimeMultipleAfterFindingFirstSolution / 4
			}
		}
	}

	if len(sss.solutions) >= sss.psc.pc.planOpts.MaxSolutions {
		// sufficient solutions found, stopping early
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
			sss.psc.pc.logger.Infof("ending early because of time elapsed: %v firstSolutionTime: %v", elapsed, sss.firstSolutionTime)
			return true
		}
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
	var err error

	if psc.pc.planOpts.MaxSolutions == 0 {
		psc.pc.planOpts.MaxSolutions = defaultSolutionsToSeed
	}

	if len(psc.start) == 0 {
		return nil, fmt.Errorf("getSolutions start can't be empty")
	}

	solvingState := solutionSolvingState{
		psc:               psc,
		solutions:         []*node{},
		failures:          newIkConstraintError(psc.pc.fs, psc.checker),
		startTime:         time.Now(),
		firstSolutionTime: time.Hour,
		bestScore:         10000000,
		ikTimeMultiple:    psc.pc.planOpts.TimeMultipleAfterFindingFirstSolution,
	}

	solvingState.linearSeed, err = psc.pc.lfs.mapToSlice(psc.start)
	if err != nil {
		return nil, err
	}

	err = solvingState.setup(psc.goal)
	if err != nil {
		return nil, err
	}

	// Spawn the IK solver to generate solutions until done
	minFunc := psc.pc.linearizeFSmetric(psc.pc.planOpts.getGoalMetric(psc.goal))

	psc.pc.logger.Debugf("seed: %v", psc.start)

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	solutionGen := make(chan *ik.Solution, psc.pc.planOpts.NumThreads*20)
	defer func() {
		// In lieu of creating a separate WaitGroup to wait on before returning, we simply wait to
		// see the `solutionGen` channel get closed to know that the goroutine we spawned has
		// finished.
		for range solutionGen {
		}
	}()

	solver, err := ik.CreateCombinedIKSolver(psc.pc.lfs.dof, psc.pc.logger, psc.pc.planOpts.NumThreads, psc.pc.planOpts.GoalThreshold)
	if err != nil {
		return nil, err
	}

	var solveError error
	var solveErrorLock sync.Mutex

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

solutionLoop:
	for {
		select {
		case <-ctx.Done():
			// We've been canceled. So have our workers. Can just return.
			return nil, ctx.Err()
		case stepSolution, ok := <-solutionGen:
			if !ok || solvingState.process(stepSolution) {
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
