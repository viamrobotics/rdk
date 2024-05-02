package motionplan

import (
	"errors"
	"fmt"
	"strings"

	"go.uber.org/multierr"
	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/motionplan/tpspace"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

// solverFrames are meant to be ephemerally created each time a frame system solution is created, and fulfills the
// Frame interface so that it can be passed to inverse kinematics.
type solverFrame struct {
	name string
	fss  frame.FrameSystem
	// List of names of all frames that could move, used for collision detection
	// As an example a gripper attached to an arm which is moving relative to World, would not be in frames below but in this object
	movingFS       frame.FrameSystem
	frames         []frame.Frame // all frames directly between and including solveFrame and goalFrame. Order not important.
	solveFrameName string
	goalFrameName  string
	// If this is true, then goals are translated to their position in `World` before solving.
	// This is useful when e.g. moving a gripper relative to a point seen by a camera built into that gripper
	// TODO(pl): explore allowing this to be frames other than world
	worldRooted bool
	origSeed    map[string][]frame.Input // stores starting locations of all frames in fss that are NOT in `frames`

	ptgs []tpspace.PTGSolver
}

func newSolverFrame(fs frame.FrameSystem, solveFrameName, goalFrameName string, seedMap map[string][]frame.Input) (*solverFrame, error) {
	// get goal frame
	goalFrame := fs.Frame(goalFrameName)
	if goalFrame == nil {
		return nil, frame.NewFrameMissingError(goalFrameName)
	}
	goalFrameList, err := fs.TracebackFrame(goalFrame)
	if err != nil {
		return nil, err
	}

	// get solve frame
	solveFrame := fs.Frame(solveFrameName)
	if solveFrame == nil {
		return nil, frame.NewFrameMissingError(solveFrameName)
	}
	// fmt.Println("SOLVER FRAME fs.FrameNames(): ", fs.FrameNames())
	solveFrameList, err := fs.TracebackFrame(solveFrame)
	if err != nil {
		return nil, err
	}
	if len(solveFrameList) == 0 {
		return nil, errors.New("solveFrameList was empty")
	}

	movingFS := func(frameList []frame.Frame) (frame.FrameSystem, error) {
		// Find first moving frame
		var moveF frame.Frame
		for i := len(frameList) - 1; i >= 0; i-- {
			if len(frameList[i].DoF()) != 0 {
				moveF = frameList[i]
				break
			}
		}
		if moveF == nil {
			return frame.NewEmptyFrameSystem(""), nil
		}
		return fs.FrameSystemSubset(moveF)
	}

	// find pivot frame between goal and solve frames
	var moving frame.FrameSystem
	var frames []frame.Frame
	worldRooted := false
	pivotFrame, err := findPivotFrame(solveFrameList, goalFrameList)
	if err != nil {
		return nil, err
	}
	// fmt.Println("pivotFrame.Name(): ", pivotFrame.Name())
	if pivotFrame.Name() == frame.World {
		// fmt.Println("printing members of solveFrameList")
		// for _, f := range solveFrameList {
		// 	fmt.Println("f.Name(): ", f.Name())
		// }
		// fmt.Println("printing members of goalFrameList")
		// for _, f := range goalFrameList {
		// 	fmt.Println("f.Name(): ", f.Name())
		// }
		// fmt.Println("DONE")
		frames = uniqInPlaceSlice(append(solveFrameList, goalFrameList...))
		moving, err = movingFS(solveFrameList)
		if err != nil {
			return nil, err
		}
		movingSubset2, err := movingFS(goalFrameList)
		if err != nil {
			return nil, err
		}
		if err = moving.MergeFrameSystem(movingSubset2, moving.World()); err != nil {
			return nil, err
		}
	} else {
		dof := 0
		var solveMovingList []frame.Frame
		var goalMovingList []frame.Frame

		// Get minimal set of frames from solve frame to goal frame
		for _, frame := range solveFrameList {
			if frame == pivotFrame {
				break
			}
			dof += len(frame.DoF())
			// fmt.Println("will now append: ", frame.Name())
			frames = append(frames, frame)
			solveMovingList = append(solveMovingList, frame)
		}
		for _, frame := range goalFrameList {
			if frame == pivotFrame {
				break
			}
			dof += len(frame.DoF())
			// fmt.Println("will now append: ", frame.Name())
			frames = append(frames, frame)
			goalMovingList = append(goalMovingList, frame)
		}

		// If shortest path has 0 dof (e.g. a camera attached to a gripper), translate goal to world frame
		if dof == 0 {
			worldRooted = true
			frames = solveFrameList
			// fmt.Println("we are here 1")
			moving, err = movingFS(solveFrameList)
			if err != nil {
				return nil, err
			}
		} else {
			// fmt.Println("we are here 2")
			// Get all child nodes of pivot node
			moving, err = movingFS(solveMovingList)
			if err != nil {
				return nil, err
			}
			movingSubset2, err := movingFS(goalMovingList)
			if err != nil {
				return nil, err
			}
			if err = moving.MergeFrameSystem(movingSubset2, moving.World()); err != nil {
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

	// fmt.Println("THIS IS THE ORIG SEED: ", origSeed)

	var ptgs []tpspace.PTGSolver
	anyPTG := false // Whether PTG frames have been observed
	// TODO(nf): uncomment
	// anyNonzero := false // Whether non-PTG frames
	for _, movingFrame := range frames {
		if ptgFrame, isPTGframe := movingFrame.(tpspace.PTGProvider); isPTGframe {
			if anyPTG {
				return nil, errors.New("only one PTG frame can be planned for at a time")
			}
			anyPTG = true
			ptgs = ptgFrame.PTGSolvers()
		} // else if len(movingFrame.DoF()) > 0 {
		// anyNonzero = true
		// }
		// if anyNonzero && anyPTG {
		// 	return nil, errors.New("cannot combine ptg with other nonzero DOF frames in a single planning call")
		// }
	}

	return &solverFrame{
		name:           solveFrame.Name() + "_" + goalFrame.Name(),
		fss:            fs,
		movingFS:       moving,
		frames:         frames,
		solveFrameName: solveFrame.Name(),
		goalFrameName:  goalFrame.Name(),
		worldRooted:    worldRooted,
		origSeed:       origSeed,
		ptgs:           ptgs,
	}, nil
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
	pf := frame.NewPoseInFrame(sf.solveFrameName, spatial.NewZeroPose())
	solveName := sf.goalFrameName
	if sf.worldRooted {
		solveName = frame.World
	}
	tf, err := sf.fss.Transform(sf.sliceToMap(inputs), pf, solveName)
	if err != nil {
		return nil, err
	}
	return tf.(*frame.PoseInFrame).Pose(), nil
}

// Interpolate interpolates the given amount between the two sets of inputs.
func (sf *solverFrame) Interpolate(from, to []frame.Input, by float64) ([]frame.Input, error) {
	if len(from) != len(sf.DoF()) {
		return nil, frame.NewIncorrectInputLengthError(len(from), len(sf.DoF()))
	}
	if len(to) != len(sf.DoF()) {
		return nil, frame.NewIncorrectInputLengthError(len(to), len(sf.DoF()))
	}
	interp := make([]frame.Input, 0, len(to))
	posIdx := 0
	for _, currFrame := range sf.frames {
		// if we are dealing with the execution frame, no need to interpolate, just return what we got
		dof := len(currFrame.DoF()) + posIdx
		fromSubset := from[posIdx:dof]
		toSubset := to[posIdx:dof]
		posIdx = dof
		var interpSub []frame.Input
		var err error
		// fmt.Println("currFrame.Name(): ", currFrame.Name())
		if strings.Contains(currFrame.Name(), "ExecutionFrame") {
			fmt.Println("this conditional is hit!")
			interp = append(interp, from...)
			continue
		}
		interpSub, err = currFrame.Interpolate(fromSubset, toSubset, by)
		if err != nil {
			return nil, err
		}

		interp = append(interp, interpSub...)
	}
	return interp, nil
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
	sfGeometries := []spatial.Geometry{}

	for _, fName := range sf.movingFS.FrameNames() {
		if strings.Contains(fName, "ExecutionFrame") {
			continue
		}
		f := sf.fss.Frame(fName)
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
		tf, err = sf.fss.Transform(inputMap, gf, frame.World)
		if err != nil {
			return nil, err
		}
		sfGeometries = append(sfGeometries, tf.(*frame.GeometriesInFrame).Geometries()...)
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

// PTGSolvers passes through the PTGs of the solving tp-space frame if it exists, otherwise nil.
func (sf *solverFrame) PTGSolvers() []tpspace.PTGSolver {
	return sf.ptgs
}

func (sf *solverFrame) movingFrame(name string) bool {
	return sf.movingFS.Frame(name) != nil
}

// mapToSlice will flatten a map of inputs into a slice suitable for input to inverse kinematics, by concatenating
// the inputs together in the order of the frames in sf.frames.
func (sf *solverFrame) mapToSlice(inputMap map[string][]frame.Input) ([]frame.Input, error) {
	var inputs []frame.Input
	fmt.Println("inputMap: ", inputMap)
	fmt.Println("printing all members of sf.frames below")
	for _, f := range sf.frames {
		fmt.Println("f.Name(): ", f.Name())
	}
	fmt.Println("DONE -- printing all members of sf.frames")
	for _, f := range sf.frames {
		if len(f.DoF()) == 0 {
			fmt.Println("we are continuing here?")
			fmt.Println("f.Name(): ", f.Name())
			fmt.Println("f.DoF(): ", f.DoF())
			continue
		}
		input, err := frame.GetFrameInputs(f, inputMap)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, input...)
	}
	return inputs, nil
}

func (sf *solverFrame) sliceToMap(inputSlice []frame.Input) map[string][]frame.Input {
	inputs := map[string][]frame.Input{}
	for k, v := range sf.origSeed {
		inputs[k] = v
	}
	i := 0
	for _, frame := range sf.frames {
		if len(frame.DoF()) == 0 {
			continue
		}
		fLen := i + len(frame.DoF())
		inputs[frame.Name()] = inputSlice[i:fLen]
		i = fLen
	}
	return inputs
}

func (sf solverFrame) MarshalJSON() ([]byte, error) {
	return nil, errors.New("cannot serialize solverFrame")
}

func (sf *solverFrame) AlmostEquals(otherFrame frame.Frame) bool {
	return false
}

// TODO: move this from being a method on sf to a normal helper in plan.go
// nodesToTrajectory takes a slice of nodes and converts it to a trajectory.
func (sf solverFrame) nodesToTrajectory(nodes []node) Trajectory {
	traj := make(Trajectory, 0, len(nodes))
	for _, n := range nodes {
		stepMap := sf.sliceToMap(n.Q())
		traj = append(traj, stepMap)
	}
	return traj
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
