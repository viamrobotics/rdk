package motionplan

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sort"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

// const (
// 	// The maximum percent of a joints range of motion to allow per step.
// 	frameStep = 0.015
// 	// If the dot product between two sets of joint angles is less than this, consider them identical.
// 	jointSolveDist = 0.0001
// 	// Number of planner iterations before giving up.
// 	planIter = 2000
// 	// Number of IK solutions with which to seed the goal side of the bidirectional tree.
// 	solutionsToSeed = 10
// 	// Check constraints are still met every this many mm/degrees of movement.
// 	stepSize = 2
// 	// Name of joint swing scorer.
// 	jointConstraint = "defaultJointSwingConstraint"
// 	// Max number of iterations of path smoothing to run.
// 	smoothIter = 250
// 	// Number of iterations to mrun before beginning to accept randomly seeded locations.
// 	iterBeforeRand = 50
// )

type rrtStarMotionPlanner struct {
	solDist         float64
	solver          InverseKinematics
	fastGradDescent *NloptIK
	frame           referenceframe.Frame
	logger          golog.Logger
	qstep           []float64
	iter            int
	nCPU            int
	stepSize        float64
	randseed        *rand.Rand
}

// NewrrtStarMotionPlanner creates a rrtStarMotionPlanner object.
func NewRRTStarMotionPlanner(frame referenceframe.Frame, nCPU int, logger golog.Logger) (MotionPlanner, error) {
	ik, err := CreateCombinedIKSolver(frame, logger, nCPU)
	if err != nil {
		return nil, err
	}
	// nlopt should try only once
	nlopt, err := CreateNloptIKSolver(frame, logger, 1)
	if err != nil {
		return nil, err
	}
	mp := &rrtStarMotionPlanner{solver: ik, fastGradDescent: nlopt, frame: frame, logger: logger, solDist: jointSolveDist, nCPU: nCPU}

	mp.qstep = getFrameSteps(frame, frameStep)
	mp.iter = planIter
	mp.stepSize = stepSize

	//nolint:gosec
	mp.randseed = rand.New(rand.NewSource(1))

	return mp, nil
}

func (mp *rrtStarMotionPlanner) Frame() referenceframe.Frame {
	return mp.frame
}

func (mp *rrtStarMotionPlanner) Resolution() float64 {
	return mp.stepSize
}

func (mp *rrtStarMotionPlanner) Plan(ctx context.Context,
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
func (mp *rrtStarMotionPlanner) planRunner(ctx context.Context,
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
		endpointPreview = nil
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
		map1reached := mp.constrainedExtend(ctx, opt, map1, nearest1, target)
		map2reached := mp.constrainedExtend(ctx, opt, map2, nearest2, map1reached)

		if inputDist(map1reached.inputs, map2reached.inputs) < mp.solDist {
			cancel()

			path := extractPath(startMap, goalMap, map1reached, map2reached)
			// if endpointPreview != nil {
			// 	endpointPreview <- inputSteps[len(inputSteps)-1]
			// }

			solutionChan <- &planReturn{steps: finalSteps}
			return
		}

		target = mp.sample(map1reached, i)

		// Swap the maps
		map1, map2 = map2, map1
	}

	solutionChan <- &planReturn{err: errors.New("could not solve path")}
}

func (mp *rrtStarMotionPlanner) sample(nearby *configuration, sampleNum int) *configuration {
	// If we have done more than 50 iterations, start seeding off completely random positions 2 at a time
	// The 2 at a time is to ensure random seeds are added onto both the seed and goal maps.
	if sampleNum >= iterBeforeRand && sampleNum%4 >= 2 {
		return &configuration{referenceframe.RandomFrameInputs(mp.frame, mp.randseed)}
	} else {
		// Seeding nearby to valid points results in much faster convergence in less constrained space
		q := &configuration{referenceframe.RestrictedRandomFrameInputs(mp.frame, mp.randseed, 0.2)}
		for j, v := range nearby.inputs {
			q.inputs[j].Value += v.Value
		}
		return q
	}
}

// constrainedExtend will try to extend the map towards the target while meeting constraints along the way. It will
// return the closest solution to the target that it reaches, which may or may not actually be the target.
func (mp *rrtStarMotionPlanner) constrainedExtend(
	ctx context.Context,
	opt *PlannerOptions,
	rrtMap map[*configuration]*configuration,
	near, target *configuration,
) *configuration {
	oldNear := near
	for i := 0; true; i++ {
		switch {
		case inputDist(near.inputs, target.inputs) < mp.solDist:
			return near
		case inputDist(near.inputs, target.inputs) > inputDist(oldNear.inputs, target.inputs):
			return oldNear
		case i > 2 && inputDist(near.inputs, oldNear.inputs) < math.Pow(mp.solDist, 3):
			// not moving enough to make meaningful progress. Do not trigger on first iteration.
			return oldNear
		}

		oldNear = near

		newNear := make([]referenceframe.Input, 0, len(near.inputs))

		// alter near to be closer to target
		for j, nearInput := range near.inputs {
			if nearInput.Value == target.inputs[j].Value {
				newNear = append(newNear, nearInput)
			} else {
				v1, v2 := nearInput.Value, target.inputs[j].Value
				newVal := math.Min(mp.qstep[j], math.Abs(v2-v1))
				// get correct sign
				newVal *= (v2 - v1) / math.Abs(v2-v1)
				newNear = append(newNear, referenceframe.Input{nearInput.Value + newVal})
			}
		}
		// if we are not meeting a constraint, gradient descend to the constraint
		newNear = mp.constrainNear(ctx, opt, oldNear.inputs, newNear)

		if newNear != nil {
			// constrainNear will ensure path between oldNear and newNear satisfies constraints along the way
			near = &configuration{newNear}
			rrtMap[near] = oldNear
		} else {
			break
		}
	}
	return oldNear
}

// constrainNear will do a IK gradient descent from seedInputs to target. If a gradient descent distance
// function has been specified, this will use that.
func (mp *rrtStarMotionPlanner) constrainNear(
	ctx context.Context,
	opt *PlannerOptions,
	seedInputs,
	target []referenceframe.Input,
) []referenceframe.Input {
	seedPos, err := mp.frame.Transform(seedInputs)
	if err != nil {
		return nil
	}
	goalPos, err := mp.frame.Transform(target)
	if err != nil {
		return nil
	}
	// Check if constraints need to be met
	ok, _ := opt.CheckConstraintPath(&ConstraintInput{
		seedPos,
		goalPos,
		seedInputs,
		target,
		mp.frame,
	}, mp.Resolution())
	if ok {
		return target
	}

	solutionGen := make(chan []referenceframe.Input, 1)
	// Spawn the IK solver to generate solutions until done
	err = mp.fastGradDescent.Solve(ctx, solutionGen, goalPos, target, opt.pathDist)
	// We should have zero or one solutions
	var solved []referenceframe.Input
	select {
	case solved = <-solutionGen:
	default:
	}
	close(solutionGen)
	if err != nil {
		return nil
	}

	ok, failpos := opt.CheckConstraintPath(&ConstraintInput{StartInput: seedInputs, EndInput: solved, Frame: mp.frame}, mp.Resolution())
	if !ok {
		if failpos != nil && inputDist(target, failpos.EndInput) > mp.solDist {
			// If we have a first failing position, and that target is updating (no infinite loop), then recurse
			return mp.constrainNear(ctx, opt, failpos.StartInput, failpos.EndInput)
		}
		return nil
	}
	return solved
}
