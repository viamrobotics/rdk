// Package motionplan is a motion planning library.
package motionplan

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math"
	"sort"

	"go.viam.com/utils"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	vutil "go.viam.com/rdk/utils"
)

const (
	// When setting default constraints, the translation and orientation distances between the start/end are calculated and multiplied by
	// this value. At no point during a movement may the minimum distance to the start or end exceed these values.
	deviationFactor = 1.0
	// Default distance below which two distances are considered equal.
	defaultEpsilon = 0.001
	// Default motion constraint name.
	defaultMotionConstraint = "defaultMotionConstraint"
	// Solve for waypoints this far apart to speed solving.
	pathStepSize = 10.0
)

// MotionPlanner provides an interface to path planning methods, providing ways to request a path to be planned, and
// management of the constraints used to plan paths.
type MotionPlanner interface {
	// Plan will take a context, a goal position, and an input start state and return a series of state waypoints which
	// should be visited in order to arrive at the goal while satisfying all constraints
	Plan(context.Context, *commonpb.Pose, []frame.Input, *PlannerOptions) ([][]frame.Input, error)
	Resolution() float64 // Resolution specifies how narrowly to check for constraints
	Frame() frame.Frame  // Frame will return the frame used for planning
}

// needed to wrap slices so we can use them as map keys.
type node struct {
	q    []frame.Input
	cost float64
}

type nodePair struct{ a, b *node }

type planReturn struct {
	steps []*node
	err   error
}

// PlannerOptions are a set of options to be passed to a planner which will specify how to solve a motion planning problem.
type PlannerOptions struct {
	constraintHandler
	metric   Metric
	pathDist Metric
	// For the below values, if left uninitialized, default values will be used. To disable, set < 0
	// Max number of ik solutions to consider
	maxSolutions int
	// Movements that score below this amount are considered "good enough" and returned immediately
	minScore float64
}

// NewDefaultPlannerOptions specifies a set of default options for the planner.
func NewDefaultPlannerOptions() *PlannerOptions {
	opt := &PlannerOptions{}
	opt.AddConstraint(jointConstraint, NewJointConstraint(math.Inf(1)))
	opt.metric = NewSquaredNormMetric()
	opt.pathDist = NewSquaredNormMetric()
	return opt
}

// DefaultConstraint creates a default constraint and metric that constrains the position and orientation. The allowed magnitude of
// deviation of the position and orientation from the start or goal shall never be greater than than the magnitude of deviation between
// the start and goal poses.
// For example- if a user requests a translation, orientation will not change during the movement. If there is an obstacle, deflection
// from the ideal path is allowed as a function of the length of the ideal path.
func DefaultConstraint(
	from, to spatial.Pose,
	f frame.Frame,
	opt *PlannerOptions,
) *PlannerOptions {
	pathDist := newDefaultMetric(from, to)

	validFunc := func(cInput *ConstraintInput) (bool, float64) {
		err := resolveInputsToPositions(cInput)
		if err != nil {
			return false, 0
		}
		dist := pathDist(cInput.StartPos, cInput.EndPos)
		if dist < defaultEpsilon*defaultEpsilon {
			return true, 0
		}
		return false, dist
	}
	opt.pathDist = pathDist
	opt.AddConstraint(defaultMotionConstraint, validFunc)

	// Add self-collision check if available
	collisionConst := NewCollisionConstraint(f, map[string]spatial.Geometry{}, map[string]spatial.Geometry{})
	opt.AddConstraint("self-collision", collisionConst)
	return opt
}

// SetMetric sets the distance metric for the solver.
func (p *PlannerOptions) SetMetric(m Metric) {
	p.metric = m
}

// SetPathDist sets the distance metric for the solver to move a constraint-violating point into a valid manifold.
func (p *PlannerOptions) SetPathDist(m Metric) {
	p.pathDist = m
}

// SetMaxSolutions sets the maximum number of IK solutions to generate for the planner.
func (p *PlannerOptions) SetMaxSolutions(maxSolutions int) {
	p.maxSolutions = maxSolutions
}

// SetMinScore specifies the IK stopping score for the planner.
func (p *PlannerOptions) SetMinScore(minScore float64) {
	p.minScore = minScore
}

// Clone makes a deep copy of the PlannerOptions.
func (p *PlannerOptions) Clone() *PlannerOptions {
	opt := &PlannerOptions{}
	opt.constraints = p.constraints
	opt.metric = p.metric
	opt.pathDist = p.pathDist
	opt.maxSolutions = p.maxSolutions
	opt.minScore = p.minScore

	return opt
}

