package motionplan

import (
	"fmt"

	"github.com/pkg/errors"
	"go.viam.com/rdk/motionplan/motiontypes"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// ExecutionState describes a plan and a particular state along it.
type ExecutionState struct {
	plan  motiontypes.Plan
	index int

	// The current inputs of input-enabled elements described by the plan
	currentInputs referenceframe.FrameSystemInputs

	// The current PoseInFrames of input-enabled elements described by this plan.
	currentPose referenceframe.FrameSystemPoses
}

// NewExecutionState will construct an ExecutionState struct.
func NewExecutionState(
	plan motiontypes.Plan,
	index int,
	currentInputs referenceframe.FrameSystemInputs,
	currentPose referenceframe.FrameSystemPoses,
) (ExecutionState, error) {
	if plan == nil {
		return ExecutionState{}, errors.New("cannot create new ExecutionState with nil plan")
	}
	if currentInputs == nil {
		return ExecutionState{}, errors.New("cannot create new ExecutionState with nil currentInputs")
	}
	if currentPose == nil {
		return ExecutionState{}, errors.New("cannot create new ExecutionState with nil currentPose")
	}
	return ExecutionState{
		plan:          plan,
		index:         index,
		currentInputs: currentInputs,
		currentPose:   currentPose,
	}, nil
}

// Plan returns the plan associated with the execution state.
func (e *ExecutionState) Plan() motiontypes.Plan {
	return e.plan
}

// Index returns the currently-executing index of the execution state's Plan.
func (e *ExecutionState) Index() int {
	return e.index
}

// CurrentInputs returns the current inputs of the components associated with the ExecutionState.
func (e *ExecutionState) CurrentInputs() referenceframe.FrameSystemInputs {
	return e.currentInputs
}

// CurrentPoses returns the current poses in frame of the components associated with the ExecutionState.
func (e *ExecutionState) CurrentPoses() referenceframe.FrameSystemPoses {
	return e.currentPose
}

// CalculateFrameErrorState takes an ExecutionState and a Frame and calculates the error between the Frame's expected
// and actual positions.
func CalculateFrameErrorState(e ExecutionState, executionFrame, localizationFrame referenceframe.Frame) (spatialmath.Pose, error) {
	currentInputs, ok := e.CurrentInputs()[executionFrame.Name()]
	if !ok {
		return nil, newFrameNotFoundError(executionFrame.Name())
	}
	currentPose, ok := e.CurrentPoses()[localizationFrame.Name()]
	if !ok {
		return nil, newFrameNotFoundError(localizationFrame.Name())
	}
	currPoseInArc, err := executionFrame.Transform(currentInputs)
	if err != nil {
		return nil, err
	}
	path := e.Plan().Path()
	if path == nil {
		return nil, errors.New("cannot calculate error state on a nil Path")
	}
	if len(path) == 0 {
		return spatialmath.NewZeroPose(), nil
	}
	index := e.Index() - 1
	if index < 0 || index >= len(path) {
		return nil, fmt.Errorf("index %d out of bounds for Path of length %d", index, len(path))
	}
	pose, ok := path[index][executionFrame.Name()]
	if !ok {
		return nil, newFrameNotFoundError(executionFrame.Name())
	}
	if pose.Parent() != currentPose.Parent() {
		return nil, errors.New("cannot compose two PoseInFrames with different parents")
	}
	nominalPose := spatialmath.Compose(pose.Pose(), currPoseInArc)
	return spatialmath.PoseBetween(nominalPose, currentPose.Pose()), nil
}

// newFrameNotFoundError returns an error indicating that a given frame was not found in the given ExecutionState.
func newFrameNotFoundError(frameName string) error {
	return fmt.Errorf("could not find frame %s in ExecutionState", frameName)
}
