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
	return plan.GetFrameSteps(f.Name())
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
	seed, err := sf.mapToSlice(request.StartConfiguration)
	if err != nil {
		return nil, err
	}
	startPose, err := sf.Transform(seed)
	if err != nil {
		return nil, err
	}
	// make sure there is no transformation between the PTG frame and World frame in the Solver frame
	if len(sf.PTGSolvers()) > 0 && !spatialmath.PoseAlmostEqual(startPose, spatialmath.NewZeroPose()) {
		return nil, errors.New("cannot have non-zero transformation between the PTG frame and World frame in the Solver frame")
	}

	request.Logger.Infof(
		"planning motion for frame %s\nGoal: %v\nStarting seed map %v\n, startPose %v\n, worldstate: %v\n",
		request.Frame.Name(),
		frame.PoseInFrameToProtobuf(request.Goal),
		request.StartConfiguration,
		spatialmath.PoseToProtobuf(startPose),
		request.WorldState.String(),
	)
	request.Logger.Debugf("constraint specs for this step: %v", request.ConstraintSpecs)
	request.Logger.Debugf("motion config for this step: %v", request.Options)

	rseed := defaultRandomSeed
	if seed, ok := request.Options["rseed"].(int); ok {
		rseed = seed
	}
	sfPlanner, err := newPlanManager(sf, request.FrameSystem, request.Logger, rseed)
	if err != nil {
		return nil, err
	}

	resultSlices, err := sfPlanner.PlanSingleWaypoint(
		ctx,
		request.StartConfiguration,
		request.Goal.Pose(),
		request.WorldState,
		request.ConstraintSpecs,
		currentPlan,
		request.Options,
	)
	if err != nil {
		return nil, err
	}
	newPlan := sf.inputsToPlan(resultSlices)

	if replanCostFactor > 0 && currentPlan != nil {
		initialPlanCost := currentPlan.Evaluate(sfPlanner.opt().ScoreFunc)
		finalPlanCost := newPlan.Evaluate(sfPlanner.opt().ScoreFunc)
		request.Logger.Debugf(
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
	worldState *frame.WorldState,
	fs frame.FrameSystem,
	currentPosition spatialmath.Pose,
	currentInputs []frame.Input,
	errorState spatialmath.Pose,
	lookAheadDistanceMM float64,
	logger logging.Logger,
) error {
	// ensure that we can actually perform the check
	if len(plan) < 1 {
		return errors.New("plan must have at least one element")
	}

	// construct solverFrame
	// Note that this requires all frames which move as part of the plan, to have an
	// entry in the very first plan waypoint
	sf, err := newSolverFrame(fs, checkFrame.Name(), frame.World, plan[0])
	if err != nil {
		return err
	}

	// construct planager
	sfPlanner, err := newPlanManager(sf, fs, logger, defaultRandomSeed)
	if err != nil {
		return err
	}

	// convert plan into nodes
	planNodes, err := sf.planToNodes(plan)
	if err != nil {
		return err
	}

	// This should be done for any plan whose configurations are specified in relative terms rather than absolute ones.
	// Currently this is only TP-space, so we check if the PTG length is >0.
	// The solver frame will have had its PTGs filled in the newPlanManager() call, if applicable.
	relative := len(sf.PTGSolvers()) > 0

	if relative {
		// get pose of robot along the current trajectory it is executing
		lastPose, err := sf.Transform(currentInputs)
		if err != nil {
			return err
		}

		// where ought the robot be on the plan
		pathPosition := spatialmath.PoseBetweenInverse(errorState, currentPosition)

		// absolute pose of the previous node we've passed
		formerRunningPose := spatialmath.PoseBetweenInverse(lastPose, pathPosition)

		// convert planNode's poses to be in absolute coordinates
		if planNodes, err = rectifyTPspacePath(planNodes, sf, formerRunningPose); err != nil {
			return err
		}
	}

	// adjust planNodes by the errorState
	// this only changes a node's pose and not its inputs
	planNodes = transformNodes(planNodes, errorState)

	// pre-pend node with current position of robot to planNodes
	// Note that currentPosition is assumed to have accounted for the errorState
	// Note that currentInputs is assumed to have NOT accounted for the errorState
	planNodes = append([]node{&basicNode{pose: currentPosition, q: currentInputs}}, planNodes...)

	// create constraints
	if sfPlanner.planOpts, err = sfPlanner.plannerSetupFromMoveRequest(
		currentPosition,                    // starting pose
		planNodes[len(planNodes)-1].Pose(), // goalPose
		plan[0],                            // starting configuration
		worldState,
		nil, // no pb.Constraints
		nil, // no plannOpts
	); err != nil {
		return err
	}

	// go through plan and check that we can move from plan[i] to plan[i+1]
	var totalTravelDistanceMM float64
	for i := 0; i < len(planNodes)-1; i++ {
		currentPose := planNodes[i].Pose()
		nextPose := planNodes[i+1].Pose()
		startConfiguration := planNodes[i].Q()
		endConfiguration := planNodes[i+1].Q()

		// If we are working with a PTG plan we redefine the startConfiguration in terms of the endConfiguration.
		// This allows us the properly interpolate along the same arc family and sub-arc within that family.
		if relative {
			startConfiguration = []frame.Input{
				{Value: endConfiguration[0].Value}, {Value: endConfiguration[1].Value}, {Value: 0},
			}
		}
		segment := &ik.Segment{
			StartPosition:      currentPose,
			EndPosition:        nextPose,
			StartConfiguration: startConfiguration,
			EndConfiguration:   endConfiguration,
			Frame:              sf,
		}

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
			// If we are working with a PTG plan the returned value for poseInPath will only
			// tell us how far along the arc we have traveled. Since this is only the relative position,
			// i.e. relative to where the robot started executing the arc,
			// we must compose poseInPath with currentPose to get the absolute position.
			// In both cases we ultimately compose with errorState.
			if relative {
				rectifyBy := spatialmath.Compose(currentPose, errorState)
				poseInPath = spatialmath.Compose(rectifyBy, poseInPath)
			} else {
				poseInPath = spatialmath.Compose(poseInPath, errorState)
			}

			modifiedState := &ik.State{Frame: sf, Position: poseInPath}

			// Checks for collision along the interpolated route and returns a the first interpolated pose where a
			// collision is detected.
			if isValid, _ := sfPlanner.planOpts.CheckStateConstraints(modifiedState); !isValid {
				return fmt.Errorf("found collision between positions %v and %v", currentPose.Point(), nextPose.Point())
			}
		}

		// Update total traveled distance after segment has been checked
		totalTravelDistanceMM += segment.EndPosition.Point().Distance(segment.StartPosition.Point())
	}

	return nil
}
