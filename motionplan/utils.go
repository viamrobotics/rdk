//go:build !no_cgo

package motionplan

import (
	"errors"
	"fmt"
	"math"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// CalculateStepCount will determine the number of steps which should be used to get from the seed to the goal.
// The returned value is guaranteed to be at least 1.
// stepSize represents both the max mm movement per step, and max R4AA degrees per step.
func CalculateStepCount(seedPos, goalPos spatialmath.Pose, stepSize float64) int {
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

// linearizedFrameSystem wraps a framesystem, allowing conversion in a known order between a FrameConfiguratinos and a flat array of floats,
// useful for being able to call IK solvers against framesystems.
type linearizedFrameSystem struct {
	fs     referenceframe.FrameSystem
	frames []referenceframe.Frame // cached ordering of frames. Order is unimportant but cannot change once set.
	dof    []referenceframe.Limit
}

func newLinearizedFrameSystem(fs referenceframe.FrameSystem) (*linearizedFrameSystem, error) {
	frames := []referenceframe.Frame{}
	dof := []referenceframe.Limit{}
	for _, fName := range fs.FrameNames() {
		frame := fs.Frame(fName)
		if frame == nil {
			return nil, fmt.Errorf("frame %s was returned in list of frame names, but was not found in frame system", fName)
		}
		frames = append(frames, frame)
		dof = append(dof, frame.DoF()...)
	}
	return &linearizedFrameSystem{
		fs:     fs,
		frames: frames,
		dof:    dof,
	}, nil
}

// mapToSlice will flatten a map of inputs into a slice suitable for input to inverse kinematics, by concatenating
// the inputs together in the order of the frames in sf.frames.
func (lfs *linearizedFrameSystem) mapToSlice(inputs referenceframe.FrameSystemInputs) ([]float64, error) {
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

func (lfs *linearizedFrameSystem) sliceToMap(floatSlice []float64) (referenceframe.FrameSystemInputs, error) {
	inputs := referenceframe.FrameSystemInputs{}
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
// path between components in the framesystem allowing knowledge of which frames may move.
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
}

func motionChainFromGoal(fs referenceframe.FrameSystem, moveFrame, goalFrameName string) (*motionChain, error) {
	// get goal frame
	goalFrame := fs.Frame(goalFrameName)
	if goalFrame == nil {
		return nil, referenceframe.NewFrameMissingError(goalFrameName)
	}
	goalFrameList, err := fs.TracebackFrame(goalFrame)
	if err != nil {
		return nil, err
	}

	// get solve frame
	solveFrame := fs.Frame(moveFrame)
	if solveFrame == nil {
		return nil, referenceframe.NewFrameMissingError(moveFrame)
	}
	solveFrameList, err := fs.TracebackFrame(solveFrame)
	if err != nil {
		return nil, err
	}
	if len(solveFrameList) == 0 {
		return nil, errors.New("solveFrameList was empty")
	}

	movingFS := func(frameList []referenceframe.Frame) (referenceframe.FrameSystem, error) {
		// Find first moving frame
		var moveF referenceframe.Frame
		for i := len(frameList) - 1; i >= 0; i-- {
			if len(frameList[i].DoF()) != 0 {
				moveF = frameList[i]
				break
			}
		}
		if moveF == nil {
			return referenceframe.NewEmptyFrameSystem(""), nil
		}
		return fs.FrameSystemSubset(moveF)
	}

	// find pivot frame between goal and solve frames
	var moving referenceframe.FrameSystem
	var frames []referenceframe.Frame
	worldRooted := false
	pivotFrame, err := findPivotFrame(solveFrameList, goalFrameList)
	if err != nil {
		return nil, err
	}
	if pivotFrame.Name() == referenceframe.World {
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
		var solveMovingList []referenceframe.Frame
		var goalMovingList []referenceframe.Frame

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

	return &motionChain{
		movingFS:       moving,
		frames:         frames,
		solveFrameName: solveFrame.Name(),
		goalFrameName:  goalFrame.Name(),
		worldRooted:    worldRooted,
	}, nil
}

// findPivotFrame finds the first common frame in two ordered lists of frames.
func findPivotFrame(frameList1, frameList2 []referenceframe.Frame) (referenceframe.Frame, error) {
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

func nodeConfigurationDistanceFunc(node1, node2 node) float64 {
	return ik.FSConfigurationL2Distance(&ik.SegmentFS{StartConfiguration: node1.Q(), EndConfiguration: node2.Q()})
}

// If a motion chain is worldrooted, then goals are translated to their position in `World` before solving.
// This is useful when e.g. moving a gripper relative to a point seen by a camera built into that gripper.
func alterGoals(
	chains []*motionChain,
	fs referenceframe.FrameSystem,
	start referenceframe.FrameSystemInputs,
	goal *PlanState,
) (*PlanState, error) {
	alteredGoals := referenceframe.FrameSystemPoses{}
	if goal.poses != nil {
		for _, chain := range chains {
			// chain solve frame may only be in the goal configuration, in which case we skip as the configuration will be passed through
			if goalPif, ok := goal.poses[chain.solveFrameName]; ok {
				if chain.worldRooted {
					tf, err := fs.Transform(start, goalPif, referenceframe.World)
					if err != nil {
						return nil, err
					}
					alteredGoals[chain.solveFrameName] = tf.(*referenceframe.PoseInFrame)
				} else {
					alteredGoals[chain.solveFrameName] = goalPif
				}
			}
		}
		return &PlanState{poses: alteredGoals, configuration: goal.configuration}, nil
	}
	return goal, nil
}
