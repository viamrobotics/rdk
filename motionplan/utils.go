//go:build !no_cgo

package motionplan

import (
	"fmt"
	"math"

	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// PathStepCount will determine the number of steps which should be used to get from the seed to the goal.
// The returned value is guaranteed to be at least 1.
// stepSize represents both the max mm movement per step, and max R4AA degrees per step.
func PathStepCount(seedPos, goalPos spatialmath.Pose, stepSize float64) int {
	// use a default size of 1 if zero is passed in to avoid divide-by-zero
	if stepSize == 0 {
		stepSize = 1.
	}

	mmDist := seedPos.Point().Distance(goalPos.Point())
	rDist := spatialmath.OrientationBetween(seedPos.Orientation(), goalPos.Orientation()).AxisAngles()

	nSteps := math.Max(math.Abs(mmDist/stepSize), math.Abs(utils.RadToDeg(rDist.Theta)/stepSize))
	return int(nSteps) + 1
}

type resultPromise struct {
	steps  []node
	future chan *rrtSolution
}

func (r *resultPromise) result() ([]node, error) {
	if r.steps != nil && len(r.steps) > 0 { //nolint:gosimple
		return r.steps, nil
	}
	// wait for a context cancel or a valid channel result
	planReturn := <-r.future
	if planReturn.err != nil {
		return nil, planReturn.err
	}
	return planReturn.steps, nil
}

// linearizedFrameSystem wraps a framesystem, allowing conversion in a known order between a map[string][]inputs and a flat array of floats,
// useful for being able to call IK solvers against framesystems.
type linearizedFrameSystem struct {
	fss    referenceframe.FrameSystem
	frames []referenceframe.Frame // cached ordering of frames. Order is unimportant but cannot change once set.
	dof    []referenceframe.Limit
}

func newLinearizedFrameSystem(fss referenceframe.FrameSystem) (*linearizedFrameSystem, error) {
	frames := []referenceframe.Frame{}
	dof := []referenceframe.Limit{}
	for _, fName := range fss.FrameNames() {
		frame := fss.Frame(fName)
		if frame == nil {
			return nil, fmt.Errorf("frame %s was returned in list of frame names, but was not found in frame system", fName)
		}
		frames = append(frames, frame)
		dof = append(dof, frame.DoF()...)
	}
	return &linearizedFrameSystem{
		fss:    fss,
		frames: frames,
		dof:    dof,
	}, nil
}

// mapToSlice will flatten a map of inputs into a slice suitable for input to inverse kinematics, by concatenating
// the inputs together in the order of the frames in sf.frames.
func (lfs *linearizedFrameSystem) mapToSlice(inputs map[string][]referenceframe.Input) ([]float64, error) {
	var floatSlice []float64
	for _, frame := range lfs.frames {
		if len(frame.DoF()) == 0 {
			continue
		}
		input, ok := inputs[frame.Name()]
		if !ok {
			return nil, fmt.Errorf("frame %s missing from input map", frame.Name())
		}
		for _, i := range input {
			floatSlice = append(floatSlice, i.Value)
		}
	}
	return floatSlice, nil
}

func (lfs *linearizedFrameSystem) sliceToMap(floatSlice []float64) (map[string][]referenceframe.Input, error) {
	inputs := map[string][]referenceframe.Input{}
	i := 0
	for _, frame := range lfs.frames {
		if len(frame.DoF()) == 0 {
			continue
		}
		frameInputs := make([]referenceframe.Input, len(frame.DoF()))
		for j := range frame.DoF() {
			if i >= len(floatSlice) {
				return nil, fmt.Errorf("not enough values in float slice for frame %s", frame.Name())
			}
			frameInputs[j] = referenceframe.Input{Value: floatSlice[i]}
			i++
		}
		inputs[frame.Name()] = frameInputs
	}
	return inputs, nil
}

// motionChain structs are meant to be ephemerally created for each individual goal in a motion request, and calculates the shortest
// path between components in the framesystem allowing knowledge of which frames may move
type motionChain struct {
	// List of names of all frames that could move, used for collision detection
	// As an example a gripper attached to an arm which is moving relative to World, would not be in frames below but in this object
	movingFS       referenceframe.FrameSystem
	frames         []referenceframe.Frame // all frames directly between and including solveFrame and goalFrame. Order not important.
	solveFrameName string
	goalFrameName  string
	// If this is true, then goals are translated to their position in `World` before solving.
	// This is useful when e.g. moving a gripper relative to a point seen by a camera built into that gripper
	// TODO(pl): explore allowing this to be frames other than world
	worldRooted bool
	origSeed    map[string][]referenceframe.Input // stores starting locations of all frames in fss that are NOT in `frames`

	ptgs []tpspace.PTGSolver
}

func motionChainFromGoal(fs referenceframe.FrameSystem, moveFrame string, goal *referenceframe.PoseInFrame) (*motionChain, error) {
	// get goal frame
	goalFrame := fs.Frame(goal.Parent())
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
	if pivotFrame.Name() == frame.World {
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
			frames = append(frames, frame)
			solveMovingList = append(solveMovingList, frame)
		}
		for _, frame := range goalFrameList {
			if frame == pivotFrame {
				break
			}
			dof += len(frame.DoF())
			frames = append(frames, frame)
			goalMovingList = append(goalMovingList, frame)
		}

		// If shortest path has 0 dof (e.g. a camera attached to a gripper), translate goal to world frame
		if dof == 0 {
			worldRooted = true
			frames = solveFrameList
			moving, err = movingFS(solveFrameList)
			if err != nil {
				return nil, err
			}
		} else {
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

	var ptgs []tpspace.PTGSolver
	anyPTG := false // Whether PTG frames have been observed
	for _, movingFrame := range frames {
		if ptgFrame, isPTGframe := movingFrame.(tpspace.PTGProvider); isPTGframe {
			if anyPTG {
				return nil, errors.New("only one PTG frame can be planned for at a time")
			}
			anyPTG = true
			ptgs = ptgFrame.PTGSolvers()
		}
	}

	return &motionChain{
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
