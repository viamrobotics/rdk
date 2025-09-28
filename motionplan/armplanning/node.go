package armplanning

import (
	"context"
	"fmt"
	"math"
	"sort"
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

// This function is the entrypoint to IK for all cases. Everything prior to here is poses or
// configurations as the user passed in, which here are converted to a list of nodes which are to be
// used as the from or to states for a motionPlanner.
func generateNodeListForGoalState(
	ctx context.Context,
	motionPlanner *cBiRRTMotionPlanner,
	goal referenceframe.FrameSystemPoses,
	ikSeed referenceframe.FrameSystemInputs,
) ([]*node, error) {

	return motionPlanner.getSolutions(ctx, ikSeed, goal)
}

type solutionSolvingState struct {
	pm *planManager
	
	seed referenceframe.FrameSystemInputs
	goodCost float64
	
	solutions         []*node
	failures          *IkConstraintError
	startTime         time.Time
	firstSolutionTime time.Duration
	bestScore         float64
	ikTimeMultiple    int
}

func (sss * solutionSolvingState) computeGoodCost() {
	ratios := mp.lfs.inputChangeRatio(mp.motionChains, seed, mp.planOpts.getGoalMetric(goal), mp.logger)
	
	adjusted := []float64{}
	for idx, r := range ratios {
		adjusted = append(adjusted, mp.lfs.jog(idx, linearSeed[idx], r))
	}
	step, err := mp.lfs.sliceToMap(adjusted)
	if err != nil {
		return nil, err
	}
	stepArc := &motionplan.SegmentFS{
		StartConfiguration: seed,
		EndConfiguration:   step,
		FS:                 mp.fs,
	}
	sss.goodCost = mp.configurationDistanceFunc(stepArc)
	sss.pm.logger.Debugf("goodCost: %v", sss.goodCost)
}

// return bool is if we should stop because we're done.
func (sss *solutionSolvingState) process(stepSolution *ik.Solution,
) bool {
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
		sss.failures.add(step, err)
		return false
	}

	stepArc := &motionplan.SegmentFS{
		StartConfiguration: seed,
		EndConfiguration:   step,
		FS:                 mp.fs,
	}
	err = mp.checker.CheckSegmentFSConstraints(stepArc)
	if err != nil {
		sss.failures.add(step, err)
		return false
	}

	for _, oldSol := range sss.solutions {
		similarity := &motionplan.SegmentFS{
			StartConfiguration: oldSol.inputs,
			EndConfiguration:   step,
			FS:                 mp.fs,
		}
		simscore := mp.configurationDistanceFunc(similarity)
		if simscore < defaultSimScore {
			return false
		}
	}

	myNode := &node{inputs: step, cost: mp.configurationDistanceFunc(stepArc)}
	sss.solutions = append(sss.solutions, myNode)

	const goodCostStopDivier = 3.0

	if myNode.cost < goodCost || // this checks the absolute score of the plan
		// if we've got something sane, and it's really good, let's check
		(myNode.cost < (sss.bestScore*defaultOptimalityMultiple) && myNode.cost < goodCost) {
		whyNot := mp.checkPath(seed, step)
		mp.logger.Debugf("got score %0.4f and goodCost: %0.2f - result: %v", myNode.cost, goodCost, whyNot)
		if whyNot == nil {
			myNode.checkPath = true
			if (myNode.cost < (goodCost / goodCostStopDivier)) ||
				(myNode.cost < mp.planOpts.MinScore && mp.planOpts.MinScore > 0) {
				mp.logger.Debugf("\tscore %0.4f stopping early (%0.2f)", myNode.cost, goodCost/goodCostStopDivier)
				return true // good solution, stopping early
			} else if myNode.cost < (goodCost / (.5 * goodCostStopDivier)) {
				sss.ikTimeMultiple = mp.planOpts.TimeMultipleAfterFindingFirstSolution / 4
			}
		}
	}

	if len(sss.solutions) >= mp.planOpts.MaxSolutions {
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
			mp.logger.Infof("ending early because of time elapsed: %v firstSolutionTime: %v", elapsed, sss.firstSolutionTime)
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
func (mp *cBiRRTMotionPlanner) getSolutions(
	ctx context.Context,
	pm *planManager,
	seed referenceframe.FrameSystemInputs,
	goal referenceframe.FrameSystemPoses,
) ([]*node, error) {
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

	linearSeed, err := mp.lfs.mapToSlice(seed)
	if err != nil {
		return nil, err
	}

	// Spawn the IK solver to generate solutions until done
	minFunc := mp.linearizeFSmetric(mp.planOpts.getGoalMetric(goal))

	mp.logger.Debugf("seed: %v", seed)



	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	solutionGen := make(chan *ik.Solution, mp.planOpts.NumThreads*20)
	defer func() {
		// In lieu of creating a separate WaitGroup to wait on before returning, we simply wait to
		// see the `solutionGen` channel get closed to know that the goroutine we spawned has
		// finished.
		for range solutionGen {
		}
	}()

	// Spawn the IK solver to generate solutions until done
	utils.PanicCapturingGo(func() {
		// This channel close doubles as signaling that the goroutine has exited.
		defer close(solutionGen)
		_, err := mp.solver.Solve(ctxWithCancel, solutionGen, linearSeed, ratios, minFunc, mp.randseed.Int())
		if err != nil {
			mp.logger.Warnf("solver had an error: %v", err)
		}
	})

	solvingState := solutionSolvingState{
		seed: seed,
		pm: pm,
		solutions:         []*node{},
		failures:          newIkConstraintError(mp.fs, mp.checker),
		startTime:         time.Now(),
		firstSolutionTime: time.Hour,
		bestScore:         10000000,
		ikTimeMultiple:    mp.planOpts.TimeMultipleAfterFindingFirstSolution,
	}
	solvingState.computeGoodCost()
	
solutionLoop:
	for {
		select {
		case <-ctx.Done():
			// We've been canceled. So have our workers. Can just return.
			return nil, ctx.Err()
		case stepSolution, ok := <-solutionGen:
			if !ok || mp.process(&solvingState, seed, stepSolution, goodCost) {
				// No longer using the generated solutions. Cancel the workers.
				cancel()
				break solutionLoop
			}
		}
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

func checkPath(request PlanRequest, checker *motionplan.ConstraintChecker, seedInputs, target referenceframe.FrameSystemInputs) error {
	_, err := checker.CheckSegmentAndStateValidityFS(
		&motionplan.SegmentFS{
			StartConfiguration: seedInputs,
			EndConfiguration:   target,
			FS:                 request.FrameSystem,
		},
		request.PlannerOptions.Resolution,
	)
	return err
}
