// Package kinematicbase contains wrappers that augment bases with information needed for higher level
// control over the base
package kinematicbase

import (
	"context"
	"errors"
	"fmt"
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
	updateStepSeconds = 0.35 // Update CurrentInputs and check deviation every this many seconds.
	lookaheadDistMult = 2.   // Look ahead distance for path correction will be this times the turning radius
	goalsToAttempt    = 10   // Divide the lookahead distance into this many discrete goals to attempt to correct towards.

	// Before post-processing trajectory will have velocities every this many mm (or degs if spinning in place).
	stepDistResolution = 1.

	// Used to determine minimum linear deviation allowed before correction attempt. Determined by multiplying max linear speed by
	// inputUpdateStepSeconds, and will correct if deviation is larger than this percent of that amount.
	minDeviationToCorrectPct = 50.

	microsecondsPerSecond = 1e6
)

type arcStep struct {
	linVelMMps      r3.Vector
	angVelDegps     r3.Vector
	durationSeconds float64

	// arcSegment.StartPosition is the pose at dist=0 for the PTG these traj nodes are derived from, such that
	// Compose(arcSegment.StartPosition, subTraj[n].Pose) is the expected pose at that node.
	// A single trajectory may be broken into multiple arcSteps, so we need to be able to track the total distance elapsed through
	// the trajectory.
	arcSegment ik.Segment

	subTraj []*tpspace.TrajNode
}

func (step *arcStep) String() string {
	return fmt.Sprintf("Step: lin velocity: %f,\n\t ang velocity: %f,\n\t duration: %f s,\n\t arcSegment %s,\n\t arc start pose %v",
		step.linVelMMps,
		step.angVelDegps,
		step.durationSeconds,
		step.arcSegment.String(),
		spatialmath.PoseToProtobuf(step.arcSegment.StartPosition),
	)
}

type courseCorrectionGoal struct {
	Goal     spatialmath.Pose
	Solution []referenceframe.Input
	stepIdx  int
	trajIdx  int
}

func (ptgk *ptgBaseKinematics) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	var err error
	// Cancel any prior GoToInputs calls
	if ptgk.cancelFunc != nil {
		ptgk.cancelFunc()
	}
	ctx, cancelFunc := context.WithCancel(ctx)
	ptgk.cancelFunc = cancelFunc

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
	ptgk.inputLock.Lock()
	ptgk.currentExecutingSteps, err = ptgk.arcStepsFromInputs(inputSteps, startPose)
	arcSteps := ptgk.currentExecutingSteps
	ptgk.inputLock.Unlock()
	if err != nil {
		return tryStop(err)
	}

	for i := 0; i < len(arcSteps); i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		step := arcSteps[i]
		ptgk.inputLock.Lock() // In the case where there's actual contention here, this could cause timing issues; how to solve?
		ptgk.currentIdx = i
		ptgk.currentInputs = step.arcSegment.StartConfiguration
		ptgk.inputLock.Unlock()

		ptgk.logger.Debugf("step, i %d \n %s", i, step.String())

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
		// Now we are moving. We need to do several things simultaneously:
		// - move until we think we have finished the arc, then move on to the next step
		// - update our CurrentInputs tracking where we are through the arc
		// - Check where we are relative to where we think we are, and tweak velocities accordingly
		for timeElapsedSeconds := updateStepSeconds; timeElapsedSeconds <= step.durationSeconds; timeElapsedSeconds += updateStepSeconds {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Account for 1) timeElapsedSeconds being inputUpdateStepSeconds ahead of actual elapsed time, and the fact that the loop takes
			// nonzero time to run especially when using the localizer.
			actualTimeElapsed := time.Since(arcStartTime)
			// Time durations are ints, not floats. 0.9 * time.Second is zero. Thus we use microseconds for math.
			remainingTimeStep := time.Duration(microsecondsPerSecond*timeElapsedSeconds)*time.Microsecond - actualTimeElapsed

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
			currentInputs := []referenceframe.Input{
				step.arcSegment.StartConfiguration[0],
				step.arcSegment.StartConfiguration[1],
				{Value: step.arcSegment.StartConfiguration[2].Value + math.Abs(distIncVel)*timeElapsedSeconds},
			}
			ptgk.inputLock.Lock()
			ptgk.currentInputs = currentInputs
			ptgk.inputLock.Unlock()

			// If we have a localizer, we are able to attempt to correct to stay on the path.
			// For now we do not try to correct while in a correction.
			if ptgk.Localizer != nil {
				newArcSteps, err := ptgk.courseCorrect(ctx, currentInputs, arcSteps, i)
				if err != nil {
					// If this (or anywhere else in this closure) has an error, the only consequence is that we are unable to solve a
					// valid course correction trajectory. We are still continuing to follow the plan, so if we ignore this error, we
					// will either try to course correct again and succeed or fail, or else the motion service will replan due to
					// either position or obstacles.
					ptgk.logger.Debugf("encountered an error while course correcting: %v", err)
				}
				if newArcSteps != nil {
					// newArcSteps will be nil if there is no course correction needed
					ptgk.inputLock.Lock()
					ptgk.currentExecutingSteps = newArcSteps
					ptgk.inputLock.Unlock()
					arcSteps = newArcSteps
					break
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
		trajArcSteps, err := ptgk.trajectoryToArcSteps(selectedTraj, runningPose, inputs)
		if err != nil {
			return nil, err
		}

		arcSteps = append(arcSteps, trajArcSteps...)
		runningPose = spatialmath.Compose(runningPose, selectedTraj[len(selectedTraj)-1].Pose)
	}
	return arcSteps, nil
}

