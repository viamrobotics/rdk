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
	"go.viam.com/utils"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	lookaheadDistMult = 3  // Look ahead distance for path correction will be this times the turning radius
	goalsToAttempt    = 10 // Divide the lookahead distance into this many discrete goals to attempt to correct towards.

	// Before post-processing trajectory will have velocities every this many mm (or degs if spinning in place).
	stepDistResolution = 1.

	// Used to determine minimum linear deviation allowed before correction attempt. Determined by multiplying max linear speed by
	// inputUpdateStepSeconds, and will correct if deviation is larger than this percent of that amount.
	minDeviationToCorrectPct = 50.
	microsecondsPerSecond    = 1e6
	courseCorrectionMaxScore = 50. // Course correction solutions must score better than this
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
		ptgk.currentState = baseState{currentInputs: zeroInput}
		ptgk.inputLock.Unlock()
	}()

	tryStop := func(errToWrap error) error {
		stopCtx, cancelFn := context.WithTimeout(context.Background(), time.Second*5)
		defer cancelFn()
		return multierr.Combine(errToWrap, ptgk.Base.Stop(stopCtx, nil))
	}

	startPose := spatialmath.NewZeroPose() // This is the location of the base at call time
	if ptgk.Localizer != nil {
		startPoseInFrame, err := ptgk.CurrentPosition(ctx)
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
	ptgk.inputLock.Lock()
	ptgk.currentState.currentExecutingSteps = arcSteps
	ptgk.inputLock.Unlock()
	updateDuration := ptgk.opts.UpdateStepSeconds

	for i := 0; i < len(arcSteps); i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		step := arcSteps[i]
		ptgk.inputLock.Lock() // In the case where there's actual contention here, this could cause timing issues; how to solve?
		ptgk.currentState.currentIdx = i
		ptgk.currentState.currentInputs = step.arcSegment.StartConfiguration
		ptgk.inputLock.Unlock()

		ptgk.logger.Debugf("step, i %d \n %s", i, step.String())

		err = ptgk.Base.SetVelocity(
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
		stepDuration := time.Duration(step.durationSeconds*1000) * time.Millisecond

		// Check if this arc is shorter than our typical check time; if so just run that and do not course correct.
		if step.durationSeconds < updateDuration {
			utils.SelectContextOrWait(ctx, stepDuration)
			if ctx.Err() != nil {
				return tryStop(ctx.Err())
			}
			ptgk.logger.Debugf("step %d done", i)
			continue
		}
		courseCorrected := false // used to distinguish between a break due to course correction, or running out the loop

		for timeElapsedSeconds := updateDuration; timeElapsedSeconds <= step.durationSeconds; timeElapsedSeconds += updateDuration {
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
			inputValDiff := step.arcSegment.EndConfiguration[endDistanceAlongTrajectoryIndex].Value -
				step.arcSegment.EndConfiguration[startDistanceAlongTrajectoryIndex].Value
			elapsedPct := math.Min(1.0, timeElapsedSeconds/step.durationSeconds)
			currentInputs := []referenceframe.Input{
				step.arcSegment.StartConfiguration[ptgIndex],
				step.arcSegment.StartConfiguration[trajectoryAlphaWithinPTG],
				step.arcSegment.StartConfiguration[startDistanceAlongTrajectoryIndex],
				{step.arcSegment.StartConfiguration[startDistanceAlongTrajectoryIndex].Value + inputValDiff*elapsedPct},
			}
			ptgk.inputLock.Lock()
			ptgk.currentState.currentInputs = currentInputs
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
					ptgk.currentState.currentExecutingSteps = newArcSteps
					ptgk.inputLock.Unlock()
					arcSteps = newArcSteps
					courseCorrected = true
					break
				}
			}
		}
		if time.Since(arcStartTime) < stepDuration && !courseCorrected {
			utils.SelectContextOrWait(ctx, stepDuration-time.Since(arcStartTime))
			if ctx.Err() != nil {
				return tryStop(ctx.Err())
			}
		}
		ptgk.logger.Debugf("step %d done", i)
	}
	return tryStop(nil)
}

func (ptgk *ptgBaseKinematics) arcStepsFromInputs(inputSteps [][]referenceframe.Input, startPose spatialmath.Pose) ([]arcStep, error) {
	var arcSteps []arcStep
	runningPose := startPose
	for _, inputs := range inputSteps {
		trajArcSteps, err := ptgk.trajectoryArcSteps(runningPose, inputs)
		if err != nil {
			return nil, err
		}

		arcSteps = append(arcSteps, trajArcSteps...)
		runningPose = trajArcSteps[len(trajArcSteps)-1].arcSegment.EndPosition
	}
	return arcSteps, nil
}

