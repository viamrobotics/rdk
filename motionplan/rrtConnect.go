package motionplan

import (
	"context"
	"errors"
	"math/rand"
	"sort"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
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
		iter:     planIter,
		nCPU:     nCPU,
		stepSize: stepSize,
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
		finalSteps := make([][]referenceframe.Input, 0, len(plan.steps))
		for _, step := range plan.steps {
			finalSteps = append(finalSteps, step.inputs)
		}
		return finalSteps, plan.err
	}
}

// planRunner will execute the plan. When Plan() is called, it will call planRunner in a separate thread and wait for the results.
// Separating this allows other things to call planRunner in parallel while also enabling the thread-agnostic Plan to be accessible.
func (mp *rrtConnectMotionPlanner) planRunner(ctx context.Context,
	goal *commonpb.Pose,
	seed []referenceframe.Input,
	opt *PlannerOptions,
	endpointPreview chan *configuration,
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

	// TODO(rb) this will go away when we move to a new configuration scheme
	if opt.maxSolutions == 0 {
		opt.maxSolutions = solutionsToSeed
	}

	// get many potential end goals from IK solver
	solutions, err := getSolutions(ctx, opt, mp.solver, goal, seed, mp.Frame())
	if err != nil {
		solutionChan <- &planReturn{err: err}
		return
	}

	// TODO(rb) this code should be moved into getSolutions
	keys := make([]float64, 0, len(solutions))
	for k := range solutions {
		keys = append(keys, k)
	}
	sort.Float64s(keys)
	if len(keys) < opt.maxSolutions {
		opt.maxSolutions = len(keys)
	}
	if opt.maxSolutions == 1 && endpointPreview != nil {
		endpointPreview <- &configuration{solutions[keys[0]]}
	}

	// Initialize maps for start and goal
	goalMap := make(map[*configuration]*configuration, opt.maxSolutions)
	for _, k := range keys[:opt.maxSolutions] {
		goalMap[&configuration{solutions[k]}] = nil
	}
	startMap := make(map[*configuration]*configuration)
	startMap[&configuration{seed}] = nil

	// TODO(rb) package neighborManager better
	nm := &neighborManager{nCPU: mp.nCPU}
	nmContext, cancel := context.WithCancel(ctx)
	defer cancel()

	// for the first iteration, we try the 0.5 interpolation between seed and goal[0]
	target := &configuration{referenceframe.InterpolateInputs(seed, solutions[keys[0]], 0.5)}

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
		map1reached := mp.checkPath(ctx, opt, nearest1.inputs, target.inputs)
		if map1reached {
			map1[target] = nearest1
		}
		map2reached := mp.checkPath(ctx, opt, nearest2.inputs, target.inputs)
		if map2reached {
			map2[target] = nearest2
		}

		if map1reached && map2reached {
			cancel()
			solutionChan <- &planReturn{steps: extractPath(startMap, goalMap, target, target)}
			return
		}

		target = mp.sample()

		map1, map2 = map2, map1
	}

	solutionChan <- &planReturn{err: errors.New("could not solve path")}
}

func (mp *rrtConnectMotionPlanner) sample() *configuration {
	return &configuration{referenceframe.RandomFrameInputs(mp.frame, mp.randseed)}
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
