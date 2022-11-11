package motionplan

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"fmt"
	"time"

	"github.com/edaniels/golog"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
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
var nloptCnt = 0

// planRunner will execute the plan. When Plan() is called, it will call planRunner in a separate thread and wait for the results.
// Separating this allows other things to call planRunner in parallel while also enabling the thread-agnostic Plan to be accessible.
func (mp *cBiRRTMotionPlanner) planRunner(ctx context.Context,
	goal *commonpb.Pose,
	seed []referenceframe.Input,
	planOpts *PlannerOptions,
	endpointPreview chan node,
	solutionChan chan *planReturn,
) {
	defer close(solutionChan)
	solved := false
	
	// initialize maps
	goalMap := make(map[node]node)
	corners := map[node]bool{}
	seedMap := make(map[node]node)
	seedMap[&basicNode{q: seed}] = nil

	// Create a reference to the two maps so that we can alternate which one is grown
	//~ map1, map2 := seedMap, goalMap

	// TODO(rb) package neighborManager better
	nm := &neighborManager{nCPU: mp.nCPU}
	nmContext, cancel := context.WithCancel(ctx)
	defer cancel()
		var nn time.Duration
		var ext time.Duration
	doIter := func(){
		start := time.Now()
		
		
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
		//~ iktime := time.Now()
		solutions, err := getSolutions(ctx, planOpts, mp.solver, goal, seed, mp.Frame(), mp.randseed.Int())
		
		fmt.Println("sol len", len(solutions))
		
		if err != nil && len(goalMap) == 0{
			fmt.Println("err", err)
			solutionChan <- &planReturn{err: err}
			return
		}
		
		//~ fmt.Println("ik", time.Since(iktime))
		for i, solution := range solutions {
			
			// Check if we can directly interpolate to any solutions
			if i == 0 && mp.checkPath(planOpts, seed, solution.Q()) {
				fmt.Println("IK SOLVE", i)
				solutionChan <- &planReturn{steps: []node{&basicNode{q: seed}, solution}}
				solved = true
				return
			}
			
			// if we got more solutions, add them
			if _, ok := goalMap[solution]; !ok {
				goalMap[solution] = nil
			}
		}
		
		target := mp.sample(algOpts, &basicNode{q: seed}, mp.randseed.Int())

		if len(solutions) > 0 {
			// publish endpoint of plan if it is known
			if planOpts.MaxSolutions == 1 && endpointPreview != nil {
				endpointPreview <- solutions[0]
				endpointPreview = nil
			}


			// main sampling loop - for the first sample we try the 0.5 interpolation between seed and goal[0]
			target = referenceframe.InterpolateInputs(seed, solutions[0].Q(), 0.5)
		}

		map1, map2 := seedMap, goalMap
		//~ fmt.Println("maps", len(map1), len(map2))
		
		m1chan := make(chan node, 1)
		m2chan := make(chan node, 1)
		defer close(m1chan)
		defer close(m2chan)

		for i := 0; i < algOpts.PlanIter; i++ {
			//~ fmt.Println("i", i)
			
			if time.Since(start) > time.Duration(algOpts.Timeout) * time.Second {
				//~ fmt.Println("i", i)
				//~ fmt.Println("nn", nn, "ext", ext, "neart", neart, "descent", descent, "cpatht", cpatht, "rt", rt, "nlopt", nloptCnt)
				//~ fmt.Println("timeout")
				return
			}
			
			select {
			case <-ctx.Done():
				solutionChan <- &planReturn{err: ctx.Err()}
				return
			default:
			}

			// attempt to extend map1 first
			nns := time.Now()
			nearest1 := nm.nearestNeighbor(nmContext, planOpts, target, map1)
			//~ nearest2 := nm.nearestNeighbor(nmContext, planOpts, map1reached.Q(), map2)
			nearest2 := nm.nearestNeighbor(nmContext, planOpts, target, map2)
			nn += time.Since(nns)
			
			exts := time.Now()
			utils.PanicCapturingGo(func() {
				mp.constrainedExtend(ctx, algOpts, map1, nearest1, &basicNode{q: target}, m1chan)
			})
			utils.PanicCapturingGo(func() {
				mp.constrainedExtend(ctx, algOpts, map2, nearest2, &basicNode{q: target}, m2chan)
			})
			map1reached := <- m1chan
			map2reached := <- m2chan
			ext += time.Since(exts)

			// then attempt to extend map2 towards map 1
			//~ nns = time.Now()
			//~ nearest2 := nm.nearestNeighbor(nmContext, planOpts, map1reached.Q(), map2)
			//~ nn += time.Since(nns)

			corners[map1reached] = true
			corners[map2reached] = true

			_, reachedDelta := planOpts.DistanceFunc(&ConstraintInput{StartInput: map1reached.Q(), EndInput: map2reached.Q()})
			
			// Second iteration to connect the reached points
			if reachedDelta > algOpts.JointSolveDist {
				nns := time.Now()
				nearest1 := nm.nearestNeighbor(nmContext, planOpts, map2reached.Q(), map1)
				//~ nearest2 := nm.nearestNeighbor(nmContext, planOpts, map1reached.Q(), map2)
				nearest2 := nm.nearestNeighbor(nmContext, planOpts, map1reached.Q(), map2)
				nn += time.Since(nns)
				
				exts := time.Now()
				utils.PanicCapturingGo(func() {
					mp.constrainedExtend(ctx, algOpts, map1, nearest1, map2reached, m1chan)
				})
				utils.PanicCapturingGo(func() {
					mp.constrainedExtend(ctx, algOpts, map2, nearest2, map1reached, m2chan)
				})
				map1reached = <- m1chan
				map2reached = <- m2chan
				ext += time.Since(exts)
				_, reachedDelta = planOpts.DistanceFunc(&ConstraintInput{StartInput: map1reached.Q(), EndInput: map2reached.Q()})
			}
			
			// Solved!
			if reachedDelta <= algOpts.JointSolveDist {
				cancel()
				path := extractPath(seedMap, goalMap, &nodePair{map1reached, map2reached})
				if endpointPreview != nil {
					endpointPreview <- path[len(path)-1]
				}
				//~ fmt.Println("solved!")
				sm := time.Now()
				finalSteps := mp.SmoothPath(ctx, algOpts, path, corners)
				fmt.Println("smooth time", time.Since(sm))
				solutionChan <- &planReturn{steps: finalSteps}
				solved = true
				fmt.Println("nn", nn, "ext", ext, "neart", neart, "descent", descent, "cpatht", cpatht, "rt", rt, "nlopt", nloptCnt)
				return
			}

			// sample near map 1 and switch which map is which to keep adding to them even
			target = mp.sample(algOpts, map1reached, i)
			map1, map2 = map2, map1
		}
	}
	doIter()
	if solved {
		fmt.Println("returning solved")
		return
	}
	select {
	case <-ctx.Done():
		solutionChan <- &planReturn{err: ctx.Err()}
		return
	default:
	}
	for planOpts.Fallback != nil {
		planOpts = planOpts.Fallback
		fmt.Println("fallback")
		
		doIter()
		
		if solved {
			fmt.Println("fb solved!")
			return
		}
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

var dfunc time.Duration
var neart time.Duration
var matht time.Duration


// constrainedExtend will try to extend the map towards the target while meeting constraints along the way. It will
// return the closest solution to the target that it reaches, which may or may not actually be the target.
func (mp *cBiRRTMotionPlanner) constrainedExtend(
	ctx context.Context,
	algOpts *cbirrtOptions,
	rrtMap map[node]node,
	near, target node,
	mchan chan node,
) {
	
	oldNear := near
	for i := 0; i<1000; i++ {
		start := time.Now()
		_, dist := algOpts.planOpts.DistanceFunc(&ConstraintInput{StartInput: near.Q(), EndInput: target.Q()})
		_, oldDist := algOpts.planOpts.DistanceFunc(&ConstraintInput{StartInput: oldNear.Q(), EndInput: target.Q()})
		_, nearDist := algOpts.planOpts.DistanceFunc(&ConstraintInput{StartInput: near.Q(), EndInput: oldNear.Q()})
		dfunc += time.Since(start)
		switch {
		case dist < algOpts.JointSolveDist:
			mchan <- near
			return
		case dist > oldDist:
			mchan <- oldNear
			return
		case i > 2 && nearDist < math.Pow(algOpts.JointSolveDist, 3):
			// not moving enough to make meaningful progress. Do not trigger on first iteration.
			mchan <- oldNear
			return
		}

		oldNear = near

		newNear := make([]referenceframe.Input, 0, len(near.Q()))

		// alter near to be closer to target
		start = time.Now()
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
		matht += time.Since(start)
		// if we are not meeting a constraint, gradient descend to the constraint
		start = time.Now()
		newNear = mp.constrainNear(ctx, algOpts, oldNear.Q(), newNear)
		neart += time.Since(start)

		if newNear != nil {
			// constrainNear will ensure path between oldNear and newNear satisfies constraints along the way
			near = &basicNode{q: newNear}
			//~ fmt.Println(oldNear)
			rrtMap[near] = oldNear
		} else {
			break
		}
	}
	mchan <- oldNear
	return
}

var descent time.Duration
var cpatht time.Duration

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
	start := time.Now()
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
	cpatht += time.Since(start)
	
	start = time.Now()
	solutionGen := make(chan []referenceframe.Input, 1)
	// Spawn the IK solver to generate solutions until done
	nloptCnt++
	err = mp.fastGradDescent.Solve(ctx, solutionGen, goalPos, target, algOpts.planOpts.pathDist, mp.randseed.Int())
	// We should have zero or one solutions
	var solved []referenceframe.Input
	descent += time.Since(start)
	select {
	case solved = <-solutionGen:
	default:
	}
	close(solutionGen)
	if err != nil {
		return nil
	}

	start = time.Now()
	ok, failpos := algOpts.planOpts.CheckConstraintPath(
		&ConstraintInput{StartInput: seedInputs, EndInput: solved, Frame: mp.frame},
		algOpts.planOpts.Resolution,
	)
	cpatht += time.Since(start)
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
	
	schan := make(chan node, 1)
	defer close(schan)

	for iter := 0; iter < toIter && len(inputSteps) > 4; iter++ {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		// Pick two random non-adjacent indices, excepting the ends
		//nolint:gosec
		j := 2 + mp.randseed.Intn(len(inputSteps)-3)
		//nolint:gosec
		i := mp.randseed.Intn(j) + 1

		ok, hitCorners := smoothable(inputSteps, i, j, corners)
		if !ok {
			continue
		}

		shortcutGoal := make(map[node]node)

		iSol := inputSteps[i]
		jSol := inputSteps[j]
		shortcutGoal[jSol] = nil

		// extend backwards for convenience later. Should work equally well in both directions
		mp.constrainedExtend(ctx, algOpts, shortcutGoal, jSol, iSol, schan)
		reached := <- schan

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
