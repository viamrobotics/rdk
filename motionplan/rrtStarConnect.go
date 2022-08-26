package motionplan

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
)

const (
	// If a solution is found that is within this percentage of the optimal unconstrained solution, exit early
	defaultOptimalityThreshold = .95

	// The number of nearest neighbors to consider when adding a new sample to the tree
	defaultNeighborhoodSize = 10
)

type rrtStarConnectOptions struct {
	// If a solution is found that is within this percentage of the optimal unconstrained solution, exit early
	OptimalityThreshold float64 `json:"optimality_threshold"`

	// The number of nearest neighbors to consider when adding a new sample to the tree
	NeighborhoodSize int `json:"neighborhood_size"`

	// Parameters common to all RRT implementations
	*rrtOptions
}

// newRRTStarConnectOptions creates a struct controlling the running of a single invocation of the algorithm.
// All values are pre-set to reasonable defaults, but can be tweaked if needed.
func newRRTStarConnectOptions(planOpts *PlannerOptions) (*rrtStarConnectOptions, error) {
	algOpts := &rrtStarConnectOptions{
		OptimalityThreshold: defaultOptimalityThreshold,
		NeighborhoodSize:    defaultNeighborhoodSize,
		rrtOptions:          newRRTOptions(planOpts),
	}
	// convert map to json
	jsonString, err := json.Marshal(planOpts.extra)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonString, algOpts)
	if err != nil {
		return nil, err
	}
	return algOpts, nil
}

type rrtStarConnectMotionPlanner struct {
	solver   InverseKinematics
	frame    referenceframe.Frame
	logger   golog.Logger
	nCPU     int
	randseed *rand.Rand
}

// TODO(rb): find a reasonable default for this
// neighborhoodSize represents the number of neighbors to find in a k-nearest neighbors search
const neighborhoodSize = 10

// NewRRTStarConnectMotionPlanner creates a rrtStarConnectMotionPlanner object.
func NewRRTStarConnectMotionPlanner(frame referenceframe.Frame, nCPU int, logger golog.Logger) (MotionPlanner, error) {
	//nolint:gosec
	return NewRRTStarConnectMotionPlannerWithSeed(frame, nCPU, rand.New(rand.NewSource(1)), logger)
}

// NewRRTStarConnectMotionPlannerWithSeed creates a rrtStarConnectMotionPlanner object with a user specified random seed.
func NewRRTStarConnectMotionPlannerWithSeed(
	frame referenceframe.Frame,
	nCPU int,
	seed *rand.Rand,
	logger golog.Logger,
) (MotionPlanner, error) {
	ik, err := CreateCombinedIKSolver(frame, logger, nCPU)
	if err != nil {
		return nil, err
	}
	return &rrtStarConnectMotionPlanner{
		solver:   ik,
		frame:    frame,
		logger:   logger,
		nCPU:     nCPU,
		randseed: seed,
	}, nil
}

func (mp *rrtStarConnectMotionPlanner) Frame() referenceframe.Frame {
	return mp.frame
}

