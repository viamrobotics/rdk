//go:build !no_cgo

// Package motionplan is a motion planning library.
package motionplan

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
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
	sample(node, int) (node, error)
}

type plannerConstructor func(frame.Frame, *rand.Rand, logging.Logger, *plannerOptions) (motionPlanner, error)

// PlanRequest is a struct to store all the data necessary to make a call to PlanMotion.
type PlanRequest struct {
	Logger             logging.Logger
	Goal               *frame.PoseInFrame
	Frame              frame.Frame
	FrameSystem        frame.FrameSystem
	StartPose          spatialmath.Pose
	StartConfiguration map[string][]frame.Input
	WorldState         *frame.WorldState
	ConstraintSpecs    *pb.Constraints
	Options            map[string]interface{}
}

// validatePlanRequest ensures PlanRequests are not malformed.
func (req *PlanRequest) validatePlanRequest() error {
	if req == nil {
		return errors.New("PlanRequest cannot be nil")
	}
	if req.Logger == nil {
		return errors.New("PlanRequest cannot have nil logger")
	}
	if req.Frame == nil {
		return errors.New("PlanRequest cannot have nil frame")
	}

	if req.FrameSystem == nil {
		return errors.New("PlanRequest cannot have nil framesystem")
	} else if req.FrameSystem.Frame(req.Frame.Name()) == nil {
		return frame.NewFrameMissingError(req.Frame.Name())
	}

	if req.Goal == nil {
		return errors.New("PlanRequest cannot have nil goal")
	}

	goalParentFrame := req.Goal.Parent()
	if req.FrameSystem.Frame(goalParentFrame) == nil {
		return frame.NewParentFrameMissingError(req.Goal.Name(), goalParentFrame)
	}

	frameDOF := len(req.Frame.DoF())
	seedMap, ok := req.StartConfiguration[req.Frame.Name()]
	if frameDOF > 0 {
		if !ok {
			return errors.Errorf("%s does not have a start configuration", req.Frame.Name())
		}
		if frameDOF != len(seedMap) {
			return frame.NewIncorrectInputLengthError(len(seedMap), len(req.Frame.DoF()))
		}
	} else if ok && frameDOF != len(seedMap) {
		return frame.NewIncorrectInputLengthError(len(seedMap), len(req.Frame.DoF()))
	}

	return nil
}

// PlanMotion plans a motion from a provided plan request.
func PlanMotion(ctx context.Context, request *PlanRequest) (Plan, error) {
	// Calls Replan but without a seed plan
	return Replan(ctx, request, nil, 0)
}

// PlanFrameMotion plans a motion to destination for a given frame with no frame system. It will create a new FS just for the plan.
// WorldState is not supported in the absence of a real frame system.
func PlanFrameMotion(ctx context.Context,
	logger logging.Logger,
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
	plan, err := PlanMotion(ctx, &PlanRequest{
		Logger:             logger,
		Goal:               frame.NewPoseInFrame(frame.World, dst),
		Frame:              f,
		StartConfiguration: map[string][]frame.Input{f.Name(): seed},
		FrameSystem:        fs,
		ConstraintSpecs:    constraintSpec,
		Options:            planningOpts,
	})
	if err != nil {
		return nil, err
	}
	return plan.Trajectory().GetFrameInputs(f.Name())
}

