// Package armplanning is a motion planning library.
package armplanning

import (
	"context"
	"math"
	"math/rand"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// When we generate solutions, if a new solution is within this level of similarity to an existing one, discard it as a duplicate.
// This prevents seeding the solution tree with 50 copies of essentially the same configuration.
const defaultSimScore = 0.05

// motionPlanner provides an interface to path planning methods, providing ways to request a path to be planned, and
// management of the constraints used to plan paths.
type motionPlanner interface {
	// Plan will take a context, a goal position, and an input start state and return a series of state waypoints which
	// should be visited in order to arrive at the goal while satisfying all constraints
	plan(ctx context.Context, seed, goal *PlanState) ([]node, error)

	// Everything below this point should be covered by anything that wraps the generic `planner`
	smoothPath(context.Context, []node) []node
	checkPath(referenceframe.FrameSystemInputs, referenceframe.FrameSystemInputs) bool
	checkInputs(referenceframe.FrameSystemInputs) bool
	getSolutions(context.Context, referenceframe.FrameSystemInputs, motionplan.StateFSMetric) ([]node, error)
	opt() *PlannerOptions
	sample(node, int) (node, error)
	getScoringFunction() motionplan.SegmentFSMetric
}

type planner struct {
	*ConstraintHandler
	fs                        *referenceframe.FrameSystem
	lfs                       *linearizedFrameSystem
	solver                    ik.Solver
	logger                    logging.Logger
	randseed                  *rand.Rand
	start                     time.Time
	scoringFunction           motionplan.SegmentFSMetric
	poseDistanceFunc          motionplan.SegmentMetric
	configurationDistanceFunc motionplan.SegmentFSMetric
	planOpts                  *PlannerOptions
	motionChains              *motionChains
}

func newPlannerFromPlanRequest(logger logging.Logger, request *PlanRequest) (*planner, error) {
	mChains, err := motionChainsFromPlanState(request.FrameSystem, request.Goals[0])
	if err != nil {
		return nil, err
	}

	// Theoretically, a plan could be made between two poses, by running IK on both the start and end poses to create sets of seed and
	// goal configurations. However, the blocker here is the lack of a "known good" configuration used to determine which obstacles
	// are allowed to collide with one another.
	if !mChains.useTPspace && (request.StartState.configuration == nil) {
		return nil, errors.New("must populate start state configuration if not planning for 2d base/tpspace")
	}

	if mChains.useTPspace {
		if request.StartState.poses == nil {
			return nil, errors.New("must provide a startPose if solving for PTGs")
		}
		if len(request.Goals) != 1 {
			return nil, errors.New("can only provide one goal if solving for PTGs")
		}
	}

	opt, err := updateOptionsForPlanning(request.PlannerOptions, mChains.useTPspace)
	if err != nil {
		return nil, err
	}

	boundingRegions, err := spatialmath.NewGeometriesFromProto(request.BoundingRegions)
	if err != nil {
		return nil, err
	}

	constraintHandler, err := newConstraintHandler(
		opt,
		logger,
		request.Constraints,
		request.StartState,
		request.Goals[0],
		request.FrameSystem,
		mChains,
		request.StartState.configuration,
		request.WorldState,
		boundingRegions,
	)
	if err != nil {
		return nil, err
	}
	seed := opt.RandomSeed

	//nolint:gosec
	return newPlanner(
		request.FrameSystem,
		rand.New(rand.NewSource(int64(seed))),
		logger,
		opt,
		constraintHandler,
		mChains,
	)
}

func newPlanner(
	fs *referenceframe.FrameSystem,
	seed *rand.Rand,
	logger logging.Logger,
	opt *PlannerOptions,
	constraintHandler *ConstraintHandler,
	chains *motionChains,
) (*planner, error) {
	lfs, err := newLinearizedFrameSystem(fs)
	if err != nil {
		return nil, err
	}
	if opt == nil {
		opt = NewBasicPlannerOptions()
	}
	if constraintHandler == nil {
		constraintHandler = newEmptyConstraintHandler()
	}
	if chains == nil {
		chains = &motionChains{}
	}

	solver, err := ik.CreateCombinedIKSolver(lfs.dof, logger, opt.NumThreads, opt.GoalThreshold)
	if err != nil {
		return nil, err
	}
	mp := &planner{
		ConstraintHandler:         constraintHandler,
		solver:                    solver,
		fs:                        fs,
		lfs:                       lfs,
		logger:                    logger,
		randseed:                  seed,
		planOpts:                  opt,
		scoringFunction:           opt.getScoringFunction(chains),
		poseDistanceFunc:          opt.getPoseDistanceFunc(),
		configurationDistanceFunc: motionplan.GetConfigurationDistanceFunc(opt.ConfigurationDistanceMetric),
		motionChains:              chains,
	}
	return mp, nil
}

func (mp *planner) checkInputs(inputs referenceframe.FrameSystemInputs) bool {
	return mp.CheckStateFSConstraints(&motionplan.StateFS{
		Configuration: inputs,
		FS:            mp.fs,
	}) == nil
}

func (mp *planner) checkPath(seedInputs, target referenceframe.FrameSystemInputs) bool {
	ok, _ := mp.CheckSegmentAndStateValidityFS(
		&motionplan.SegmentFS{
			StartConfiguration: seedInputs,
			EndConfiguration:   target,
			FS:                 mp.fs,
		},
		mp.planOpts.Resolution,
	)
	return ok
}

func (mp *planner) sample(rSeed node, sampleNum int) (node, error) {
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

func (mp *planner) opt() *PlannerOptions {
	return mp.planOpts
}

func (mp *planner) getScoringFunction() motionplan.SegmentFSMetric {
	return mp.scoringFunction
}

type solutionSolvingState struct {
	solutions         map[float64]referenceframe.FrameSystemInputs
	failures          map[string]int // A map keeping track of which constraints fail
	constraintFailCnt int
	startTime         time.Time
	firstSolutionTime time.Duration
}

// return bool is if we should stop because we're done.
func (mp *planner) process(sss *solutionSolvingState, seed referenceframe.FrameSystemInputs, stepSolution *ik.Solution) bool {
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
	err = mp.CheckStateFSConstraints(&motionplan.StateFS{
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
	err = mp.CheckSegmentFSConstraints(stepArc)
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
func (mp *planner) getSolutions(
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
		orderedSolutions = append(orderedSolutions, &basicNode{q: solvingState.solutions[key], cost: key})
	}
	return orderedSolutions, nil
}

// linearize the goal metric for use with solvers.
// Since our solvers operate on arrays of floats, there needs to be a way to map bidirectionally between the framesystem configuration
// of FrameSystemInputs and the []float64 that the solver expects. This is that mapping.
func (mp *planner) linearizeFSmetric(metric motionplan.StateFSMetric) func([]float64) float64 {
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
func (mp *planner) nonchainMinimize(seed, step referenceframe.FrameSystemInputs) referenceframe.FrameSystemInputs {
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

	_, lastGood := mp.CheckStateConstraintsAcrossSegmentFS(
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
