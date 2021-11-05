package motionplan

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"fmt"
	"sort"

	"github.com/edaniels/golog"
	//~ "github.com/golang/geo/r3"

	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	//~ "go.viam.com/core/utils"
)

func NewCBiRRTMotionPlanner(frame frame.Frame, logger golog.Logger, nCPU int) (*cBiRRTMotionPlanner, error) {
	ik, err := kinematics.CreateCombinedIKSolver(frame, logger, nCPU)
	if err != nil {
		return nil, err
	}
	nlopt, err := kinematics.CreateNloptIKSolver(frame, logger, 1)
	if err != nil {
		return nil, err
	}
	// nlopt should try only once
	nlopt.SetMaxIter(1)
	mp := &cBiRRTMotionPlanner{solver: ik, fastGradDescent: nlopt, frame: frame, logger: logger, solDist: 0.01}
	
	// Max individual step of 0.5% of full range of motion
	mp.qstep = getFrameSteps(frame, 0.015)
	mp.iter = 2000
	//~ mp.iter = 1
	
	mp.AddConstraint("jointSwingScorer", NewJointScorer())
	mp.AddConstraint("obstacle", fakeObstacle)
	mp.AddConstraint("orientation", NewPoseConstraint())
	
	return mp, nil
}

// MotionPlanner defines a struct able to plan motion
type cBiRRTMotionPlanner struct {
	constraintHandler // joint movement minimization, collision detection, etc can be handled here
	solDist float64
	solver kinematics.InverseKinematics
	fastGradDescent *kinematics.NloptIK
	frame   frame.Frame
	distFunc   func(spatial.Pose, spatial.Pose) float64
	logger     golog.Logger
	qstep      []float64
	iter       int
}

func (mp *cBiRRTMotionPlanner) Frame() frame.Frame {
	return mp.frame
}

func (mp *cBiRRTMotionPlanner) Plan(ctx context.Context, goal *pb.ArmPosition, seed []frame.Input) ([][]frame.Input, error) {
	inputSteps := [][]frame.Input{}
	seedCopy := make([]frame.Input, 0, len(seed))
	for _, s := range seed {
		seedCopy = append(seedCopy, s)
	}
	
	// How many of the top solutions to try
	nSolutions := 5

	solutions, err := getSolutions(ctx, mp.frame, mp.solver, goal, seed, mp.constraintHandler)
	if err != nil {
		return nil, err
	}

	mp.logger.Debug("getting best")
	// Get the N best solutions
	keys := make([]float64, 0, len(solutions))
	for k := range solutions {
		keys = append(keys, k)
	}
	sort.Float64s(keys)

	goalMap := make(map[*solution]*solution)

	if len(keys) < nSolutions {
		nSolutions = len(keys)
	}

	for _, k := range keys[:nSolutions] {
		goalMap[&solution{solutions[k]}] = nil
	}
	mp.logger.Debug("got valid")
	
	seedMap := make(map[*solution]*solution)
	seedMap[&solution{seed}] = nil
	
	mp.logger.Debug("making RRT map")
	
	// Alternate to which map our random sample is added
	// for the first iteration, we try the 0.5 interpolation between seed and goal[0]
	addSeed := true
	target := &solution{frame.InterpolateInputs(seed, solutions[keys[0]], 0.5)}
	
	
	for i := 0; i < mp.iter; i++ {
		select {
		case <-ctx.Done():
			return nil, errors.New("context Done signal")
		default:
		}
		if i % 10 == 0 {
			fmt.Println("i:", i, len(seedMap), len(goalMap))
		}
		
		var seedReached, goalReached *solution
		
		// Alternate which tree we extend
		if addSeed {
			// extend seed tree first
			nearest := nearestNeighbor(target, seedMap)
			seedReached, goalReached = mp.constrainedExtendWrapper(seedMap, goalMap, nearest, target)
		}else{
			// extend goal tree first
			nearest := nearestNeighbor(target, goalMap)
			goalReached, seedReached = mp.constrainedExtendWrapper(goalMap, seedMap, nearest, target)
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
			// extract the path to the goal
			for goalReached != nil {
				inputSteps = append(inputSteps, goalReached.inputs)
				goalReached = goalMap[goalReached]
			}
			inputSteps = mp.smoothPath(ctx, inputSteps)
			mp.logger.Debug("got path!")
			return inputSteps, nil
		}
		//~ mp.fastGradDescent.SetDistFunc(constantOrient(500))
		target = &solution{frame.RandomFrameInputs(mp.frame, nil)}
		//~ // Guarantee random sample meets constraints
		//~ target.inputs = mp.constrainNear(target.inputs, target.inputs)
		//~ for len(target.inputs) == 0 {
			//~ select {
			//~ case <-ctx.Done():
				//~ return nil, errors.New("context Done signal")
			//~ default:
			//~ }
			//~ target = &solution{frame.RandomFrameInputs(mp.frame, nil)}
			//~ target.inputs = mp.constrainNear(target.inputs, target.inputs)
		//~ }
		mp.fastGradDescent.SetDistFunc(constantOrient(5))
		addSeed = !addSeed
	}
	
	return nil, errors.New("could not solve path")
}

func (mp *cBiRRTMotionPlanner) constrainedExtendWrapper(m1, m2 map[*solution]*solution, near1, target *solution) (*solution, *solution) {
	//~ fmt.Println("wrap start")
	// Extend tree m1 as far towards target as it can get. It may or may not reach it.
	m1reach := mp.constrainedExtend(m1, near1, target)
	
	//~ fmt.Println("wrapped 1")
	// Find the nearest point in m2 to the furthest point reached in m1
	near2 := nearestNeighbor(m1reach, m2)
	
	//~ fmt.Println("wrapping 2")
	// extend m2 towards the point in m1
	m2reach := mp.constrainedExtend(m2, near2, m1reach)
	//~ fmt.Println("wrap end")
	
	return m1reach, m2reach
}

