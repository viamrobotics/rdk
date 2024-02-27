//go:build !no_cgo

// Package kinematicbase contains wrappers that augment bases with information needed for higher level
// control over the base
package kinematicbase

import (
	"context"
	"math"
	"time"

	"github.com/golang/geo/r3"
	"go.uber.org/multierr"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	inputUpdateStepSeconds    = 0.2 // Update CurrentInputs (and check deviation) every this many seconds.
	lookaheadTimeSeconds = 1.
	stepDistResolution = 1.  // Before post-processing trajectory will have velocities every this many mm (or degs if spinning in place)
	
	// Used to determine minimum linear deviation allowed before correction attempt. Determined by multiplying max linear speed by 
	// inputUpdateStepSeconds, and will correct if deviation is larger than this percent of that amount.
	minDeviationToCorrectPct = 30.
)

type arcStep struct {
	linVelMMps      r3.Vector
	angVelDegps     r3.Vector
	durationSeconds float64

	// A single trajectory may be broken into multiple arcSteps, so we need to be able to track the total distance elapsed through
	// the trajectory
	startDist float64
	ptgIdx float64
	
	// Pose at dist=0 for the PTG these traj nodes are derived from, such that Compose(trajStartPose, TrajNode.Pose) is the expected
	// pose at that node.
	trajStartPose spatialmath.Pose
	subTraj []*tpspace.TrajNode
}

type courseCorrectionGoal struct {
	Goal spatialmath.Pose
	Solution []referenceframe.Input
	stepIdx int
	trajIdx int
}

