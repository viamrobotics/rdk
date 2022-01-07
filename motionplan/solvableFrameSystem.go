package motionplan

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

// SolvableFrameSystem wraps a FrameSystem to allow solving between frames of the frame system.
// Note that this needs to live in motionplan, not referenceframe, to avoid circular dependencies.
type SolvableFrameSystem struct {
	referenceframe.FrameSystem
	logger golog.Logger
	mpFunc func(referenceframe.Frame, int, golog.Logger) (MotionPlanner, error)
}

// NewSolvableFrameSystem will create a new solver for a frame system.
func NewSolvableFrameSystem(fs referenceframe.FrameSystem, logger golog.Logger) *SolvableFrameSystem {
	return &SolvableFrameSystem{FrameSystem: fs, logger: logger}
}

// SolvePose will take a set of starting positions, a goal frame, a frame to solve for, and a pose. The function will
// then try to path plan the full frame system such that the solveFrame has the goal pose from the perspective of the goalFrame.
// For example, if a world system has a gripper attached to an arm attached to a gantry, and the system was being solved
// to place the gripper at a particular pose in the world, the solveFrame would be the gripper and the goalFrame would be
// the world frame. It will use the default planner options.
func (fss *SolvableFrameSystem) SolvePose(ctx context.Context,
	seedMap map[string][]referenceframe.Input,
	goal spatial.Pose,
	solveFrame, goalFrame referenceframe.Frame,
) ([]map[string][]referenceframe.Input, error) {
	return fss.SolvePoseWithOptions(ctx, seedMap, goal, solveFrame, goalFrame, nil)
}

// SolvePoseWithOptions will take a set of starting positions, a goal frame, a frame to solve for, a pose, and a configurable
// set of PlannerOptions. It will solve the solveFrame to the goal pose with respect to the goal frame using the provided
// planning options.
func (fss *SolvableFrameSystem) SolvePoseWithOptions(ctx context.Context,
	seedMap map[string][]referenceframe.Input,
	goal spatial.Pose,
	solveFrame, goalFrame referenceframe.Frame,
	opt *PlannerOptions,
) ([]map[string][]referenceframe.Input, error) {
	return fss.SolveWaypointsWithOptions(ctx, seedMap, []spatial.Pose{goal}, solveFrame, goalFrame, []*PlannerOptions{opt})
}

// SolvePoseWithOptions will take a set of starting positions, a goal frame, a frame to solve for, a pose, and a configurable
// set of PlannerOptions. It will solve the solveFrame to the goal pose with respect to the goal frame using the provided
// planning options.
func (fss *SolvableFrameSystem) SolveWaypointsWithOptions(ctx context.Context,
	seedMap map[string][]referenceframe.Input,
	goals []spatial.Pose,
	solveFrame, goalFrame referenceframe.Frame,
	opts []*PlannerOptions,
) ([]map[string][]referenceframe.Input, error) {
	if len(opts) == 0 {
		for i := 0; i < len(goals); i++ {
			opts = append(opts, NewDefaultPlannerOptions())
		}
	}
	if len(opts) != len(goals){
		return nil, errors.New("goals and options had different lengths")
	}
	
	steps := make([]map[string][]referenceframe.Input, 0, len(goals)*2)

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

	// Create a frame to solve for, and an IK solver with that referenceframe.
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

	resultSlices, err := plannerRunner(ctx, planner, goals, seed, opts, 0)
	for _, resultSlice := range resultSlices {
		steps = append(steps, sf.sliceToMap(resultSlice))
	}

	return steps, nil
}

