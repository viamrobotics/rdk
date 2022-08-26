package motionplan

import (
	"context"
	"math/rand"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
)

type rrtConnectMotionPlanner struct {
	solver   InverseKinematics
	frame    referenceframe.Frame
	logger   golog.Logger
	iter     int
	nCPU     int
	stepSize float64
	randseed *rand.Rand
}

// NewRRTConnectMotionPlanner creates a rrtConnectMotionPlanner object.
func NewRRTConnectMotionPlanner(frame referenceframe.Frame, nCPU int, seed *rand.Rand, logger golog.Logger) (MotionPlanner, error) {
	//nolint:gosec
	return NewRRTConnectMotionPlannerWithSeed(frame, nCPU, rand.New(rand.NewSource(1)), logger)
}

// NewRRTConnectMotionPlannerWithSeed creates a rrtConnectMotionPlanner object with a user specified random seed.
func NewRRTConnectMotionPlannerWithSeed(frame referenceframe.Frame, nCPU int, seed *rand.Rand, logger golog.Logger) (MotionPlanner, error) {
	ik, err := CreateCombinedIKSolver(frame, logger, nCPU)
	if err != nil {
		return nil, err
	}
	return &rrtConnectMotionPlanner{
		solver:   ik,
		frame:    frame,
		logger:   logger,
		nCPU:     nCPU,
		randseed: seed,
	}, nil
}

func (mp *rrtConnectMotionPlanner) Frame() referenceframe.Frame {
	return mp.frame
}

func (mp *rrtConnectMotionPlanner) Resolution() float64 {
	return mp.stepSize
}

func (mp *rrtConnectMotionPlanner) Plan(ctx context.Context,
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
		return plan.toInputs(), plan.err
	}
}

// planRunner will execute the plan. When Plan() is called, it will call planRunner in a separate thread and wait for the results.
// Separating this allows other things to call planRunner in parallel while also enabling the thread-agnostic Plan to be accessible.
func (mp *rrtConnectMotionPlanner) planRunner(ctx context.Context,
	goal *commonpb.Pose,
	seed []referenceframe.Input,
	opt *PlannerOptions,
	endpointPreview chan *node,
	solutionChan chan *planReturn,
) {
	defer close(solutionChan)

	// use default options if none are provided
	if opt == nil {
		opt = NewBasicPlannerOptions()
	}

	// get many potential end goals from IK solver
	solutions, err := getSolutions(ctx, opt, mp.solver, goal, seed, mp.Frame())
	if err != nil {
		solutionChan <- &planReturn{err: err}
		return
	}

	// publish endpoint of plan if it is known
	if opt.MaxSolutions == 1 && endpointPreview != nil {
		endpointPreview <- &node{q: solutions[0]}
	}

	// initialize maps
	goalMap := make(map[*node]*node, len(solutions))
	for _, solution := range solutions {
		goalMap[&node{q: solution}] = nil
	}
	startMap := make(map[*node]*node)
	startMap[&node{q: seed}] = nil

	// TODO(rb) package neighborManager better
	nm := &neighborManager{nCPU: mp.nCPU}
	nmContext, cancel := context.WithCancel(ctx)
	defer cancel()

	// for the first iteration, we try the 0.5 interpolation between seed and goal[0]
	target := referenceframe.InterpolateInputs(seed, solutions[0], 0.5)

	// Create a reference to the two maps so that we can alternate which one is grown
	map1, map2 := startMap, goalMap

	for i := 0; i < mp.iter; i++ {
		select {
		case <-ctx.Done():
			solutionChan <- &planReturn{err: ctx.Err()}
			return
		default:
		}

		// for each map get the nearest neighbor to the target
		nearest1 := nm.nearestNeighbor(nmContext, target, map1)
		nearest2 := nm.nearestNeighbor(nmContext, target, map2)

		// attempt to extend the map to connect the target to map 1, then try to connect the maps together
		map1reached := mp.checkPath(ctx, opt, nearest1.q, target)
		targetNode := &node{q: target}
		if map1reached {
			map1[targetNode] = nearest1
		}
		map2reached := mp.checkPath(ctx, opt, nearest2.q, target)
		if map2reached {
			map2[targetNode] = nearest2
		}

		if map1reached && map2reached {
			cancel()
			solutionChan <- &planReturn{steps: extractPath(startMap, goalMap, &nodePair{targetNode, targetNode})}
			return
		}

		// get next sample, switch map pointers
		target = mp.sample()
		map1, map2 = map2, map1
	}

	solutionChan <- &planReturn{err: NewPlannerFailedError()}
}

func (mp *rrtConnectMotionPlanner) sample() []referenceframe.Input {
	return referenceframe.RandomFrameInputs(mp.frame, mp.randseed)
}

func (mp *rrtConnectMotionPlanner) checkPath(ctx context.Context, opt *PlannerOptions, seedInputs, target []referenceframe.Input) bool {
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