// trajectoryArcSteps takes a set of inputs and breaks the trajectory apart into steps of velocities. It returns the list of
// steps to execute, including the timing of how long to maintain each velocity, and the expected starting and ending positions.
func (ptgk *ptgBaseKinematics) trajectoryArcSteps(
	startPose spatialmath.Pose,
	inputs []referenceframe.Input,
) ([]arcStep, error) {
	selectedPTG := int(math.Round(inputs[ptgIndex].Value))

	traj, err := ptgk.ptgs[selectedPTG].Trajectory(
		inputs[trajectoryAlphaWithinPTG].Value,
		inputs[startDistanceAlongTrajectoryIndex].Value,
		inputs[endDistanceAlongTrajectoryIndex].Value,
		stepDistResolution,
	)
	if err != nil {
		return nil, err
	}

	finalSteps := []arcStep{}
	timeStep := 0.
	curDist := inputs[startDistanceAlongTrajectoryIndex].Value
	startInputs := []referenceframe.Input{
		inputs[ptgIndex],
		inputs[trajectoryAlphaWithinPTG],
		inputs[startDistanceAlongTrajectoryIndex],
		inputs[startDistanceAlongTrajectoryIndex],
	}
	runningPose := startPose
	segment := ik.Segment{
		StartConfiguration: startInputs,
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

		distIncrement := trajPt.Dist - curDist

		curDist += distIncrement
		if lastLinVel.Y != 0 {
			timeStep += math.Abs(distIncrement / (math.Abs(lastLinVel.Y)))
		} else if lastAngVel.Z != 0 {
			timeStep += math.Abs(distIncrement / (math.Abs(lastAngVel.Z)))
		}
		// Check if this traj node has different velocities from the last one. If so, end our segment and start a new segment.
		if nextStep.linVelMMps.Sub(nextLinVel).Norm2() > 1e-6 || nextStep.angVelDegps.Sub(nextAngVel).Norm2() > 1e-6 {
			// Changed velocity, make a new step
			nextStep.durationSeconds = timeStep

			stepEndInputs := []referenceframe.Input{
				inputs[ptgIndex],
				inputs[trajectoryAlphaWithinPTG],
				nextStep.arcSegment.StartConfiguration[startDistanceAlongTrajectoryIndex],
				{curDist},
			}
			nextStep.arcSegment.EndConfiguration = stepEndInputs

			arcPose, err := ptgk.Kinematics().Transform(stepEndInputs)
			if err != nil {
				return nil, err
			}
			runningPose = spatialmath.Compose(runningPose, arcPose)
			nextStep.arcSegment.EndPosition = runningPose
			finalSteps = append(finalSteps, nextStep)

			stepStartInputs := []referenceframe.Input{
				inputs[ptgIndex],
				inputs[trajectoryAlphaWithinPTG],
				{curDist},
				{curDist},
			}
			segment = ik.Segment{
				StartConfiguration: stepStartInputs,
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
		lastLinVel = nextLinVel
		lastAngVel = nextAngVel
	}
	nextStep.durationSeconds = timeStep
	finalInputs := []referenceframe.Input{
		inputs[ptgIndex],
		inputs[trajectoryAlphaWithinPTG],
		nextStep.arcSegment.StartConfiguration[startDistanceAlongTrajectoryIndex],
		{curDist},
	}
	nextStep.arcSegment.EndConfiguration = finalInputs
	arcPose, err := ptgk.Kinematics().Transform(finalInputs)
	if err != nil {
		return nil, err
	}
	runningPose = spatialmath.Compose(runningPose, arcPose)
	nextStep.arcSegment.EndPosition = runningPose
	finalSteps = append(finalSteps, nextStep)

	return finalSteps, nil
}

// courseCorrect will check whether the base is sufficiently off-course, and if so, attempt to calculate a set of corrective arcs to arrive
// back at a point along the planned path. If successful will return the new set of steps to execute to reflect the correction.
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
	// trajPose is the pose we should have nominally reached along the currently executing arc from the start position.
	trajPose, err := ptgk.Kinematics().Transform(currentInputs)
	if err != nil {
		return nil, err
	}

	// This is where we expected to be on the trajectory.
	expectedPose := spatialmath.Compose(arcSteps[arcIdx].arcSegment.StartPosition, trajPose)

	// This is where actually are on the trajectory
	poseDiff := spatialmath.PoseBetween(actualPose.Pose(), expectedPose)

	allowableDiff := ptgk.linVelocityMMPerSecond * ptgk.opts.UpdateStepSeconds * (minDeviationToCorrectPct / 100)
	ptgk.logger.Debug(
		"allowable diff ", allowableDiff,
		" linear diff now ", poseDiff.Point().Norm(),
		" angle diff ", rdkutils.RadToDeg(poseDiff.Orientation().AxisAngles().Theta),
	)
	ptgk.logger.Debug("expected to be at ", spatialmath.PoseToProtobuf(expectedPose))
	ptgk.logger.Debug("Localizer says at ", spatialmath.PoseToProtobuf(actualPose.Pose()))
	if poseDiff.Point().Norm() > allowableDiff || rdkutils.RadToDeg(poseDiff.Orientation().AxisAngles().Theta) > allowableDiff {
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
				newArcSteps, err := ptgk.trajectoryArcSteps(
					actualPoseTracked,
					[]referenceframe.Input{{float64(ptgk.courseCorrectionIdx)}, solution.Solution[i], {0}, solution.Solution[i+1]},
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

			// We need to update the connection point. The starting configuration and position need to be updated, as well as
			// the ending configuration's arc start value.
			// The connection point is the point along the already-created plan where course correction will rejoin.
			connectionPoint := arcSteps[solution.stepIdx]

			// Construct deep copy of the connectionPoint to avoid data race
			connectionPointDeepCopy := copyArcStep(connectionPoint)

			arcOriginalLength := math.Abs(
				connectionPointDeepCopy.arcSegment.EndConfiguration[endDistanceAlongTrajectoryIndex].Value -
					connectionPointDeepCopy.arcSegment.EndConfiguration[startDistanceAlongTrajectoryIndex].Value,
			)

			// Use distances to calculate the % completion of the arc, used to update the time remaining.
			// We can't use step.durationSeconds because we might connect to a different arc than we're currently in.
			// This is valid because each individual arcstep is guaranteed to have constant velocities across the whole step.
			pctTrajRemaining := math.Abs(connectionPointDeepCopy.subTraj[len(connectionPointDeepCopy.subTraj)-1].Dist-
				connectionPointDeepCopy.subTraj[solution.trajIdx].Dist) / arcOriginalLength

			// TODO (RSDK-7515) Start value rewriting here is somewhat complicated. Imagine the old trajectory was [0, 200] and we
			// reconnect at Dist=40. The new start configuration should be [40, 40] and the new end configuration should be [40, 200].
			// However, traj dist values are always positive. Imagine if the old trajectory was [0,-200] and we reconnect at Dist=40.
			// Now, the new start configuration should be [-160, -160] and the new end configuration should be [0, -160]. RSDK-7515 will
			// simplify this significantly.
			startVal := connectionPointDeepCopy.subTraj[solution.trajIdx].Dist

			// We need to know the point along the segment where we are expecting to reconnect
			skippedSegment := []referenceframe.Input{
				connectionPointDeepCopy.arcSegment.EndConfiguration[ptgIndex],
				connectionPointDeepCopy.arcSegment.EndConfiguration[trajectoryAlphaWithinPTG],
				connectionPointDeepCopy.arcSegment.EndConfiguration[startDistanceAlongTrajectoryIndex],
				{startVal},
			}
			skippedPose, err := ptgk.Kinematics().Transform(skippedSegment)
			if err != nil {
				return nil, err
			}

			isReverse := connectionPointDeepCopy.arcSegment.EndConfiguration[endDistanceAlongTrajectoryIndex].Value < 0
			if isReverse {
				startVal += connectionPointDeepCopy.arcSegment.EndConfiguration[endDistanceAlongTrajectoryIndex].Value
			}

			connectionPointDeepCopy.arcSegment.StartConfiguration[startDistanceAlongTrajectoryIndex].Value = startVal
			connectionPointDeepCopy.arcSegment.StartConfiguration[endDistanceAlongTrajectoryIndex].Value = startVal
			if isReverse {
				connectionPointDeepCopy.arcSegment.EndConfiguration[endDistanceAlongTrajectoryIndex].Value = startVal
			} else {
				connectionPointDeepCopy.arcSegment.EndConfiguration[startDistanceAlongTrajectoryIndex].Value = startVal
			}
			// The start position should be where the connection tried to get to.
			// This needs to be the Goal, as that is the point along the original path, not the solved point, which is just somewhere near
			// that based on courseCorrectionMaxScore.
			connectionPointDeepCopy.arcSegment.StartPosition = spatialmath.Compose(connectionPointDeepCopy.arcSegment.StartPosition, skippedPose)

			connectionPointDeepCopy.durationSeconds *= pctTrajRemaining
			connectionPointDeepCopy.subTraj = connectionPointDeepCopy.subTraj[solution.trajIdx:]

			// set the end position of the end position of the arcstep we were just on to the end position of the corrective trajectory
			ptgk.inputLock.Lock()
			arcSteps[arcIdx].arcSegment.EndPosition = correctiveArcSteps[0].arcSegment.StartPosition
			ptgk.inputLock.Unlock()

			// Start with the already-executed steps.
			// We need to include the i-th step because we're about to increment i and want to start with the correction, then
			// continue with the connection point.
			var newArcSteps []arcStep
			newArcSteps = append(newArcSteps, arcSteps[:arcIdx+1]...)
			newArcSteps = append(newArcSteps, correctiveArcSteps...)
			newArcSteps = append(newArcSteps, connectionPointDeepCopy)
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
		solveMetric := ik.NewScaledSquaredNormMetric(goal.Goal, 50)
		ptgk.logger.Debug("attempting goal ", spatialmath.PoseToProtobuf(goal.Goal))
		seed := []referenceframe.Input{{math.Pi / 2}, {ptgk.linVelocityMMPerSecond / 2}, {math.Pi / 2}, {ptgk.linVelocityMMPerSecond / 2}}
		if goal.Goal.Point().X > 0 {
			seed[0].Value *= -1
		} else {
			seed[2].Value *= -1
		}
		// Attempt to use our course correction solver to solve for a new set of trajectories which will get us from our current position
		// to our goal point along our original trajectory.
		solution, err := ptgk.ptgs[ptgk.courseCorrectionIdx].Solve(
			ctx,
			seed,
			solveMetric,
		)
		if err != nil {
			return courseCorrectionGoal{}, err
		}
		ptgk.logger.Debug("solution ", solution)
		if solution.Score < courseCorrectionMaxScore {
			goal.Solution = referenceframe.FloatsToInputs(solution.Configuration)
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
	currDist := currentInputs[endDistanceAlongTrajectoryIndex].Value
	stepsPerGoal := int((ptgk.nonzeroBaseTurningRadiusMeters*lookaheadDistMult*1000)/stepDistResolution) / nGoals

	if stepsPerGoal < 1 {
		return []courseCorrectionGoal{}
	}

	startingTrajPt := 0
	for i := 0; i < len(steps[currStep].subTraj); i++ {
		// Determine the index of the current subtraj point
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

			// Since the arc may not be starting at 0, we must compute the transform for this particular traj pt.
			// The pose in trajPt.Pose is from the zero position.
			arcTrajInputs := []referenceframe.Input{
				steps[i].arcSegment.StartConfiguration[ptgIndex],
				steps[i].arcSegment.StartConfiguration[trajectoryAlphaWithinPTG],
				steps[i].arcSegment.StartConfiguration[startDistanceAlongTrajectoryIndex],
				{steps[i].subTraj[goalTrajPtIdx].Dist},
			}

			arcPose, err := ptgk.Kinematics().Transform(arcTrajInputs)
			if err != nil {
				return []courseCorrectionGoal{}
			}

			goalPose := spatialmath.PoseBetween(
				currPose,
				spatialmath.Compose(steps[i].arcSegment.StartPosition, arcPose),
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

func (ptgk *ptgBaseKinematics) stepsToPlan(steps []arcStep, parentFrame string) motionplan.Plan {
	traj := motionplan.Trajectory{}
	path := motionplan.Path{}
	for _, step := range steps {
		traj = append(traj, referenceframe.FrameSystemInputs{ptgk.Kinematics().Name(): step.arcSegment.EndConfiguration})
		path = append(path, map[string]*referenceframe.PoseInFrame{
			ptgk.Kinematics().Name(): referenceframe.NewPoseInFrame(parentFrame, step.arcSegment.EndPosition),
		})
	}

	return motionplan.NewSimplePlan(path, traj)
}

func copyArcStep(step arcStep) arcStep {
	// Construct deep copy of the connectionPoint to avoid data race
	stepDeepCopy := arcStep{
		linVelMMps:      step.linVelMMps,
		angVelDegps:     step.angVelDegps,
		durationSeconds: step.durationSeconds,
		arcSegment:      ik.Segment{},
		subTraj:         make([]*tpspace.TrajNode, len(step.subTraj)),
	}

	// Deep copy subTraj slice
	for i, trajNode := range step.subTraj {
		if trajNode != nil {
			copiedNode := *trajNode
			stepDeepCopy.subTraj[i] = &copiedNode
		}
	}

	// Copy arcSegment
	startCfgCopy := make([]referenceframe.Input, len(step.arcSegment.StartConfiguration))
	copy(startCfgCopy, step.arcSegment.StartConfiguration)
	stepDeepCopy.arcSegment.StartConfiguration = startCfgCopy

	endCfgCopy := make([]referenceframe.Input, len(step.arcSegment.EndConfiguration))
	copy(endCfgCopy, step.arcSegment.EndConfiguration)
	stepDeepCopy.arcSegment.EndConfiguration = endCfgCopy

	stepDeepCopy.arcSegment.StartPosition = step.arcSegment.StartPosition
	stepDeepCopy.arcSegment.EndPosition = step.arcSegment.EndPosition
	stepDeepCopy.arcSegment.Frame = step.arcSegment.Frame

	return stepDeepCopy
}