// Replan plans a motion from a provided plan request, and then will return that plan only if its cost is better than the cost of the
// passed-in plan multiplied by `replanCostFactor`.
func Replan(ctx context.Context, request *PlanRequest, currentPlan Plan, replanCostFactor float64) (Plan, error) {
	// make sure request is well formed and not missing vital information
	if err := request.validatePlanRequest(); err != nil {
		return nil, err
	}

	// Create a frame to solve for, and an IK solver with that frame.
	sf, err := newSolverFrame(request.FrameSystem, request.Frame.Name(), request.Goal.Parent(), request.StartConfiguration)
	if err != nil {
		return nil, err
	}
	if len(sf.DoF()) == 0 {
		return nil, errors.New("solver frame has no degrees of freedom, cannot perform inverse kinematics")
	}

	request.Logger.CDebugf(ctx, "constraint specs for this step: %v", request.ConstraintSpecs)
	request.Logger.CDebugf(ctx, "motion config for this step: %v", request.Options)

	rseed := defaultRandomSeed
	if seed, ok := request.Options["rseed"].(int); ok {
		rseed = seed
	}
	sfPlanner, err := newPlanManager(sf, request.Logger, rseed)
	if err != nil {
		return nil, err
	}

	newPlan, err := sfPlanner.PlanSingleWaypoint(ctx, request, currentPlan)
	if err != nil {
		return nil, err
	}

	if replanCostFactor > 0 && currentPlan != nil {
		initialPlanCost := currentPlan.Trajectory().EvaluateCost(sfPlanner.opt().ScoreFunc)
		finalPlanCost := newPlan.Trajectory().EvaluateCost(sfPlanner.opt().ScoreFunc)
		request.Logger.CDebugf(ctx,
			"initialPlanCost %f adjusted with cost factor to %f, replan cost %f",
			initialPlanCost, initialPlanCost*replanCostFactor, finalPlanCost,
		)

		if finalPlanCost > initialPlanCost*replanCostFactor {
			return nil, errHighReplanCost
		}
	}

	return newPlan, nil
}

type planner struct {
	solver   ik.InverseKinematics
	frame    frame.Frame
	logger   logging.Logger
	randseed *rand.Rand
	start    time.Time
	planOpts *plannerOptions
}

func newPlanner(frame frame.Frame, seed *rand.Rand, logger logging.Logger, opt *plannerOptions) (*planner, error) {
	solver, err := ik.CreateCombinedIKSolver(frame, logger, opt.NumThreads, opt.GoalThreshold)
	if err != nil {
		return nil, err
	}
	mp := &planner{
		solver:   solver,
		frame:    frame,
		logger:   logger,
		randseed: seed,
		planOpts: opt,
	}
	return mp, nil
}

func (mp *planner) checkInputs(inputs []frame.Input) bool {
	ok, _ := mp.planOpts.CheckStateConstraints(&ik.State{
		Configuration: inputs,
		Frame:         mp.frame,
	})
	return ok
}

func (mp *planner) checkPath(seedInputs, target []frame.Input) bool {
	ok, _ := mp.planOpts.CheckSegmentAndStateValidity(
		&ik.Segment{
			StartConfiguration: seedInputs,
			EndConfiguration:   target,
			Frame:              mp.frame,
		},
		mp.planOpts.Resolution,
	)
	return ok
}

func (mp *planner) sample(rSeed node, sampleNum int) (node, error) {
	// If we have done more than 50 iterations, start seeding off completely random positions 2 at a time
	// The 2 at a time is to ensure random seeds are added onto both the seed and goal maps.
	if sampleNum >= mp.planOpts.IterBeforeRand && sampleNum%4 >= 2 {
		return newConfigurationNode(frame.RandomFrameInputs(mp.frame, mp.randseed)), nil
	}
	// Seeding nearby to valid points results in much faster convergence in less constrained space
	q, err := frame.RestrictedRandomFrameInputs(mp.frame, mp.randseed, 0.1, rSeed.Q())
	if err != nil {
		return nil, err
	}
	return newConfigurationNode(q), nil
}

func (mp *planner) opt() *plannerOptions {
	return mp.planOpts
}