func plannerRunner(ctx context.Context,
	planner MotionPlanner,
	goals []spatial.Pose,
	seed []referenceframe.Input,
	opts []*PlannerOptions,
	iter int,
) ([][]referenceframe.Input, error) {
	var err error
	goal := goals[iter]
	opt := opts[iter]
	if opt == nil {
		opt = NewDefaultPlannerOptions()
	}
	if cbert, ok := planner.(*cBiRRTMotionPlanner); ok {
		// cBiRRT supports solution look-ahead for parallel waypoint solving
		opt.solutionPreview = make(chan *solution, 1)
		solutionChan := make(chan *planReturn, 1)
		utils.PanicCapturingGo(func() {
			cbert.planRunner(ctx, spatial.PoseToProtobuf(goal), seed, opt, solutionChan)
		})
		for{
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			select {
			case nextSeed := <-opt.solutionPreview:
				var remainingSteps [][]referenceframe.Input
				if iter < len(goals) - 2 {
					// in this case, we create the next step (and thus the remaining steps) and the
					// step from our iteration hangs out in the channel buffer until we're done with it
					remainingSteps, err = plannerRunner(ctx, planner, goals, nextSeed.inputs, opts, iter + 1)
					if err != nil {
						return nil, err
					}
				}
				for{
					// get the step from this runner invocation, and return everything in order
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					default:
					}
					
					select {
					case finalSteps := <- solutionChan:
						if finalSteps.err != nil {
							return nil, finalSteps.err
						}
						return append(finalSteps.steps, remainingSteps...), nil
					default:
					}
				}
			case finalSteps := <- solutionChan:
				// We didn't get a solution 
				if finalSteps.err != nil {
					return nil, finalSteps.err
				}
				var remainingSteps [][]referenceframe.Input
				if iter < len(goals) - 2 {
					// in this case, we create the next step (and thus the remaining steps) and the
					// step from our iteration hangs out in the channel buffer until we're done with it
					remainingSteps, err = plannerRunner(ctx, planner, goals, finalSteps.steps[len(finalSteps.steps) - 1], opts, iter + 1)
					if err != nil {
						return nil, err
					}
				}
				return append(finalSteps.steps, remainingSteps...), nil
			default:
			}
		}
	}else{
		resultSlices, err := planner.Plan(ctx, spatial.PoseToProtobuf(goal), seed, opt)
		var remainingSteps [][]referenceframe.Input
		if iter < len(goals) - 2 {
			// in this case, we create the next step (and thus the remaining steps) and the
			// step from our iteration hangs out in the channel buffer until we're done with it
			remainingSteps, err = plannerRunner(ctx, planner, goals, resultSlices[len(resultSlices) - 1], opts, iter + 1)
			if err != nil {
				return nil, err
			}
		}
		return append(resultSlices, remainingSteps...), nil
	}
}

// SetPlannerGen sets the function which is used to create the motion planner to solve a requested plan.
// A SolvableFrameSystem wraps a complete frame system, and will make solverFrames on the fly to solve for. These
// solverFrames are used to create the planner here.
func (fss *SolvableFrameSystem) SetPlannerGen(mpFunc func(referenceframe.Frame, int, golog.Logger) (MotionPlanner, error)) {
	fss.mpFunc = mpFunc
}

// solverFrames are meant to be ephemerally created each time a frame system solution is created, and fulfills the
// Frame interface so that it can be passed to inverse kinematics.
type solverFrame struct {
	name       string
	fss        *SolvableFrameSystem
	frames     []referenceframe.Frame
	solveFrame referenceframe.Frame
	goalFrame  referenceframe.Frame
}

// Name returns the name of the solver referenceframe.
func (sf *solverFrame) Name() string {
	return sf.name
}

// Transform returns the pose between the two frames of this solver for a given set of inputs.
func (sf *solverFrame) Transform(inputs []referenceframe.Input) (spatial.Pose, error) {
	if len(inputs) != len(sf.DoF()) {
		return nil, fmt.Errorf("incorrect number of inputs to Transform got %d want %d", len(inputs), len(sf.DoF()))
	}
	return sf.fss.TransformFrame(sf.sliceToMap(inputs), sf.solveFrame, sf.goalFrame)
}

// VerboseTransform takes a solverFrame and a list of joint angles in radians and computes the dual quaterions
// representing poses of each of the intermediate frames (if any exist) up to and including the end effector, and
// returns a map of frame names to poses. The key for each frame in the map will be the string
// "<model_name>:<frame_name>".
func (sf *solverFrame) VerboseTransform(inputs []referenceframe.Input) (map[string]spatial.Pose, error) {
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
func (sf *solverFrame) DoF() []referenceframe.Limit {
	var limits []referenceframe.Limit
	for _, frame := range sf.frames {
		limits = append(limits, frame.DoF()...)
	}
	return limits
}

// mapToSlice will flatten a map of inputs into a slice suitable for input to inverse kinematics, by concatenating
// the inputs together in the order of the frames in sf.frames.
func (sf *solverFrame) mapToSlice(inputMap map[string][]referenceframe.Input) []referenceframe.Input {
	var inputs []referenceframe.Input
	for _, frame := range sf.frames {
		inputs = append(inputs, inputMap[frame.Name()]...)
	}
	return inputs
}

func (sf *solverFrame) sliceToMap(inputSlice []referenceframe.Input) map[string][]referenceframe.Input {
	inputs := referenceframe.StartPositions(sf.fss)
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

func (sf *solverFrame) AlmostEquals(otherFrame referenceframe.Frame) bool {
	return false
}

// uniqInPlaceSlice will deduplicate the values in a slice using in-place replacement on the slice. This is faster than
// a solution using append().
// This function does not remove anything from the input slice, but it does rearrange the elements.
func uniqInPlaceSlice(s []referenceframe.Frame) []referenceframe.Frame {
	seen := make(map[referenceframe.Frame]struct{}, len(s))
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