func (ptgk *ptgBaseKinematics) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	defer func() {
		ptgk.inputLock.Lock()
		ptgk.currentInputs = zeroInput
		ptgk.inputLock.Unlock()
	}()
	
	tryStop := func(errToWrap error) error {
		stopCtx, cancelFn := context.WithTimeout(context.Background(), time.Second*5)
		defer cancelFn()
		return multierr.Combine(errToWrap, ptgk.Base.Stop(stopCtx, nil))
	}
	
	startPose := spatialmath.NewZeroPose() // This is the location of the base at call time
	if ptgk.Localizer != nil {
		startPoseInFrame, err := ptgk.Localizer.CurrentPosition(ctx)
		if err != nil {
			return tryStop(err)
		}
		startPose = startPoseInFrame.Pose()
	}
	
	// Pre-process all steps into a series of velocities
	arcSteps, err := ptgk.arcStepsFromInputs(inputSteps, startPose)
	if err != nil {
		return tryStop(err)
	}

	for i, step := range arcSteps {
		alpha := step.subTraj[0].Alpha
		if step.ptgIdx >= 0 {
			// Only update inputs if we are not in a corrective arc
			ptgk.inputLock.Lock() // In the case where there's actual contention here, this could cause timing issues; how to solve?
			ptgk.currentInputs = []referenceframe.Input{{step.ptgIdx}, {alpha}, {step.startDist}}
			ptgk.inputLock.Unlock()
		}

		timestep := time.Duration(step.durationSeconds*1000*1000) * time.Microsecond

		ptgk.logger.CDebugf(ctx,
			"setting velocity to linear %v angular %v and running velocity step for %s",
			step.linVelMMps,
			step.angVelDegps,
			timestep,
		)

		err := ptgk.Base.SetVelocity(
			ctx,
			step.linVelMMps,
			step.angVelDegps,
			nil,
		)
		if err != nil {
			return tryStop(err)
		}
		arcStartTime := time.Now()
		ptgk.logger.Debug(step)
		// Now we are moving. We need to do several things simultaneously:
		// - move until we think we have finished the arc, then move on to the next step
		// - update our CurrentInputs tracking where we are through the arc
		// - Check where we are relative to where we think we are, and tweak velocities accordingly
		for timeElapsed := inputUpdateStepSeconds; timeElapsed <= step.durationSeconds; timeElapsed += inputUpdateStepSeconds {
			// Account for 1) timeElapsed being inputUpdateStepSeconds ahead of actual elapsed time, and the fact that the loop takes nonzero time
			// to run especially when using the localizer.
			actualTimeElapsed := time.Since(arcStartTime)
			remainingTimeStep := time.Duration(1000*1000*timeElapsed)*time.Microsecond - actualTimeElapsed
			
			if remainingTimeStep > 0 {
				utils.SelectContextOrWait(ctx, remainingTimeStep)
				if ctx.Err() != nil {
					return tryStop(ctx.Err())
				}
			}
			distIncVel := step.linVelMMps.Y
			if distIncVel == 0 {
				distIncVel = step.angVelDegps.Z
			}
			currentInputs := []referenceframe.Input{{step.ptgIdx}, {alpha}, {step.startDist + math.Abs(distIncVel) * timeElapsed}}
			if step.ptgIdx >= 0 {
				// Only update inputs if we are not in a corrective arc
				ptgk.inputLock.Lock()
				ptgk.currentInputs = currentInputs
				ptgk.inputLock.Unlock()
			}
			
			// If we have a localizer, we are able to attempt to correct to stay on the path.
			if ptgk.Localizer != nil {
				actualPose, err := ptgk.Localizer.CurrentPosition(ctx)
				if err != nil {
					return err
				}
				expectedPoseRel, err := ptgk.frame.Transform(currentInputs)
				if err != nil {
					return err
				}
				
				// This is where we expected to be on the trajectory
				expectedPose := spatialmath.Compose(step.trajStartPose, expectedPoseRel)
				
				// This is where actually are on the trajectory
				poseDiff := spatialmath.PoseBetween(actualPose.Pose(), expectedPose)
				
				allowableDiff := ptgk.linVelocityMMPerSecond * inputUpdateStepSeconds * (minDeviationToCorrectPct/100)
				ptgk.logger.Debug("allowable diff ", allowableDiff)
				ptgk.logger.Debug("diff now ", poseDiff.Point().Norm())
				if poseDiff.Point().Norm() > allowableDiff {
					ptgk.logger.Debug("correcting")
					// Accumulate list of points along the path to try to connect to
					goalsToAttempt := int(lookaheadTimeSeconds / inputUpdateStepSeconds) + 1
					goals := nPosesPastDist(i, goalsToAttempt, currentInputs[distanceAlongTrajectoryIndex].Value, actualPose.Pose(), arcSteps)

					// Attempt to solve from `actualPose` to each of those points
					ptgk.logger.Debug("calling course correct")
					solution, err := ptgk.courseCorrect(ctx, goals)
					if err != nil {
						ptgk.logger.Debug(err)
					}
					if solution.Solution != nil {
						ptgk.logger.Debug("got new solution")
						
						// We've got a course correction solution. Swap out the relevant arcsteps.
						correctiveTraj, err := ptgk.courseCorrectionSolver.Trajectory(solution.Solution[0].Value, solution.Solution[1].Value, stepDistResolution)
						if err != nil {
							ptgk.logger.Debug(err)
							continue
						}
						correctiveArcSteps := ptgk.trajectoryToArcSteps(correctiveTraj, actualPose.Pose(), -1)
						
						// Update the connection point
						connectionPoint := arcSteps[solution.stepIdx]
						connectionPoint.startDist += connectionPoint.subTraj[solution.trajIdx].Dist
						connectionPoint.subTraj = connectionPoint.subTraj[solution.trajIdx:]
						connectionPoint.durationSeconds -= (stepDistResolution * float64(solution.trajIdx))
						
						newArcSteps := arcSteps[:i]
						newArcSteps = append(newArcSteps, correctiveArcSteps...)
						newArcSteps = append(newArcSteps, connectionPoint)
						newArcSteps = append(newArcSteps, arcSteps[solution.stepIdx+1:]...)
						arcSteps = newArcSteps
						// Break our timing loop to go to the next step
						break
					}
					ptgk.logger.Debug("no new solution")
				}
			}
		}
	}
	return tryStop(nil)
}

func (ptgk *ptgBaseKinematics) arcStepsFromInputs(inputSteps [][]referenceframe.Input, startPose spatialmath.Pose) ([]arcStep, error) {
	var arcSteps []arcStep
	runningPose := startPose
	for _, inputs := range inputSteps {
		selectedPTG := ptgk.ptgs[int(math.Round(inputs[ptgIndex].Value))]

		selectedTraj, err := selectedPTG.Trajectory(
			inputs[trajectoryIndexWithinPTG].Value,
			inputs[distanceAlongTrajectoryIndex].Value,
			stepDistResolution,
		)
		if err != nil {
			return nil, err
		}
		trajArcSteps := ptgk.trajectoryToArcSteps(selectedTraj, runningPose, inputs[ptgIndex].Value)
		for _, step := range trajArcSteps {
			step.ptgIdx = inputs[ptgIndex].Value
		}
		
		arcSteps = append(arcSteps, trajArcSteps...)
		runningPose = spatialmath.Compose(runningPose, selectedTraj[len(selectedTraj)-1].Pose)
	}
	return arcSteps, nil
}