func (mp *rrtStarConnectMotionPlanner) Plan(ctx context.Context,
	goal *commonpb.Pose,
	seed []referenceframe.Input,
	planOpts *PlannerOptions,
) ([][]referenceframe.Input, error) {
	if planOpts == nil {
		planOpts = NewBasicPlannerOptions()
	}
	solutionChan := make(chan *planReturn, 1)
	utils.PanicCapturingGo(func() {
		mp.planRunner(ctx, goal, seed, planOpts, nil, solutionChan)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case plan := <-solutionChan:
		return plan.toInputs(), plan.err
	}
}

// planRunner will execute the plan. When Plan() is called, it will call planRunner in a separate thread and wait for the results.
// Separating this allows other things to call planRunner in parallel while also enabling the thread-agnostic Plan to be accessible.
func (mp *rrtStarConnectMotionPlanner) planRunner(ctx context.Context,
	goal *commonpb.Pose,
	seed []referenceframe.Input,
	planOpts *PlannerOptions,
	endpointPreview chan *node,
	solutionChan chan *planReturn,
) {
	defer close(solutionChan)

	// setup planner options
	if planOpts == nil {
		planOpts = NewBasicPlannerOptions()
	}
	algOpts, err := newRRTStarConnectOptions(planOpts)
	if err != nil {
		solutionChan <- &planReturn{err: err}
		return
	}

	// get many potential end goals from IK solver
	solutions, err := getSolutions(ctx, planOpts, mp.solver, goal, seed, mp.Frame())
	if err != nil {
		solutionChan <- &planReturn{err: err}
		return
	}

	// publish endpoint of plan if it is known
	if planOpts.MaxSolutions == 1 && endpointPreview != nil {
		endpointPreview <- &node{q: solutions[0]}
	}

	// initialize maps
	goalMap := make(map[*node]*node, len(solutions))
	for _, solution := range solutions {
		goalMap[&node{q: solution}] = nil
	}
	startMap := make(map[*node]*node)
	startMap[&node{q: seed}] = nil

	// for the first iteration, we try the 0.5 interpolation between seed and goal[0]
	target := referenceframe.InterpolateInputs(seed, solutions[0], 0.5)

	// Create a reference to the two maps so that we can alternate which one is grown
	map1, map2 := startMap, goalMap

	// Keep a list of the node pairs that have the same inputs
	shared := make([]*nodePair, 0)

	// Number of iterations after which a log will be printed
	logIteration := int(float64(algOpts.PlanIter) * planOpts.LoggingInterval)

	// sample until the max number of iterations is reached
	for i := 1; i <= algOpts.PlanIter; i++ {
		select {
		case <-ctx.Done():
			solutionChan <- &planReturn{err: ctx.Err()}
			return
		default:
		}

		// try to connect the target to map 1
		if map1reached := mp.extend(planOpts, map1, target); map1reached != nil {
			// try to connect the target to map 2
			if map2reached := mp.extend(planOpts, map2, target); map2reached != nil {
				// target was added to both map
				shared = append(shared, &nodePair{map1reached, map2reached})
			}
		}

		// get next sample, switch map pointers
		target = mp.sample()
		map1, map2 = map2, map1

		// log status of planner to periodically inform user
		if i%logIteration == 0 {
			mp.logger.Debugf("RRT* progress: %d%%\tpath cost: %.3f",
				100*i/algOpts.PlanIter,
				EvaluatePlan(shortestPath(startMap, goalMap, shared).toInputs()),
			)
		}
	}

	solutionChan <- shortestPath(startMap, goalMap, shared)
}

func (mp *rrtStarConnectMotionPlanner) sample() []referenceframe.Input {
	return referenceframe.RandomFrameInputs(mp.frame, mp.randseed)
}

func (mp *rrtStarConnectMotionPlanner) extend(opt *PlannerOptions, tree map[*node]*node, target []referenceframe.Input) *node {
	if validTarget := mp.checkInputs(opt, target); !validTarget {
		return nil
	}

	// iterate over the k nearest neighbors and find the minimum cost to connect the target node to the tree
	neighbors := kNearestNeighbors(tree, target)
	minCost := math.Inf(1)
	var minIndex int
	for i, neighbor := range neighbors {
		cost := neighbor.node.cost + neighbor.dist
		if cost < minCost && mp.checkPath(opt, neighbor.node.q, target) {
			minIndex = i
			minCost = cost
		}
	}

	// add new node to tree as a child of the minimum cost neighbor node
	targetNode := &node{q: target, cost: minCost}
	tree[targetNode] = neighbors[minIndex].node

	// rewire the tree
	for i, neighbor := range neighbors {
		// dont need to try to rewire minIndex, so skip it
		if i == minIndex {
			continue
		}

		// check to see if a shortcut is possible, and rewire the node if it is
		cost := targetNode.cost + inputDist(targetNode.q, neighbor.node.q)
		if cost < neighbor.node.cost && mp.checkPath(opt, target, neighbor.node.q) {
			neighbor.node.cost = cost
			tree[neighbor.node] = targetNode
		}
	}
	return targetNode
}

func (mp *rrtStarConnectMotionPlanner) checkInputs(opt *PlannerOptions, inputs []referenceframe.Input) bool {
	position, err := mp.frame.Transform(inputs)
	if err != nil {
		return false
	}
	ok, _ := opt.CheckConstraints(&ConstraintInput{
		StartPos:   position,
		EndPos:     position,
		StartInput: inputs,
		EndInput:   inputs,
		Frame:      mp.frame,
	})
	return ok
}

func (mp *rrtStarConnectMotionPlanner) checkPath(opt *PlannerOptions, seedInputs, target []referenceframe.Input) bool {
	seedPos, err := mp.frame.Transform(seedInputs)
	if err != nil {
		return false
	}
	goalPos, err := mp.frame.Transform(target)
	if err != nil {
		return false
	}
	ok, _ := opt.CheckConstraintPath(
		&ConstraintInput{
			StartPos:   seedPos,
			EndPos:     goalPos,
			StartInput: seedInputs,
			EndInput:   target,
			Frame:      mp.frame,
		},
		opt.Resolution,
	)
	return ok
}
