// Package motionplan is a motion planning library.
package motionplan

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/utils"

	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/spatialmath"
	vutil "go.viam.com/rdk/utils"
)

// motionPlanner provides an interface to path planning methods, providing ways to request a path to be planned, and
// management of the constraints used to plan paths.
type motionPlanner interface {
	// Plan will take a context, a goal position, and an input start state and return a series of state waypoints which
	// should be visited in order to arrive at the goal while satisfying all constraints
	Plan(context.Context, spatialmath.Pose, []frame.Input, *plannerOptions) ([][]frame.Input, error)
	Frame() frame.Frame // Frame will return the frame used for planning
}

type seededPlannerConstructor func(frame frame.Frame, nCPU int, seed *rand.Rand, logger golog.Logger) (motionPlanner, error)

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

// PlanFrameMotion plans a motion to destination for a given frame with no frame system. It will create a new FS just for the plan.
// WorldState is not supported in the absence of a real frame system.
func PlanFrameMotion(ctx context.Context,
	logger golog.Logger,
	dst spatialmath.Pose,
	f frame.Frame,
	seed []frame.Input,
	planningOpts map[string]interface{},
) ([][]frame.Input, error) {
	// ephemerally create a framesystem containing just the frame for the solve
	fs := frame.NewEmptySimpleFrameSystem("")
	err := fs.AddFrame(f, fs.World())
	if err != nil {
		return nil, err
	}
	destination := frame.NewPoseInFrame(frame.World, dst)
	seedMap := map[string][]frame.Input{f.Name(): seed}
	solutionMap, err := PlanWaypoints(
		ctx,
		logger,
		[]*frame.PoseInFrame{destination},
		f,
		seedMap,
		fs,
		nil,
		[]map[string]interface{}{planningOpts},
	)
	if err != nil {
		return nil, err
	}
	return FrameStepsFromRobotPath(f.Name(), solutionMap)
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

// FrameStepsFromRobotPath is a helper function which will extract the waypoints of a single frame from the map output of a robot path.
func FrameStepsFromRobotPath(frameName string, path []map[string][]frame.Input) ([][]frame.Input, error) {
	solution := make([][]frame.Input, 0, len(path))
	for _, step := range path {
		frameStep, ok := step[frameName]
		if !ok {
			return nil, fmt.Errorf("frame named %s not found in solved motion path", frameName)
		}
		solution = append(solution, frameStep)
	}
	return solution, nil
}

type planner struct {
	solver   InverseKinematics
	frame    frame.Frame
	logger   golog.Logger
	randseed *rand.Rand
	start    time.Time
}

func newPlanner(frame frame.Frame, nCPU int, seed *rand.Rand, logger golog.Logger) (*planner, error) {
	ik, err := CreateCombinedIKSolver(frame, logger, nCPU)
	if err != nil {
		return nil, err
	}
	mp := &planner{
		solver:   ik,
		frame:    frame,
		logger:   logger,
		randseed: seed,
	}
	return mp, nil
}

func (mp *planner) Frame() frame.Frame {
	return mp.frame
}

func (mp *planner) checkInputs(planOpts *plannerOptions, inputs []frame.Input) bool {
	frame := mp.Frame()
	position, err := frame.Transform(inputs)
	if err != nil {
		return false
	}
	ok, _ := planOpts.CheckConstraints(&ConstraintInput{
		StartPos:   position,
		EndPos:     position,
		StartInput: inputs,
		EndInput:   inputs,
		Frame:      frame,
	})
	return ok
}

func (mp *planner) checkPath(planOpts *plannerOptions, seedInputs, target []frame.Input) bool {
	ok, _ := planOpts.CheckConstraintPath(
		&ConstraintInput{
			StartInput: seedInputs,
			EndInput:   target,
			Frame:      mp.Frame(),
		},
		planOpts.Resolution,
	)
	return ok
}

// node interface is used to wrap a configuration for planning purposes.
type node interface {
	// return the configuration associated with the node
	Q() []frame.Input
}

type planReturn interface {
	// return the steps in Input form
	toInputs() [][]frame.Input
	err() error
}

type basicNode struct {
	q []frame.Input
}

func (n *basicNode) Q() []frame.Input {
	return n.q
}

type costNode struct {
	node
	cost float64
}

func newCostNode(q []frame.Input, cost float64) *costNode {
	return &costNode{&basicNode{q: q}, cost}
}

// nodePair groups together nodes in a tuple
// TODO(rb): in the future we might think about making this into a list of nodes.
type nodePair struct{ a, b node }

func (np *nodePair) sumCosts() float64 {
	a, aok := np.a.(*costNode)
	if !aok {
		return 0
	}
	b, bok := np.b.(*costNode)
	if !bok {
		return 0
	}
	return a.cost + b.cost
}

// EvaluatePlan assigns a numeric score to a plan that corresponds to the cumulative distance between input waypoints in the plan.
func EvaluatePlan(plan planReturn, planOpts *plannerOptions) (totalCost float64) {
	if errors.Is(plan.err(), errPlannerFailed) {
		return math.Inf(1)
	}
	steps := plan.toInputs()
	for i := 0; i < len(steps)-1; i++ {
		_, cost := planOpts.DistanceFunc(&ConstraintInput{StartInput: steps[i], EndInput: steps[i+1]})
		totalCost += cost
	}
	return totalCost
}

// GetSteps will determine the number of steps which should be used to get from the seed to the goal.
// The returned value is guaranteed to be at least 1.
// stepSize represents both the max mm movement per step, and max R4AA degrees per step.
func GetSteps(seedPos, goalPos spatialmath.Pose, stepSize float64) int {
	// use a default size of 1 if zero is passed in to avoid divide-by-zero
	if stepSize == 0 {
		stepSize = 1.
	}

	mmDist := seedPos.Point().Distance(goalPos.Point())
	rDist := spatialmath.OrientationBetween(seedPos.Orientation(), goalPos.Orientation()).AxisAngles()

	nSteps := math.Max(math.Abs(mmDist/stepSize), math.Abs(vutil.RadToDeg(rDist.Theta)/stepSize))
	return int(nSteps) + 1
}

// getSolutions will initiate an IK solver for the given position and seed, collect solutions, and score them by constraints.
// If maxSolutions is positive, once that many solutions have been collected, the solver will terminate and return that many solutions.
// If minScore is positive, if a solution scoring below that amount is found, the solver will terminate and return that one solution.
func getSolutions(ctx context.Context,
	planOpts *plannerOptions,
	solver InverseKinematics,
	goal spatialmath.Pose,
	seed []frame.Input,
	f frame.Frame,
	rseed int,
) ([]*costNode, error) {
	// Linter doesn't properly handle loop labels
	nSolutions := planOpts.MaxSolutions
	if nSolutions == 0 {
		nSolutions = defaultSolutionsToSeed
	}

	seedPos, err := f.Transform(seed)
	if err != nil {
		return nil, err
	}
	goalPos := fixOvIncrement(goal, seedPos)

	solutionGen := make(chan []frame.Input)
	ikErr := make(chan error, 1)
	defer func() { <-ikErr }()

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	// Spawn the IK solver to generate solutions until done
	utils.PanicCapturingGo(func() {
		defer close(ikErr)
		ikErr <- solver.Solve(ctxWithCancel, solutionGen, goalPos, seed, planOpts.metric, rseed)
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
			cPass, cScore := planOpts.CheckConstraints(&ConstraintInput{
				seedPos,
				goalPos,
				seed,
				step,
				f,
			})
			endPass, _ := planOpts.CheckConstraints(&ConstraintInput{
				goalPos,
				goalPos,
				step,
				step,
				f,
			})

			if cPass && endPass {
				if cScore < planOpts.MinScore && planOpts.MinScore > 0 {
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
		return nil, errIKSolve
	}

	keys := make([]float64, 0, len(solutions))
	for k := range solutions {
		keys = append(keys, k)
	}
	sort.Float64s(keys)

	orderedSolutions := make([]*costNode, 0)
	for _, key := range keys {
		orderedSolutions = append(orderedSolutions, newCostNode(solutions[key], key))
	}
	return orderedSolutions, nil
}

func extractPath(startMap, goalMap map[node]node, pair *nodePair) []node {
	// need to figure out which of the two nodes is in the start map
	var startReached, goalReached node
	if _, ok := startMap[pair.a]; ok {
		startReached, goalReached = pair.a, pair.b
	} else {
		startReached, goalReached = pair.b, pair.a
	}

	// extract the path to the seed
	path := make([]node, 0)
	for startReached != nil {
		path = append(path, startReached)
		startReached = startMap[startReached]
	}

	// reverse the slice
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	// skip goalReached node and go directly to its parent in order to not repeat this node
	goalReached = goalMap[goalReached]

	// extract the path to the goal
	for goalReached != nil {
		path = append(path, goalReached)
		goalReached = goalMap[goalReached]
	}
	return path
}

