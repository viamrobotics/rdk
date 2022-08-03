package motionplan

import (
	"context"
	"errors"
	"math"
	"math/rand"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

// TODO(rb) all these should be tied to default values for config somehow.
const (
	// The maximum percent of a joints range of motion to allow per step.
	frameStep = 0.015
	// If the dot product between two sets of joint angles is less than this, consider them identical.
	jointSolveDist = 0.0001
	// Number of planner iterations before giving up.
	planIter = 2000
	// Number of IK solutions with which to seed the goal side of the bidirectional tree.
	solutionsToSeed = 10
	// Check constraints are still met every this many mm/degrees of movement.
	stepSize = 2
	// Name of joint swing scorer.
	jointConstraint = "defaultJointSwingConstraint"
	// Max number of iterations of path smoothing to run.
	smoothIter = 250
	// Number of iterations to mrun before beginning to accept randomly seeded locations.
	iterBeforeRand = 50
)

// cBiRRTMotionPlanner an object able to solve constrained paths around obstacles to some goal for a given referenceframe.
// It uses the Constrained Bidirctional Rapidly-expanding Random Tree algorithm, Berenson et al 2009
// https://ieeexplore.ieee.org/document/5152399/
type cBiRRTMotionPlanner struct {
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

// NewCBiRRTMotionPlanner creates a cBiRRTMotionPlanner object.
func NewCBiRRTMotionPlanner(frame referenceframe.Frame, nCPU int, logger golog.Logger) (MotionPlanner, error) {
	//nolint:gosec
	return NewCBiRRTMotionPlannerWithSeed(frame, nCPU, rand.New(rand.NewSource(1)), logger)
}

// NewCBiRRTMotionPlannerWithSeed creates a cBiRRTMotionPlanner object with a user specified random seed.
func NewCBiRRTMotionPlannerWithSeed(frame referenceframe.Frame, nCPU int, seed *rand.Rand, logger golog.Logger) (MotionPlanner, error) {
	ik, err := CreateCombinedIKSolver(frame, logger, nCPU)
	if err != nil {
		return nil, err
	}
	// nlopt should try only once
	nlopt, err := CreateNloptIKSolver(frame, logger, 1)
	if err != nil {
		return nil, err
	}
	return &cBiRRTMotionPlanner{
		solDist:         jointSolveDist,
		solver:          ik,
		fastGradDescent: nlopt,
		frame:           frame,
		logger:          logger,
		qstep:           getFrameSteps(frame, frameStep),
		iter:            planIter,
		stepSize:        stepSize,
		randseed:        seed,
	}, nil
}

func (mp *cBiRRTMotionPlanner) Frame() referenceframe.Frame {
	return mp.frame
}

func (mp *cBiRRTMotionPlanner) Resolution() float64 {
	return mp.stepSize
}

func (mp *cBiRRTMotionPlanner) Plan(ctx context.Context,
	goal *commonpb.Pose,
	seed []referenceframe.Input,
	opt *PlannerOptions,
) ([][]referenceframe.Input, error) {
	if opt != nil {
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

	seedPos, err := mp.frame.Transform(seed)
	if err != nil {
		return nil, err
	}
	goalPos := spatial.NewPoseFromProtobuf(goal)

	numSteps := GetSteps(seedPos, goalPos, pathStepSize)
	goals := make([]spatial.Pose, 0, numSteps)
	for i := 1; i < numSteps; i++ {
		by := float64(i) / float64(numSteps)
		goals = append(goals, spatial.Interpolate(seedPos, goalPos, by))
	}
	goals = append(goals, goalPos)

	// We want to make this of len(goals), not len(0) so that indexing does not fail
	opts := make([]*PlannerOptions, len(goals))
	results, err := RunPlannerWithWaypoints(ctx, mp, goals, seed, opts, 0)
	if err != nil {
		return nil, err
	}
	return results, nil
}

// planRunner will execute the plan. When Plan() is called, it will call planRunner in a separate thread and wait for the results.
// Separating this allows other things to call planRunner in parallel while also enabling the thread-agnostic Plan to be accessible.
func (mp *cBiRRTMotionPlanner) planRunner(ctx context.Context,
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

	// get many potential end goals from IK solver
	solutions, err := getSolutions(ctx, opt, mp.solver, goal, seed, mp.Frame())
	if err != nil {
		solutionChan <- &planReturn{err: err}
		return
	}

	// publish endpoint of plan if it is known
	if opt.maxSolutions == 1 && endpointPreview != nil {
		endpointPreview <- &configuration{inputs: solutions[0]}
		endpointPreview = nil
	}

	// initialize maps
	goalMap := make(map[*configuration]*configuration, len(solutions))
	for _, solution := range solutions {
		goalMap[&configuration{inputs: solution}] = nil
	}
	corners := map[*configuration]bool{}
	seedMap := make(map[*configuration]*configuration)
	seedMap[&configuration{inputs: seed}] = nil

	// Create a reference to the two maps so that we can alternate which one is grown
	map1, map2 := seedMap, goalMap

	// TODO(rb) package neighborManager better
	nm := &neighborManager{nCPU: mp.nCPU}
	nmContext, cancel := context.WithCancel(ctx)
	defer cancel()

	// main sampling loop - for the first sample we try the 0.5 interpolation between seed and goal[0]
	target := &configuration{inputs: referenceframe.InterpolateInputs(seed, solutions[0], 0.5)}
	for i := 0; i < mp.iter; i++ {
		select {
		case <-ctx.Done():
			solutionChan <- &planReturn{err: ctx.Err()}
			return
		default:
		}

		// attempt to extend map1 first
		nearest1 := nm.nearestNeighbor(nmContext, target, map1)
		map1reached := mp.constrainedExtend(ctx, opt, map1, nearest1, target)

		// then attempt to extend map2 towards map 1
		nearest2 := nm.nearestNeighbor(nmContext, map1reached, map2)
		map2reached := mp.constrainedExtend(ctx, opt, map2, nearest2, map1reached)

		corners[map1reached] = true
		corners[map2reached] = true

		if inputDist(map1reached.inputs, map2reached.inputs) < mp.solDist {
			cancel()
			path := extractPath(seedMap, goalMap, map1reached, map2reached)
			if endpointPreview != nil {
				endpointPreview <- path[len(path)-1]
			}
			finalSteps := mp.SmoothPath(ctx, opt, path, corners)
			solutionChan <- &planReturn{steps: finalSteps}
			return
		}

		// sample near map 1 and switch which map is which to keep adding to them even
		target = mp.sample(map1reached, i)
		map1, map2 = map2, map1
	}

	solutionChan <- &planReturn{err: errors.New("could not solve path")}
}

func (mp *cBiRRTMotionPlanner) sample(rSeed *configuration, sampleNum int) *configuration {
	// If we have done more than 50 iterations, start seeding off completely random positions 2 at a time
	// The 2 at a time is to ensure random seeds are added onto both the seed and goal maps.
	if sampleNum >= iterBeforeRand && sampleNum%4 >= 2 {
		return &configuration{inputs: referenceframe.RandomFrameInputs(mp.frame, mp.randseed)}
	}
	// Seeding nearby to valid points results in much faster convergence in less constrained space
	q := &configuration{inputs: referenceframe.RestrictedRandomFrameInputs(mp.frame, mp.randseed, 0.2)}
	for j, v := range rSeed.inputs {
		q.inputs[j].Value += v.Value
	}
	return q
}

// constrainedExtend will try to extend the map towards the target while meeting constraints along the way. It will
// return the closest solution to the target that it reaches, which may or may not actually be the target.
func (mp *cBiRRTMotionPlanner) constrainedExtend(
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
			near = &configuration{inputs: newNear}
			rrtMap[near] = oldNear
		} else {
			break
		}
	}
	return oldNear
}

// constrainNear will do a IK gradient descent from seedInputs to target. If a gradient descent distance
// function has been specified, this will use that.
func (mp *cBiRRTMotionPlanner) constrainNear(
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

// SmoothPath will pick two points at random along the path and attempt to do a fast gradient descent directly between
// them, which will cut off randomly-chosen points with odd joint angles into something that is a more intuitive motion.
func (mp *cBiRRTMotionPlanner) SmoothPath(
	ctx context.Context,
	opt *PlannerOptions,
	inputSteps []*configuration,
	corners map[*configuration]bool,
) []*configuration {
	toIter := int(math.Min(float64(len(inputSteps)*len(inputSteps)), smoothIter))

	for iter := 0; iter < toIter && len(inputSteps) > 4; iter++ {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		// Pick two random non-adjacent indices, excepting the ends
		//nolint:gosec
		j := 2 + rand.Intn(len(inputSteps)-3)
		//nolint:gosec
		i := rand.Intn(j) + 1

		ok, hitCorners := smoothable(inputSteps, i, j, corners)
		if !ok {
			continue
		}

		shortcutGoal := make(map[*configuration]*configuration)

		iSol := inputSteps[i]
		jSol := inputSteps[j]
		shortcutGoal[jSol] = nil

		// extend backwards for convenience later. Should work equally well in both directions
		reached := mp.constrainedExtend(ctx, opt, shortcutGoal, jSol, iSol)

		// Note this could technically replace paths with "longer" paths i.e. with more waypoints.
		// However, smoothed paths are invariably more intuitive and smooth, and lend themselves to future shortening,
		// so we allow elongation here.
		if inputDist(inputSteps[i].inputs, reached.inputs) < mp.solDist && len(reached.inputs) < j-i {
			corners[iSol] = true
			corners[jSol] = true
			for _, hitCorner := range hitCorners {
				corners[hitCorner] = false
			}
			newInputSteps := append([]*configuration{}, inputSteps[:i]...)
			for reached != nil {
				newInputSteps = append(newInputSteps, reached)
				reached = shortcutGoal[reached]
			}
			newInputSteps = append(newInputSteps, inputSteps[j+1:]...)
			inputSteps = newInputSteps
		}
	}

	return inputSteps
}

// Check if there is more than one joint direction change. If not, then not a good candidate for smoothing.
func smoothable(inputSteps []*configuration, i, j int, corners map[*configuration]bool) (bool, []*configuration) {
	startPos := inputSteps[i]
	nextPos := inputSteps[i+1]
	// Whether joints are increasing
	incDir := make([]int, 0, len(startPos.inputs))
	hitCorners := []*configuration{}

	if corners[startPos] {
		hitCorners = append(hitCorners, startPos)
	}
	if corners[nextPos] {
		hitCorners = append(hitCorners, nextPos)
	}

	check := func(v1, v2 float64) int {
		if v1 > v2 {
			return 1
		} else if v1 < v2 {
			return -1
		}
		return 0
	}

	// Get initial directionality
	for h, v := range startPos.inputs {
		incDir = append(incDir, check(v.Value, nextPos.inputs[h].Value))
	}

	// Check for any direction changes
	changes := 0
	for k := i + 2; k < j; k++ {
		for h, v := range nextPos.inputs {
			// Get 1, 0, or -1 depending on directionality
			newV := check(v.Value, inputSteps[k].inputs[h].Value)
			if incDir[h] == 0 {
				incDir[h] = newV
			} else if incDir[h] == newV*-1 {
				changes++
			}
			if changes > 1 && len(hitCorners) > 0 {
				return true, hitCorners
			}
		}
		nextPos = inputSteps[k]
		if corners[nextPos] {
			hitCorners = append(hitCorners, nextPos)
		}
	}
	return false, hitCorners
}

// getFrameSteps will return a slice of positive values representing the largest amount a particular DOF of a frame should
// move in any given step.
func getFrameSteps(f referenceframe.Frame, by float64) []float64 {
	dof := f.DoF()
	pos := make([]float64, len(dof))
	for i, lim := range dof {
		l, u := lim.Min, lim.Max

		// Default to [-999,999] as range if limits are infinite
		if l == math.Inf(-1) {
			l = -999
		}
		if u == math.Inf(1) {
			u = 999
		}

		jRange := math.Abs(u - l)
		pos[i] = jRange * by
	}
	return pos
}

func extractPath(startMap, goalMap map[*configuration]*configuration, q1, q2 *configuration) []*configuration {
	// need to figure out which of the two configurations is in the start map
	var startReached, goalReached *configuration
	if _, ok := startMap[q1]; ok {
		startReached, goalReached = q1, q2
	} else {
		startReached, goalReached = q2, q1
	}

	// extract the path to the seed
	path := []*configuration{}
	for startReached != nil {
		path = append(path, startReached)
		startReached = startMap[startReached]
	}

	// reverse the slice
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	// skip goalReached configuration and go directly to its parent in order to not repeat this node
	goalReached = goalMap[goalReached]

	// extract the path to the goal
	for goalReached != nil {
		path = append(path, goalReached)
		goalReached = goalMap[goalReached]
	}
	return path
}
