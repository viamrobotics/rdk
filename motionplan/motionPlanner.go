// Package motionplan is a motion planning library.
package motionplan

import (
	"context"
	"errors"
	"math"
	"sort"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	spatial "go.viam.com/rdk/spatialmath"
	vutil "go.viam.com/rdk/utils"
)

// MotionPlanner provides an interface to path planning methods, providing ways to request a path to be planned, and
// management of the constraints used to plan paths.
type MotionPlanner interface {
	// Plan will take a context, a goal position, and an input start state and return a series of state waypoints which
	// should be visited in order to arrive at the goal while satisfying all constraints
	Plan(context.Context, *commonpb.Pose, []frame.Input, *PlannerOptions) ([][]frame.Input, error)
	Frame() frame.Frame // Frame will return the frame used for planning
}

// needed to wrap slices so we can use them as map keys.
type configuration struct {
	inputs []frame.Input
}

type planReturn struct {
	steps []*configuration
	err   error
}

// PlanRobotMotion plans a motion to destination for a given frame. A robot object is passed in and current position inputs are determined.
func PlanRobotMotion(ctx context.Context,
	dst *frame.PoseInFrame,
	f frame.Frame,
	r robot.Robot,
	fs frame.FrameSystem,
	worldState *commonpb.WorldState,
	planningOpts map[string]interface{},
) ([]map[string][]frame.Input, error) {
	seedMap, _, err := framesystem.RobotFsCurrentInputs(ctx, r, fs)
	if err != nil {
		return nil, err
	}

	return PlanWaypoints(ctx, r.Logger(), []*frame.PoseInFrame{dst}, f, seedMap, fs, worldState, []map[string]interface{}{planningOpts})
}

// PlanMotion plans a motion to destination for a given frame. It takes a given frame system, wraps it with a SolvableFS, and solves.
func PlanMotion(ctx context.Context,
	logger golog.Logger,
	dst *frame.PoseInFrame,
	f frame.Frame,
	seedMap map[string][]frame.Input,
	fs frame.FrameSystem,
	worldState *commonpb.WorldState,
	planningOpts map[string]interface{},
) ([]map[string][]frame.Input, error) {
	return PlanWaypoints(ctx, logger, []*frame.PoseInFrame{dst}, f, seedMap, fs, worldState, []map[string]interface{}{planningOpts})
}

// PlanWaypoints plans motions to a list of destinations in order for a given frame. It takes a given frame system, wraps it with a
// SolvableFS, and solves. It will generate a list of intermediate waypoints as well to pass to the solvable framesystem if possible.
func PlanWaypoints(ctx context.Context,
	logger golog.Logger,
	dst []*frame.PoseInFrame,
	f frame.Frame,
	seedMap map[string][]frame.Input,
	fs frame.FrameSystem,
	worldState *commonpb.WorldState,
	planningOpts []map[string]interface{},
) ([]map[string][]frame.Input, error) {
	solvableFS := NewSolvableFrameSystem(fs, logger)
	if len(dst) == 0 {
		return nil, errors.New("no destinations passed to PlanWaypoints")
	}

	return solvableFS.SolveWaypointsWithOptions(ctx, seedMap, dst, f.Name(), worldState, planningOpts)
}

