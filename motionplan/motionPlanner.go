// Package motionplan is a motion planning library.
package motionplan

import (
	"context"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/utils"

	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const defaultRandomSeed = 0

// motionPlanner provides an interface to path planning methods, providing ways to request a path to be planned, and
// management of the constraints used to plan paths.
type motionPlanner interface {
	// Plan will take a context, a goal position, and an input start state and return a series of state waypoints which
	// should be visited in order to arrive at the goal while satisfying all constraints
	plan(context.Context, spatialmath.Pose, []frame.Input) ([]node, error)

	// Everything below this point should be covered by anything that wraps the generic `planner`
	smoothPath(context.Context, []node) []node
	checkPath([]frame.Input, []frame.Input) bool
	checkInputs([]frame.Input) bool
	getSolutions(context.Context, []frame.Input) ([]node, error)
	opt() *plannerOptions
}

type plannerConstructor func(frame.Frame, *rand.Rand, golog.Logger, *plannerOptions) (motionPlanner, error)

// PlanMotion plans a motion to destination for a given frame. It takes a given frame system, wraps it with a SolvableFS, and solves.
func PlanMotion(ctx context.Context,
	logger golog.Logger,
	dst *frame.PoseInFrame,
	f frame.Frame,
	seedMap map[string][]frame.Input,
	fs frame.FrameSystem,
	worldState *frame.WorldState,
	constraintSpec *pb.Constraints,
	planningOpts map[string]interface{},
) ([]map[string][]frame.Input, error) {
	return motionPlanInternal(ctx, logger, dst, f, seedMap, fs, worldState, constraintSpec, planningOpts)
}

// PlanFrameMotion plans a motion to destination for a given frame with no frame system. It will create a new FS just for the plan.
// WorldState is not supported in the absence of a real frame system.
func PlanFrameMotion(ctx context.Context,
	logger golog.Logger,
	dst spatialmath.Pose,
	f frame.Frame,
	seed []frame.Input,
	constraintSpec *pb.Constraints,
	planningOpts map[string]interface{},
) ([][]frame.Input, error) {
	// ephemerally create a framesystem containing just the frame for the solve
	fs := frame.NewEmptyFrameSystem("")
	if err := fs.AddFrame(f, fs.World()); err != nil {
		return nil, err
	}
	destination := frame.NewPoseInFrame(frame.World, dst)
	seedMap := map[string][]frame.Input{f.Name(): seed}
	solutionMap, err := motionPlanInternal(ctx, logger, destination, f, seedMap, fs, nil, constraintSpec, planningOpts)
	if err != nil {
		return nil, err
	}
	return FrameStepsFromRobotPath(f.Name(), solutionMap)
}

// motionPlanInternal is the internal private function that all motion planning access calls. This will construct the plan manager for each
// waypoint, and return at the end.
func motionPlanInternal(ctx context.Context,
	logger golog.Logger,
	goal *frame.PoseInFrame,
	f frame.Frame,
	seedMap map[string][]frame.Input,
	fs frame.FrameSystem,
	worldState *frame.WorldState,
	constraintSpec *pb.Constraints,
	motionConfig map[string]interface{},
) ([]map[string][]frame.Input, error) {
	if goal == nil {
		return nil, errors.New("no destination passed to Motion")
	}

	steps := []map[string][]frame.Input{}

	// Create a frame to solve for, and an IK solver with that frame.
	sf, err := newSolverFrame(fs, f.Name(), goal.Parent(), seedMap)
	if err != nil {
		return nil, err
	}
	if len(sf.DoF()) == 0 {
		return nil, errors.New("solver frame has no degrees of freedom, cannot perform inverse kinematics")
	}
	seed, err := sf.mapToSlice(seedMap)
	if err != nil {
		return nil, err
	}
	startPose, err := sf.Transform(seed)
	if err != nil {
		return nil, err
	}

	logger.Infof(
		"planning motion for frame %s\nGoal: %v\nStarting seed map %v\n, startPose %v\n, worldstate: %v\n",
		f.Name(),
		frame.PoseInFrameToProtobuf(goal),
		seedMap,
		spatialmath.PoseToProtobuf(startPose),
		worldState.String(),
	)
	logger.Debugf("constraint specs for this step: %v", constraintSpec)
	logger.Debugf("motion config for this step: %v", motionConfig)

	rseed := defaultRandomSeed
	if seed, ok := motionConfig["rseed"].(int); ok {
		rseed = seed
	}

	sfPlanner, err := newPlanManager(sf, fs, logger, rseed)
	if err != nil {
		return nil, err
	}
	resultSlices, err := sfPlanner.PlanSingleWaypoint(ctx, seedMap, goal.Pose(), worldState, constraintSpec, motionConfig)
	if err != nil {
		return nil, err
	}
	for _, resultSlice := range resultSlices {
		stepMap := sf.sliceToMap(resultSlice)
		steps = append(steps, stepMap)
	}

	logger.Debugf("final plan steps: %v", steps)

	return steps, nil
}

type planner struct {
	solver   InverseKinematics
	frame    frame.Frame
	logger   golog.Logger
	randseed *rand.Rand
	start    time.Time
	planOpts *plannerOptions
}

func newPlanner(frame frame.Frame, seed *rand.Rand, logger golog.Logger, opt *plannerOptions) (*planner, error) {
	ik, err := CreateCombinedIKSolver(frame, logger, opt.NumThreads, opt.GoalThreshold)
	if err != nil {
		return nil, err
	}
	mp := &planner{
		solver:   ik,
		frame:    frame,
		logger:   logger,
		randseed: seed,
		planOpts: opt,
	}
	return mp, nil
}

func (mp *planner) checkInputs(inputs []frame.Input) bool {
	ok, _ := mp.planOpts.CheckStateConstraints(&State{
		Configuration: inputs,
		Frame:         mp.frame,
	})
	return ok
}

func (mp *planner) checkPath(seedInputs, target []frame.Input) bool {
	ok, _ := mp.planOpts.CheckSegmentAndStateValidity(
		&Segment{
			StartConfiguration: seedInputs,
			EndConfiguration:   target,
			Frame:              mp.frame,
		},
		mp.planOpts.Resolution,
	)
	return ok
}

func (mp *planner) sample(rSeed node, sampleNum int) ([]frame.Input, error) {
	// If we have done more than 50 iterations, start seeding off completely random positions 2 at a time
	// The 2 at a time is to ensure random seeds are added onto both the seed and goal maps.
	if sampleNum >= mp.planOpts.IterBeforeRand && sampleNum%4 >= 2 {
		return frame.RandomFrameInputs(mp.frame, mp.randseed), nil
	}
	// Seeding nearby to valid points results in much faster convergence in less constrained space
	return frame.RestrictedRandomFrameInputs(mp.frame, mp.randseed, 0.1, rSeed.Q())
}

func (mp *planner) opt() *plannerOptions {
	return mp.planOpts
}

// smoothPath will try to naively smooth the path by picking points partway between waypoints and seeing if it can interpolate
// directly between them. This will significantly improve paths from RRT*, as it will shortcut the randomly-selected configurations.
// This will only ever improve paths (or leave them untouched), and runs very quickly.
func (mp *planner) smoothPath(ctx context.Context, path []node) []node {
	mp.logger.Debugf("running simple smoother on path of len %d", len(path))
	if mp.planOpts == nil {
		mp.logger.Debug("nil opts, cannot shortcut")
		return path
	}
	if len(path) <= 2 {
		mp.logger.Debug("path too short, cannot shortcut")
		return path
	}

	// Randomly pick which quarter of motion to check from; this increases flexibility of smoothing.
	waypoints := []float64{0.25, 0.5, 0.75}

	for i := 0; i < mp.planOpts.SmoothIter; i++ {
		select {
		case <-ctx.Done():
			return path
		default:
		}
		// get start node of first edge. Cannot be either the last or second-to-last node.
		// Intn will return an int in the half-open interval half-open interval [0,n)
		firstEdge := mp.randseed.Intn(len(path) - 2)
		secondEdge := firstEdge + 1 + mp.randseed.Intn((len(path)-2)-firstEdge)
		mp.logger.Debugf("checking shortcut between nodes %d and %d", firstEdge, secondEdge+1)

		wayPoint1 := frame.InterpolateInputs(path[firstEdge].Q(), path[firstEdge+1].Q(), waypoints[mp.randseed.Intn(3)])
		wayPoint2 := frame.InterpolateInputs(path[secondEdge].Q(), path[secondEdge+1].Q(), waypoints[mp.randseed.Intn(3)])

		if mp.checkPath(wayPoint1, wayPoint2) {
			newpath := []node{}
			newpath = append(newpath, path[:firstEdge+1]...)
			newpath = append(newpath, newConfigurationNode(wayPoint1), newConfigurationNode(wayPoint2))
			// have to split this up due to go compiler quirk where elipses operator can't be mixed with other vars in append
			newpath = append(newpath, path[secondEdge+1:]...)
			path = newpath
		}
	}
	return path
}

// getSolutions will initiate an IK solver for the given position and seed, collect solutions, and score them by constraints.
// If maxSolutions is positive, once that many solutions have been collected, the solver will terminate and return that many solutions.
// If minScore is positive, if a solution scoring below that amount is found, the solver will terminate and return that one solution.
func (mp *planner) getSolutions(ctx context.Context, seed []frame.Input) ([]node, error) {
	// Linter doesn't properly handle loop labels
	nSolutions := mp.planOpts.MaxSolutions
	if nSolutions == 0 {
		nSolutions = defaultSolutionsToSeed
	}

	seedPos, err := mp.frame.Transform(seed)
	if err != nil {
		return nil, err
	}
	if mp.planOpts.goalMetric == nil {
		return nil, errors.New("metric is nil")
	}

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	solutionGen := make(chan *IKSolution, mp.planOpts.NumThreads)
	ikErr := make(chan error, 1)
	var activeSolvers sync.WaitGroup
	defer activeSolvers.Wait()
	activeSolvers.Add(1)
	// Spawn the IK solver to generate solutions until done
	utils.PanicCapturingGo(func() {
		defer close(ikErr)
		defer activeSolvers.Done()
		ikErr <- mp.solver.Solve(ctxWithCancel, solutionGen, seed, mp.planOpts.goalMetric, mp.randseed.Int())
	})

	solutions := map[float64][]frame.Input{}

	// A map keeping track of which constraints fail
	failures := map[string]int{}
	constraintFailCnt := 0

	// Solve the IK solver. Loop labels are required because `break` etc in a `select` will break only the `select`.
IK:
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		select {
		case stepSolution := <-solutionGen:
			step := stepSolution.Configuration
			// Ensure the end state is a valid one
			statePass, failName := mp.planOpts.CheckStateConstraints(&State{
				Configuration: step,
				Frame:         mp.frame,
			})
			if statePass {
				stepArc := &Segment{
					StartConfiguration: seed,
					StartPosition:      seedPos,
					EndConfiguration:   step,
					Frame:              mp.frame,
				}
				arcPass, failName := mp.planOpts.CheckSegmentConstraints(stepArc)

				if arcPass {
					score := mp.planOpts.goalArcScore(stepArc)
					if score < mp.planOpts.MinScore && mp.planOpts.MinScore > 0 {
						solutions = map[float64][]frame.Input{}
						solutions[score] = step
						// good solution, stopping early
						break IK
					}

					solutions[score] = step
					if len(solutions) >= nSolutions {
						// sufficient solutions found, stopping early
						break IK
					}
				} else {
					constraintFailCnt++
					failures[failName]++
				}
			} else {
				constraintFailCnt++
				failures[failName]++
			}
			// Skip the return check below until we have nothing left to read from solutionGen
			continue IK
		default:
		}

		select {
		case <-ikErr:
			// If we have a return from the IK solver, there are no more solutions, so we finish processing above
			// until we've drained the channel, handled by the `continue` above
			break IK
		default:
		}
	}

	// Cancel any ongoing processing within the IK solvers if we're done receiving solutions
	cancel()

	if len(solutions) == 0 {
		// We have failed to produce a usable IK solution. Let the user know if zero IK solutions were produced, or if non-zero solutions
		// were produced, which constraints were failed
		if constraintFailCnt == 0 {
			return nil, errIKSolve
		}

		return nil, genIKConstraintErr(failures, constraintFailCnt)
	}

	keys := make([]float64, 0, len(solutions))
	for k := range solutions {
		keys = append(keys, k)
	}
	sort.Float64s(keys)

	orderedSolutions := make([]node, 0)
	for _, key := range keys {
		orderedSolutions = append(orderedSolutions, &basicNode{q: solutions[key], cost: key})
	}
	return orderedSolutions, nil
}
