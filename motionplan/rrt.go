//go:build !no_cgo

package motionplan

import (
	"context"
	"errors"
	"math"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	// Number of planner iterations before giving up.
	defaultPlanIter = 3000

	// The maximum percent of a joints range of motion to allow per step.
	defaultFrameStep = 0.01

	// If the dot product between two sets of configurations is less than this, consider them identical.
	defaultInputIdentDist = 0.0001

	// Number of iterations to run before beginning to accept randomly seeded locations.
	defaultIterBeforeRand = 50
)

type rrtParallelPlanner interface {
	motionPlanner
	rrtBackgroundRunner(context.Context, map[string][]referenceframe.Input, *rrtParallelPlannerShared)
}

type rrtParallelPlannerShared struct {
	maps            *rrtMaps
	endpointPreview chan node
	solutionChan    chan *rrtSolution
}

type rrtMap map[node]node

type rrtSolution struct {
	steps []node
	err   error
	maps  *rrtMaps
}

type rrtMaps struct {
	startMap rrtMap
	goalMap  rrtMap
	optNode  node // The highest quality IK solution
}

func (maps *rrtMaps) fillPosOnlyGoal(goal PathStep, posSeeds int) error {
	thetaStep := 360. / float64(posSeeds)
	if maps == nil {
		return errors.New("cannot call method fillPosOnlyGoal on nil maps")
	}
	if maps.goalMap == nil {
		maps.goalMap = map[node]node{}
	}
	for i := 0; i < posSeeds; i++ {
		newMap := PathStep{}
		for frame, goal := range goal {
			newMap[frame] = referenceframe.NewPoseInFrame(
				frame,
				spatialmath.NewPose(goal.Pose().Point(), &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: float64(i) * thetaStep}),
			)
		}

		goalNode := &basicNode{
			q:     make(map[string][]referenceframe.Input),
			poses: newMap,
		}
		maps.goalMap[goalNode] = nil
	}
	return nil
}

// initRRTsolutions will create the maps to be used by a RRT-based algorithm. It will generate IK solutions to pre-populate the goal
// map, and will check if any of those goals are able to be directly interpolated to.
func initRRTSolutions(ctx context.Context, mp motionPlanner, seed map[string][]referenceframe.Input) *rrtSolution {
	rrt := &rrtSolution{
		maps: &rrtMaps{
			startMap: map[node]node{},
			goalMap:  map[node]node{},
		},
	}
	seedNode := &basicNode{q: seed, cost: 0}
	rrt.maps.startMap[seedNode] = nil

	// get many potential end goals from IK solver
	solutions, err := mp.getSolutions(ctx, seed)
	if err != nil {
		rrt.err = err
		return rrt
	}

	// the smallest interpolated distance between the start and end input represents a lower bound on cost
	optimalCost := mp.opt().confDistanceFunc(&ik.SegmentFS{StartConfiguration: seed, EndConfiguration: solutions[0].Q()})
	rrt.maps.optNode = &basicNode{q: solutions[0].Q(), cost: optimalCost}

	// Check for direct interpolation for the subset of IK solutions within some multiple of optimal
	// Since solutions are returned ordered, we check until one is out of bounds, then skip remaining checks
	canInterp := true
	// initialize maps and check whether direct interpolation is an option
	for _, solution := range solutions {
		if canInterp {
			cost := mp.opt().confDistanceFunc(&ik.SegmentFS{StartConfiguration: seed, EndConfiguration: solution.Q()})
			if cost < optimalCost*defaultOptimalityMultiple {
				if mp.checkPath(seed, solution.Q()) {
					rrt.steps = []node{seedNode, solution}
					return rrt
				}
			} else {
				canInterp = false
			}
		}
		rrt.maps.goalMap[&basicNode{q: solution.Q(), cost: 0}] = nil
	}
	return rrt
}

func shortestPath(maps *rrtMaps, nodePairs []*nodePair) *rrtSolution {
	if len(nodePairs) == 0 {
		return &rrtSolution{err: errPlannerFailed, maps: maps}
	}
	minIdx := 0
	minDist := nodePairs[0].sumCosts()
	for i := 1; i < len(nodePairs); i++ {
		if dist := nodePairs[i].sumCosts(); dist < minDist {
			minDist = dist
			minIdx = i
		}
	}
	return &rrtSolution{steps: extractPath(maps.startMap, maps.goalMap, nodePairs[minIdx], true), maps: maps}
}

