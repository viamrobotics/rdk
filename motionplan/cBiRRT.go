package motionplan

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	// The maximum percent of a joints range of motion to allow per step.
	defaultFrameStep = 0.015

	// If the dot product between two sets of joint angles is less than this, consider them identical.
	defaultJointSolveDist = 0.0001

	// Max number of iterations of path smoothing to run.
	defaultSmoothIter = 750

	// Number of iterations to run before beginning to accept randomly seeded locations.
	defaultIterBeforeRand = 50
)

type cbirrtOptions struct {
	// The maximum percent of a joints range of motion to allow per step.
	FrameStep float64 `json:"frame_step"`

	// If the dot product between two sets of joint angles is less than this, consider them identical.
	JointSolveDist float64 `json:"joint_solve_dist"`

	// Number of IK solutions with which to seed the goal side of the bidirectional tree.
	SolutionsToSeed int `json:"solutions_to_seed"`

	// Max number of iterations of path smoothing to run.
	SmoothIter int `json:"smooth_iter"`

	// Number of iterations to mrun before beginning to accept randomly seeded locations.
	IterBeforeRand int `json:"iter_before_rand"`

	// This is how far cbirrt will try to extend the map towards a goal per-step. Determined from FrameStep
	qstep []float64

	// Parameters common to all RRT implementations
	*rrtOptions
}

