//go:build !no_cgo

// Package kinematicbase contains wrappers that augment bases with information needed for higher level
// control over the base
package kinematicbase

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.uber.org/multierr"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

func (ptgk *ptgBaseKinematics) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	defer func() {
		ptgk.inputLock.Lock()
		ptgk.currentInput = zeroInput
		ptgk.inputLock.Unlock()
	}()
	
	tryStop := func(errToWrap error) error {
		stopCtx, cancelFn := context.WithTimeout(context.Background(), time.Second*5)
		defer cancelFn()
		return multierr.Combine(errToWrap, ptgk.Base.Stop(stopCtx, nil))
	}
	for _, inputs := range inputSteps {

		ptgk.logger.CDebugf(ctx, "GoToInputs going to %v", inputs)

		selectedPTG := ptgk.ptgs[int(math.Round(inputs[ptgIndex].Value))]

		selectedTraj, err := selectedPTG.Trajectory(
			inputs[trajectoryIndexWithinPTG].Value,
			inputs[distanceAlongTrajectoryIndex].Value,
			stepDistResolution,
		)
		if err != nil {
			return tryStop(err)
		}
		arcSteps := ptgk.trajectoryToArcSteps(selectedTraj)

		for _, step := range arcSteps {
			ptgk.inputLock.Lock() // In the case where there's actual contention here, this could cause timing issues; how to solve?
			ptgk.currentInput = []referenceframe.Input{inputs[0], inputs[1], {step.startDist}}
			ptgk.inputLock.Unlock()

			timestep := time.Duration(step.timestepSeconds*1000*1000) * time.Microsecond

			ptgk.logger.CDebugf(ctx,
				"setting velocity to linear %v angular %v and running velocity step for %s",
				step.linVelMMps,
				step.angVelDegps,
				timestep,
			)

			var startPose *referenceframe.PoseInFrame
			if ptgk.Localizer != nil {
				startPose, err = ptgk.Localizer.CurrentPosition(ctx)
				if err != nil {
					return tryStop(err)
				}
			}
			startTrajPose, err := selectedPTG.Transform([]referenceframe.Input{inputs[1], {step.startDist}})
			if err != nil {
				return tryStop(err)
			}

			err = ptgk.Base.SetVelocity(
				ctx,
				step.linVelMMps,
				step.angVelDegps,
				nil,
			)
			if err != nil {
				return tryStop(err)
			}
			lastLinVel := step.linVelMMps
			lastAngVel := step.angVelDegps
			moveStartTime := time.Now()
			
			// Now we are moving. We need to do several things simultaneously:
			// - move until we think we have finished the arc, then move on to the next step
			// - update our CurrentInputs tracking where we are through the arc
			// - Check where we are relative to where we think we are, and tweak velocities accordingly
			for timeElapsed := inputUpdateStep; timeElapsed <= step.timestepSeconds; timeElapsed += inputUpdateStep {
				// Account for 1) timeElapsed being inputUpdateStep ahead of actual elapsed time, and the fact that the loop takes nonzero time
				// to run especially when using the localizer.
				actualTimeElapsed := time.Since(moveStartTime)
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
				ptgk.inputLock.Lock()
				ptgk.currentInput = []referenceframe.Input{inputs[0], inputs[1], {step.startDist + math.Abs(distIncVel) * timeElapsed}}
				ptgk.inputLock.Unlock()
				
				if ptgk.Localizer != nil {
					
				}
			}
		}
	}

	return tryStop(nil)
}

func (ptgk *ptgBaseKinematics) pathCorrection(ctx context.Context) 
					currPose, err := ptgk.Localizer.CurrentPosition(ctx)
					if err != nil {
						return tryStop(err)
					}
					currRelPose := spatialmath.PoseBetween(startPose.Pose(), currPose.Pose())
					expectedPoseRaw, err := selectedPTG.Transform([]referenceframe.Input{ptgk.currentInput[1], ptgk.currentInput[2]})
					if err != nil {
						return tryStop(err)
					}
					expectedPose := spatialmath.PoseBetween(startTrajPose, expectedPoseRaw)
					poseDiff := spatialmath.PoseBetween(currRelPose, expectedPose)
					poseDiffPt := poseDiff.Point()
					poseDiffAngle := poseDiff.Orientation().OrientationVectorDegrees().Theta
					fmt.Println("curr pose", spatialmath.PoseToProtobuf(currPose.Pose()))
					fmt.Println("curr rel pose", spatialmath.PoseToProtobuf(currRelPose))
					fmt.Println("exp pose", spatialmath.PoseToProtobuf(expectedPose))
					fmt.Println("diff pose", spatialmath.PoseToProtobuf(poseDiff))
					adjLinVel := step.linVelMMps
					adjAngVel := step.angVelDegps
					
					if math.Abs(poseDiffPt.Y) > 100 {
						// Positive Y means we are behind where we want to be and should speed up. Speed up 5% at a time
						adjLinVel.Y = lastLinVel.Y * (1. + math.Copysign(0.05, poseDiffPt.Y))
					}
					//~ if math.Abs(poseDiffPt.X) > 100 {
						//~ // If we are to the right, we want to rotate left, and vice versa.
						//~ adjAngVel.Z += math.Copysign(10., poseDiffPt.X)
					//~ } else if math.Abs(poseDiffAngle) > 10 {
						//~ // If we are at the correct X position but angled, adjust so we do not go off course
						//~ adjAngVel.Z += -1 * math.Copysign(10., poseDiffAngle)
					//~ }
					if !lastLinVel.ApproxEqual(adjLinVel) || !lastAngVel.ApproxEqual(adjAngVel) {
						err = ptgk.Base.SetVelocity(
							ctx,
							adjLinVel,
							adjAngVel,
							nil,
						)
						if err != nil {
							return tryStop(err)
						}
						lastLinVel = adjLinVel
						lastAngVel = adjAngVel
					}