// fixedStepInterpolation returns inputs at qstep distance along the path from start to target
// if start and target have the same Input value, then no step increment is made.
func fixedStepInterpolation(start, target node, qstep map[string][]float64) map[string][]referenceframe.Input {
	newNear := make(map[string][]referenceframe.Input)

	// Iterate through each frame's inputs
	for frameName, startInputs := range start.Q() {
		targetInputs := target.Q()[frameName]
		frameSteps := make([]referenceframe.Input, len(startInputs))

		for j, nearInput := range startInputs {
			if nearInput.Value == targetInputs[j].Value {
				frameSteps[j] = nearInput
			} else {
				v1, v2 := nearInput.Value, targetInputs[j].Value
				newVal := math.Min(qstep[frameName][j], math.Abs(v2-v1))
				// get correct sign
				newVal *= (v2 - v1) / math.Abs(v2-v1)
				frameSteps[j] = referenceframe.Input{Value: nearInput.Value + newVal}
			}
		}
		newNear[frameName] = frameSteps
	}
	return newNear
}

// node interface is used to wrap a configuration for planning purposes.
// TODO: This is somewhat redundant with a State.
type node interface {
	// return the configuration associated with the node
	Q() map[string][]referenceframe.Input
	Cost() float64
	SetCost(float64)
	Poses() PathStep
	Corner() bool
	SetCorner(bool)
}

type basicNode struct {
	q      map[string][]referenceframe.Input
	cost   float64
	poses  PathStep
	corner bool
}

// Special case constructors for nodes without costs to return NaN.
func newConfigurationNode(q map[string][]referenceframe.Input) node {
	return &basicNode{
		q:      q,
		cost:   math.NaN(),
		corner: false,
	}
}

func (n *basicNode) Q() map[string][]referenceframe.Input {
	return n.q
}

func (n *basicNode) Cost() float64 {
	return n.cost
}

func (n *basicNode) SetCost(cost float64) {
	n.cost = cost
}

func (n *basicNode) Poses() PathStep {
	return n.poses
}

func (n *basicNode) Corner() bool {
	return n.corner
}

func (n *basicNode) SetCorner(corner bool) {
	n.corner = corner
}

// nodePair groups together nodes in a tuple
// TODO(rb): in the future we might think about making this into a list of nodes.
type nodePair struct{ a, b node }

func (np *nodePair) sumCosts() float64 {
	aCost := np.a.Cost()
	if math.IsNaN(aCost) {
		return 0
	}
	bCost := np.b.Cost()
	if math.IsNaN(bCost) {
		return 0
	}
	return aCost + bCost
}

func extractPath(startMap, goalMap map[node]node, pair *nodePair, matched bool) []node {
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

	if goalReached != nil {
		if matched {
			// skip goalReached node and go directly to its parent in order to not repeat this node
			goalReached = goalMap[goalReached]
		}

		// extract the path to the goal
		for goalReached != nil {
			path = append(path, goalReached)
			goalReached = goalMap[goalReached]
		}
	}
	return path
}

func sumCosts(path []node) float64 {
	cost := 0.
	for _, wp := range path {
		cost += wp.Cost()
	}
	return cost
}

type rrtPlan struct {
	SimplePlan

	// nodes corresponding to inputs can be cached with the Plan for easy conversion back into a form usable by RRT
	// depending on how the trajectory is constructed these may be nil and should be computed before usage
	nodes []node
}

func newRRTPlan(solution []node, fss referenceframe.FrameSystem, relative bool, offsetPose spatialmath.Pose) (Plan, error) {
	if len(solution) < 2 {
		if len(solution) == 1 {
			// Started at the goal, nothing to do
			solution = append(solution, solution[0])
		} else {
			return nil, errors.New("cannot construct a Plan using fewer than two nodes")
		}
	}
	traj := nodesToTrajectory(solution)
	path, err := newPath(solution, fss)
	if err != nil {
		return nil, err
	}
	if relative {
		path, err = newPathFromRelativePath(path)
		if err != nil {
			return nil, err
		}
	}
	var plan Plan
	plan = &rrtPlan{SimplePlan: *NewSimplePlan(path, traj), nodes: solution}
	if relative {
		// TODO: This works currently because relative plans can only have one
		plan = OffsetPlan(plan, offsetPose)
	}
	return plan, nil
}