// runPlannerWithWaypoints will plan to each of a list of goals in order, optionally also taking a new planner option for each goal.
func runPlannerWithWaypoints(ctx context.Context,
	planner MotionPlanner,
	goals []spatial.Pose,
	seed []frame.Input,
	opts []*PlannerOptions,
	iter int,
) ([][]frame.Input, error) {
	var err error
	goal := goals[iter]
	opt := opts[iter]
	if opt == nil {
		opt = NewBasicPlannerOptions()
	}
	remainingSteps := [][]frame.Input{}
	if cbert, ok := planner.(*cBiRRTMotionPlanner); ok {
		// cBiRRT supports solution look-ahead for parallel waypoint solving
		// TODO(pl): other planners will support lookaheads, so this should be made to be an interface
		endpointPreview := make(chan *configuration, 1)
		solutionChan := make(chan *planReturn, 1)
		utils.PanicCapturingGo(func() {
			// TODO(rb) fix me
			cbert.planRunner(
				ctx,
				spatial.PoseToProtobuf(goal),
				seed,
				opt,
				endpointPreview,
				solutionChan,
			)
		})
		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			select {
			case nextSeed := <-endpointPreview:
				// Got a solution preview, start solving the next motion in a new thread.
				if iter+1 < len(goals) {
					// In this case, we create the next step (and thus the remaining steps) and the
					// step from our iteration hangs out in the channel buffer until we're done with it.
					remainingSteps, err = runPlannerWithWaypoints(ctx, planner, goals, nextSeed.inputs, opts, iter+1)
					if err != nil {
						return nil, err
					}
				}
				for {
					// Get the step from this runner invocation, and return everything in order.
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					default:
					}

					select {
					case finalSteps := <-solutionChan:
						if finalSteps.err != nil {
							return nil, finalSteps.err
						}
						results := make([][]frame.Input, 0, len(finalSteps.steps)+len(remainingSteps))
						for _, step := range finalSteps.steps {
							results = append(results, step.inputs)
						}
						results = append(results, remainingSteps...)
						return results, nil
					default:
					}
				}
			case finalSteps := <-solutionChan:
				// We didn't get a solution preview (possible error), so we get and process the full step set and error.
				if finalSteps.err != nil {
					return nil, finalSteps.err
				}
				if iter+1 < len(goals) {
					// in this case, we create the next step (and thus the remaining steps) and the
					// step from our iteration hangs out in the channel buffer until we're done with it
					remainingSteps, err = runPlannerWithWaypoints(ctx, planner, goals, finalSteps.steps[len(finalSteps.steps)-1].inputs, opts, iter+1)
					if err != nil {
						return nil, err
					}
				}
				results := make([][]frame.Input, 0, len(finalSteps.steps)+len(remainingSteps))
				for _, step := range finalSteps.steps {
					results = append(results, step.inputs)
				}
				results = append(results, remainingSteps...)
				return results, nil
			default:
			}
		}
	} else {
		resultSlicesRaw, err := planner.Plan(ctx, spatial.PoseToProtobuf(goal), seed, opt)
		if err != nil {
			return nil, err
		}
		if iter < len(goals)-2 {
			// in this case, we create the next step (and thus the remaining steps) and the
			// step from our iteration hangs out in the channel buffer until we're done with it
			remainingSteps, err = runPlannerWithWaypoints(ctx, planner, goals, resultSlicesRaw[len(resultSlicesRaw)-1], opts, iter+1)
			if err != nil {
				return nil, err
			}
		}
		return append(resultSlicesRaw, remainingSteps...), nil
	}
}

// GetSteps will determine the number of steps which should be used to get from the seed to the goal.
// The returned value is guaranteed to be at least 1.
// stepSize represents both the max mm movement per step, and max R4AA degrees per step.
func GetSteps(seedPos, goalPos spatial.Pose, stepSize float64) int {
	// use a default size of 1 if zero is passed in to avoid divide-by-zero
	if stepSize == 0 {
		stepSize = 1.
	}

	mmDist := seedPos.Point().Distance(goalPos.Point())
	rDist := spatial.OrientationBetween(seedPos.Orientation(), goalPos.Orientation()).AxisAngles()

	nSteps := math.Max(math.Abs(mmDist/stepSize), math.Abs(vutil.RadToDeg(rDist.Theta)/stepSize))
	return int(nSteps) + 1
}

