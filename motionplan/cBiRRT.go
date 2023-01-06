package motionplan

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"time"

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

	// Number of iterations to run before beginning to accept randomly seeded locations.
	defaultIterBeforeRand = 50

	// Maximum number of iterations that constrainNear will run before exiting nil.
	// Typically it will solve in the first five iterations, or not at all.
	maxNearIter = 20

	// Maximum number of iterations that constrainedExtend will run before exiting.
	maxExtendIter = 5000
)

type cbirrtOptions struct {
	// The maximum percent of a joints range of motion to allow per step.
	FrameStep float64 `json:"frame_step"`

	// If the dot product between two sets of joint angles is less than this, consider them identical.
	JointSolveDist float64 `json:"joint_solve_dist"`

	// Number of IK solutions with which to seed the goal side of the bidirectional tree.
	SolutionsToSeed int `json:"solutions_to_seed"`

	// Number of iterations to mrun before beginning to accept randomly seeded locations.
	IterBeforeRand int `json:"iter_before_rand"`

	// This is how far cbirrt will try to extend the map towards a goal per-step. Determined from FrameStep
	qstep []float64

	// Parameters common to all RRT implementations
	*rrtOptions
}

// newCbirrtOptions creates a struct controlling the running of a single invocation of cbirrt. All values are pre-set to reasonable
// defaults, but can be tweaked if needed.
func newCbirrtOptions(planOpts *plannerOptions, frame referenceframe.Frame) (*cbirrtOptions, error) {
	algOpts := &cbirrtOptions{
		FrameStep:       defaultFrameStep,
		JointSolveDist:  defaultJointSolveDist,
		SolutionsToSeed: defaultSolutionsToSeed,
		IterBeforeRand:  defaultIterBeforeRand,
		rrtOptions:      newRRTOptions(),
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
	algOpts         *cbirrtOptions
	corners         map[node]bool
}

// newCBiRRTMotionPlannerWithSeed creates a cBiRRTMotionPlanner object with a user specified random seed.
func newCBiRRTMotionPlanner(
	frame referenceframe.Frame,
	seed *rand.Rand,
	logger golog.Logger,
	opt *plannerOptions,
) (motionPlanner, error) {
	if opt == nil {
		opt = newBasicPlannerOptions()
	}
	mp, err := newPlanner(frame, seed, logger, opt)
	if err != nil {
		return nil, err
	}
	// nlopt should try only once
	nlopt, err := CreateNloptIKSolver(frame, logger, 1)
	if err != nil {
		return nil, err
	}
	algOpts, err := newCbirrtOptions(opt, mp.frame)
	if err != nil {
		return nil, err
	}
	return &cBiRRTMotionPlanner{
		planner:         mp,
		fastGradDescent: nlopt,
		algOpts:         algOpts,
		corners:         map[node]bool{},
	}, nil
}

func (mp *cBiRRTMotionPlanner) plan(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
) ([][]referenceframe.Input, error) {
	solutionChan := make(chan *rrtPlanReturn, 1)
	utils.PanicCapturingGo(func() {
		mp.rrtBackgroundRunner(ctx, goal, seed, &rrtParallelPlannerShared{nil, nil, solutionChan})
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case plan := <-solutionChan:
		return plan.toInputs(), plan.err()
	}
}

// rrtBackgroundRunner will execute the plan. Plan() will call rrtBackgroundRunner in a separate thread and wait for results.
// Separating this allows other things to call rrtBackgroundRunner in parallel allowing the thread-agnostic Plan to be accessible.
func (mp *cBiRRTMotionPlanner) rrtBackgroundRunner(
	ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
	rrt *rrtParallelPlannerShared,
) {
	defer close(rrt.solutionChan)

	// setup planner options
	if mp.planOpts == nil {
		rrt.solutionChan <- &rrtPlanReturn{planerr: errNoPlannerOptions}
		return
	}
	// initialize maps
	corners := map[node]bool{}
	// TODO(rb) package neighborManager better
	nm := &neighborManager{nCPU: mp.planOpts.NumThreads}
	nmContext, cancel := context.WithCancel(ctx)
	defer cancel()
	mp.start = time.Now()

	if rrt.maps == nil || len(rrt.maps.goalMap) == 0 {
		planSeed := initRRTSolutions(ctx, mp, goal, seed)
		if planSeed.planerr != nil || planSeed.steps != nil {
			rrt.solutionChan <- planSeed
			return
		}
		rrt.maps = planSeed.maps
	}
	target := referenceframe.InterpolateInputs(seed, rrt.maps.optNode.Q(), 0.5)

	map1, map2 := rrt.maps.startMap, rrt.maps.goalMap

	m1chan := make(chan node, 1)
	m2chan := make(chan node, 1)
	defer close(m1chan)
	defer close(m2chan)

	mp.logger.Debugf("running CBiRRT with start map of size %d and goal map of size %d", len(rrt.maps.startMap), len(rrt.maps.goalMap))

	for i := 0; i < mp.algOpts.PlanIter; i++ {
		select {
		case <-ctx.Done():
			mp.logger.Debugf("CBiRRT timed out after %d iterations", i)
			rrt.solutionChan <- &rrtPlanReturn{planerr: ctx.Err(), maps: rrt.maps}
			return
		default:
		}

		tryExtend := func(target []referenceframe.Input) (node, node, float64) {
			// attempt to extend maps 1 and 2 towards the target
			nearest1 := nm.nearestNeighbor(nmContext, mp.planOpts, target, map1)
			nearest2 := nm.nearestNeighbor(nmContext, mp.planOpts, target, map2)
			//nolint: gosec
			rseed1 := rand.New(rand.NewSource(int64(mp.randseed.Int())))
			//nolint: gosec
			rseed2 := rand.New(rand.NewSource(int64(mp.randseed.Int())))

			utils.PanicCapturingGo(func() {
				mp.constrainedExtend(ctx, rseed1, map1, nearest1, &basicNode{q: target}, m1chan)
			})
			utils.PanicCapturingGo(func() {
				mp.constrainedExtend(ctx, rseed2, map2, nearest2, &basicNode{q: target}, m2chan)
			})
			map1reached := <-m1chan
			map2reached := <-m2chan

			corners[map1reached] = true
			corners[map2reached] = true

			_, reachedDelta := mp.planOpts.DistanceFunc(&ConstraintInput{StartInput: map1reached.Q(), EndInput: map2reached.Q()})
			return map1reached, map2reached, reachedDelta
		}

		map1reached, map2reached, reachedDelta := tryExtend(target)
		// Second iteration; extend maps 1 and 2 towards the halfway point between where they reached
		if reachedDelta > mp.algOpts.JointSolveDist {
			target = referenceframe.InterpolateInputs(map1reached.Q(), map2reached.Q(), 0.5)
			map1reached, map2reached, reachedDelta = tryExtend(target)
		}

		// Solved!
		if reachedDelta <= mp.algOpts.JointSolveDist {
			mp.logger.Debugf("CBiRRT found solution after %d iterations", i)
			cancel()
			path := extractPath(rrt.maps.startMap, rrt.maps.goalMap, &nodePair{map1reached, map2reached})
			rrt.solutionChan <- &rrtPlanReturn{steps: path, maps: rrt.maps}
			return
		}

		// sample near map 1 and switch which map is which to keep adding to them even
		target = mp.sample(map1reached, i)
		map1, map2 = map2, map1
	}
	rrt.solutionChan <- &rrtPlanReturn{planerr: errPlannerFailed, maps: rrt.maps}
}

func (mp *cBiRRTMotionPlanner) sample(rSeed node, sampleNum int) []referenceframe.Input {
	// If we have done more than 50 iterations, start seeding off completely random positions 2 at a time
	// The 2 at a time is to ensure random seeds are added onto both the seed and goal maps.
	if sampleNum >= mp.algOpts.IterBeforeRand && sampleNum%4 >= 2 {
		return referenceframe.RandomFrameInputs(mp.frame, mp.randseed)
	}
	// Seeding nearby to valid points results in much faster convergence in less constrained space
	q := referenceframe.RestrictedRandomFrameInputs(mp.frame, mp.randseed, 0.1)
	for j, v := range rSeed.Q() {
		q[j].Value += v.Value
	}
	return q
}

// constrainedExtend will try to extend the map towards the target while meeting constraints along the way. It will
// return the closest solution to the target that it reaches, which may or may not actually be the target.
func (mp *cBiRRTMotionPlanner) constrainedExtend(
	ctx context.Context,
	randseed *rand.Rand,
	rrtMap map[node]node,
	near, target node,
	mchan chan node,
) {
	oldNear := near
	// This should iterate until one of the following conditions:
	// 1) we have reached the target
	// 2) the request is cancelled/times out
	// 3) we are no longer approaching the target and our "best" node is further away than the previous best
	// 4) further iterations change our best node by close-to-zero amounts
	for i := 0; i < maxExtendIter; i++ {
		select {
		case <-ctx.Done():
			mchan <- oldNear
			return
		default:
		}

		_, dist := mp.planOpts.DistanceFunc(&ConstraintInput{StartInput: near.Q(), EndInput: target.Q()})
		_, oldDist := mp.planOpts.DistanceFunc(&ConstraintInput{StartInput: oldNear.Q(), EndInput: target.Q()})
		_, nearDist := mp.planOpts.DistanceFunc(&ConstraintInput{StartInput: near.Q(), EndInput: oldNear.Q()})
		switch {
		case dist < mp.algOpts.JointSolveDist:
			mchan <- near
			return
		case dist > oldDist:
			mchan <- oldNear
			return
		case i > 2 && nearDist < math.Pow(mp.algOpts.JointSolveDist, 3):
			// not moving enough to make meaningful progress. Do not trigger on first iteration.
			mchan <- oldNear
			return
		}

		oldNear = near

		newNear := make([]referenceframe.Input, 0, len(near.Q()))

		// alter near to be closer to target
		for j, nearInput := range near.Q() {
			if nearInput.Value == target.Q()[j].Value {
				newNear = append(newNear, nearInput)
			} else {
				v1, v2 := nearInput.Value, target.Q()[j].Value
				newVal := math.Min(mp.algOpts.qstep[j], math.Abs(v2-v1))
				// get correct sign
				newVal *= (v2 - v1) / math.Abs(v2-v1)
				newNear = append(newNear, referenceframe.Input{nearInput.Value + newVal})
			}
		}
		// if we are not meeting a constraint, gradient descend to the constraint
		newNear = mp.constrainNear(ctx, randseed, oldNear.Q(), newNear)

		if newNear != nil {
			// constrainNear will ensure path between oldNear and newNear satisfies constraints along the way
			near = &basicNode{q: newNear}
			rrtMap[near] = oldNear
		} else {
			break
		}
	}
	mchan <- oldNear
}

// constrainNear will do a IK gradient descent from seedInputs to target. If a gradient descent distance
// function has been specified, this will use that.
// This function will return either a valid configuration that meets constraints, or nil.
func (mp *cBiRRTMotionPlanner) constrainNear(
	ctx context.Context,
	randseed *rand.Rand,
	seedInputs,
	target []referenceframe.Input,
) []referenceframe.Input {
	for i := 0; i < maxNearIter; i++ {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		seedPos, err := mp.frame.Transform(seedInputs)
		if err != nil {
			return nil
		}
		goalPos, err := mp.frame.Transform(target)
		if err != nil {
			return nil
		}
		// Check if constraints need to be met
		ok, _ := mp.planOpts.CheckConstraintPath(&ConstraintInput{
			seedPos,
			goalPos,
			seedInputs,
			target,
			mp.frame,
		}, mp.planOpts.Resolution)
		if ok {
			return target
		}
		solutionGen := make(chan []referenceframe.Input, 1)
		// Spawn the IK solver to generate solutions until done
		err = mp.fastGradDescent.Solve(ctx, solutionGen, goalPos, target, mp.planOpts.pathDist, randseed.Int())
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

		ok, failpos := mp.planOpts.CheckConstraintPath(
			&ConstraintInput{StartInput: seedInputs, EndInput: solved, Frame: mp.frame},
			mp.planOpts.Resolution,
		)
		if ok {
			return solved
		}
		if failpos != nil {
			_, dist := mp.planOpts.DistanceFunc(&ConstraintInput{StartInput: target, EndInput: failpos.EndInput})
			if dist > mp.algOpts.JointSolveDist {
				// If we have a first failing position, and that target is updating (no infinite loop), then recurse
				seedInputs = failpos.StartInput
				target = failpos.EndInput
			}
		} else {
			return nil
		}
	}
	return nil
}

// smoothPath will pick two points at random along the path and attempt to do a fast gradient descent directly between
// them, which will cut off randomly-chosen points with odd joint angles into something that is a more intuitive motion.
func (mp *cBiRRTMotionPlanner) smoothPath(
	ctx context.Context,
	inputSteps []node,
) []node {
	toIter := int(math.Min(float64(len(inputSteps)*len(inputSteps)), float64(mp.planOpts.SmoothIter)))

	schan := make(chan node, 1)
	defer close(schan)

	for iter := 0; iter < toIter && len(inputSteps) > 4; iter++ {
		select {
		case <-ctx.Done():
			mp.logger.Debug("CBiRRT timed out during smoothing, returning best path")
			return inputSteps
		default:
		}
		// Pick two random non-adjacent indices, excepting the ends

		j := 2 + mp.randseed.Intn(len(inputSteps)-3)

		i := mp.randseed.Intn(j) + 1

		ok, hitCorners := smoothable(inputSteps, i, j, mp.corners)
		if !ok {
			continue
		}

		shortcutGoal := make(map[node]node)

		iSol := inputSteps[i]
		jSol := inputSteps[j]
		shortcutGoal[jSol] = nil

		// extend backwards for convenience later. Should work equally well in both directions
		mp.constrainedExtend(ctx, mp.randseed, shortcutGoal, jSol, iSol, schan)
		reached := <-schan

		// Note this could technically replace paths with "longer" paths i.e. with more waypoints.
		// However, smoothed paths are invariably more intuitive and smooth, and lend themselves to future shortening,
		// so we allow elongation here.
		_, dist := mp.planOpts.DistanceFunc(&ConstraintInput{StartInput: inputSteps[i].Q(), EndInput: reached.Q()})
		if dist < mp.algOpts.JointSolveDist && len(reached.Q()) < j-i {
			mp.corners[iSol] = true
			mp.corners[jSol] = true
			for _, hitCorner := range hitCorners {
				mp.corners[hitCorner] = false
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
	for k := i + 1; k < j; k++ {
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