// newCbirrtOptions creates a struct controlling the running of a single invocation of cbirrt. All values are pre-set to reasonable
// defaults, but can be tweaked if needed.
func newCbirrtOptions(planOpts *PlannerOptions, frame referenceframe.Frame) (*cbirrtOptions, error) {
	algOpts := &cbirrtOptions{
		FrameStep:       defaultFrameStep,
		JointSolveDist:  defaultJointSolveDist,
		SolutionsToSeed: defaultSolutionsToSeed,
		SmoothIter:      defaultSmoothIter,
		IterBeforeRand:  defaultIterBeforeRand,
		rrtOptions:      newRRTOptions(planOpts),
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

	algOpts.qstep = getFrameSteps(frame, algOpts.FrameStep)

	return algOpts, nil
}

// cBiRRTMotionPlanner an object able to solve constrained paths around obstacles to some goal for a given referenceframe.
// It uses the Constrained Bidirctional Rapidly-expanding Random Tree algorithm, Berenson et al 2009
// https://ieeexplore.ieee.org/document/5152399/
type cBiRRTMotionPlanner struct {
	*planner
	fastGradDescent *NloptIK
}

// NewCBiRRTMotionPlanner creates a cBiRRTMotionPlanner object.
func NewCBiRRTMotionPlanner(frame referenceframe.Frame, nCPU int, logger golog.Logger) (MotionPlanner, error) {
	//nolint:gosec
	return NewCBiRRTMotionPlannerWithSeed(frame, nCPU, rand.New(rand.NewSource(1)), logger)
}

// NewCBiRRTMotionPlannerWithSeed creates a cBiRRTMotionPlanner object with a user specified random seed.
func NewCBiRRTMotionPlannerWithSeed(frame referenceframe.Frame, nCPU int, seed *rand.Rand, logger golog.Logger) (MotionPlanner, error) {
	planner, err := newPlanner(frame, nCPU, seed, logger)
	if err != nil {
		return nil, err
	}
	// nlopt should try only once
	nlopt, err := CreateNloptIKSolver(frame, logger, 1)
	if err != nil {
		return nil, err
	}
	return &cBiRRTMotionPlanner{
		planner:         planner,
		fastGradDescent: nlopt,
	}, nil
}

func (mp *cBiRRTMotionPlanner) Plan(ctx context.Context,
	goal spatialmath.Pose,
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
func (mp *cBiRRTMotionPlanner) planRunner(
	ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
	planOpts *PlannerOptions,
	endpointPreview chan node,
	solutionChan chan *planReturn,
) {
	defer close(solutionChan)

	// setup planner options
	if planOpts == nil {
		solutionChan <- &planReturn{err: errNoPlannerOptions}
		return
	}
	algOpts, err := newCbirrtOptions(planOpts, mp.frame)
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
		endpointPreview <- solutions[0]
		endpointPreview = nil
	}

	// initialize maps
	goalMap := make(map[node]node, len(solutions))
	for _, solution := range solutions {
		goalMap[solution] = nil
	}
	corners := map[node]bool{}
	seedMap := make(map[node]node)
	seedMap[&basicNode{q: seed}] = nil

	// Create a reference to the two maps so that we can alternate which one is grown
	map1, map2 := seedMap, goalMap

	// TODO(rb) package neighborManager better
	nm := &neighborManager{nCPU: mp.nCPU}
	nmContext, cancel := context.WithCancel(ctx)
	defer cancel()

	// main sampling loop - for the first sample we try the 0.5 interpolation between seed and goal[0]
	target := referenceframe.InterpolateInputs(seed, solutions[0].Q(), 0.5)
	for i := 0; i < algOpts.PlanIter; i++ {
		select {
		case <-ctx.Done():
			solutionChan <- &planReturn{err: ctx.Err()}
			return
		default:
		}

		// attempt to extend map1 first
		nearest1 := nm.nearestNeighbor(nmContext, planOpts, target, map1)
		map1reached := mp.constrainedExtend(ctx, algOpts, map1, nearest1, &basicNode{q: target})

		// then attempt to extend map2 towards map 1
		nearest2 := nm.nearestNeighbor(nmContext, planOpts, map1reached.Q(), map2)
		map2reached := mp.constrainedExtend(ctx, algOpts, map2, nearest2, map1reached)

		corners[map1reached] = true
		corners[map2reached] = true

		_, reachedDelta := planOpts.DistanceFunc(&ConstraintInput{StartInput: map1reached.Q(), EndInput: map2reached.Q()})
		if reachedDelta < algOpts.JointSolveDist {
			cancel()
			path := extractPath(seedMap, goalMap, &nodePair{map1reached, map2reached})
			if endpointPreview != nil {
				endpointPreview <- path[len(path)-1]
			}
			finalSteps := mp.SmoothPath(ctx, algOpts, path, corners)
			solutionChan <- &planReturn{steps: finalSteps}
			return
		}

		// sample near map 1 and switch which map is which to keep adding to them even
		target = mp.sample(algOpts, map1reached, i)
		map1, map2 = map2, map1
	}

	solutionChan <- &planReturn{err: errPlannerFailed}
}

func (mp *cBiRRTMotionPlanner) sample(algOpts *cbirrtOptions, rSeed node, sampleNum int) []referenceframe.Input {
	// If we have done more than 50 iterations, start seeding off completely random positions 2 at a time
	// The 2 at a time is to ensure random seeds are added onto both the seed and goal maps.
	if sampleNum >= algOpts.IterBeforeRand && sampleNum%4 >= 2 {
		return referenceframe.RandomFrameInputs(mp.frame, mp.randseed)
	}
	// Seeding nearby to valid points results in much faster convergence in less constrained space
	q := referenceframe.RestrictedRandomFrameInputs(mp.frame, mp.randseed, 0.5)
	for j, v := range rSeed.Q() {
		q[j].Value += v.Value
	}
	return q
}

// constrainedExtend will try to extend the map towards the target while meeting constraints along the way. It will
// return the closest solution to the target that it reaches, which may or may not actually be the target.
func (mp *cBiRRTMotionPlanner) constrainedExtend(
	ctx context.Context,
	algOpts *cbirrtOptions,
	rrtMap map[node]node,
	near, target node,
) node {
	oldNear := near
	for i := 0; true; i++ {
		_, dist := algOpts.planOpts.DistanceFunc(&ConstraintInput{StartInput: near.Q(), EndInput: target.Q()})
		_, oldDist := algOpts.planOpts.DistanceFunc(&ConstraintInput{StartInput: oldNear.Q(), EndInput: target.Q()})
		_, nearDist := algOpts.planOpts.DistanceFunc(&ConstraintInput{StartInput: near.Q(), EndInput: oldNear.Q()})
		switch {
		case dist < algOpts.JointSolveDist:
			return near
		case dist > oldDist:
			return oldNear
		case i > 2 && nearDist < math.Pow(algOpts.JointSolveDist, 3):
			// not moving enough to make meaningful progress. Do not trigger on first iteration.
			return oldNear
		}

		oldNear = near

		newNear := make([]referenceframe.Input, 0, len(near.Q()))

		// alter near to be closer to target
		for j, nearInput := range near.Q() {
			if nearInput.Value == target.Q()[j].Value {
				newNear = append(newNear, nearInput)
			} else {
				v1, v2 := nearInput.Value, target.Q()[j].Value
				newVal := math.Min(algOpts.qstep[j], math.Abs(v2-v1))
				// get correct sign
				newVal *= (v2 - v1) / math.Abs(v2-v1)
				newNear = append(newNear, referenceframe.Input{nearInput.Value + newVal})
			}
		}
		// if we are not meeting a constraint, gradient descend to the constraint
		newNear = mp.constrainNear(ctx, algOpts, oldNear.Q(), newNear)

		if newNear != nil {
			// constrainNear will ensure path between oldNear and newNear satisfies constraints along the way
			near = &basicNode{q: newNear}
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
	algOpts *cbirrtOptions,
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
	ok, _ := algOpts.planOpts.CheckConstraintPath(&ConstraintInput{
		seedPos,
		goalPos,
		seedInputs,
		target,
		mp.frame,
	}, algOpts.planOpts.Resolution)
	if ok {
		return target
	}

	solutionGen := make(chan []referenceframe.Input, 1)
	// Spawn the IK solver to generate solutions until done
	err = mp.fastGradDescent.Solve(ctx, solutionGen, goalPos, target, algOpts.planOpts.pathDist)
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

	ok, failpos := algOpts.planOpts.CheckConstraintPath(
		&ConstraintInput{StartInput: seedInputs, EndInput: solved, Frame: mp.frame},
		algOpts.planOpts.Resolution,
	)
	if !ok {
		if failpos != nil {
			_, dist := algOpts.planOpts.DistanceFunc(&ConstraintInput{StartInput: target, EndInput: failpos.EndInput})
			if dist > algOpts.JointSolveDist {
				// If we have a first failing position, and that target is updating (no infinite loop), then recurse
				return mp.constrainNear(ctx, algOpts, failpos.StartInput, failpos.EndInput)
			}
		}
		return nil
	}
	return solved
}

// SmoothPath will pick two points at random along the path and attempt to do a fast gradient descent directly between
// them, which will cut off randomly-chosen points with odd joint angles into something that is a more intuitive motion.
func (mp *cBiRRTMotionPlanner) SmoothPath(
	ctx context.Context,
	algOpts *cbirrtOptions,
	inputSteps []node,
	corners map[node]bool,
) []node {
	toIter := int(math.Min(float64(len(inputSteps)*len(inputSteps)), float64(algOpts.SmoothIter)))

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

		shortcutGoal := make(map[node]node)

		iSol := inputSteps[i]
		jSol := inputSteps[j]
		shortcutGoal[jSol] = nil

		// extend backwards for convenience later. Should work equally well in both directions
		reached := mp.constrainedExtend(ctx, algOpts, shortcutGoal, jSol, iSol)

		// Note this could technically replace paths with "longer" paths i.e. with more waypoints.
		// However, smoothed paths are invariably more intuitive and smooth, and lend themselves to future shortening,
		// so we allow elongation here.
		_, dist := algOpts.planOpts.DistanceFunc(&ConstraintInput{StartInput: inputSteps[i].Q(), EndInput: reached.Q()})
		if dist < algOpts.JointSolveDist && len(reached.Q()) < j-i {
			corners[iSol] = true
			corners[jSol] = true
			for _, hitCorner := range hitCorners {
				corners[hitCorner] = false
			}
			newInputSteps := append([]node{}, inputSteps[:i]...)
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
func smoothable(inputSteps []node, i, j int, corners map[node]bool) (bool, []node) {
	startPos := inputSteps[i]
	nextPos := inputSteps[i+1]
	// Whether joints are increasing
	incDir := make([]int, 0, len(startPos.Q()))
	hitCorners := []node{}

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
	for h, v := range startPos.Q() {
		incDir = append(incDir, check(v.Value, nextPos.Q()[h].Value))
	}

	// Check for any direction changes
	changes := 0
	for k := i + 2; k < j; k++ {
		for h, v := range nextPos.Q() {
			// Get 1, 0, or -1 depending on directionality
			newV := check(v.Value, inputSteps[k].Q()[h].Value)
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
