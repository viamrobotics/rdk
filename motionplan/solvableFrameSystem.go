package motionplan

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

// SolvableFrameSystem wraps a FrameSystem to allow solving between frames of the frame system.
// Note that this needs to live in motionplan, not referenceframe, to avoid circular dependencies.
type SolvableFrameSystem struct {
	frame.FrameSystem
	logger golog.Logger
	mpFunc func(frame.Frame, int, golog.Logger) (MotionPlanner, error)
}

// NewSolvableFrameSystem will create a new solver for a frame system.
func NewSolvableFrameSystem(fs frame.FrameSystem, logger golog.Logger) *SolvableFrameSystem {
	return &SolvableFrameSystem{FrameSystem: fs, logger: logger}
}

// SolvePose will take a set of starting positions, a goal frame, a frame to solve for, and a pose. The function will
// then try to path plan the full frame system such that the solveFrame has the goal pose from the perspective of the goalFrame.
// For example, if a world system has a gripper attached to an arm attached to a gantry, and the system was being solved
// to place the gripper at a particular pose in the world, the solveFrame would be the gripper and the goalFrame would be
// the world frame. It will use the default planner options.
func (fss *SolvableFrameSystem) SolvePose(ctx context.Context,
	seedMap map[string][]frame.Input,
	goal spatial.Pose,
	solveFrameName, goalFrameName string,
) ([]map[string][]frame.Input, error) {
	return fss.SolveWaypointsWithOptions(ctx, seedMap, []spatial.Pose{goal}, solveFrameName, goalFrameName, nil, nil)
}

// SolvePoseWithOptions will take a set of starting positions, a goal frame, a frame to solve for, a pose, and a configurable
// set of PlannerOptions. It will solve the solveFrame to the goal pose with respect to the goal frame using the provided
// planning options.
func (fss *SolvableFrameSystem) SolvePoseWithOptions(ctx context.Context,
	seedMap map[string][]frame.Input,
	goal spatial.Pose,
	solveFrameName, goalFrameName string,
	worldState *commonpb.WorldState,
	opt *PlannerOptions,
) ([]map[string][]frame.Input, error) {
	return fss.SolveWaypointsWithOptions(ctx,
		seedMap,
		[]spatial.Pose{goal},
		solveFrameName,
		goalFrameName,
		worldState,
		[]*PlannerOptions{opt},
	)
}

