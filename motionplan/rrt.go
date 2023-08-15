package motionplan

import (
	"context"
	"math"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	// Number of planner iterations before giving up.
	defaultPlanIter = 20000
)

type rrtParallelPlanner interface {
	motionPlanner
	rrtBackgroundRunner(context.Context, []referenceframe.Input, *rrtParallelPlannerShared)
}

type rrtParallelPlannerShared struct {
	maps            *rrtMaps
	endpointPreview chan node
	solutionChan    chan *rrtPlanReturn
}

type rrtOptions struct {
	// Number of planner iterations before giving up.
	PlanIter int `json:"plan_iter"`
}

func newRRTOptions() *rrtOptions {
	return &rrtOptions{
		PlanIter: defaultPlanIter,
	}
}

type rrtMap map[node]node

type rrtPlanReturn struct {
	steps   []node
	planerr error
	maps    *rrtMaps
}

func nodesToInputs(nodes []node) [][]referenceframe.Input {
	inputs := make([][]referenceframe.Input, 0, len(nodes))
	for _, step := range nodes {
		inputs = append(inputs, step.Q())
	}
	return inputs
}

func (plan *rrtPlanReturn) err() error {
	return plan.planerr
}

type rrtMaps struct {
	startMap rrtMap
	goalMap  rrtMap
	optNode  node // The highest quality IK solution
}

// initRRTsolutions will create the maps to be used by a RRT-based algorithm. It will generate IK solutions to pre-populate the goal
// map, and will check if any of those goals are able to be directly interpolated to.
func initRRTSolutions(ctx context.Context, mp motionPlanner, seed []referenceframe.Input) *rrtPlanReturn {
	rrt := &rrtPlanReturn{
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
		rrt.planerr = err
		return rrt
	}

	// the smallest interpolated distance between the start and end input represents a lower bound on cost
	optimalCost := mp.opt().DistanceFunc(&Segment{StartConfiguration: seed, EndConfiguration: solutions[0].Q()})
	rrt.maps.optNode = &basicNode{q: solutions[0].Q(), cost: optimalCost}

	// Check for direct interpolation for the subset of IK solutions within some multiple of optimal
	// Since solutions are returned ordered, we check until one is out of bounds, then skip remaining checks
	canInterp := true
	// initialize maps and check whether direct interpolation is an option
	for _, solution := range solutions {
		if canInterp {
			cost := mp.opt().DistanceFunc(&Segment{StartConfiguration: seed, EndConfiguration: solution.Q()})
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

func shortestPath(maps *rrtMaps, nodePairs []*nodePair) *rrtPlanReturn {
	if len(nodePairs) == 0 {
		return &rrtPlanReturn{planerr: errPlannerFailed, maps: maps}
	}
	minIdx := 0
	minDist := nodePairs[0].sumCosts()
	for i := 1; i < len(nodePairs); i++ {
		if dist := nodePairs[i].sumCosts(); dist < minDist {
			minDist = dist
			minIdx = i
		}
	}
	return &rrtPlanReturn{steps: extractPath(maps.startMap, maps.goalMap, nodePairs[minIdx], true), maps: maps}
}

// node interface is used to wrap a configuration for planning purposes.
// TODO: This is somewhat redundant with a State.
type node interface {
	// return the configuration associated with the node
	Q() []referenceframe.Input
	Cost() float64
	SetCost(float64)
	Pose() spatialmath.Pose
	Corner() bool
	SetCorner(bool)
}

type basicNode struct {
	q      []referenceframe.Input
	cost   float64
	pose   spatialmath.Pose
	corner bool
}

// Special case constructors for nodes without costs to return NaN.
func newConfigurationNode(q []referenceframe.Input) node {
	return &basicNode{
		q:      q,
		cost:   math.NaN(),
		corner: false,
	}
}

func (n *basicNode) Q() []referenceframe.Input {
	return n.q
}

func (n *basicNode) Cost() float64 {
	return n.cost
}

func (n *basicNode) SetCost(cost float64) {
	n.cost = cost
}

func (n *basicNode) Pose() spatialmath.Pose {
	return n.pose
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
			// special rewriting poses for inverted tree
			if lastNode, ok :=path[len(path)-1].(*basicNode); ok {
				lastNode.pose = goalReached.Pose()
			}
			
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
