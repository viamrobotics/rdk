package motionplan

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sort"

	"github.com/edaniels/golog"

	"go.viam.com/core/kinematics"
	commonpb "go.viam.com/core/proto/api/common/v1"
	frame "go.viam.com/core/referenceframe"
)

const (
	// The maximum percent of a joints range of motion to allow per step
	frameStep = 0.015
	// If the dot product between two sets of joint angles is less than this, consider them identical
	jointSolveDist = 0.0001
	// Number of planner iterations before giving up
	planIter = 2000
	// Number of IK solutions with which to seed the goal side of the bidirectional tree
	solutionsToSeed = 50
	// Name of joint swing scorer
	jointConstraint = "defaultJointSwingConstraint"
)

// cBiRRTMotionPlanner an object able to solve constrained paths around obstacles to some goal for a given frame.
// It uses the Constrained Bidirctional Rapidly-expanding Random Tree algorithm, Berenson et al 2009
// https://ieeexplore.ieee.org/document/5152399/
type cBiRRTMotionPlanner struct {
	solDist         float64
	solver          kinematics.InverseKinematics
	fastGradDescent *kinematics.NloptIK
	frame           frame.Frame
	logger          golog.Logger
	qstep           []float64
	iter            int
	stepSize        float64
	randseed        *rand.Rand
	opt             *PlannerOptions
}

// needed to wrap slices so we can use them as map keys
type solution struct {
	inputs []frame.Input
}

// NewCBiRRTMotionPlanner creates a cBiRRTMotionPlanner object
func NewCBiRRTMotionPlanner(frame frame.Frame, nCPU int, logger golog.Logger) (MotionPlanner, error) {
	ik, err := kinematics.CreateCombinedIKSolver(frame, logger, nCPU)
	if err != nil {
		return nil, err
	}
	nlopt, err := kinematics.CreateNloptIKSolver(frame, logger)
	if err != nil {
		return nil, err
	}
	// nlopt should try only once
	nlopt.SetMaxIter(1)
	mp := &cBiRRTMotionPlanner{solver: ik, fastGradDescent: nlopt, frame: frame, logger: logger, solDist: jointSolveDist}

	mp.qstep = getFrameSteps(frame, frameStep)
	mp.iter = planIter
	mp.stepSize = 1

	mp.randseed = rand.New(rand.NewSource(1))
	mp.opt = NewDefaultPlannerOptions()

	return mp, nil
}

func (mp *cBiRRTMotionPlanner) SetMetric(m kinematics.Metric) {
	mp.solver.SetMetric(m)
}

func (mp *cBiRRTMotionPlanner) SetPathDistFunc(m kinematics.Metric) {
	mp.fastGradDescent.SetMetric(m)
}

func (mp *cBiRRTMotionPlanner) Frame() frame.Frame {
	return mp.frame
}

func (mp *cBiRRTMotionPlanner) Resolution() float64 {
	return mp.stepSize
}

func (mp *cBiRRTMotionPlanner) SetOptions(opt *PlannerOptions) {
	mp.opt = opt
	mp.SetMetric(opt.metric)
	mp.SetPathDistFunc(opt.pathDist)
}