func (mp *cBiRRTMotionPlanner) constrainedExtend(rrtMap map[*solution]*solution, near, target *solution) *solution {
	oldNear := near
	// How close to get to the solution
	i := 0
	for {
		i++
		if inputDist(near.inputs, target.inputs) < mp.solDist {
			return near
		} else if inputDist(near.inputs, target.inputs) > inputDist(oldNear.inputs, target.inputs) {
			return oldNear
		} else if i > 2 && inputDist(near.inputs, oldNear.inputs) < mp.solDist*mp.solDist*mp.solDist {
			return oldNear
		}
		
		oldNear = near
		
		newNear := make([]frame.Input, 0, len(near.inputs))
		
		// alter near to be closer to target
		for i, nearInput := range near.inputs {
			if nearInput.Value == target.inputs[i].Value {
				newNear = append(newNear, nearInput)
			}else{
				v1, v2 := nearInput.Value, target.inputs[i].Value
				newVal := math.Min(mp.qstep[i], math.Abs(v2 - v1))
				// get correct sign
				newVal *= (v2 - v1)/math.Abs(v2 - v1)
				newNear = append(newNear, frame.Input{nearInput.Value + newVal})
			}
		}
		// if we are not meeting a constraint, gradient descend to the constraint
		newNear = mp.constrainNear(oldNear.inputs, newNear)
		if newNear != nil {
			// ensure path between oldNear and newNear satisfys constraints along the way
			ok := mp.CheckConstraintPath(constraintInput{startInput: oldNear.inputs, endInput: newNear, frame: mp.frame})
			if ok {
				near = &solution{newNear}
				if oldNear == nil {
				}
				rrtMap[near] = oldNear
			}else{
				return oldNear
			}
		}else{
			return oldNear
		}
	}
}

// constrainNear will do a IK gradient descent from seedInputs to target. If a gradient descent distance function has
// been specified, this will 
func (mp *cBiRRTMotionPlanner) constrainNear(seedInputs, target []frame.Input) []frame.Input {
	seedPos, err := mp.frame.Transform(seedInputs)
	if err != nil{
		return nil
	}
	goalPos, err := mp.frame.Transform(target)
	if err != nil{
		return nil
	}
	//~ fmt.Println("checking")
	// Check if constraints need to be met
	ok, _ := mp.CheckConstraints(constraintInput{
				seedPos,
				goalPos,
				seedInputs,
				target,
				mp.frame})
	//~ fmt.Println("checked")
	if ok {
		return target
	}
	
	//~ fmt.Println("solving")
	solutionGen := make(chan []frame.Input, 1)
	// This should run very fast and does not need to be cancelled. A cancellation will be caught above in Plan()
	ctx := context.Background()
	// Spawn the IK solver to generate solutions until done
	err = mp.fastGradDescent.Solve(ctx, solutionGen, goalPos, seedInputs)
	// We should have zero or one solutions
	var solved []frame.Input
	select{
		case solved = <- solutionGen:
		default:
	}
	close(solutionGen)
	if err != nil {
		return nil
	}
	return solved
}

func (mp *cBiRRTMotionPlanner) smoothPath(ctx context.Context, inputSteps [][]frame.Input) [][]frame.Input {
	fmt.Println("smoothing path of len", len(inputSteps))
	
	iter := int(math.Max(float64(mp.iter - len(inputSteps)*len(inputSteps)), float64(mp.iter - 500)))
	
	//~ iter = mp.iter - 8
	
	for iter < mp.iter && len(inputSteps) > 2 {
		//~ fmt.Println("before", inputSteps)
		iter++
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		j := 2 + rand.Intn(len(inputSteps) - 2)
		i := rand.Intn(j)
		
		//~ fmt.Println("i, j", i, j)
		
		shortcut := make(map[*solution]*solution)
		
		iSol := &solution{inputSteps[i]}
		jSol := &solution{inputSteps[j]}
		shortcut[jSol] = nil
		
		// extend backwards for convenience later. Should work equally well in both directions
		reached := mp.constrainedExtend(shortcut, jSol, iSol)
		if inputDist(inputSteps[i], reached.inputs) < mp.solDist && len(shortcut) <= j - i {
			newInputSteps := inputSteps[:i]
			for reached != nil {
				newInputSteps = append(newInputSteps, reached.inputs)
				reached = shortcut[reached]
			}
			newInputSteps = append(newInputSteps, inputSteps[j+1:]...)
			inputSteps = newInputSteps
			fmt.Println("smoothed to", len(inputSteps))
		}
	}
	//~ fmt.Println("final", len(inputSteps), iter, inputSteps)
	return inputSteps
}

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
	for k, _ := range rrtMap {
		dist := inputDist(seed.inputs, k.inputs)
		if dist < bestDist {
			bestDist = dist
			best = k
		}
	}
	return best
}

// betterPath returns whether the new path has less joint swing than the old one
func betterPath(new, old [][]frame.Input) bool {
	newDist := 0.
	oldDist := 0.
	for i, sol := range new {
		if i == 0 {
			continue
		}
		newDist += inputDist(sol, new[i-1])
	}
	for i, sol := range old {
		if i == 0 {
			continue
		}
		oldDist += inputDist(sol, old[i-1])
	}
	return newDist < oldDist
}
