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

var (
	errCheckFrameNotInPath = errors.New("checkFrame given not in plan.Path() map")
	ptgDoFLen              = 4
)

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
	startingInputs := plan.Trajectory()[0]
	wayPointIdx := executionState.Index()

	// ensure that we can actually perform the check
	if len(plan.Path()) < 1 {
		return errors.New("plan must have at least one element")
	}
	if len(plan.Path()) <= wayPointIdx || wayPointIdx < 0 {
		return errors.New("wayPointIdx outside of plan bounds")
	}

	// construct solverFrame
	// Note that this requires all frames which move as part of the plan, to have an
	// entry in the very first plan waypoint
	sf, err := newSolverFrame(fs, checkFrame.Name(), referenceframe.World, startingInputs)
	if err != nil {
		return err
	}
	// construct planager
	sfPlanner, err := newPlanManager(sf, logger, defaultRandomSeed)
	if err != nil {
		return err
	}
	// This should be done for any plan whose configurations are specified in relative terms rather than absolute ones.
	// Currently this is only TP-space, so we check if the PTG length is >0.
	// The solver frame will have had its PTGs filled in the newPlanManager() call, if applicable.
	if sfPlanner.useTPspace {
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
	// Validate the given PoseInFrame of the relative frame. Relative frame poses cannot be given in their own frame, or the frame of
	// any of their children.
	// TODO(RSDK-7421): there will need to be checks once there is a real possibility of multiple, hierarchical relative frames, or
	// something expressly forbidding it.
	validateRelPiF := func(pif *referenceframe.PoseInFrame) error {
		observingFrame := fs.Frame(pif.Parent())
		// Ensure the frame of the pose-in-frame is in the frame system
		if observingFrame == nil {
			sfPlanner.logger.Errorf(
				"pose of %s was given in frame of %s, but no frame with that name was found in the frame system",
				checkFrame.Name(),
				pif.Parent(),
			)
			return nil
		}
		// Ensure nothing between the PiF's frame and World is the relative frame
		observingParentage, err := fs.TracebackFrame(observingFrame)
		if err != nil {
			return err
		}
		for _, parent := range observingParentage {
			if parent.Name() == checkFrame.Name() {
				return fmt.Errorf(
					"pose of %s was given in frame of %s, but current pose of checked frame must not be observed by self or child",
					checkFrame.Name(),
					pif.Parent(),
				)
			}
		}
		return nil
	}

	toWorld := func(pif *referenceframe.PoseInFrame, inputs map[string][]referenceframe.Input) (*referenceframe.PoseInFrame, error) {
		// Check our current position is valid
		err := validateRelPiF(pif)
		if err != nil {
			return nil, err
		}
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

	// determine plan's starting pose
	planStartPiF, ok := plan.Path()[0][checkFrame.Name()]
	if !ok {
		return errCheckFrameNotInPath
	}
	planStartPoseWorld, err := toWorld(planStartPiF, plan.Trajectory()[0])
	if err != nil {
		return err
	}

	// determine plan's ending pose
	planEndPiF, ok := plan.Path()[len(plan.Path())-1][checkFrame.Name()]
	if !ok {
		return errCheckFrameNotInPath
	}
	planEndPoseWorld, err := toWorld(planEndPiF, plan.Trajectory()[len(plan.Path())-1])
	if err != nil {
		return err
	}

	// setup the planOpts. Poses should be in world frame. This allows us to know e.g. which obstacles may ephemerally collide.
	if sfPlanner.planOpts, err = sfPlanner.plannerSetupFromMoveRequest(
		planStartPoseWorld.Pose(),
		planEndPoseWorld.Pose(),
		plan.Trajectory()[0],
		worldState,
		nil,
		nil, // no pb.Constraints
		nil, // no plannOpts
	); err != nil {
		return err
	}
	// change from 60mm to 30mm so we have finer interpolation along segments
	sfPlanner.planOpts.Resolution = 30

	currentInputs := executionState.CurrentInputs()
	wayPointIdx := executionState.Index()
	sf := sfPlanner.frame

	// construct first segment's start configuration
	// get checkFrame's currentInputs
	// *currently* it is guaranteed that a relative frame will constitute 100% of a solver frame's dof
	checkFrameCurrentInputs, err := sf.mapToSlice(currentInputs)
	if err != nil {
		return err
	}

	// construct first segment's end configuration
	currentWayPointTraj := plan.Trajectory()[wayPointIdx]
	arcEndInputs, err := sf.mapToSlice(currentWayPointTraj)
	if err != nil {
		return err
	}

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
	// Check that path pose is valid
	stepEndPiF, ok := plan.Path()[wayPointIdx][checkFrame.Name()]
	if !ok {
		return errCheckFrameNotInPath
	}

	expectedArcEndInWorld, err := toWorld(stepEndPiF, plan.Trajectory()[wayPointIdx])
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

	segments := make([]*ik.Segment, 0, len(plan.Path())-wayPointIdx)
	// pre-pend to segments so we can connect to the input we have not finished actuating yet
	segments = append(segments, &ik.Segment{
		StartPosition:      currentPoseInWorld.Pose(),
		EndPosition:        currentArcEndPose,
		StartConfiguration: checkFrameCurrentInputs,
		EndConfiguration:   arcEndInputs,
		Frame:              sf,
	})

	lastArcEndPose := currentArcEndPose

	// iterate through remaining plan and append remaining segments to check
	for i := wayPointIdx + 1; i <= len(plan.Path())-1; i++ {
		thisArcEndPoseTf, ok := plan.Path()[i][checkFrame.Name()]
		if !ok {
			return errCheckFrameNotInPath
		}
		thisArcEndPoseInWorld, err := toWorld(thisArcEndPoseTf, plan.Trajectory()[i])
		if err != nil {
			return err
		}
		thisArcEndPose := spatialmath.Compose(thisArcEndPoseInWorld.Pose(), errorState)
		startInputs := map[string][]referenceframe.Input{}
		for k, v := range plan.Trajectory()[i] {
			if k == checkFrame.Name() {
				// Starting inputs for relative frames should be all-zero
				correctedInputs := make([]referenceframe.Input, ptgDoFLen)
				correctedInputs = append(correctedInputs, v[ptgDoFLen:]...)
				startInputs[k] = correctedInputs
			} else {
				startInputs[k] = v
			}
		}
		nextInputs := plan.Trajectory()[i]
		segment, err := createSegment(sf, lastArcEndPose, thisArcEndPose, startInputs, nextInputs)
		if err != nil {
			return err
		}
		lastArcEndPose = thisArcEndPose
		segments = append(segments, segment)
	}

	return checkSegments(sfPlanner, segments, lookAheadDistanceMM)
}

func checkPlanAbsolute(
	checkFrame referenceframe.Frame, // TODO(RSDK-7421): remove this
	executionState ExecutionState,
	worldState *referenceframe.WorldState,
	fs referenceframe.FrameSystem,
	lookAheadDistanceMM float64,
	sfPlanner *planManager,
) error {
	sf := sfPlanner.frame
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
	poses, err := offsetPlan.Path().GetFramePoses(checkFrame.Name())
	if err != nil {
		return err
	}
	startPose := currentPoseIF.Pose()

	// setup the planOpts
	if sfPlanner.planOpts, err = sfPlanner.plannerSetupFromMoveRequest(
		startPose,
		poses[len(poses)-1],
		startingInputs,
		worldState,
		nil,
		nil, // no pb.Constraints
		nil, // no plannOpts
	); err != nil {
		return err
	}

	// create a list of segments to iterate through
	segments := make([]*ik.Segment, 0, len(poses)-wayPointIdx)

	// iterate through remaining plan and append remaining segments to check
	for i := wayPointIdx; i < len(offsetPlan.Path())-1; i++ {
		segment, err := createSegment(sf, poses[i], poses[i+1], offsetPlan.Trajectory()[i], offsetPlan.Trajectory()[i+1])
		if err != nil {
			return err
		}
		segments = append(segments, segment)
	}

	return checkSegments(sfPlanner, segments, lookAheadDistanceMM)
}

// createSegment is a function to ease segment creation for solver frames.
func createSegment(
	sf *solverFrame,
	currPose, nextPose spatialmath.Pose,
	currInput, nextInput map[string][]referenceframe.Input,
) (*ik.Segment, error) {
	var currInputSlice, nextInputSlice []referenceframe.Input
	var err error
	if currInput != nil {
		currInputSlice, err = sf.mapToSlice(currInput)
		if err != nil {
			return nil, err
		}
	}
	nextInputSlice, err = sf.mapToSlice(nextInput)
	if err != nil {
		return nil, err
	}

	segment := &ik.Segment{
		StartPosition:      currPose,
		EndPosition:        nextPose,
		StartConfiguration: currInputSlice,
		EndConfiguration:   nextInputSlice,
		Frame:              sf,
	}

	return segment, nil
}

func checkSegments(sfPlanner *planManager, segments []*ik.Segment, lookAheadDistanceMM float64) error {
	// go through segments and check that we satisfy constraints
	// TODO(RSDK-5007): If we can make interpolate a method on Frame the need to write this out will be lessened and we should be
	// able to call CheckStateConstraintsAcrossSegment directly.
	var totalTravelDistanceMM float64
	for _, segment := range segments {
		interpolatedConfigurations, err := interpolateSegment(segment, sfPlanner.planOpts.Resolution)
		if err != nil {
			return err
		}
		for _, interpConfig := range interpolatedConfigurations {
			poseInPath, err := sfPlanner.frame.Transform(interpConfig)
			if err != nil {
				return err
			}

			// Check if look ahead distance has been reached
			currentTravelDistanceMM := totalTravelDistanceMM + poseInPath.Point().Distance(segment.StartPosition.Point())
			if currentTravelDistanceMM > lookAheadDistanceMM {
				return nil
			}

			// define State which only houses inputs, pose information not needed since we cannot get arcs from
			// an interpolating poses, this would only yield a straight line.
			interpolatedState := &ik.State{
				Frame:         sfPlanner.frame,
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
