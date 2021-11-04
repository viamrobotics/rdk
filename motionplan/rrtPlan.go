package motionplan

import (
	"context"
	"errors"
	"math"
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

func NewRRTMotionPlanner(frame frame.Frame, logger golog.Logger, nCPU int) (MotionPlanner, error) {
	ik, err := kinematics.CreateCombinedIKSolver(frame, logger, nCPU)
	if err != nil {
		return nil, err
	}
	mp := &rrtMotionPlanner{solver: ik, frame: frame, logger: logger}
	mp.AddConstraint("jointSwingScorer", NewJointScorer())
	return mp, nil
}

// MotionPlanner defines a struct able to plan motion
type rrtMotionPlanner struct {
	constraintHandler
	solver kinematics.InverseKinematics
	frame   frame.Frame
	logger        golog.Logger
}

type solution struct {
	inputs []frame.Input
}

func (mp *rrtMotionPlanner) Frame() frame.Frame {
	return mp.frame
}

func (mp *rrtMotionPlanner) Plan(ctx context.Context, goal *pb.ArmPosition, seed []frame.Input) ([][]frame.Input, error) {
	var inputSteps [][]frame.Input
	
	// How many of the top solutions to try
	nSolutions := 5
	// How close to get to the solution
	solDist := 0.5

	seedPos, err := mp.frame.Transform(seed)
	if err != nil {
		return nil, err
	}
	goalPos := spatial.NewPoseFromArmPos(fixOvIncrement(goal, spatial.PoseToArmPos(seedPos)))
	
	solutionGen := make(chan []frame.Input)
	ikErr := make(chan error)
	ctxWithCancel, cancel := context.WithCancel(ctx)
	
	// Spawn the IK solver to generate solutions until done
	go func(){
		defer close(ikErr)
		ikErr <- mp.solver.Solve(ctxWithCancel, solutionGen, goalPos, seed)
	}()
	
	solutions := map[float64][]frame.Input{}
	
	// Solve the IK solver
	IK:
	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("context Done signal")
		case step := <- solutionGen:
			cPass, cScore := mp.CheckConstraints(constraintInput{
				seedPos,
				goalPos,
				seed,
				step,
				mp.frame})
			
			//~ fmt.Println(cPass, cScore)
			if cPass {
				solutions[cScore] = step
			}
			// Skip the return check below until we have nothing left to read from solutionGen
			continue IK
		default:
		}
		
		select{
		case err = <- ikErr:
			// If we have a return from the IK solver, there are no more solutions, so we finish processing above
			// until we've drained the channel
			break IK
		default:
		}
	}
	mp.logger.Debug("done, cancelling")
	cancel()
	//~ close(solutionGen)
	//~ close(ikErr)
	mp.logger.Debug("closed")
	if len(solutions) == 0 {
		mp.logger.Debug("no solutions")
		return nil, errors.New("unable to solve for position")
	}
	mp.logger.Debug("getting best")
	// Get the N best solutions
	valid := make([][]frame.Input, 0, nSolutions)
	keys := make([]float64, 0, len(solutions))
	for k := range solutions {
		keys = append(keys, k)
	}
	sort.Float64s(keys)
	for _, k := range keys[:nSolutions] {
		valid = append(valid, solutions[k])
	}
	mp.logger.Debug("got valid")
	
	rrtMap := make(map[*solution]*solution)
	current := &solution{seed}
	rrtMap[current] = nil
	
	orig := &solution{seed}
	
	i := 0
	
	// prune solutions worse than this
	maxDist := 3 * inputDist(seed, valid[0])
	
	mp.logger.Debug("making RRT map")
	for checkValid(seed, valid, solDist) < 0 {
		select {
		case <-ctx.Done():
			return nil, errors.New("context Done signal")
		default:
		}
		if i >= 2000000 {
			return nil, errors.New("no path, RRT did not happen to hit the goal")
		}
		
		if i % 10000 == 0 {
			fmt.Println(len(rrtMap), "/", i)
		}
		i++
		
		seed = frame.RandomFrameInputs(mp.frame, nil)
		
		if inputDist(seed, valid[0]) > maxDist || inputDist(seed, orig.inputs) > maxDist {
			continue
		}
		//~ fmt.Println(orig.inputs)
		
		current = &solution{seed}
		bestDist := math.Inf(1)
		var best *solution
		for k, _ := range rrtMap {
			dist := inputDist(seed, k.inputs)
			//~ fmt.Println(dist)
			if dist < bestDist {
				best = k
			}
		}
		rrtMap[current] = best
	}
	fmt.Println("done! constructing path")
	// got a good solution, trace to beginning
	parent := current
	for parent != nil {
		inputSteps = append(inputSteps, parent.inputs)
		parent = rrtMap[parent]
	}
	// reverse the slice
	for i, j := 0, len(inputSteps)-1; i < j; i, j = i+1, j-1 {
		inputSteps[i], inputSteps[j] = inputSteps[j], inputSteps[i]
	}
	
	inputSteps = append(inputSteps, valid[checkValid(seed, valid, solDist)])
	
	return inputSteps, nil
}

// return index of valid solution, -1 otherwise
func checkValid(seed []frame.Input, valid [][]frame.Input, thresh float64) int {
	for i, to := range valid {
		if inputDist(seed, to) < thresh {
			return i
		}
	}
	return -1
}

func inputDist(from, to []frame.Input) float64 {
	dist := 0.
	for i, f := range from {
		dist += math.Pow(to[i].Value - f.Value, 2)
	}
	return dist
}
