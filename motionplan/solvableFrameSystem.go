package motionplan

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"

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
	// TODO(rb): this probably shouldn't live here
	mpFunc plannerConstructor
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
	goal *frame.PoseInFrame,
	solveFrameName string,
) ([]map[string][]frame.Input, error) {
	return fss.SolveWaypointsWithOptions(ctx, seedMap, []*frame.PoseInFrame{goal}, solveFrameName, nil, nil)
}

// SolveWaypointsWithOptions will take a set of starting positions, a goal frame, a frame to solve for, goal poses, and a configurable
// set of PlannerOptions. It will solve the solveFrame to the goal poses with respect to the goal frame using the provided
// planning options.
func (fss *SolvableFrameSystem) SolveWaypointsWithOptions(ctx context.Context,
	seedMap map[string][]frame.Input,
	goals []*frame.PoseInFrame,
	solveFrameName string,
	worldState *commonpb.WorldState,
	motionConfigs []map[string]interface{},
) ([]map[string][]frame.Input, error) {
	steps := make([]map[string][]frame.Input, 0, len(goals)*2)

	// Get parentage of solver frame. This will also verify the frame is in the frame system
	solveFrame := fss.Frame(solveFrameName)
	if solveFrame == nil {
		return nil, fmt.Errorf("frame with name %s not found in frame system", solveFrameName)
	}
	solveFrameList, err := fss.TracebackFrame(solveFrame)
	if err != nil {
		return nil, err
	}

	opts := make([]map[string]interface{}, 0, len(goals))

	// If no planning opts, use default. If one, use for all goals. If one per goal, use respective option. Otherwise error.
	if len(motionConfigs) != len(goals) {
		switch len(motionConfigs) {
		case 0:
			for range goals {
				opts = append(opts, map[string]interface{}{})
			}
		case 1:
			// If one config passed, use it for all waypoints
			for range goals {
				opts = append(opts, motionConfigs[0])
			}
		default:
			return nil, errors.New("goals and motion configs had different lengths")
		}
	} else {
		opts = motionConfigs
	}

	// Each goal is a different PoseInFrame and so may have a different destination Frame. Since the motion can be solved from either end,
	// each goal is solved independently.
	for i, goal := range goals {
		// Create a frame to solve for, and an IK solver with that frame.
		sf, err := newSolverFrame(fss, solveFrameList, goal.FrameName(), seedMap)
		if err != nil {
			return nil, err
		}
		if len(sf.DoF()) == 0 {
			return nil, errors.New("solver frame has no degrees of freedom, cannot perform inverse kinematics")
		}

		resultSlices, err := sf.planSingleWaypoint(ctx, seedMap, goal.Pose(), worldState, opts[i])
		if err != nil {
			return nil, err
		}
		for j, resultSlice := range resultSlices {
			stepMap := sf.sliceToMap(resultSlice)
			steps = append(steps, stepMap)
			if j == len(resultSlices)-1 {
				// update seed map
				seedMap = stepMap
			}
		}
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
	completeFs *SolvableFrameSystem
	movingFs   frame.FrameSystem
	frames     []frame.Frame // all frames directly between and including solveFrame and goalFrame. Order not important.
	solveFrame frame.Frame
	goalFrame  frame.Frame
	// If this is true, then goals are translated to their position in `World` before solving.
	// This is useful when e.g. moving a gripper relative to a point seen by a camera built into that gripper
	// TODO(pl): explore allowing this to be frames other than world
	worldRooted bool
	origSeed    map[string][]frame.Input // stores starting locations of all frames in fss that are NOT in `frames`
}

func newSolverFrame(
	fss *SolvableFrameSystem,
	solveFrameList []frame.Frame,
	goalFrameName string,
	seedMap map[string][]frame.Input,
) (*solverFrame, error) {
	var movingFs frame.FrameSystem
	var frames []frame.Frame
	worldRooted := false

	// get goal frame
	goalFrame := fss.Frame(goalFrameName)
	if goalFrame == nil {
		return nil, frame.NewFrameMissingError(goalFrameName)
	}
	goalFrameList, err := fss.TracebackFrame(goalFrame)
	if err != nil {
		return nil, err
	}

	// get solve frame
	if len(solveFrameList) == 0 {
		return nil, errors.New("solveFrameList was empty")
	}
	solveFrame := solveFrameList[0]

	// find pivot frame between goal and solve frames
	pivotFrame, err := findPivotFrame(solveFrameList, goalFrameList)
	if err != nil {
		return nil, err
	}
	if pivotFrame.Name() == frame.World {
		movingFs = fss
		frames = uniqInPlaceSlice(append(solveFrameList, goalFrameList...))
	} else {
		// Get minimal set of frames from solve frame to goal frame
		dof := 0
		for _, frame := range solveFrameList {
			if frame == pivotFrame {
				break
			}
			dof += len(frame.DoF())
			frames = append(frames, frame)
		}
		for _, frame := range goalFrameList {
			if frame == pivotFrame {
				break
			}
			dof += len(frame.DoF())
			frames = append(frames, frame)
		}

		// If shortest path has 0 dof (e.g. a camera attached to a gripper), translate goal to world frame
		if dof == 0 {
			worldRooted = true
			movingFs = fss
			frames = solveFrameList
		} else {
			// Get all child nodes of pivot node
			movingFs, err = fss.FrameSystemSubset(pivotFrame)
			if err != nil {
				return nil, err
			}
		}
	}

	origSeed := map[string][]frame.Input{}
	// deep copy of seed map
	for k, v := range seedMap {
		origSeed[k] = v
	}
	for _, frame := range frames {
		delete(origSeed, frame.Name())
	}

	return &solverFrame{
		name:        solveFrame.Name() + "_" + goalFrame.Name(),
		completeFs:  fss,
		movingFs:    movingFs,
		frames:      frames,
		solveFrame:  solveFrame,
		goalFrame:   goalFrame,
		worldRooted: worldRooted,
		origSeed:    origSeed,
	}, nil
}

func (sf *solverFrame) planSingleWaypoint(ctx context.Context,
	seedMap map[string][]frame.Input,
	goalPos spatial.Pose,
	worldState *commonpb.WorldState,
	motionConfig map[string]interface{},
) ([][]frame.Input, error) {
	seed := sf.mapToSlice(seedMap)
	seedPos, err := sf.Transform(seed)
	if err != nil {
		return nil, err
	}

	// Build planner
	var planner MotionPlanner
	if sf.completeFs.mpFunc != nil {
		planner, err = sf.completeFs.mpFunc(sf, runtime.NumCPU()/2, sf.completeFs.logger)
	} else {
		planner, err = NewCBiRRTMotionPlanner(sf, runtime.NumCPU()/2, sf.completeFs.logger)
	}
	if err != nil {
		return nil, err
	}

	// If we are world rooted, translate the goal pose into the world frame
	if sf.worldRooted {
		tf, err := sf.completeFs.Transform(seedMap, frame.NewPoseInFrame(sf.goalFrame.Name(), goalPos), frame.World)
		if err != nil {
			return nil, err
		}
		goalPos = tf.(*frame.PoseInFrame).Pose()
	}

	goals := []spatial.Pose{goalPos}
	opts := []*PlannerOptions{}

	// linear motion profile has known intermediate points, so solving can be broken up and sped up
	if profile, ok := motionConfig["motion_profile"]; ok && profile == LinearMotionProfile {
		pathStepSize, ok := motionConfig["path_step_size"].(float64)
		if !ok {
			pathStepSize = defaultPathStepSize
		}
		numSteps := GetSteps(seedPos, goalPos, pathStepSize)
		goals = make([]spatial.Pose, 0, numSteps)

		from := seedPos
		for i := 1; i < numSteps; i++ {
			by := float64(i) / float64(numSteps)
			to := spatial.Interpolate(seedPos, goalPos, by)
			goals = append(goals, to)
			opt, err := plannerSetupFromMoveRequest(from, to, sf, sf.completeFs, seedMap, worldState, motionConfig)
			if err != nil {
				return nil, err
			}
			opts = append(opts, opt)

			from = to
		}
		goals = append(goals, goalPos)
		opt, err := plannerSetupFromMoveRequest(from, goalPos, sf, sf.completeFs, seedMap, worldState, motionConfig)
		if err != nil {
			return nil, err
		}
		opts = append(opts, opt)
	} else {
		opt, err := plannerSetupFromMoveRequest(seedPos, goalPos, sf, sf.completeFs, seedMap, worldState, motionConfig)
		if err != nil {
			return nil, err
		}
		opts = append(opts, opt)
	}

	resultSlices, err := runPlannerWithWaypoints(ctx, planner, goals, seed, opts, 0)
	if err != nil {
		return nil, err
	}
	return resultSlices, nil
}

// Name returns the name of the solver referenceframe.
func (sf *solverFrame) Name() string {
	return sf.name
}

// Transform returns the pose between the two frames of this solver for a given set of inputs.
func (sf *solverFrame) Transform(inputs []frame.Input) (spatial.Pose, error) {
	if len(inputs) != len(sf.DoF()) {
		return nil, frame.NewIncorrectInputLengthError(len(inputs), len(sf.DoF()))
	}
	pf := frame.NewPoseInFrame(sf.solveFrame.Name(), spatial.NewZeroPose())
	solveName := sf.goalFrame.Name()
	if sf.worldRooted {
		solveName = frame.World
	}
	tf, err := sf.movingFs.Transform(sf.sliceToMap(inputs), pf, solveName)
	if err != nil {
		return nil, err
	}
	return tf.(*frame.PoseInFrame).Pose(), nil
}

// InputFromProtobuf converts pb.JointPosition to inputs.
func (sf *solverFrame) InputFromProtobuf(jp *pb.JointPositions) []frame.Input {
	inputs := make([]frame.Input, 0, len(jp.Values))
	posIdx := 0
	for _, transform := range sf.frames {
		dof := len(transform.DoF()) + posIdx
		jPos := jp.Values[posIdx:dof]
		posIdx = dof

		inputs = append(inputs, transform.InputFromProtobuf(&pb.JointPositions{Values: jPos})...)
	}
	return inputs
}

// ProtobufFromInput converts inputs to pb.JointPosition.
func (sf *solverFrame) ProtobufFromInput(input []frame.Input) *pb.JointPositions {
	jPos := &pb.JointPositions{}
	posIdx := 0
	for _, transform := range sf.frames {
		dof := len(transform.DoF()) + posIdx
		jPos.Values = append(jPos.Values, transform.ProtobufFromInput(input[posIdx:dof]).Values...)
		posIdx = dof
	}
	return jPos
}

// Geometry takes a solverFrame and a list of joint angles in radians and computes the 3D space occupied by each of the
// geometries in the solverFrame in the reference frame of the World frame.
func (sf *solverFrame) Geometries(inputs []frame.Input) (*frame.GeometriesInFrame, error) {
	if len(inputs) != len(sf.DoF()) {
		return nil, frame.NewIncorrectInputLengthError(len(inputs), len(sf.DoF()))
	}
	var errAll error
	inputMap := sf.sliceToMap(inputs)
	sfGeometries := make(map[string]spatial.Geometry)
	for _, fName := range sf.movingFs.FrameNames() {
		f := sf.movingFs.Frame(fName)
		if f == nil {
			return nil, frame.NewFrameMissingError(fName)
		}
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
		tf, err = sf.completeFs.Transform(inputMap, gf, frame.World)
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
	inputs := map[string][]frame.Input{}
	for k, v := range sf.origSeed {
		inputs[k] = v
	}
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

// findPivotFrame finds the first common frame in two ordered lists of frames.
func findPivotFrame(frameList1, frameList2 []frame.Frame) (frame.Frame, error) {
	// find shorter list
	shortList := frameList1
	longList := frameList2
	if len(frameList1) > len(frameList2) {
		shortList = frameList2
		longList = frameList1
	}

	// cache names seen in shorter list
	nameSet := make(map[string]struct{}, len(shortList))
	for _, frame := range shortList {
		nameSet[frame.Name()] = struct{}{}
	}

	// look for already seen names in longer list
	for _, frame := range longList {
		if _, ok := nameSet[frame.Name()]; ok {
			return frame, nil
		}
	}
	return nil, errors.New("no path from solve frame to goal frame")
}
