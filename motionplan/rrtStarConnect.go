package motionplan

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

type rrtStarConnectMotionPlanner struct {
	solver   InverseKinematics
	frame    referenceframe.Frame
	logger   golog.Logger
	iter     int
	nCPU     int
	stepSize float64
	randseed *rand.Rand
}

// TODO(rb): find a reasonable default for this
// neighborhoodSize represents the number of neighbors to find in a k-nearest neighbors search
const neighborhoodSize = 10

// NewRRTStarConnectMotionPlanner creates a rrtStarConnectMotionPlanner object.
func NewRRTStarConnectMotionPlanner(frame referenceframe.Frame, nCPU int, seed *rand.Rand, logger golog.Logger) (MotionPlanner, error) {
	//nolint:gosec
	return NewRRTStarConnectMotionPlannerWithSeed(frame, nCPU, rand.New(rand.NewSource(1)), logger)
}

// NewRRTStarConnectMotionPlannerWithSeed creates a rrtStarConnectMotionPlanner object with a user specified random seed.
func NewRRTStarConnectMotionPlannerWithSeed(frame referenceframe.Frame, nCPU int, seed *rand.Rand, logger golog.Logger) (MotionPlanner, error) {
	ik, err := CreateCombinedIKSolver(frame, logger, nCPU)
	if err != nil {
		return nil, err
	}
	return &rrtStarConnectMotionPlanner{
		solver:   ik,
		frame:    frame,
		logger:   logger,
		iter:     planIter,
		nCPU:     nCPU,
		stepSize: stepSize,
		randseed: seed,
	}, nil
}

func (mp *rrtStarConnectMotionPlanner) Frame() referenceframe.Frame {
	return mp.frame
}

func (mp *rrtStarConnectMotionPlanner) Resolution() float64 {
	return mp.stepSize
}

func (mp *rrtStarConnectMotionPlanner) Plan(ctx context.Context,
	goal *commonpb.Pose,
	seed []referenceframe.Input,
	opt *PlannerOptions,
) ([][]referenceframe.Input, error) {
	solutionChan := make(chan *planReturn, 1)
	utils.PanicCapturingGo(func() {
		mp.planRunner(ctx, goal, seed, opt, nil, solutionChan)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case plan := <-solutionChan:
		finalSteps := make([][]referenceframe.Input, 0, len(plan.steps))
		for _, step := range plan.steps {
			finalSteps = append(finalSteps, step.q)
		}
		return finalSteps, plan.err
	}
}

// planRunner will execute the plan. When Plan() is called, it will call planRunner in a separate thread and wait for the results.
// Separating this allows other things to call planRunner in parallel while also enabling the thread-agnostic Plan to be accessible.
func (mp *rrtStarConnectMotionPlanner) planRunner(ctx context.Context,
	goal *commonpb.Pose,
	seed []referenceframe.Input,
	opt *PlannerOptions,
	endpointPreview chan *node,
	solutionChan chan *planReturn,
) {
	defer close(solutionChan)

	// use default options if none are provided
	if opt == nil {
		opt = NewDefaultPlannerOptions()
		seedPos, err := mp.frame.Transform(seed)
		if err != nil {
			solutionChan <- &planReturn{err: err}
			return
		}
		goalPos := spatial.NewPoseFromProtobuf(goal)

		opt = DefaultConstraint(seedPos, goalPos, mp.Frame(), opt)
	}

	// get many potential end goals from IK solver
	solutions, err := getSolutions(ctx, opt, mp.solver, goal, seed, mp.Frame())
	if err != nil {
		solutionChan <- &planReturn{err: err}
		return
	}

	// publish endpoint of plan if it is known
	if opt.maxSolutions == 1 && endpointPreview != nil {
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

	// sample until the max number of iterations is reached
	prevCost := math.Inf(1)
	var plan *planReturn
	for i := 0; i < mp.iter; i++ {
		select {
		case <-ctx.Done():
			solutionChan <- &planReturn{err: ctx.Err()}
			return
		default:
		}

		if i == 98 {
			fmt.Println("ok")
		}

		// try to connect the target to map 1
		if map1reached := mp.extend(opt, map1, target); map1reached != nil {
			// try to connect the target to map 2
			if map2reached := mp.extend(opt, map2, target); map2reached != nil {
				// target was added to both map
				shared = append(shared, &nodePair{map1reached, map2reached})
			}
		}

		// get next sample, switch map pointers
		target = mp.sample()

		// TODO(rb): figure out why the hell uncommenting this matters
		// map1, map2 = map2, map1
		plan = shortestPath(startMap, goalMap, shared)
		if len(plan.steps) != 0 && (plan.steps[0].q[0] != seed[0] || plan.steps[0].q[1] != seed[1]) {
			fmt.Println("what the actual fuck")
		}
		cost := evaluatePlanNodes(plan.steps)
		if prevCost < cost {
			fmt.Println("RRT* should never have the cost of the total path increase")
		}
		prevCost = cost
		fmt.Println(cost)
	}

	solutionChan <- shortestPath(startMap, goalMap, shared)
}

func (mp *rrtStarConnectMotionPlanner) sample() []referenceframe.Input {
	return referenceframe.RandomFrameInputs(mp.frame, mp.randseed)
}

func (mp *rrtStarConnectMotionPlanner) extend(opt *PlannerOptions, tree map[*node]*node, target []referenceframe.Input) *node {
	neighbors := kNearestNeighbors(tree, target)

	// TODO(rb): potentially either add a steer() function or get the closest valid point from constraint checker
	map1reached := mp.checkPath(opt, neighbors[0].node.q, target)
	if !map1reached {
		return nil
	}

	// iterate over neighbors and find the minimum cost to connect the target node to the tree
	minIndex := 0
	minCost := neighbors[0].node.cost + neighbors[0].dist
	for i := 1; i < len(neighbors); i++ {
		cost := neighbors[i].node.cost + neighbors[i].dist
		if cost < minCost && mp.checkPath(opt, neighbors[i].node.q, target) {
			minIndex = i
			minCost = cost
		}
	}

	// add new node to tree as a child of the minimum cost neighbor node
	targetNode := &node{q: target, cost: minCost}
	tree[targetNode] = neighbors[minIndex].node

	// rewire the tree
	for i := 0; i < len(neighbors); i++ {
		// dont need to try to rewire minIndex, so skip it
		if i == minIndex {
			continue
		}

		cost := targetNode.cost + inputDist(targetNode.q, neighbors[i].node.q)
		if cost < neighbors[i].node.cost && mp.checkPath(opt, target, neighbors[i].node.q) {
			// shortcut possible, rewire the node
			neighbors[i].node.cost = cost
			tree[neighbors[i].node] = targetNode
		}
	}
	return targetNode
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
	// Check if constraints need to be met
	ok, _ := opt.CheckConstraintPath(
		&ConstraintInput{
			StartPos:   seedPos,
			EndPos:     goalPos,
			StartInput: seedInputs,
			EndInput:   target,
			Frame:      mp.frame,
		},
		mp.Resolution(),
	)
	return ok
}