func (mp *cBiRRTMotionPlanner) Plan(ctx context.Context, goal *commonpb.Pose, seed []frame.Input) ([][]frame.Input, error) {
	var inputSteps [][]frame.Input

	// Store copy of planner options for duration of solve
	opt := mp.opt

	// How many of the top solutions to try
	// Solver will terminate after getting this many to save time
	nSolutions := solutionsToSeed

	if opt.maxSolutions == 0 {
		opt.maxSolutions = nSolutions
	}

	solutions, err := getSolutions(ctx, opt, mp.solver, goal, seed, mp)
	if err != nil {
		return nil, err
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

	goalMap := make(map[*solution]*solution, nSolutions)

	for _, k := range keys[:nSolutions] {
		goalMap[&solution{solutions[k]}] = nil
	}

	seedMap := make(map[*solution]*solution)
	seedMap[&solution{seed}] = nil

	// Alternate to which map our random sample is added
	// for the first iteration, we try the 0.5 interpolation between seed and goal[0]
	addSeed := true
	target := &solution{frame.InterpolateInputs(seed, solutions[keys[0]], 0.5)}

	for i := 0; i < mp.iter; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var seedReached, goalReached, rSeed *solution

		// Alternate which tree we extend
		if addSeed {
			// extend seed tree first
			nearest := nearestNeighbor(target, seedMap)
			seedReached, goalReached = mp.constrainedExtendWrapper(opt, seedMap, goalMap, nearest, target)
			rSeed = seedReached
		} else {
			// extend goal tree first
			nearest := nearestNeighbor(target, goalMap)
			goalReached, seedReached = mp.constrainedExtendWrapper(opt, goalMap, seedMap, nearest, target)
			rSeed = goalReached
		}

		if inputDist(seedReached.inputs, goalReached.inputs) < mp.solDist {

			// extract the path to the seed
			for seedReached != nil {
				inputSteps = append(inputSteps, seedReached.inputs)
				seedReached = seedMap[seedReached]
			}
			// reverse the slice
			for i, j := 0, len(inputSteps)-1; i < j; i, j = i+1, j-1 {
				inputSteps[i], inputSteps[j] = inputSteps[j], inputSteps[i]
			}
			goalReached = goalMap[goalReached]
			// extract the path to the goal
			for goalReached != nil {
				inputSteps = append(inputSteps, goalReached.inputs)
				goalReached = goalMap[goalReached]
			}
			inputSteps = mp.SmoothPath(ctx, opt, inputSteps)

			return inputSteps, nil
		}

		target = &solution{frame.RestrictedRandomFrameInputs(mp.frame, mp.randseed, 0.1)}
		for j, v := range rSeed.inputs {
			target.inputs[j].Value += v.Value
		}

		addSeed = !addSeed
	}

	return nil, errors.New("could not solve path")
}

// constrainedExtendWrapper wraps two calls to constrainedExtend, adding to one map first, then the other
func (mp *cBiRRTMotionPlanner) constrainedExtendWrapper(opt *PlannerOptions, m1, m2 map[*solution]*solution, near1, target *solution) (*solution, *solution) {
	// Extend tree m1 as far towards target as it can get. It may or may not reach it.
	m1reach := mp.constrainedExtend(opt, m1, near1, target)

	// Find the nearest point in m2 to the furthest point reached in m1
	near2 := nearestNeighbor(m1reach, m2)

	// extend m2 towards the point in m1
	m2reach := mp.constrainedExtend(opt, m2, near2, m1reach)

	return m1reach, m2reach
}

// constrainedExtend will try to extend the map towards the target while meeting constraints along the way. It will
// return the closest solution to the target that it reaches, which may or may not actually be the target.
func (mp *cBiRRTMotionPlanner) constrainedExtend(opt *PlannerOptions, rrtMap map[*solution]*solution, near, target *solution) *solution {
	oldNear := near
	for i := 0; true; i++ {
		if inputDist(near.inputs, target.inputs) < mp.solDist {
			return near
		} else if inputDist(near.inputs, target.inputs) > inputDist(oldNear.inputs, target.inputs) {
			break
		} else if i > 2 && inputDist(near.inputs, oldNear.inputs) < math.Pow(mp.solDist, 3) {
			// not moving enough to make meaningful progress. Do not trigger on first iteration.
			break
		}

		oldNear = near

		newNear := make([]frame.Input, 0, len(near.inputs))

		// alter near to be closer to target
		for j, nearInput := range near.inputs {
			if nearInput.Value == target.inputs[j].Value {
				newNear = append(newNear, nearInput)
			} else {
				v1, v2 := nearInput.Value, target.inputs[j].Value
				newVal := math.Min(mp.qstep[j], math.Abs(v2-v1))
				// get correct sign
				newVal *= (v2 - v1) / math.Abs(v2-v1)
				newNear = append(newNear, frame.Input{nearInput.Value + newVal})
			}
		}
		// if we are not meeting a constraint, gradient descend to the constraint
		newNear = mp.constrainNear(opt, oldNear.inputs, newNear)
		if newNear != nil {
			// constrainNear will ensure path between oldNear and newNear satisfies constraints along the way
			near = &solution{newNear}
			rrtMap[near] = oldNear
		} else {
			break
		}
	}
	return oldNear
}

