//go:build !no_cgo

package armplanning

import (
	"fmt"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// CheckPlan checks if obstacles intersect the trajectory of the frame following the plan. If one is
// detected, the interpolated position of the rover when a collision is detected is returned along
// with an error with additional collision details.
func CheckPlan(
	checkFrame referenceframe.Frame, // TODO(RSDK-7421): remove this
	executionState ExecutionState,
	worldState *referenceframe.WorldState,
	fs *referenceframe.FrameSystem,
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

	return checkPlanAbsolute(logger, checkFrame, executionState, worldState, fs, lookAheadDistanceMM)
}

func checkPlanAbsolute(
	logger logging.Logger,
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

	planOpts := NewBasicPlannerOptions()

	constraintHandler, err := newConstraintHandler(
		planOpts,
		logger,
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
	constraintHandler *ConstraintHandler,
	fs *referenceframe.FrameSystem,
) error {
	// go through segments and check that we satisfy constraints
	moving, _ := motionChains.framesFilteredByMovingAndNonmoving(fs)
	dists := map[string]float64{}
	for _, segment := range segments {
		ok, lastValid := constraintHandler.CheckSegmentAndStateValidityFS(segment, resolution)
		if !ok {
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