func (ptgk *ptgBaseKinematics) trajectoryToArcSteps(traj []*tpspace.TrajNode, startPose spatialmath.Pose, ptgIdx float64) []arcStep {
	finalSteps := []arcStep{}
	timeStep := 0.
	curDist := 0.
	// Trajectory distance is either length in mm, or if linear distance is not increasing, number of degrees to rotate in place.
	lastLinVel := r3.Vector{0, traj[0].LinVel * ptgk.linVelocityMMPerSecond, 0}
	lastAngVel := r3.Vector{0, 0, traj[0].AngVel * ptgk.angVelocityDegsPerSecond}
	nextStep := arcStep{
		linVelMMps:      lastLinVel,
		angVelDegps:     lastAngVel,
		startDist: curDist,
		durationSeconds: 0,
		ptgIdx: ptgIdx,
		trajStartPose: startPose,
	}
	for _, trajPt := range traj {
		nextStep.subTraj = append(nextStep.subTraj, trajPt)
		nextLinVel := r3.Vector{0, trajPt.LinVel * ptgk.linVelocityMMPerSecond, 0}
		nextAngVel := r3.Vector{0, 0, trajPt.AngVel * ptgk.angVelocityDegsPerSecond}
		if nextStep.linVelMMps.Sub(nextLinVel).Norm2() > 1e-6 || nextStep.angVelDegps.Sub(nextAngVel).Norm2() > 1e-6 {
			// Changed velocity, make a new step
			nextStep.durationSeconds = timeStep
			finalSteps = append(finalSteps, nextStep)
			nextStep = arcStep{
				linVelMMps:      nextLinVel,
				angVelDegps:     nextAngVel,
				startDist: curDist,
				durationSeconds: 0,
				ptgIdx: ptgIdx,
				trajStartPose: startPose,
			}
			timeStep = 0.
		}
		distIncrement := trajPt.Dist - curDist
		curDist += distIncrement
		if nextStep.linVelMMps.Y != 0 {
			timeStep += distIncrement / (math.Abs(nextStep.linVelMMps.Y))
		} else if nextStep.angVelDegps.Z != 0 {
			timeStep += distIncrement / (math.Abs(nextStep.angVelDegps.Z))
		}
	}
	nextStep.durationSeconds = timeStep
	finalSteps = append(finalSteps, nextStep)
	return finalSteps
}

func (ptgk *ptgBaseKinematics) courseCorrect(ctx context.Context, goals []courseCorrectionGoal) (courseCorrectionGoal, error)  {
	for _, goal := range goals {
		solveMetric := ik.NewPosWeightSquaredNormMetric(goal.Goal)
		solutionChan := make(chan *ik.Solution, 1)
		ptgk.logger.Debug("attempting goal")
		ptgk.logger.Debug(spatialmath.PoseToProtobuf(goal.Goal))
		err := ptgk.courseCorrectionSolver.Solve(
			ctx,
			solutionChan,
			nil,
			solveMetric,
			0,
		)
		if err != nil {
			ptgk.logger.Debug("non nil err")
			return courseCorrectionGoal{}, err
		}
		var solution *ik.Solution
		ptgk.logger.Debug("selecting solution")
		select {
		case solution = <-solutionChan:
		default:
		}
		ptgk.logger.Debug("done")
		ptgk.logger.Debug(solution)
		
		if solution.Score < 1. {
			ptgk.logger.Debug("got solution")
			goal.Solution = solution.Configuration
			return goal, nil
		}
	}
	ptgk.logger.Debug("unable to course correct")
	return courseCorrectionGoal{}, nil
}
func nPosesPastDist(currStep, nGoals int, currDist float64, currPose spatialmath.Pose, steps []arcStep) []courseCorrectionGoal {
	goals := []courseCorrectionGoal{}
	
	for i := currStep; i < len(steps); i++ {
		pastDist := false
		for j, trajNode := range steps[i].subTraj {
			if pastDist {
				goalPose := spatialmath.PoseBetween(currPose, spatialmath.Compose(steps[i].trajStartPose, trajNode.Pose))
				goals = append(goals, courseCorrectionGoal{Goal: goalPose, stepIdx: i, trajIdx: j})
				if len(goals) == nGoals {
					return goals
				}
			} else {
				if trajNode.Dist >= currDist {
					pastDist = true
					currDist = 0
				}
			}
		}
	}
	return goals
}