// smoothPath will try to naively smooth the path by picking points partway between waypoints and seeing if it can interpolate
// directly between them. This will significantly improve paths from RRT*, as it will shortcut the randomly-selected configurations.
// This will only ever improve paths (or leave them untouched), and runs very quickly.
func (mp *planner) smoothPath(ctx context.Context, path []node) []node {
	mp.logger.CDebugf(ctx, "running simple smoother on path of len %d", len(path))
	if mp.planOpts == nil {
		mp.logger.CDebug(ctx, "nil opts, cannot shortcut")
		return path
	}
	if len(path) <= 2 {
		mp.logger.CDebug(ctx, "path too short, cannot shortcut")
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

		wayPoint1, err := mp.frame.Interpolate(path[firstEdge].Q(), path[firstEdge+1].Q(), waypoints[mp.randseed.Intn(3)])
		if err != nil {
			return path
		}
		wayPoint2, err := mp.frame.Interpolate(path[secondEdge].Q(), path[secondEdge+1].Q(), waypoints[mp.randseed.Intn(3)])
		if err != nil {
			return path
		}

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

	solutionGen := make(chan *ik.Solution, mp.planOpts.NumThreads*2)
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
			statePass, failName := mp.planOpts.CheckStateConstraints(&ik.State{
				Configuration: step,
				Frame:         mp.frame,
			})
			if statePass {
				stepArc := &ik.Segment{
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
	for done := false; !done; {
		select {
		case <-solutionGen:
		default:
			done = true
		}
	}

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
	// TODO: switch this to slices.Sort when golang 1.21 is supported by RDK
	sort.Float64s(keys)

	orderedSolutions := make([]node, 0)
	for _, key := range keys {
		orderedSolutions = append(orderedSolutions, &basicNode{q: solutions[key], cost: key})
	}
	return orderedSolutions, nil
}

// CheckPlan checks if obstacles intersect the trajectory of the frame following the plan. If one is
// detected, the interpolated position of the rover when a collision is detected is returned along
// with an error with additional collision details.
func CheckPlan(
	checkFrame frame.Frame,
	plan Plan,
	wayPointIdx int,
	worldState *frame.WorldState,
	fs frame.FrameSystem,
	currentPose spatialmath.Pose,
	currentInputs map[string][]frame.Input,
	errorState spatialmath.Pose,
	lookAheadDistanceMM float64,
	logger logging.Logger,
) error {
	// ensure that we can actually perform the check
	if len(plan.Path()) < 1 {
		return errors.New("plan must have at least one element")
	}
	fmt.Println("WE ARE MAKING THE SOLVERFRAME NOW")
	fmt.Println("fs.FrameNames(): ", fs.FrameNames())

	// construct solverFrame
	// Note that this requires all frames which move as part of the plan, to have an
	// entry in the very first plan waypoint
	sf, err := newSolverFrame(fs, checkFrame.Name(), frame.World, currentInputs)
	if err != nil {
		return err
	}

	// construct planager
	sfPlanner, err := newPlanManager(sf, logger, defaultRandomSeed)
	if err != nil {
		return err
	}

	// This should be done for any plan whose configurations are specified in relative terms rather than absolute ones.
	// Currently this is only TP-space, so we check if the PTG length is >0.
	// The solver frame will have had its PTGs filled in the newPlanManager() call, if applicable.
	relative := len(sf.PTGSolvers()) > 0

	// offset the plan using the errorState
	offsetPlan := OffsetPlan(plan, errorState)

	// get plan poses for checkFrame
	poses, err := offsetPlan.Path().GetFramePoses(checkFrame.Name())
	if err != nil {
		return err
	}
	plannerSetUpInputs := currentInputs
	plannerSetUpInputs[checkFrame.Name()] = make([]frame.Input, len(checkFrame.DoF()))
	// setup the planOpts
	if sfPlanner.planOpts, err = sfPlanner.plannerSetupFromMoveRequest(
		currentPose,
		poses[len(poses)-1],
		plannerSetUpInputs,
		worldState,
		nil, // no pb.Constraints
		nil, // no plannOpts
	); err != nil {
		return err
	}

	// create a list of segments to iterate through
	segments := make([]*ik.Segment, 0, len(poses)-wayPointIdx)
	if relative {
		// get checkFrame's currentInputs
		// *currently* it is guaranteed that a relative frame will constitute 100% of a solver frame's dof
		fmt.Println("currentInputs: ", currentInputs)
		checkFrameCurrentInputs, err := sf.mapToSlice(currentInputs)
		if err != nil {
			return err
		}
		fmt.Println("checkFrameCurrentInputs: ", checkFrameCurrentInputs)

		// pre-pend to segments so we can connect to the input we have not finished actuating yet
		segments = append(segments, &ik.Segment{
			StartPosition:      currentPose,
			EndPosition:        poses[wayPointIdx],
			StartConfiguration: checkFrameCurrentInputs,
			EndConfiguration:   checkFrameCurrentInputs,
			Frame:              sf,
		})
	}

	// function to ease further segment creation
	createSegment := func(
		currPose, nextPose spatialmath.Pose,
		currInput, nextInput map[string][]frame.Input,
	) (*ik.Segment, error) {
		// there will be an issue here since currInputs is not of the correct length
		currInputSlice, err := sf.mapToSlice(currInput)
		if err != nil {
			return nil, err
		}
		nextInputSlice, err := sf.mapToSlice(nextInput)
		if err != nil {
			return nil, err
		}
		// If we are working with a PTG plan we redefine the startConfiguration in terms of the endConfiguration.
		// This allows us the properly interpolate along the same arc family and sub-arc within that family.
		if relative {
			currInputSlice = nextInputSlice
		}
		return &ik.Segment{
			StartPosition:      currPose,
			EndPosition:        nextPose,
			StartConfiguration: currInputSlice,
			EndConfiguration:   nextInputSlice,
			Frame:              sf,
		}, nil
	}

	// iterate through remaining plan and append remaining segments to check
	for i := wayPointIdx; i < len(offsetPlan.Path())-1; i++ {
		currInput := offsetPlan.Trajectory()[i]
		nextInput := offsetPlan.Trajectory()[i+1]
		if relative {
			currInput[checkFrame.Name()+"ExecutionFrame"] = frame.FloatsToInputs(
				[]float64{poses[i].Point().X, poses[i].Point().Y, poses[i].Orientation().OrientationVectorRadians().Theta},
			)
			nextInput[checkFrame.Name()+"ExecutionFrame"] = frame.FloatsToInputs(
				[]float64{poses[i+1].Point().X, poses[i+1].Point().Y, poses[i+1].Orientation().OrientationVectorRadians().Theta},
			)
		}
		segment, err := createSegment(poses[i], poses[i+1], currInput, nextInput)
		if err != nil {
			return err
		}
		segments = append(segments, segment)
	}

	for _, s := range segments {
		fmt.Println("s: ", s)
	}

	// go through segments and check that we satisfy constraints
	// TODO(RSDK-5007): If we can make interpolate a method on Frame the need to write this out will be lessened and we should be
	// able to call CheckStateConstraintsAcrossSegment directly.
	var totalTravelDistanceMM float64
	for _, segment := range segments {
		interpolatedConfigurations, err := interpolateSegment(segment, sfPlanner.planOpts.Resolution)
		if err != nil {
			return err
		}
		for _, interpConfig := range interpolatedConfigurations {
			poseInPath, err := sf.Transform(interpConfig)
			if err != nil {
				return err
			}
			// Check if look ahead distance has been reached
			currentTravelDistanceMM := totalTravelDistanceMM + poseInPath.Point().Distance(segment.StartPosition.Point())
			if currentTravelDistanceMM > lookAheadDistanceMM {
				return nil
			}

			// define State which only houses inputs, pose information not needed
			interpolatedState := &ik.State{Frame: sf}
			interpolatedState.Configuration = interpConfig
			fmt.Println("interpConfig: ", interpConfig)
			fmt.Println("interpolatedState: ", interpolatedState)

			// Checks for collision along the interpolated route and returns a the first interpolated pose where a collision is detected.
			if isValid, err := sfPlanner.planOpts.CheckStateConstraints(interpolatedState); !isValid {
				return fmt.Errorf("found error between positions %v and %v: %s",
					segment.StartPosition.Point(),
					segment.EndPosition.Point(),
					err,
				)
			}
		}

		// Update total traveled distance after segment has been checked
		totalTravelDistanceMM += segment.EndPosition.Point().Distance(segment.StartPosition.Point())
	}

	return nil
}
