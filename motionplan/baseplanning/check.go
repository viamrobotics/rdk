//go:build !no_cgo

package baseplanning

import (
	"fmt"

	"github.com/pkg/errors"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

var errCheckFrameNotInPath = errors.New("checkFrame given not in plan.Path() map")

const relativePlanOptsResolution = 30

// CheckPlan checks if obstacles intersect the trajectory of the frame following the plan. If one is
// detected, the interpolated position of the rover when a collision is detected is returned along
// with an error with additional collision details.
func CheckPlan(
	checkFrame referenceframe.Frame, // TODO(RSDK-7421): remove this
	executionState ExecutionState,
	worldState *referenceframe.WorldState,
	fs *referenceframe.FrameSystem,
	lookAheadDistanceMM float64,
) error {
	plan := executionState.Plan()
	wayPointIdx := executionState.Index()

	// ensure that we can actually perform the check
	if len(plan.Path()) < 1 || len(plan.Trajectory()) < 1 {
		return errors.New("plan's path and trajectory both must have at least one element")
	}
	if len(plan.Path()) <= wayPointIdx || wayPointIdx < 0 {
		return errors.New("wayPointIdx outside of plan bounds")
	}

	motionChains, err := motionChainsFromPlanState(fs, &PlanState{poses: plan.Path()[len(plan.Path())-1]})
	if err != nil {
		return err
	}

	// This should be done for any plan whose configurations are specified in relative terms rather than absolute ones.
	// Currently this is only TP-space, so we check if the PTG length is >0.
	if motionChains.useTPspace {
		return checkPlanRelative(checkFrame, executionState, worldState, fs, lookAheadDistanceMM)
	}
	return checkPlanAbsolute(checkFrame, executionState, worldState, fs, lookAheadDistanceMM)
}

func checkPlanRelative(
	checkFrame referenceframe.Frame, // TODO(RSDK-7421): remove this
	executionState ExecutionState,
	worldState *referenceframe.WorldState,
	fs *referenceframe.FrameSystem,
	lookAheadDistanceMM float64,
) error {
	var err error
	toWorld := func(pif *referenceframe.PoseInFrame, inputs referenceframe.FrameSystemInputs) (*referenceframe.PoseInFrame, error) {
		transformable, err := fs.Transform(inputs, pif, referenceframe.World)
		if err != nil {
			return nil, err
		}
		poseInWorld, ok := transformable.(*referenceframe.PoseInFrame)
		if !ok {
			// Should never happen
			return nil, errors.New("could not convert transformable to a PoseInFrame")
		}
		return poseInWorld, nil
	}

	plan := executionState.Plan()
	zeroPosePIF := referenceframe.NewPoseInFrame(checkFrame.Name(), spatialmath.NewZeroPose())

	motionChains, err := motionChainsFromPlanState(fs, &PlanState{poses: plan.Path()[len(plan.Path())-1]})
	if err != nil {
		return err
	}

	// setup the planOpts. Poses should be in world frame. This allows us to know e.g. which obstacles may ephemerally collide.
	planOpts, err := updateOptionsForPlanning(NewBasicPlannerOptions(), motionChains.useTPspace)
	if err != nil {
		return err
	}

	// change from 60mm to 30mm so we have finer interpolation along segments
	planOpts.Resolution = relativePlanOptsResolution

	constraintHandler, err := newConstraintChecker(
		planOpts,
		nil,
		&PlanState{poses: plan.Path()[0], configuration: plan.Trajectory()[0]},
		&PlanState{poses: plan.Path()[len(plan.Path())-1]},
		fs,
		motionChains,
		plan.Trajectory()[0],
		worldState,
		nil,
	)
	if err != nil {
		return err
	}

	currentInputs := executionState.CurrentInputs()
	wayPointIdx := executionState.Index()

	// construct first segment's startPosition
	currentPoses := executionState.CurrentPoses()
	if currentPoses == nil {
		return errors.New("executionState had nil return from CurrentPoses")
	}

	currentPoseIF, ok := currentPoses[checkFrame.Name()]
	if !ok {
		return errors.New("checkFrame not found in current pose map")
	}
	currentPoseInWorld, err := toWorld(currentPoseIF, currentInputs)
	if err != nil {
		return err
	}

	// construct first segment's endPosition
	expectedArcEndInWorld, err := toWorld(zeroPosePIF, plan.Trajectory()[wayPointIdx])
	if err != nil {
		return err
	}

	arcInputs, ok := plan.Trajectory()[wayPointIdx][checkFrame.Name()]
	if !ok {
		return errCheckFrameNotInPath
	}
	fullArcPose, err := checkFrame.Transform(arcInputs)
	if err != nil {
		return err
	}

	// Relative current inputs will give us the arc the base has executed. Calculating that transform and subtracting it from the
	// arc end position (that is, the same-index node in plan.Path()) gives us our expected location.
	frameCurrentInputs, ok := currentInputs[checkFrame.Name()]
	if !ok {
		return errors.New("given checkFrame had no inputs in CurrentInputs map")
	}

	poseThroughArc, err := checkFrame.Transform(frameCurrentInputs)
	if err != nil {
		return err
	}
	remainingArcPose := spatialmath.PoseBetween(poseThroughArc, fullArcPose)
	expectedCurrentPose := spatialmath.PoseBetweenInverse(remainingArcPose, expectedArcEndInWorld.Pose())
	errorState := spatialmath.PoseBetween(expectedCurrentPose, currentPoseInWorld.Pose())
	currentArcEndPose := spatialmath.Compose(expectedArcEndInWorld.Pose(), errorState)
	frameTrajectory, err := plan.Trajectory().GetFrameInputs(checkFrame.Name())
	if err != nil {
		return err
	}

	segments := make([]*motionplan.Segment, 0, len(plan.Path())-wayPointIdx)
	// pre-pend to segments so we can connect to the input we have not finished actuating yet

	segments = append(segments, &motionplan.Segment{
		StartPosition:      currentPoseInWorld.Pose(),
		EndPosition:        currentArcEndPose,
		StartConfiguration: frameCurrentInputs,
		EndConfiguration:   frameTrajectory[wayPointIdx],
		Frame:              checkFrame,
	})

	lastArcEndPose := currentArcEndPose

	// iterate through remaining plan and append remaining segments to check
	for i := wayPointIdx + 1; i <= len(plan.Path())-1; i++ {
		thisArcEndPoseInWorld, err := toWorld(zeroPosePIF, plan.Trajectory()[i])
		if err != nil {
			return err
		}
		thisArcEndPose := spatialmath.Compose(thisArcEndPoseInWorld.Pose(), errorState)
		segment := &motionplan.Segment{
			StartPosition:      lastArcEndPose,
			EndPosition:        thisArcEndPose,
			StartConfiguration: frameTrajectory[i-1],
			EndConfiguration:   frameTrajectory[i],
			Frame:              checkFrame,
		}
		lastArcEndPose = thisArcEndPose
		segments = append(segments, segment)
	}

	return checkSegments(segments, lookAheadDistanceMM, checkFrame, relativePlanOptsResolution, fs, constraintHandler)
}

func checkPlanAbsolute(
	checkFrame referenceframe.Frame, // TODO(RSDK-7421): remove this
	executionState ExecutionState,
	worldState *referenceframe.WorldState,
	fs *referenceframe.FrameSystem,
	lookAheadDistanceMM float64,
) error {
	plan := executionState.Plan()
	startingInputs := plan.Trajectory()[0]
	currentInputs := executionState.CurrentInputs()
	currentPoseIF := executionState.CurrentPoses()[checkFrame.Name()]
	wayPointIdx := executionState.Index()

	checkFramePiF := referenceframe.NewPoseInFrame(checkFrame.Name(), spatialmath.NewZeroPose())
	expectedPoseTf, err := fs.Transform(currentInputs, checkFramePiF, currentPoseIF.Parent())
	if err != nil {
		return err
	}
	expectedPoseIF, ok := expectedPoseTf.(*referenceframe.PoseInFrame)
	if !ok {
		// Should never happen
		return errors.New("could not convert transformable to a PoseInFrame")
	}
	// Non-relative inputs yield the expected position directly from `Transform()` on the inputs
	// These PIFs are now in the same frame
	errorState := spatialmath.PoseBetween(expectedPoseIF.Pose(), currentPoseIF.Pose())

	// offset the plan using the errorState
	// TODO(RSDK-7421): this will need to be done per frame with nonzero dof
	offsetPlan := OffsetPlan(plan, errorState)

	// get plan poses for checkFrame
	poses := offsetPlan.Path()

	motionChains, err := motionChainsFromPlanState(fs, &PlanState{poses: poses[len(poses)-1]})
	if err != nil {
		return err
	}

	planOpts, err := updateOptionsForPlanning(NewBasicPlannerOptions(), motionChains.useTPspace)
	if err != nil {
		return err
	}

	constraintHandler, err := newConstraintChecker(
		planOpts,
		nil,
		&PlanState{poses: executionState.CurrentPoses(), configuration: startingInputs},
		&PlanState{poses: poses[len(poses)-1]},
		fs,
		motionChains,
		startingInputs,
		worldState,
		nil,
	)
	if err != nil {
		return err
	}

	// create a list of segments to iterate through
	segments := make([]*motionplan.SegmentFS, 0, len(poses)-wayPointIdx)

	// iterate through remaining plan and append remaining segments to check
	for i := wayPointIdx; i < len(offsetPlan.Path())-1; i++ {
		segment := &motionplan.SegmentFS{
			StartConfiguration: offsetPlan.Trajectory()[i],
			EndConfiguration:   offsetPlan.Trajectory()[i+1],
			FS:                 fs,
		}
		segments = append(segments, segment)
	}

	return checkSegmentsFS(segments, lookAheadDistanceMM, planOpts.Resolution, motionChains, constraintHandler, fs)
}

func checkSegmentsFS(
	segments []*motionplan.SegmentFS,
	lookAheadDistanceMM float64,
	resolution float64,
	motionChains *motionChains,
	constraintHandler *motionplan.ConstraintChecker,
	fs *referenceframe.FrameSystem,
) error {
	// go through segments and check that we satisfy constraints
	moving, _ := motionChains.framesFilteredByMovingAndNonmoving(fs)
	dists := map[string]float64{}
	for _, segment := range segments {
		lastValid, err := constraintHandler.CheckSegmentAndStateValidityFS(segment, resolution)
		if err != nil {
			checkConf := segment.StartConfiguration
			if lastValid != nil {
				checkConf = lastValid.EndConfiguration
			}
			var reason string
			err := constraintHandler.CheckStateFSConstraints(&motionplan.StateFS{Configuration: checkConf, FS: fs})
			if err != nil {
				reason = " reason: " + err.Error()
			} else {
				reason = ""
			}
			return fmt.Errorf("found constraint violation or collision in segment between %v and %v at %v %s",
				segment.StartConfiguration,
				segment.EndConfiguration,
				checkConf,
				reason,
			)
		}

		for _, checkFrame := range moving {
			poseInPathStart, err := fs.Transform(
				segment.EndConfiguration,
				referenceframe.NewZeroPoseInFrame(checkFrame),
				referenceframe.World,
			)
			if err != nil {
				return err
			}
			poseInPathEnd, err := fs.Transform(
				segment.StartConfiguration,
				referenceframe.NewZeroPoseInFrame(checkFrame),
				referenceframe.World,
			)
			if err != nil {
				return err
			}

			currDist := poseInPathEnd.(*referenceframe.PoseInFrame).Pose().Point().Distance(
				poseInPathStart.(*referenceframe.PoseInFrame).Pose().Point(),
			)
			// Check if look ahead distance has been reached
			currentTravelDistanceMM := dists[checkFrame] + currDist
			if currentTravelDistanceMM > lookAheadDistanceMM {
				return nil
			}
			dists[checkFrame] = currentTravelDistanceMM
		}
	}
	return nil
}

// TODO: Remove this function.
func checkSegments(
	segments []*motionplan.Segment,
	lookAheadDistanceMM float64,
	checkFrame referenceframe.Frame,
	resolution float64,
	fs *referenceframe.FrameSystem,
	cHandler *motionplan.ConstraintChecker,
) error {
	// go through segments and check that we satisfy constraints
	var totalTravelDistanceMM float64
	for _, segment := range segments {
		interpolatedConfigurations, err := motionplan.InterpolateSegment(segment, resolution)
		if err != nil {
			return err
		}
		parent, err := fs.Parent(checkFrame)
		if err != nil {
			return err
		}
		for _, interpConfig := range interpolatedConfigurations {
			poseInPathTf, err := fs.Transform(
				referenceframe.FrameSystemInputs{checkFrame.Name(): interpConfig},
				referenceframe.NewZeroPoseInFrame(checkFrame.Name()),
				parent.Name(),
			)
			if err != nil {
				return err
			}
			poseInPath := poseInPathTf.(*referenceframe.PoseInFrame).Pose()

			// Check if look ahead distance has been reached
			currentTravelDistanceMM := totalTravelDistanceMM + poseInPath.Point().Distance(segment.StartPosition.Point())
			if currentTravelDistanceMM > lookAheadDistanceMM {
				return nil
			}

			// define State which only houses inputs, pose information not needed since we cannot get arcs from
			// an interpolating poses, this would only yield a straight line.
			interpolatedState := &motionplan.State{
				Frame:         checkFrame,
				Configuration: interpConfig,
			}

			// Checks for collision along the interpolated route and returns a the first interpolated pose where a collision is detected.
			if err := cHandler.CheckStateConstraints(interpolatedState); err != nil {
				return errors.Wrapf(err,
					"found constraint violation or collision in segment between %v and %v at %v",
					segment.StartPosition.Point(),
					segment.EndPosition.Point(),
					poseInPath.Point(),
				)
			}
		}

		// Update total traveled distance after segment has been checked
		totalTravelDistanceMM += segment.EndPosition.Point().Distance(segment.StartPosition.Point())
	}
	return nil
}