// fixOvIncrement will detect whether the given goal position is a precise orientation increment of the current
// position, in which case it will detect whether we are leaving a pole. If we are an OV increment and leaving a pole,
// then Theta will be adjusted to give an expected smooth movement. The adjusted goal will be returned. Otherwise the
// original goal is returned.
// Rationale: if clicking the increment buttons in the interface, the user likely wants the most intuitive motion
// posible. If setting values manually, the user likely wants exactly what they requested.
func fixOvIncrement(pos, seed *commonpb.Pose) *commonpb.Pose {
	epsilon := 0.0001
	// Nothing to do for spatial translations or theta increments
	if pos.X != seed.X || pos.Y != seed.Y || pos.Z != seed.Z || pos.Theta != seed.Theta {
		return pos
	}
	// Check if seed is pointing directly at pole
	if 1-math.Abs(seed.OZ) > epsilon || pos.OZ != seed.OZ {
		return pos
	}

	// we only care about negative xInc
	xInc := pos.OX - seed.OX
	yInc := math.Abs(pos.OY - seed.OY)
	var adj float64
	if pos.OX == seed.OX {
		// no OX movement
		if yInc != 0.1 && yInc != 0.01 {
			// nonstandard increment
			return pos
		}
		// If wanting to point towards +Y and OZ<0, add 90 to theta, otherwise subtract 90
		if pos.OY-seed.OY > 0 {
			adj = 90
		} else {
			adj = -90
		}
	} else {
		if (xInc != -0.1 && xInc != -0.01) || pos.OY != seed.OY {
			return pos
		}
		// If wanting to point towards -X, increment by 180. Values over 180 or under -180 will be automatically wrapped
		adj = 180
	}
	if pos.OZ > 0 {
		adj *= -1
	}

	return &commonpb.Pose{
		X:     pos.X,
		Y:     pos.Y,
		Z:     pos.Z,
		Theta: pos.Theta + adj,
		OX:    pos.OX,
		OY:    pos.OY,
		OZ:    pos.OZ,
	}
}

// getSolutions will initiate an IK solver for the given position and seed, collect solutions, and score them by constraints.
// If maxSolutions is positive, once that many solutions have been collected, the solver will terminate and return that many solutions.
// If minScore is positive, if a solution scoring below that amount is found, the solver will terminate and return that one solution.
func getSolutions(ctx context.Context,
	opt *PlannerOptions,
	solver InverseKinematics,
	goal *commonpb.Pose,
	seed []frame.Input,
	f frame.Frame,
) ([][]frame.Input, error) {
	// Linter doesn't properly handle loop labels

	nSolutions := opt.MaxSolutions
	if nSolutions == 0 {
		nSolutions = defaultSolutionsToSeed
	}

	seedPos, err := f.Transform(seed)
	if err != nil {
		return nil, err
	}
	goalPos := spatial.NewPoseFromProtobuf(fixOvIncrement(goal, spatial.PoseToProtobuf(seedPos)))

	solutionGen := make(chan []frame.Input)
	ikErr := make(chan error, 1)
	defer func() { <-ikErr }()

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	// Spawn the IK solver to generate solutions until done
	utils.PanicCapturingGo(func() {
		defer close(ikErr)
		ikErr <- solver.Solve(ctxWithCancel, solutionGen, goalPos, seed, opt.metric)
	})

	solutions := map[float64][]frame.Input{}

	// Solve the IK solver. Loop labels are required because `break` etc in a `select` will break only the `select`.
IK:
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		select {
		case step := <-solutionGen:
			cPass, cScore := opt.CheckConstraints(&ConstraintInput{
				seedPos,
				goalPos,
				seed,
				step,
				f,
			})
			endPass, _ := opt.CheckConstraints(&ConstraintInput{
				goalPos,
				goalPos,
				step,
				step,
				f,
			})

			if cPass && endPass {
				if cScore < opt.MinScore && opt.MinScore > 0 {
					solutions = map[float64][]frame.Input{}
					solutions[cScore] = step
					// good solution, stopping early
					break IK
				}

				solutions[cScore] = step
				if len(solutions) >= nSolutions {
					// sufficient solutions found, stopping early
					break IK
				}
			}
			// Skip the return check below until we have nothing left to read from solutionGen
			continue IK
		default:
		}

		select {
		case <-ikErr:
			// If we have a return from the IK solver, there are no more solutions, so we finish processing above
			// until we've drained the channel
			break IK
		default:
		}
	}
	if len(solutions) == 0 {
		return nil, errors.New("unable to solve for position")
	}

	keys := make([]float64, 0, len(solutions))
	for k := range solutions {
		keys = append(keys, k)
	}
	sort.Float64s(keys)

	orderedSolutions := make([][]frame.Input, 0)
	for _, key := range keys {
		orderedSolutions = append(orderedSolutions, solutions[key])
	}
	return orderedSolutions, nil
}

func inputDist(from, to []frame.Input) float64 {
	dist := 0.
	for i, f := range from {
		dist += math.Pow(to[i].Value-f.Value, 2)
	}
	return dist
}
