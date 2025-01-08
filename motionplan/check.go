//go:build !no_cgo

// Package motionplan is a motion planning library.
package motionplan

import (
	"fmt"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
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
	fs referenceframe.FrameSystem,
	lookAheadDistanceMM float64,
	logger logging.Logger,
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

	// construct planager
	sfPlanner, err := newPlanManager(fs, logger, defaultRandomSeed)
	if err != nil {
		return err
	}

	// Spot check plan for options
	planOpts, err := sfPlanner.plannerSetupFromMoveRequest(
		&PlanState{poses: plan.Path()[0]},
		&PlanState{poses: plan.Path()[len(plan.Path())-1]},
		plan.Trajectory()[0],
		worldState,
		nil,
		nil, // no pb.Constraints
		nil, // no plannOpts
	)
	if err != nil {
		return err
	}

	// This should be done for any plan whose configurations are specified in relative terms rather than absolute ones.
	// Currently this is only TP-space, so we check if the PTG length is >0.
	if planOpts.useTPspace {
		return checkPlanRelative(checkFrame, executionState, worldState, fs, lookAheadDistanceMM, sfPlanner)
	}
	return checkPlanAbsolute(checkFrame, executionState, worldState, fs, lookAheadDistanceMM, sfPlanner)
}

func checkPlanRelative(
	checkFrame referenceframe.Frame, // TODO(RSDK-7421): remove this
	executionState ExecutionState,
	worldState *referenceframe.WorldState,
	fs referenceframe.FrameSystem,
	lookAheadDistanceMM float64,
	sfPlanner *planManager,
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

	// setup the planOpts. Poses should be in world frame. This allows us to know e.g. which obstacles may ephemerally collide.
	if sfPlanner.planOpts, err = sfPlanner.plannerSetupFromMoveRequest(
		&PlanState{poses: plan.Path()[0], configuration: plan.Trajectory()[0]},
		&PlanState{poses: plan.Path()[len(plan.Path())-1]},
		plan.Trajectory()[0],
		worldState,
		nil,
		nil, // no pb.Constraints
		nil, // no plannOpts
	); err != nil {
		return err
	}
	// change from 60mm to 30mm so we have finer interpolation along segments
	sfPlanner.planOpts.Resolution = relativePlanOptsResolution

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

	segments := make([]*ik.Segment, 0, len(plan.Path())-wayPointIdx)
	// pre-pend to segments so we can connect to the input we have not finished actuating yet

	segments = append(segments, &ik.Segment{
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
		segment := &ik.Segment{
			StartPosition:      lastArcEndPose,
			EndPosition:        thisArcEndPose,
			StartConfiguration: frameTrajectory[i-1],
			EndConfiguration:   frameTrajectory[i],
			Frame:              checkFrame,
		}
		lastArcEndPose = thisArcEndPose
		segments = append(segments, segment)
	}

	return checkSegments(sfPlanner, segments, lookAheadDistanceMM, checkFrame)
}

func checkPlanAbsolute(
	checkFrame referenceframe.Frame, // TODO(RSDK-7421): remove this
	executionState ExecutionState,
	worldState *referenceframe.WorldState,
	fs referenceframe.FrameSystem,
	lookAheadDistanceMM float64,
	sfPlanner *planManager,
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

	// setup the planOpts
	if sfPlanner.planOpts, err = sfPlanner.plannerSetupFromMoveRequest(
		&PlanState{poses: executionState.CurrentPoses(), configuration: startingInputs},
		&PlanState{poses: poses[len(poses)-1]},
		startingInputs,
		worldState,
		nil,
		nil, // no Constraints
		nil, // no planOpts
	); err != nil {
		return err
	}

	// create a list of segments to iterate through
	segments := make([]*ik.SegmentFS, 0, len(poses)-wayPointIdx)

	// iterate through remaining plan and append remaining segments to check
	for i := wayPointIdx; i < len(offsetPlan.Path())-1; i++ {
		segment := &ik.SegmentFS{
			StartConfiguration: offsetPlan.Trajectory()[i],
			EndConfiguration:   offsetPlan.Trajectory()[i+1],
			FS:                 sfPlanner.fs,
		}
		segments = append(segments, segment)
	}

	return checkSegmentsFS(sfPlanner, segments, lookAheadDistanceMM)
}

func checkSegmentsFS(sfPlanner *planManager, segments []*ik.SegmentFS, lookAheadDistanceMM float64) error {
	// go through segments and check that we satisfy constraints
	moving, _ := sfPlanner.frameLists()
	dists := map[string]float64{}
	for _, segment := range segments {
		ok, lastValid := sfPlanner.planOpts.CheckSegmentAndStateValidityFS(segment, sfPlanner.planOpts.Resolution)
		if !ok {
			checkConf := segment.StartConfiguration
			if lastValid != nil {
				checkConf = lastValid.EndConfiguration
			}
			ok, reason := sfPlanner.planOpts.CheckStateFSConstraints(&ik.StateFS{Configuration: checkConf, FS: sfPlanner.fs})
			if !ok {
				reason = " reason: " + reason
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
			poseInPathStart, err := sfPlanner.fs.Transform(
				segment.EndConfiguration,
				referenceframe.NewZeroPoseInFrame(checkFrame),
				referenceframe.World,
			)
			if err != nil {
				return err
			}
			poseInPathEnd, err := sfPlanner.fs.Transform(
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
func checkSegments(sfPlanner *planManager, segments []*ik.Segment, lookAheadDistanceMM float64, checkFrame referenceframe.Frame) error {
	// go through segments and check that we satisfy constraints
	var totalTravelDistanceMM float64
	for _, segment := range segments {
		interpolatedConfigurations, err := interpolateSegment(segment, sfPlanner.planOpts.Resolution)
		if err != nil {
			return err
		}
		parent, err := sfPlanner.fs.Parent(checkFrame)
		if err != nil {
			return err
		}
		for _, interpConfig := range interpolatedConfigurations {
			poseInPathTf, err := sfPlanner.fs.Transform(
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
			interpolatedState := &ik.State{
				Frame:         checkFrame,
				Configuration: interpConfig,
			}

			// Checks for collision along the interpolated route and returns a the first interpolated pose where a collision is detected.
			if isValid, err := sfPlanner.planOpts.CheckStateConstraints(interpolatedState); !isValid {
				return fmt.Errorf("found constraint violation or collision in segment between %v and %v at %v: %s",
					segment.StartPosition.Point(),
					segment.EndPosition.Point(),
					poseInPath.Point(),
					err,
				)
			}
		}

		// Update total traveled distance after segment has been checked
		totalTravelDistanceMM += segment.EndPosition.Point().Distance(segment.StartPosition.Point())
	}
	return nil
}