// RunPlannerWithWaypoints will plan to each of a list of goals in oder, optionally also taking a new planner option for each goal.
func RunPlannerWithWaypoints(ctx context.Context,
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
		opt = NewDefaultPlannerOptions()
	}
	remainingSteps := [][]frame.Input{}
	if cbert, ok := planner.(*cBiRRTMotionPlanner); ok {
		// cBiRRT supports solution look-ahead for parallel waypoint solving
		endpointPreview := make(chan *node, 1)
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
					remainingSteps, err = RunPlannerWithWaypoints(ctx, planner, goals, nextSeed.q, opts, iter+1)
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
							results = append(results, step.q)
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
					remainingSteps, err = RunPlannerWithWaypoints(
						ctx,
						planner,
						goals,
						finalSteps.steps[len(finalSteps.steps)-1].q,
						opts,
						iter+1,
					)
					if err != nil {
						return nil, err
					}
				}
				results := make([][]frame.Input, 0, len(finalSteps.steps)+len(remainingSteps))
				for _, step := range finalSteps.steps {
					results = append(results, step.q)
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
			remainingSteps, err = RunPlannerWithWaypoints(ctx, planner, goals, resultSlicesRaw[len(resultSlicesRaw)-1], opts, iter+1)
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
	nSolutions := opt.maxSolutions
	if nSolutions == 0 {
		nSolutions = solutionsToSeed
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
				if cScore < opt.minScore && opt.minScore > 0 {
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
		return nil, NewIKError()
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

func NewIKError() error {
	return errors.New("unable to solve for position")
}

func NewPlannerFailedError() error {
	return errors.New("motion planner failed to find path")
}

func extractPath(startMap, goalMap map[*node]*node, pair *nodePair) []*node {
	// need to figure out which of the two nodes is in the start map
	var startReached, goalReached *node
	if _, ok := startMap[pair.a]; ok {
		startReached, goalReached = pair.a, pair.b
	} else {
		startReached, goalReached = pair.b, pair.a
	}

	// extract the path to the seed
	path := []*node{}
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

func shortestPath(startMap, goalMap map[*node]*node, nodePairs []*nodePair) *planReturn {
	if len(nodePairs) == 0 {
		return &planReturn{err: NewPlannerFailedError()}
	}
	pairCost := func(pair *nodePair) float64 {
		return pair.a.cost + pair.b.cost
	}
	minIdx := 0
	minDist := pairCost(nodePairs[0])
	for i := 1; i < len(nodePairs); i++ {
		if dist := pairCost(nodePairs[i]); dist < minDist {
			minDist = dist
			minIdx = i
		}
	}
	exportMaps(startMap, goalMap)
	return &planReturn{steps: extractPath(startMap, goalMap, nodePairs[minIdx])}
}

// func shortestPath(startMap, goalMap map[*node]*node, nodePairs []*nodePair) *planReturn {
// 	if len(nodePairs) == 0 {
// 		return &planReturn{err: NewPlannerFailedError()}
// 	}
// 	pairCost := func(pair *nodePair) float64 {
// 		return pair.a.cost + pair.b.cost
// 	}
// 	minPath := []*node{}
// 	minDist := pairCost(nodePairs[0])
// 	for i := 0; i < len(nodePairs); i++ {
// 		path := extractPath(startMap, goalMap, nodePairs[i])
// 		dist := evaluatePlanNodes(path)
// 		if dist < minDist {
// 			minDist = dist
// 			minPath = path
// 		}
// 	}
// 	exportMaps(startMap, goalMap)
// 	return &planReturn{steps: minPath}
// }

func evaluatePlanNodes(path []*node) (cost float64) {
	if len(path) < 2 {
		return math.Inf(1)
	}
	for i := 0; i < len(path)-1; i++ {
		cost += inputDist(path[i].q, path[i+1].q)
	}
	return cost
}

func evaluatePlan(path [][]frame.Input) (cost float64) {
	for i := 0; i < len(path)-1; i++ {
		cost += inputDist(path[i], path[i+1])
	}
	return cost
}

func inputDist(from, to []frame.Input) float64 {
	dist := 0.
	for i, f := range from {
		dist += math.Pow(to[i].Value-f.Value, 2)
	}
	// TODO(rb): its inefficient to return the sqrt here.... take this out before the PR goes through
	return math.Sqrt(dist)
}

func exportMaps(m1, m2 map[*node]*node) {
	outputList := make([][]frame.Input, 0)
	for key, value := range m1 {
		if value != nil {
			outputList = append(outputList, key.q, value.q)
		}
	}
	for key, value := range m2 {
		if value != nil {
			outputList = append(outputList, key.q, value.q)
		}
	}
	writeJSONFile(vutil.ResolveFile("motionplan/tree.test"), [][][]frame.Input{outputList})
}

func writeJSONFile(filename string, data interface{}) error {
	bytes, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filename, bytes, 0o644); err != nil {
		return err
	}
	return nil
}