func (ptgk *ptgBaseKinematics) trajectoryToArcSteps(
	traj []*tpspace.TrajNode,
	startPose spatialmath.Pose,
	inputs []referenceframe.Input,
) ([]arcStep, error) {
	finalSteps := []arcStep{}
	timeStep := 0.
	curDist := 0.
	curInputs := []referenceframe.Input{
		inputs[0],
		inputs[1],
		{curDist},
	}
	runningPose := startPose
	segment := ik.Segment{
		StartConfiguration: curInputs,
		StartPosition:      runningPose,
		Frame:              ptgk.Kinematics(),
	}
	// Trajectory distance is either length in mm, or if linear distance is not increasing, number of degrees to rotate in place.
	lastLinVel := r3.Vector{0, traj[0].LinVel * ptgk.linVelocityMMPerSecond, 0}
	lastAngVel := r3.Vector{0, 0, traj[0].AngVel * ptgk.angVelocityDegsPerSecond}
	nextStep := arcStep{
		linVelMMps:      lastLinVel,
		angVelDegps:     lastAngVel,
		arcSegment:      segment,
		durationSeconds: 0.,
	}
	for _, trajPt := range traj {
		nextStep.subTraj = append(nextStep.subTraj, trajPt)
		nextLinVel := r3.Vector{0, trajPt.LinVel * ptgk.linVelocityMMPerSecond, 0}
		nextAngVel := r3.Vector{0, 0, trajPt.AngVel * ptgk.angVelocityDegsPerSecond}

		// Check if this traj node has different velocities from the last one. If so, end our segment and start a new segment.
		if nextStep.linVelMMps.Sub(nextLinVel).Norm2() > 1e-6 || nextStep.angVelDegps.Sub(nextAngVel).Norm2() > 1e-6 {
			// Changed velocity, make a new step
			nextStep.durationSeconds = timeStep
			curInputs = []referenceframe.Input{
				inputs[0],
				inputs[1],
				{curDist},
			}
			arcPose, err := ptgk.Kinematics().Transform(curInputs)
			if err != nil {
				return nil, err
			}
			runningPose = spatialmath.Compose(runningPose, arcPose)
			nextStep.arcSegment.EndConfiguration = curInputs
			nextStep.arcSegment.EndPosition = runningPose
			finalSteps = append(finalSteps, nextStep)
			segment = ik.Segment{
				StartConfiguration: curInputs,
				StartPosition:      runningPose,
				Frame:              ptgk.Kinematics(),
			}
			nextStep = arcStep{
				linVelMMps:      nextLinVel,
				angVelDegps:     nextAngVel,
				arcSegment:      segment,
				durationSeconds: 0,
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
	curInputs = []referenceframe.Input{
		inputs[0],
		inputs[1],
		{curDist},
	}
	arcPose, err := ptgk.Kinematics().Transform(curInputs)
	if err != nil {
		return nil, err
	}
	runningPose = spatialmath.Compose(runningPose, arcPose)
	nextStep.arcSegment.EndConfiguration = curInputs
	nextStep.arcSegment.EndPosition = runningPose
	finalSteps = append(finalSteps, nextStep)

	return finalSteps, nil
}

func (ptgk *ptgBaseKinematics) courseCorrect(
	ctx context.Context,
	currentInputs []referenceframe.Input,
	arcSteps []arcStep,
	arcIdx int,
) ([]arcStep, error) {
	actualPose, err := ptgk.Localizer.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	trajPose, err := ptgk.frame.Transform(currentInputs)
	if err != nil {
		return nil, err
	}
	// Note that the arcSegment start position corresponds to the start configuration, which may not be at pose distance 0, so we must
	// account for this.
	trajStartPose, err := ptgk.frame.Transform(arcSteps[arcIdx].arcSegment.StartConfiguration)
	if err != nil {
		return nil, err
	}
	expectedPoseRel := spatialmath.PoseBetween(trajStartPose, trajPose)

	// This is where we expected to be on the trajectory.
	expectedPose := spatialmath.Compose(arcSteps[arcIdx].arcSegment.StartPosition, expectedPoseRel)

	// This is where actually are on the trajectory
	poseDiff := spatialmath.PoseBetween(actualPose.Pose(), expectedPose)

	allowableDiff := ptgk.linVelocityMMPerSecond * updateStepSeconds * (minDeviationToCorrectPct / 100)
	ptgk.logger.Debug(
		"allowable diff ", allowableDiff, " diff now ", poseDiff.Point().Norm(), " angle diff ", poseDiff.Orientation().AxisAngles().Theta,
	)

	if poseDiff.Point().Norm() > allowableDiff || poseDiff.Orientation().AxisAngles().Theta > 0.25 {
		ptgk.logger.Debug("expected to be at ", spatialmath.PoseToProtobuf(expectedPose))
		ptgk.logger.Debug("Localizer says at ", spatialmath.PoseToProtobuf(actualPose.Pose()))
		// Accumulate list of points along the path to try to connect to
		goals := ptgk.makeCourseCorrectionGoals(
			goalsToAttempt,
			arcIdx,
			actualPose.Pose(),
			arcSteps,
			currentInputs,
		)

		ptgk.logger.Debug("wanted to attempt ", goalsToAttempt, " goals, got ", len(goals))
		// Attempt to solve from `actualPose` to each of those points
		solution, err := ptgk.getCorrectionSolution(ctx, goals)
		if err != nil {
			return nil, err
		}
		if solution.Solution != nil {
			ptgk.logger.Debug("successful course correction", solution.Solution)

			correctiveArcSteps := []arcStep{}
			actualPoseTracked := actualPose.Pose()
			for i := 0; i < len(solution.Solution); i += 2 {
				// We've got a course correction solution. Swap out the relevant arcsteps.
				correctiveTraj, err := ptgk.ptgs[ptgk.courseCorrectionIdx].Trajectory(
					solution.Solution[i].Value,
					solution.Solution[i+1].Value,
					stepDistResolution,
				)
				if err != nil {
					return nil, err
				}

				newArcSteps, err := ptgk.trajectoryToArcSteps(
					correctiveTraj,
					actualPoseTracked,
					[]referenceframe.Input{{float64(ptgk.courseCorrectionIdx)}, solution.Solution[i], solution.Solution[i+1]},
				)
				if err != nil {
					return nil, err
				}
				for _, newArcStep := range newArcSteps {
					actualPoseTracked = spatialmath.Compose(
						actualPoseTracked,
						spatialmath.PoseBetween(newArcStep.arcSegment.StartPosition, newArcStep.arcSegment.EndPosition),
					)
				}
				correctiveArcSteps = append(correctiveArcSteps, newArcSteps...)
			}

			// Update the connection point
			connectionPoint := arcSteps[solution.stepIdx]

			// Use distances to calculate the % completion of the arc, used to update the time remaining.
			// We can't use step.durationSeconds because we might connect to a different arc than we're currently in.
			pctTrajRemaining := (connectionPoint.subTraj[len(connectionPoint.subTraj)-1].Dist -
				connectionPoint.subTraj[solution.trajIdx].Dist) /
				(connectionPoint.subTraj[len(connectionPoint.subTraj)-1].Dist - connectionPoint.arcSegment.StartConfiguration[2].Value)

			connectionPoint.arcSegment.StartConfiguration[2].Value = connectionPoint.subTraj[solution.trajIdx].Dist
			connectionPoint.durationSeconds *= pctTrajRemaining
			connectionPoint.subTraj = connectionPoint.subTraj[solution.trajIdx:]

			// Start with the already-executed steps.
			// We need to include the i-th step because we're about to increment i and want to start with the correction, then
			// continue with the connection point.
			var newArcSteps []arcStep
			newArcSteps = append(newArcSteps, arcSteps[:arcIdx+1]...)
			newArcSteps = append(newArcSteps, correctiveArcSteps...)
			newArcSteps = append(newArcSteps, connectionPoint)
			if solution.stepIdx < len(arcSteps)-1 {
				newArcSteps = append(newArcSteps, arcSteps[solution.stepIdx+1:]...)
			}
			return newArcSteps, nil
		}
		return nil, errors.New("failed to find valid course correction")
	}
	return nil, nil
}

func (ptgk *ptgBaseKinematics) getCorrectionSolution(ctx context.Context, goals []courseCorrectionGoal) (courseCorrectionGoal, error) {
	for _, goal := range goals {
		solveMetric := ik.NewSquaredNormMetric(goal.Goal)
		solutionChan := make(chan *ik.Solution, 1)
		ptgk.logger.Debug("attempting goal ", spatialmath.PoseToProtobuf(goal.Goal))
		seed := []referenceframe.Input{{math.Pi / 2}, {ptgk.linVelocityMMPerSecond / 2}, {math.Pi / 2}, {ptgk.linVelocityMMPerSecond / 2}}
		if goal.Goal.Point().X > 0 {
			seed[0].Value *= -1
		} else {
			seed[2].Value *= -1
		}
		// Attempt to use our course correction solver to solve for a new set of trajectories which will get us from our current position
		// to our goal point along our original trajectory.
		err := ptgk.ptgs[ptgk.courseCorrectionIdx].Solve(
			ctx,
			solutionChan,
			seed,
			solveMetric,
			0,
		)
		if err != nil {
			return courseCorrectionGoal{}, err
		}
		var solution *ik.Solution
		select {
		case solution = <-solutionChan:
		default:
		}
		ptgk.logger.Debug("solution ", solution)
		if solution.Score < 100. {
			goal.Solution = solution.Configuration
			return goal, nil
		}
	}
	return courseCorrectionGoal{}, nil
}

// This function will select `nGoals` poses in the future from the current position, rectifying them to be relatice to `currPose`.
// It will create `courseCorrectionGoal` structs for each. The goals will be approximately evenly spaced.
func (ptgk *ptgBaseKinematics) makeCourseCorrectionGoals(
	nGoals, currStep int,
	currPose spatialmath.Pose,
	steps []arcStep,
	currentInputs []referenceframe.Input,
) []courseCorrectionGoal {
	goals := []courseCorrectionGoal{}
	currDist := currentInputs[distanceAlongTrajectoryIndex].Value
	stepsPerGoal := int((ptgk.nonzeroBaseTurningRadiusMeters*lookaheadDistMult*1000)/stepDistResolution) / nGoals

	if stepsPerGoal < 1 {
		return []courseCorrectionGoal{}
	}

	startingTrajPt := 0
	for i := 0; i < len(steps[currStep].subTraj); i++ {
		if steps[currStep].subTraj[i].Dist >= currDist {
			startingTrajPt = i
			break
		}
	}

	totalTrajSteps := 0
	for i := currStep; i < len(steps); i++ {
		totalTrajSteps += len(steps[i].subTraj)
	}
	totalTrajSteps -= startingTrajPt
	// If we have fewer steps left than needed to fill our goal list, shrink the spacing of goals
	if stepsPerGoal*nGoals > totalTrajSteps {
		stepsPerGoal = totalTrajSteps / nGoals // int division is what we want here
	}

	stepsRemainingThisGoal := stepsPerGoal
	for i := currStep; i < len(steps); i++ {
		for len(steps[i].subTraj)-startingTrajPt > stepsRemainingThisGoal {
			goalTrajPtIdx := startingTrajPt + stepsRemainingThisGoal
			goalPose := spatialmath.PoseBetween(
				currPose,
				spatialmath.Compose(steps[i].arcSegment.StartPosition, steps[i].subTraj[goalTrajPtIdx].Pose),
			)
			goals = append(goals, courseCorrectionGoal{Goal: goalPose, stepIdx: i, trajIdx: goalTrajPtIdx})
			if len(goals) == nGoals {
				return goals
			}

			startingTrajPt = goalTrajPtIdx
			stepsRemainingThisGoal = stepsPerGoal
		}
		stepsRemainingThisGoal -= len(steps[i].subTraj) - startingTrajPt
		startingTrajPt = 0
	}
	return goals
}
