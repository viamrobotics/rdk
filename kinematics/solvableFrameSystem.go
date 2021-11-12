package kinematics

import (
	"context"
	"errors"

	"runtime"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"

	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
)

// SolvableFrameSystem wraps a FrameSystem to allow solving between frames of the frame system.
// Note that this needs to live in kinematics, not referenceframe, to avoid circular dependencies
type SolvableFrameSystem struct {
	frame.FrameSystem
	logger golog.Logger
}

// NewSolvableFrameSystem will create a new solver for a frame system
func NewSolvableFrameSystem(fs frame.FrameSystem, logger golog.Logger) *SolvableFrameSystem {
	return &SolvableFrameSystem{fs, logger}
}

// SolvePose will take a set of starting positions, a goal frame, a frame to solve for, and a pose. The function will
// then try to solve the full frame system such that the solveFrame has the goal pose from the perspective of the goalFrame.
// For example, if a world system has a gripper attached to an arm attached to a gantry, and the system was being solved
// to place the gripper at a particular pose in the world, the solveFrame would be the gripper and the goalFrame would be
// the world frame.
func (fss *SolvableFrameSystem) SolvePose(ctx context.Context, seedMap map[string][]frame.Input, goal spatial.Pose, solveFrame, goalFrame frame.Frame) (map[string][]frame.Input, error) {

	// Get parentage of both frames. This will also verify the frames are in the frame system
	sFrames, err := fss.TracebackFrame(solveFrame)
	if err != nil {
		return nil, err
	}
	gFrames, err := fss.TracebackFrame(goalFrame)
	if err != nil {
		return nil, err
	}
	frames := uniqInPlaceSlice(append(sFrames, gFrames...))

	// Create a frame to solve for, and an IK solver with that frame.
	sf := &solverFrame{solveFrame.Name() + "_" + goalFrame.Name(), fss, frames, solveFrame, goalFrame}
	if len(sf.DoF()) == 0 {
		return nil, errors.New("solver frame has no degrees of freedom, cannot perform inverse kinematics")
	}
	solver, err := CreateCombinedIKSolver(sf, fss.logger, runtime.NumCPU()/2)
	if err != nil {
		return nil, err
	}

	seed := sf.mapToSlice(seedMap)

	// Solve for the goal position
	resultSlice, err := solver.Solve(ctx, spatial.PoseToProtobuf(goal), seed)
	if err != nil {
		return nil, multierr.Combine(err, solver.Close())
	}

	return sf.sliceToMap(resultSlice), solver.Close()
}

// solverFrames are meant to be ephemerally created each time a frame system solution is created, and fulfills the
// Frame interface so that it can be passed to inverse kinematics.
type solverFrame struct {
	name       string
	fss        *SolvableFrameSystem
	frames     []frame.Frame
	solveFrame frame.Frame
	goalFrame  frame.Frame
}

// Name returns the name of the solver frame
func (sf *solverFrame) Name() string {
	return sf.name
}

// Transform returns the pose between the two frames of this solver for a given set of inputs.
func (sf *solverFrame) Transform(inputs []frame.Input) (spatial.Pose, error) {
	if len(inputs) != len(sf.DoF()) {
		return nil, errors.New("incorrect number of inputs to Transform")
	}
	pos := frame.StartPositions(sf.fss)
	i := 0
	for _, frame := range sf.frames {
		pos[frame.Name()] = inputs[i : i+len(frame.DoF())]
		i += len(frame.DoF())
	}
	return sf.fss.TransformFrame(pos, sf.solveFrame, sf.goalFrame)
}

// DoF returns the summed DoF of all frames between the two solver frames.
func (sf *solverFrame) DoF() []frame.Limit {
	var limits []frame.Limit
	for _, frame := range sf.frames {
		limits = append(limits, frame.DoF()...)
	}
	return limits
}

// mapToSlice will flatten a map of inputs into a slice suitable for input to inverse kinematics, by concatenating
// the inputs together in the order of the frames in sf.frames
func (sf *solverFrame) mapToSlice(inputMap map[string][]frame.Input) []frame.Input {
	var inputs []frame.Input
	for _, frame := range sf.frames {
		inputs = append(inputs, inputMap[frame.Name()]...)
	}
	return inputs
}

func (sf *solverFrame) sliceToMap(inputSlice []frame.Input) map[string][]frame.Input {
	inputs := map[string][]frame.Input{}
	i := 0
	for _, frame := range sf.frames {
		inputs[frame.Name()] = inputSlice[i : i+len(frame.DoF())]
		i += len(frame.DoF())
	}
	return inputs
}

func (sf *solverFrame) MarshalJSON() ([]byte, error) {
	return nil, errors.New("cannot serialize solverFrame")
}

func (sf *solverFrame) AlmostEquals(otherFrame frame.Frame) bool {
	return false
}

// uniqInPlaceSlice will deduplicate the values in a slice using in-place replacement on the slice. This is faster than
// a solution using append().
// This function does not remove anything from the input slice, but it does rearrange the elements.
func uniqInPlaceSlice(s []frame.Frame) []frame.Frame {
	seen := make(map[frame.Frame]struct{}, len(s))
	j := 0
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		s[j] = v
		j++
	}
	return s[:j]
}
