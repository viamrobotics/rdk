package motionplan

import (
	"context"
	"errors"
	"fmt"

	"runtime"

	"github.com/edaniels/golog"

	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
)

// SolvableFrameSystem wraps a FrameSystem to allow solving between frames of the frame system.
// Note that this needs to live in motionplan, not referenceframe, to avoid circular dependencies
type SolvableFrameSystem struct {
	frame.FrameSystem
	logger golog.Logger
	mpFunc func(frame.Frame, int, golog.Logger) (MotionPlanner, error)
}

// NewSolvableFrameSystem will create a new solver for a frame system
func NewSolvableFrameSystem(fs frame.FrameSystem, logger golog.Logger) *SolvableFrameSystem {
	return &SolvableFrameSystem{FrameSystem: fs, logger: logger}
}

// SolvePose will take a set of starting positions, a goal frame, a frame to solve for, and a pose. The function will
// then try to path plan the full frame system such that the solveFrame has the goal pose from the perspective of the goalFrame.
// For example, if a world system has a gripper attached to an arm attached to a gantry, and the system was being solved
// to place the gripper at a particular pose in the world, the solveFrame would be the gripper and the goalFrame would be
// the world frame.
func (fss *SolvableFrameSystem) SolvePose(ctx context.Context, seedMap map[string][]frame.Input, goal spatial.Pose, solveFrame, goalFrame frame.Frame) ([]map[string][]frame.Input, error) {

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
	var planner MotionPlanner
	if fss.mpFunc != nil {
		planner, err = fss.mpFunc(sf, runtime.NumCPU()/2, fss.logger)
	} else {
		planner, err = NewCBiRRTMotionPlanner(sf, runtime.NumCPU()/2, fss.logger)
	}
	if err != nil {
		return nil, err
	}

	seed := sf.mapToSlice(seedMap)

	// Solve for the goal position
	resultSlices, err := planner.Plan(ctx, spatial.PoseToProtobuf(goal), seed)
	if err != nil {
		return nil, err
	}
	steps := make([]map[string][]frame.Input, 0, len(resultSlices))
	for _, resultSlice := range resultSlices {
		steps = append(steps, sf.sliceToMap(resultSlice))
	}

	return steps, nil
}

// SetPlannerGen sets the function which is used to create the motion planner to solve a requested plan.
// A SolvableFrameSystem wraps a complete frame system, and will make solverFrames on the fly to solve for. These
// solverFrames are used to create the planner here.
func (fss *SolvableFrameSystem) SetPlannerGen(mpFunc func(frame.Frame, int, golog.Logger) (MotionPlanner, error)) {
	fss.mpFunc = mpFunc
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
		return nil, fmt.Errorf("incorrect number of inputs to Transform got %d want %d", len(inputs), len(sf.DoF()))
	}
	return sf.fss.TransformFrame(sf.sliceToMap(inputs), sf.solveFrame, sf.goalFrame)
}

// VerboseTransform takes a solverFrame and a list of joint angles in radians and computes the dual quaterions
// representing poses of each of the intermediate frames (if any exist) up to and including the end effector, and
// returns a map of frame names to poses. The key for each frame in the map will be the string
// "<model_name>:<frame_name>"
func (sf *solverFrame) VerboseTransform(inputs []frame.Input) (map[string]spatial.Pose, error) {
	if len(inputs) != len(sf.DoF()) {
		return nil, errors.New("incorrect number of inputs to transform")
	}
	var err error
	inputMap := sf.sliceToMap(inputs)
	poseMap := make(map[string]spatial.Pose)
	for _, frame := range sf.frames {
		pm, err := sf.fss.VerboseTransformFrame(inputMap, frame, sf.goalFrame)
		if err != nil {
			return nil, err
		}
		for name, pose := range pm {
			poseMap[name] = pose
		}
	}
	return poseMap, err
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
	inputs := frame.StartPositions(sf.fss)
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