// constrainNear will do a IK gradient descent from seedInputs to target. If a gradient descent distance function has
// been specified, this will use that.
func (mp *cBiRRTMotionPlanner) constrainNear(opt *PlannerOptions, seedInputs, target []frame.Input) []frame.Input {
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
		mp.frame}, mp.Resolution())
	if ok {
		return target
	}

	solutionGen := make(chan []frame.Input, 1)
	// This should run very fast and does not need to be cancelled. A cancellation will be caught above in Plan()
	ctx := context.Background()
	// Spawn the IK solver to generate solutions until done
	err = mp.fastGradDescent.Solve(ctx, solutionGen, goalPos, target)
	// We should have zero or one solutions
	var solved []frame.Input
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
		if failpos != nil && inputDist(target, failpos.StartInput) > mp.solDist {
			// If we have a first failing position, and that target is updating (no infinite loop), then recurse
			return mp.constrainNear(opt, seedInputs, failpos.StartInput)
		}
		return nil
	}

	return solved
}

// SmoothPath will pick two points at random along the path and attempt to do a fast gradient descent directly between
// them, which will cut off randomly-chosen points with odd joint angles into something that is a more intuitive motion.
func (mp *cBiRRTMotionPlanner) SmoothPath(ctx context.Context, opt *PlannerOptions, inputSteps [][]frame.Input) [][]frame.Input {

	iter := int(math.Max(float64(mp.iter-len(inputSteps)*len(inputSteps)), float64(mp.iter-200)))

	for iter < mp.iter && len(inputSteps) > 2 {
		iter++
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		// Pick two random non-adjacent indices
		j := 2 + rand.Intn(len(inputSteps)-2)
		i := rand.Intn(j)
		shortcutGoal := make(map[*solution]*solution)

		iSol := &solution{inputSteps[i]}
		jSol := &solution{inputSteps[j]}
		shortcutGoal[jSol] = nil

		// extend backwards for convenience later. Should work equally well in both directions
		reached := mp.constrainedExtend(opt, shortcutGoal, jSol, iSol)

		// Note this could technically replace paths with "longer" paths i.e. with more waypoints.
		// However, smoothed paths are invariably more intuitive and smooth, and lend themselves to future shortening,
		// so we allow elongation here.
		if inputDist(inputSteps[i], reached.inputs) < mp.solDist {
			newInputSteps := append([][]frame.Input{}, inputSteps[:i]...)
			for reached != nil {
				newInputSteps = append(newInputSteps, reached.inputs)
				reached = shortcutGoal[reached]
			}
			newInputSteps = append(newInputSteps, inputSteps[j+1:]...)
			inputSteps = newInputSteps
		}
	}
	return inputSteps
}

// getFrameSteps will return a slice of positive values representing the largest amount a particular DOF of a frame should
// move in any given step.
func getFrameSteps(f frame.Frame, by float64) []float64 {
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

func nearestNeighbor(seed *solution, rrtMap map[*solution]*solution) *solution {
	bestDist := math.Inf(1)
	var best *solution
	for k := range rrtMap {
		dist := inputDist(seed.inputs, k.inputs)
		if dist < bestDist {
			bestDist = dist
			best = k
		}
	}
	return best
}