// SolveWaypointsWithOptions will take a set of starting positions, a goal frame, a frame to solve for, goal poses, and a configurable
// set of PlannerOptions. It will solve the solveFrame to the goal poses with respect to the goal frame using the provided
// planning options.
func (fss *SolvableFrameSystem) SolveWaypointsWithOptions(ctx context.Context,
	seedMap map[string][]frame.Input,
	goals []spatial.Pose,
	solveFrameName, goalFrameName string,
	worldState *commonpb.WorldState,
	opts []*PlannerOptions,
) ([]map[string][]frame.Input, error) {
	steps := make([]map[string][]frame.Input, 0, len(goals)*2)

	// Get parentage of both frames. This will also verify the frames are in the frame system
	solveFrame := fss.GetFrame(solveFrameName)
	if solveFrame == nil {
		return nil, fmt.Errorf("frame with name %s not found in frame system", solveFrameName)
	}
	sFrames, err := fss.TracebackFrame(solveFrame)
	if err != nil {
		return nil, err
	}
	goalFrame := fss.GetFrame(goalFrameName)
	if goalFrame == nil {
		return nil, fmt.Errorf("frame with name %s not found in frame system", goalFrameName)
	}
	gFrames, err := fss.TracebackFrame(goalFrame)
	if err != nil {
		return nil, err
	}
	frames := uniqInPlaceSlice(append(sFrames, gFrames...))

	// Create a frame to solve for, and an IK solver with that frame.
	sf := &solverFrame{solveFrameName + "_" + goalFrameName, fss, frames, solveFrame, goalFrame}
	if len(sf.DoF()) == 0 {
		return nil, errors.New("solver frame has no degrees of freedom, cannot perform inverse kinematics")
	}

	// Build planner
	var planner MotionPlanner
	if fss.mpFunc != nil {
		planner, err = fss.mpFunc(sf, runtime.NumCPU()/2, fss.logger)
	} else {
		planner, err = NewCBiRRTMotionPlanner(sf, runtime.NumCPU()/2, fss.logger)
	}
	if err != nil {
		return nil, err
	}

	collisionConstraint := NewCollisionConstraintFromWorldState(sf, fss, worldState, seedMap)

	// setup opts
	if len(opts) == 0 {
		for i := 0; i < len(goals); i++ {
			opts = append(opts, NewDefaultPlannerOptions())
		}
	}
	if len(opts) != len(goals) {
		return nil, errors.New("goals and options had different lengths")
	}
	for _, opt := range opts {
		opt.constraintHandler.AddConstraint("collision", collisionConstraint)
	}

	seed := sf.mapToSlice(seedMap)
	resultSlices, err := RunPlannerWithWaypoints(ctx, planner, goals, seed, opts, 0)
	if err != nil {
		return nil, err
	}
	for _, resultSlice := range resultSlices {
		steps = append(steps, sf.sliceToMapConf(resultSlice))
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

// Name returns the name of the solver referenceframe.
func (sf *solverFrame) Name() string {
	return sf.name
}

// Transform returns the pose between the two frames of this solver for a given set of inputs.
func (sf *solverFrame) Transform(inputs []frame.Input) (spatial.Pose, error) {
	if len(inputs) != len(sf.DoF()) {
		return nil, fmt.Errorf("incorrect number of inputs to Transform got %d want %d", len(inputs), len(sf.DoF()))
	}
	pf := frame.NewPoseInFrame(sf.solveFrame.Name(), spatial.NewZeroPose())
	tf, err := sf.fss.Transform(sf.sliceToMap(inputs), pf, sf.goalFrame.Name())
	if err != nil {
		return nil, err
	}
	return tf.(*frame.PoseInFrame).Pose(), nil
}

// InputFromProtobuf converts pb.JointPosition to inputs.
func (sf *solverFrame) InputFromProtobuf(jp *pb.JointPositions) []frame.Input {
	inputs := make([]frame.Input, 0, len(jp.Degrees))
	posIdx := 0
	for _, transform := range sf.frames {
		dof := len(transform.DoF()) + posIdx
		jPos := jp.Degrees[posIdx:dof]
		posIdx = dof

		inputs = append(inputs, transform.InputFromProtobuf(&pb.JointPositions{Degrees: jPos})...)
	}

	return inputs
}

// ProtobufFromInput converts inputs to pb.JointPosition.
func (sf *solverFrame) ProtobufFromInput(input []frame.Input) *pb.JointPositions {
	jPos := &pb.JointPositions{}
	posIdx := 0
	for _, transform := range sf.frames {
		dof := len(transform.DoF()) + posIdx
		jPos.Degrees = append(jPos.Degrees, transform.ProtobufFromInput(input[posIdx:dof]).Degrees...)
		posIdx = dof
	}

	return jPos
}

// Geometry takes a solverFrame and a list of joint angles in radians and computes the 3D space occupied by each of the
// geometries in the solverFrame in the reference frame of the World frame.
func (sf *solverFrame) Geometries(inputs []frame.Input) (*frame.GeometriesInFrame, error) {
	if len(inputs) != len(sf.DoF()) {
		return nil, errors.New("incorrect number of inputs to transform")
	}
	var errAll error
	inputMap := sf.sliceToMap(inputs)
	sfGeometries := make(map[string]spatial.Geometry)
	for _, f := range sf.frames {
		inputs, err := frame.GetFrameInputs(f, inputMap)
		if err != nil {
			return nil, err
		}
		gf, err := f.Geometries(inputs)
		if gf == nil {
			// only propagate errors that result in nil geometry
			multierr.AppendInto(&errAll, err)
			continue
		}
		var tf frame.Transformable
		tf, err = sf.fss.Transform(inputMap, gf, frame.World)
		if err != nil {
			return nil, err
		}
		for name, geometry := range tf.(*frame.GeometriesInFrame).Geometries() {
			sfGeometries[name] = geometry
		}
	}
	return frame.NewGeometriesInFrame(frame.World, sfGeometries), errAll
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
// the inputs together in the order of the frames in sf.frames.
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
		fLen := i + len(frame.DoF())
		inputs[frame.Name()] = inputSlice[i:fLen]
		i = fLen
	}
	return inputs
}

func (sf *solverFrame) sliceToMapConf(inputSlice []frame.Input) map[string][]frame.Input {
	inputs := frame.StartPositions(sf.fss)
	i := 0
	for _, frame := range sf.frames {
		fLen := i + len(frame.DoF())
		inputs[frame.Name()] = inputSlice[i:fLen]
		i = fLen
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
