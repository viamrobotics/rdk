package motionplan

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/utils"

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
	return fss.SolvePoseWithOptions(ctx, seedMap, goal, solveFrameName, goalFrameName, nil)
}

// SolvePoseWithOptions will take a set of starting positions, a goal frame, a frame to solve for, a pose, and a configurable
// set of PlannerOptions. It will solve the solveFrame to the goal pose with respect to the goal frame using the provided
// planning options.
func (fss *SolvableFrameSystem) SolvePoseWithOptions(ctx context.Context,
	seedMap map[string][]frame.Input,
	goal spatial.Pose,
	solveFrameName, goalFrameName string,
	opt *PlannerOptions,
) ([]map[string][]frame.Input, error) {
	return fss.SolveWaypointsWithOptions(ctx, seedMap, []spatial.Pose{goal}, solveFrameName, goalFrameName, []*PlannerOptions{opt})
}

// SolveWaypointsWithOptions will take a set of starting positions, a goal frame, a frame to solve for, goal poses, and a configurable
// set of PlannerOptions. It will solve the solveFrame to the goal poses with respect to the goal frame using the provided
// planning options.
func (fss *SolvableFrameSystem) SolveWaypointsWithOptions(ctx context.Context,
	seedMap map[string][]frame.Input,
	goals []spatial.Pose,
	solveFrameName, goalFrameName string,
	opts []*PlannerOptions,
) ([]map[string][]frame.Input, error) {
	if len(opts) == 0 {
		for i := 0; i < len(goals); i++ {
			opts = append(opts, NewDefaultPlannerOptions())
		}
	}
	if len(opts) != len(goals) {
		return nil, errors.New("goals and options had different lengths")
	}

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
	if err != nil {
		return nil, err
	}
	for _, resultSlice := range resultSlices {
		steps = append(steps, sf.sliceToMapConf(resultSlice))
	}

	return steps, nil
}

func plannerRunner(ctx context.Context,
	planner MotionPlanner,
	goals []spatial.Pose,
	seed []frame.Input,
	opts []*PlannerOptions,
	iter int,
) ([]*configuration, error) {
	var err error
	goal := goals[iter]
	opt := opts[iter]
	if opt == nil {
		opt = NewDefaultPlannerOptions()
	}
	remainingSteps := []*configuration{}
	if cbert, ok := planner.(*cBiRRTMotionPlanner); ok {
		// cBiRRT supports solution look-ahead for parallel waypoint solving
		endpointPreview := make(chan *configuration, 1)
		solutionChan := make(chan *planReturn, 1)
		utils.PanicCapturingGo(func() {
			// TODO(rb) fix me
			cbert.planRunner(
				ctx,
				spatial.PoseToProtobuf(goal),
				seed,
				opt,
				endpointPreview,
				solutionChan,
			)
		})
		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			select {
			case nextSeed := <-endpointPreview:
				// Got a solution preview, start solving the next motion in a new thread.
				if iter < len(goals)-1 {
					// In this case, we create the next step (and thus the remaining steps) and the
					// step from our iteration hangs out in the channel buffer until we're done with it.
					remainingSteps, err = plannerRunner(ctx, planner, goals, nextSeed.inputs, opts, iter+1)
					if err != nil {
						return nil, err
					}
				}
				for {
					// Get the step from this runner invocation, and return everything in order.
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					default:
					}

					select {
					case finalSteps := <-solutionChan:
						if finalSteps.err != nil {
							return nil, finalSteps.err
						}
						return append(finalSteps.steps, remainingSteps...), nil
					default:
					}
				}
			case finalSteps := <-solutionChan:
				// We didn't get a solution preview (possible error), so we get and process the full step set and error.
				if finalSteps.err != nil {
					return nil, finalSteps.err
				}
				if iter < len(goals)-1 {
					// in this case, we create the next step (and thus the remaining steps) and the
					// step from our iteration hangs out in the channel buffer until we're done with it
					remainingSteps, err = plannerRunner(ctx, planner, goals, finalSteps.steps[len(finalSteps.steps)-1].inputs, opts, iter+1)
					if err != nil {
						return nil, err
					}
				}
				return append(finalSteps.steps, remainingSteps...), nil
			default:
			}
		}
	} else {
		resultSlices := []*configuration{}
		resultSlicesRaw, err := planner.Plan(ctx, spatial.PoseToProtobuf(goal), seed, opt)
		if err != nil {
			return nil, err
		}
		for _, step := range resultSlicesRaw {
			resultSlices = append(resultSlices, &configuration{step})
		}
		if iter < len(goals)-2 {
			// in this case, we create the next step (and thus the remaining steps) and the
			// step from our iteration hangs out in the channel buffer until we're done with it
			remainingSteps, err = plannerRunner(ctx, planner, goals, resultSlicesRaw[len(resultSlicesRaw)-1], opts, iter+1)
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

// Geometry takes a solverFrame and a list of joint angles in radians and computes the 3D space occupied by each of the
// intermediate frames (if any exist) up to and including the end effector, and returns a map of frame names to geometries.
// The key for each frame in the map will be the string: "<model_name>:<frame_name>".
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
		tf, err = sf.fss.Transform(inputMap, gf, sf.goalFrame.Name())
		if err != nil {
			return nil, err
		}
		for name, geometry := range tf.(*frame.GeometriesInFrame).Geometries() {
			sfGeometries[name] = geometry
		}
	}
	return frame.NewGeometriesInFrame(sf.goalFrame.Name(), sfGeometries), errAll
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

func (sf *solverFrame) sliceToMapConf(inputSlice *configuration) map[string][]frame.Input {
	inputs := frame.StartPositions(sf.fss)
	i := 0
	for _, frame := range sf.frames {
		fLen := i + len(frame.DoF())
		inputs[frame.Name()] = inputSlice.inputs[i:fLen]
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
