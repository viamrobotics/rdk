package motionplan

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

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
	ik, err := CreateCombinedIKSolver(frame, logger, nCPU)
	if err != nil {
		return nil, err
	}
	// nlopt should try only once
	nlopt, err := CreateNloptIKSolver(frame, logger, 1)
	if err != nil {
		return nil, err
	}
	mp := &cBiRRTMotionPlanner{solver: ik, fastGradDescent: nlopt, frame: frame, logger: logger, solDist: jointSolveDist, nCPU: nCPU}

	mp.qstep = getFrameSteps(frame, frameStep)
	mp.iter = planIter
	mp.stepSize = stepSize

	//nolint:gosec
	mp.randseed = rand.New(rand.NewSource(1))

	return mp, nil
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
	solutionChan := make(chan *planReturn, 1)
	if opt == nil {
		opt = NewDefaultPlannerOptions()
		seedPos, err := mp.frame.Transform(seed)
		if err != nil {
			solutionChan <- &planReturn{err: err}
			return nil, err
		}
		goalPos := spatial.NewPoseFromProtobuf(fixOvIncrement(goal, spatial.PoseToProtobuf(seedPos)))
		// TODO(rb) make this worldstate, once that is incorporated
		obstacles := map[string]spatial.Geometry{}
		interactionSpaces := map[string]spatial.Geometry{}
		if len(obstacles) == 0 {
			opt = DefaultConstraint(seedPos, goalPos, mp.Frame(), opt)
		} else {
			collisionConst := NewCollisionConstraint(mp.Frame(), obstacles, interactionSpaces)
			if collisionConst != nil {
				opt.AddConstraint("self-collision", collisionConst)
			}
		}
	}

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
func (mp *cBiRRTMotionPlanner) planRunner(ctx context.Context,
	goal *commonpb.Pose,
	seed []referenceframe.Input,
	opt *PlannerOptions,
	endpointPreview chan *configuration,
	solutionChan chan *planReturn,
) {
	defer close(solutionChan)
	inputSteps := []*configuration{}

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	// How many of the top solutions to try
	// Solver will terminate after getting this many to save time
	nSolutions := solutionsToSeed

	if opt.maxSolutions == 0 {
		opt.maxSolutions = nSolutions
	}

	solutions, err := getSolutions(ctx, opt, mp.solver, goal, seed, mp.Frame())
	if err != nil {
		solutionChan <- &planReturn{err: err}
		return
	}

	// Get the N best solutions
	keys := make([]float64, 0, len(solutions))
	for k := range solutions {
		keys = append(keys, k)
	}
	sort.Float64s(keys)

	if len(keys) < nSolutions {
		nSolutions = len(keys)
	}
	if nSolutions == 1 && endpointPreview != nil && len(inputSteps) != 0 {
		endpointPreview <- inputSteps[len(inputSteps)-1]
		endpointPreview = nil
	}

	goalMap := make(map[*configuration]*configuration, nSolutions)

	for _, k := range keys[:nSolutions] {
		goalMap[&configuration{solutions[k]}] = nil
	}

	corners := map[*configuration]bool{}

	seedMap := make(map[*configuration]*configuration)
	seedMap[&configuration{seed}] = nil

	// for the first iteration, we try the 0.5 interpolation between seed and goal[0]
	target := &configuration{referenceframe.InterpolateInputs(seed, solutions[keys[0]], 0.5)}

	var rSeed *configuration

	nn := &neighborManager{nCPU: mp.nCPU}

	// Create a reference to the two maps so that wew can alternate which one is grown
	map1, map2 := seedMap, goalMap

	for i := 0; i < mp.iter; i++ {
		select {
		case <-ctx.Done():
			solutionChan <- &planReturn{err: ctx.Err()}
			return
		default:
		}

		var m1reached, m2reached *configuration

		// extend seed tree first
		nearest := nn.nearestNeighbor(ctxWithCancel, target, map1)
		// Extend map1 as far towards target as it can get. It may or may not reach it.
		m1reached = mp.constrainedExtend(ctx, opt, map1, nearest, target)
		// Find the nearest point in map2 to the furthest point reached in map1
		near2 := nn.nearestNeighbor(ctxWithCancel, m1reached, map2)
		// extend map1 towards the point in map1
		m2reached = mp.constrainedExtend(ctx, opt, map2, near2, m1reached)
		rSeed = m1reached

		corners[m1reached] = true
		corners[m2reached] = true

		if inputDist(m1reached.inputs, m2reached.inputs) < mp.solDist {
			cancel()

			// extract the path to the seed
			var seedReached, goalReached *configuration
			// Need to figure out which of m1/m2 is seed/goal
			if _, ok := seedMap[m1reached]; ok {
				seedReached, goalReached = m1reached, m2reached
			} else {
				seedReached, goalReached = m2reached, m1reached
			}

			// extract the path to the seed
			for seedReached != nil {
				inputSteps = append(inputSteps, seedReached)
				seedReached = seedMap[seedReached]
			}
			// reverse the slice
			for i, j := 0, len(inputSteps)-1; i < j; i, j = i+1, j-1 {
				inputSteps[i], inputSteps[j] = inputSteps[j], inputSteps[i]
			}
			goalReached = goalMap[goalReached]
			// extract the path to the goal
			for goalReached != nil {
				inputSteps = append(inputSteps, goalReached)
				goalReached = goalMap[goalReached]
			}
			if endpointPreview != nil {
				endpointPreview <- inputSteps[len(inputSteps)-1]
			}

			finalSteps := mp.SmoothPath(ctx, opt, inputSteps, corners)
			solutionChan <- &planReturn{steps: finalSteps}
			return
		}

		// If we have done more than 50 iterations, start seeding off completely random positions 2 at a time
		// The 2 at a time is to ensure random seeds are added onto both the seed and goal maps.
		if i >= iterBeforeRand && i%4 >= 2 {
			target = &configuration{referenceframe.RandomFrameInputs(mp.frame, mp.randseed)}
		} else {
			// Seeding nearby to valid points results in much faster convergence in less constrained space
			target = &configuration{referenceframe.RestrictedRandomFrameInputs(mp.frame, mp.randseed, 0.2)}
			for j, v := range rSeed.inputs {
				target.inputs[j].Value += v.Value
			}
		}

		// Swap the maps
		map1, map2 = map2, map1
	}

	solutionChan <- &planReturn{err: errors.New("could not solve path")}
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

// Used for coordinating parallel computations of nearestNeighbor.
type neighborManager struct {
	nnKeys    chan *configuration
	neighbors chan *neighbor
	nnLock    sync.RWMutex
	seedPos   *configuration
	ready     bool
	nCPU      int
}

type neighbor struct {
	dist float64
	sol  *configuration
}

func (nm *neighborManager) nearestNeighbor(
	ctx context.Context,
	seed *configuration,
	rrtMap map[*configuration]*configuration,
) *configuration {
	if len(rrtMap) > 1000 {
		// If the map is large, calculate distances in parallel
		return nm.parallelNearestNeighbor(ctx, seed, rrtMap)
	}
	bestDist := math.Inf(1)
	var best *configuration
	for k := range rrtMap {
		dist := inputDist(seed.inputs, k.inputs)
		if dist < bestDist {
			bestDist = dist
			best = k
		}
	}
	return best
}

func (nm *neighborManager) parallelNearestNeighbor(
	ctx context.Context,
	seed *configuration,
	rrtMap map[*configuration]*configuration,
) *configuration {
	nm.ready = false
	nm.startNNworkers(ctx)
	defer close(nm.nnKeys)
	defer close(nm.neighbors)
	nm.nnLock.Lock()
	nm.seedPos = seed
	nm.nnLock.Unlock()

	for k := range rrtMap {
		nm.nnKeys <- k
	}
	nm.nnLock.Lock()
	nm.ready = true
	nm.nnLock.Unlock()
	var best *configuration
	bestDist := math.Inf(1)
	returned := 0
	for returned < nm.nCPU {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		select {
		case nn := <-nm.neighbors:
			returned++
			if nn.dist < bestDist {
				bestDist = nn.dist
				best = nn.sol
			}
		default:
		}
	}
	return best
}

func (nm *neighborManager) startNNworkers(ctx context.Context) {
	nm.neighbors = make(chan *neighbor, nm.nCPU)
	nm.nnKeys = make(chan *configuration, nm.nCPU)
	for i := 0; i < nm.nCPU; i++ {
		utils.PanicCapturingGo(func() {
			nm.nnWorker(ctx)
		})
	}
}

func (nm *neighborManager) nnWorker(ctx context.Context) {
	var best *configuration
	bestDist := math.Inf(1)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case k := <-nm.nnKeys:
			if k != nil {
				nm.nnLock.RLock()
				dist := inputDist(nm.seedPos.inputs, k.inputs)
				nm.nnLock.RUnlock()
				if dist < bestDist {
					bestDist = dist
					best = k
				}
			}
		default:
			nm.nnLock.RLock()
			if nm.ready {
				nm.nnLock.RUnlock()
				nm.neighbors <- &neighbor{bestDist, best}
				return
			}
			nm.nnLock.RUnlock()
		}
	}
}
