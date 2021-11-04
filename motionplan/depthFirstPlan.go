package motionplan

import (
	"context"
	"errors"
	"math"
	"fmt"
	//~ "sync"

	//~ "github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	//~ "go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	//~ "go.viam.com/core/utils"
)


func getNextPosTries(seedPos, goalPos spatial.Pose) []spatial.Pose {
	var toTry []spatial.Pose
	intPos := spatial.Interpolate(seedPos, goalPos, 0.5)
	
	if getSteps(seedPos, intPos) > 1 {
		toTry = append(toTry, intPos)
	}
	// If the step forwards is too small, step outwards by 5mm perpendicular to the vector to the goal position
	// TODO (pl): this will only work well if this is primarily a translational movement. If we are mainly instead
	ovPt := goalPos.Point().Sub(seedPos.Point())
	ov := &spatial.OrientationVector{OX:ovPt.X, OY: ovPt.Y, OZ:ovPt.Z}
	movementOV := spatial.NewPoseFromOrientationVector(r3.Vector{}, ov)
	step := spatial.NewPoseFromPoint(r3.Vector{10,0,1})
	toTry = append(toTry, spatial.Compose(seedPos, spatial.NewPoseFromPoint(spatial.Compose(movementOV, step).Point())))
	step = spatial.NewPoseFromPoint(r3.Vector{0,10,1})
	toTry = append(toTry, spatial.Compose(seedPos, spatial.NewPoseFromPoint(spatial.Compose(movementOV, step).Point())))
	step = spatial.NewPoseFromPoint(r3.Vector{-10,0,1})
	toTry = append(toTry, spatial.Compose(seedPos, spatial.NewPoseFromPoint(spatial.Compose(movementOV, step).Point())))
	step = spatial.NewPoseFromPoint(r3.Vector{0,-10,1})
	toTry = append(toTry, spatial.Compose(seedPos, spatial.NewPoseFromPoint(spatial.Compose(movementOV, step).Point())))

	return toTry
}

// tryDepthSolve will attempt to solve between seed and goal, branching out from the direct path if necessary, and
// will get as close to 
func (mp *linearMotionPlanner) tryDepthSolve(ctx context.Context, seed []frame.Input, goalPos spatial.Pose, recurse bool) ([][]frame.Input, error){
	select {
	case <-ctx.Done():
		break
	default:
	}
	
	seedPos, err := mp.frame.Transform(seed)
	if err != nil {
		return nil, err
	}

	var allSteps [][]frame.Input
	var step []frame.Input
	
	solutionGen := make(chan []frame.Input)
	ikErr := make(chan error)
	ctxWithCancel, cancel := context.WithCancel(ctx)
	
	// Spawn the IK solver to generate solutions until done
	go func(){
		defer close(ikErr)
		ikErr <- mp.solver.Solve(ctxWithCancel, solutionGen, goalPos, seed)
	}()
	
	solutions := map[float64][]frame.Input{}
	
	done := false
	solverReturned := false
	
	// Solve the IK solver for the 
	for !done {
		select {
		case <-ctx.Done():
			return nil, errors.New("context Done signal")
		case step = <- solutionGen:
			cPass, cScore := mp.CheckConstraints(constraintInput{
				seedPos,
				goalPos,
				seed,
				step,
				mp.frame})
			
			
			if cPass {
				// collision check if supported
				if cScore < mp.idealMovementScore {
					// If the movement scores SO well, we will perform that movement immediately rather than
					// trying for a better one
					solutions = map[float64][]frame.Input{}
					solutions[cScore] = step
					done = true
				}else{
					solutions[cScore] = step
				}
			}
			// Skip the return check below until we have nothing left to read from solutionGen
			continue
		default:
		}
		
		select{
		case err = <- ikErr:
			// If we have a return from the IK solver, there are no more solutions, so we finish processing above
			// until we've drained the channel
			done = true
			solverReturned = true
		default:
		}
	}
	mp.logger.Debug("done, cancelling")
	cancel()
	if !solverReturned{
		err = <- ikErr
		mp.logger.Debug("got IK return", err)
	}
	if err != nil {
		mp.logger.Debug("got solution but IK returned ignorable error ", err)
	}
	close(solutionGen)
	if len(solutions) != 0 {
		// we were able to directly solve what was requested of us. Return the one step.
		minScore := math.Inf(1)
		for score, solution := range solutions {
			if score < minScore {
				step = solution
			}
		}
		return [][]frame.Input{step}, nil
	}
	// No valid solutions, split and try again
	fmt.Println("no solutions, trying again")
	if recurse {
		toTry := getNextPosTries(seedPos, goalPos)
		
		for i, attemptPos := range toTry {
			
			// check goal is valid
			//~ if !mp.validityCheck(nil, attemptPos, mp.frame) {
				//~ continue
			//~ }
			
			voxelPt := attemptPos.Point()
			voxel := r3.Vector{float64(int(voxelPt.X)), float64(int(voxelPt.Y)), float64(int(voxelPt.Z))}
			if mp.visited[voxel] {
				continue
			}
			mp.visited[voxel] = true
			
			// Do not recurse if splitting
			if len(toTry) < 5 || i > 1 {
				fmt.Println("trying split", i)
				fmt.Println(spatial.PoseToArmPos(attemptPos))
				//~ recurse = false
			}
			steps, err := mp.tryDepthSolve(ctx, seed, attemptPos, recurse)
			if err != nil {
				mp.logger.Debug("tryDepthSolve returned ignorable error ", err)
			}
			if len(steps) > 0 {
				fmt.Println("got steps!", steps)
				// Got a good solution! Try to solve for the goal again
				allSteps = append(allSteps, steps...)
				steps, err = mp.tryDepthSolve(ctx, allSteps[len(allSteps)-1], goalPos, true)
				if err != nil {
					mp.logger.Debug("tryDepthSolve returned ignorable error ", err)
				}
				if len(steps) > 0 {
					return append(allSteps, steps...), nil
				}
			}
		}
	}
	return nil, errors.New("unable to find valid DFS path")
}

func (mp *linearMotionPlanner) splittingLinearPlan(ctx context.Context, goal *pb.ArmPosition, seed []frame.Input) ([][]frame.Input, error) {
	seedPos, err := mp.frame.Transform(seed)
	if err != nil {
		return nil, err
	}
	
	goalPos := spatial.NewPoseFromArmPos(fixOvIncrement(goal, spatial.PoseToArmPos(seedPos)))
	return mp.tryDepthSolve(ctx, seed, goalPos, true)

}
