package referenceframe

import (
	"errors"
	"fmt"

	"github.com/golang/geo/r3"
	"go.uber.org/multierr"

	spatial "go.viam.com/rdk/spatialmath"
)

// World is the string "world", but made into an exported constant.
const World = "world"

// FrameSystem represents a tree of frames connected to each other, allowing for transformations between any two frames.
type FrameSystem interface {
	Name() string // return the name of this frame system
	World() Frame
	FrameNames() []string
	GetFrame(name string) Frame
	AddFrame(frame, parent Frame) error
	RemoveFrame(frame Frame)
	TracebackFrame(frame Frame) ([]Frame, error)
	Parent(frame Frame) (Frame, error)

	// TransformFrame takes in a source and destination frame, and returns the pose from the first to the second. Positions
	// is a map of inputs for any frames with non-zero DOF, with slices of inputs keyed to the frame name.
	TransformFrame(positions map[string][]Input, srcFrame, dstFrame Frame) (spatial.Pose, error)
	VolumesOfFrame(positions map[string][]Input, srcFrame, dstFrame Frame) (map[string]spatial.Volume, error)
	TransformPoint(positions map[string][]Input, point r3.Vector, srcFrame, dstFrame Frame) (r3.Vector, error)

	// TransformPose takes in a pose with respect to a source Frame, and outputs the pose with respect to the target referenceframe.
	// Positions is a map of inputs for any frames with non-zero DOF, with slices of inputs keyed to the frame name.
	// We the inputs tells us how to walk back from the input pose to the target pose
	TransformPose(positions map[string][]Input, pose spatial.Pose, srcFrame, dstFrame Frame) (spatial.Pose, error)

	DivideFrameSystem(newRoot Frame) (FrameSystem, error)
	MergeFrameSystem(systemToMerge FrameSystem, attachTo Frame) error
}

// simpleFrameSystem implements FrameSystem. It is a simple tree graph.
type simpleFrameSystem struct {
	name    string
	world   Frame // separate from the map of frames so it can be detached easily
	frames  map[string]Frame
	parents map[Frame]Frame
}

// NewEmptySimpleFrameSystem creates a graph of Frames that have.
func NewEmptySimpleFrameSystem(name string) FrameSystem {
	worldFrame := NewZeroStaticFrame(World)
	return &simpleFrameSystem{name, worldFrame, map[string]Frame{}, map[Frame]Frame{}}
}

// World returns the base world referenceframe.
func (sfs *simpleFrameSystem) World() Frame {
	return sfs.world
}

var errNoParent = errors.New("no parent")

// Parent returns the parent frame of the input referenceframe. nil if input is World.
func (sfs *simpleFrameSystem) Parent(frame Frame) (Frame, error) {
	if !sfs.frameExists(frame.Name()) {
		return nil, fmt.Errorf("frame with name %q not in frame system", frame.Name())
	}
	if frame == sfs.world {
		return nil, errNoParent
	}
	return sfs.parents[frame], nil
}

// frameExists is a helper function to see if a frame with a given name already exists in the system.
func (sfs *simpleFrameSystem) frameExists(name string) bool {
	if name == World {
		return true
	}
	if _, ok := sfs.frames[name]; ok {
		return true
	}
	return false
}

// RemoveFrame will delete the given frame and all descendents from the frame system if it exists.
func (sfs *simpleFrameSystem) RemoveFrame(frame Frame) {
	delete(sfs.frames, frame.Name())
	delete(sfs.parents, frame)

	// Remove all descendents
	for f, parent := range sfs.parents {
		if parent == frame {
			sfs.RemoveFrame(f)
		}
	}
}

// GetFrame returns the frame given the name of the referenceframe. Returns nil if the frame is not found.
func (sfs *simpleFrameSystem) GetFrame(name string) Frame {
	if !sfs.frameExists(name) {
		return nil
	}
	if name == World {
		return sfs.world
	}
	return sfs.frames[name]
}

// TracebackFrame traces the parentage of the given frame up to the world, and returns the full list of frames in between.
// The list will include both the query frame and the world referenceframe.
func (sfs *simpleFrameSystem) TracebackFrame(query Frame) ([]Frame, error) {
	if !sfs.frameExists(query.Name()) {
		return nil, fmt.Errorf("frame with name %q not in frame system", query.Name())
	}
	if query == sfs.world {
		return []Frame{query}, nil
	}
	parents, err := sfs.TracebackFrame(sfs.parents[query])
	if err != nil {
		return nil, err
	}
	return append([]Frame{query}, parents...), nil
}

// FrameNames returns the list of frame names registered in the frame system.
func (sfs *simpleFrameSystem) FrameNames() []string {
	var frameNames []string
	for k := range sfs.frames {
		frameNames = append(frameNames, k)
	}
	return frameNames
}

func (sfs *simpleFrameSystem) checkName(name string, parent Frame) error {
	// check to see if parent is in system
	if !sfs.frameExists(parent.Name()) {
		return fmt.Errorf("parent frame with name %q not in frame system", parent.Name())
	}
	// check if frame with that name is already in system
	if sfs.frameExists(name) {
		return fmt.Errorf("frame with name %q already in frame system", name)
	}
	return nil
}

// AddFrame sets an already defined Frame into the system.
func (sfs *simpleFrameSystem) AddFrame(frame, parent Frame) error {
	if parent == nil {
		return NewParentFrameMissingError()
	}
	err := sfs.checkName(frame.Name(), parent)
	if err != nil {
		return err
	}
	sfs.frames[frame.Name()] = frame
	sfs.parents[frame] = parent
	return nil
}

// TransformFrame takes in a source and destination frame, and returns the pose from the first to the second. Positions
// is a map of inputs for any frames with non-zero DOF, with slices of inputs keyed to the frame name.
func (sfs *simpleFrameSystem) TransformFrame(positions map[string][]Input, srcFrame, dstFrame Frame) (spatial.Pose, error) {
	if !sfs.frameExists(srcFrame.Name()) {
		return nil, fmt.Errorf("source frame %s not found in FrameSystem", srcFrame.Name())
	}
	return sfs.transformFromParent(positions, srcFrame, sfs.parents[srcFrame], dstFrame)
}

// VolumesOfFrame takes in a source and destination frame and returns the volumes from the srcFrame in the reference
// frame of the the second, in the form of a mapping between the name of the frame and its volume, and including any
// intermediate frames if they exist. Positions is a map of inputs for any frames with non-zero DOF, with slices of
// inputs keyed to the frame name.
func (sfs *simpleFrameSystem) VolumesOfFrame(positions map[string][]Input, srcFrame, dstFrame Frame) (map[string]spatial.Volume, error) {
	if !sfs.frameExists(srcFrame.Name()) {
		return nil, fmt.Errorf("source frame %s not found in FrameSystem", srcFrame.Name())
	}
	return sfs.volumesFromParent(positions, srcFrame, sfs.parents[srcFrame], dstFrame)
}

// TransformPoint takes in a point with respect to a source Frame, and outputs the point coordinates with respect to
// the target referenceframe. Positions is a map of inputs for any frames with non-zero DOF, with slices of inputs keyed to the frame name.
func (sfs *simpleFrameSystem) TransformPoint(positions map[string][]Input, point r3.Vector, srcFrame, dstFrame Frame) (r3.Vector, error) {
	// Turn point into an anonymous Frame
	pointFrame, err := FrameFromPoint("", point)
	if err != nil {
		return r3.Vector{}, err
	}
	// do Transform
	tf, err := sfs.transformFromParent(positions, pointFrame, srcFrame, dstFrame)
	if err != nil {
		return r3.Vector{}, err
	}
	return tf.Point(), nil
}

func (sfs *simpleFrameSystem) TransformPose(
	positions map[string][]Input,
	pose spatial.Pose,
	srcFrame, dstFrame Frame,
) (spatial.Pose, error) {
	poseFrame, err := NewStaticFrame("", pose)
	if err != nil {
		return nil, err
	}
	tf, err := sfs.transformFromParent(positions, poseFrame, srcFrame, dstFrame)
	return tf, err
}

// Name returns the name of the simpleFrameSystem.
func (sfs *simpleFrameSystem) Name() string {
	return sfs.name
}

// MergeFrameSystem will combine two frame systems together, placing the world of systemToMerge at the "attachTo" frame in sfs.
// The frame where systemToMerge will be attached to must already exist within sfs, so should be added before Merge happens.
// Merging is necessary when including remote robots, dynamically building systems of robots, or mutating a robot after it
// has already been initialized. For example, two independent rovers, each with their own frame system, need to now know where
// they are in relation to each other and need to have their frame systems combined.
func (sfs *simpleFrameSystem) MergeFrameSystem(systemToMerge FrameSystem, attachTo Frame) error {
	attachFrame := sfs.GetFrame(attachTo.Name())
	if attachFrame == nil {
		return fmt.Errorf("frame to attach to, %q, not in target frame system %q", attachTo.Name(), sfs.Name())
	}

	// make a map where the parent frame is the key and the slice of children frames is the value
	childrenMap := map[Frame][]Frame{}
	for _, name := range systemToMerge.FrameNames() {
		child := systemToMerge.GetFrame(name)
		parent, err := systemToMerge.Parent(child)
		if err != nil {
			if errors.Is(err, errNoParent) {
				continue
			}
			return err
		}
		childrenMap[parent] = append(childrenMap[parent], child)
	}
	// add every frame from systemToMerge to the base frame system.
	queue := []Frame{systemToMerge.World()}
	for len(queue) != 0 {
		parent := queue[0]
		queue = queue[1:]
		children := childrenMap[parent]
		for _, c := range children {
			queue = append(queue, c)
			if parent == systemToMerge.World() {
				err := sfs.AddFrame(c, attachFrame) // attach c to the attachFrame
				if err != nil {
					return err
				}
			} else {
				err := sfs.AddFrame(c, parent)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// DivideFrameSystem will take a frame system and a frame in that system, and return two frame systems- one being rooted
// at the given frame and containing all descendents of it, the other with the original world with the frame and its
// descendents removed. For example, if there is a frame system with two independent rovers, and one rover goes offline,
// A user could divide the frame system to remove the offline rover and have the rest of the frame system unaffected.
func (sfs *simpleFrameSystem) DivideFrameSystem(newRoot Frame) (FrameSystem, error) {
	newWorld := NewZeroStaticFrame(World)
	newFS := &simpleFrameSystem{newRoot.Name() + "_FS", newWorld, map[string]Frame{}, map[Frame]Frame{}}

	rootFrame := sfs.GetFrame(newRoot.Name())
	if rootFrame == nil {
		return nil, fmt.Errorf("newRoot frame not in fs %s", newRoot.Name())
	}

	delete(sfs.frames, newRoot.Name())
	delete(sfs.parents, newRoot)

	var traceParent func(Frame) bool
	traceParent = func(parent Frame) bool {
		// Determine to which frame system this frame and its parent should be added
		if parent == sfs.World() {
			// keep in sfs
			return false
		} else if parent == newRoot || newFS.frameExists(parent.Name()) {
			return true
		}
		return traceParent(sfs.parents[parent])
	}

	// Deleting from a map as we iterate through it is OK and safe to do in Go
	for frame, parent := range sfs.parents {
		var addNew bool
		if parent == newRoot {
			parent = newWorld
			addNew = true
		} else {
			addNew = traceParent(parent)
		}
		if addNew {
			newFS.frames[frame.Name()] = frame
			newFS.parents[frame] = parent
		}
	}

	sfs.RemoveFrame(rootFrame)

	return newFS, nil
}

// StartPositions returns a zeroed input map ensuring all frames have inputs.
func StartPositions(fs FrameSystem) map[string][]Input {
	positions := make(map[string][]Input)
	for _, fn := range fs.FrameNames() {
		frame := fs.GetFrame(fn)
		if frame != nil {
			positions[fn] = make([]Input, len(frame.DoF()))
		}
	}
	return positions
}

func (sfs *simpleFrameSystem) getSrcParentTransform(inputMap map[string][]Input, src, parent Frame) (spatial.Pose, error) {
	if src == nil {
		return nil, errors.New("source frame is nil")
	}

	// check if frames are in system. It is allowed for the src frame to be an anonymous frame not in the system, so
	// long as its parent IS in the system.
	if parent != nil && !sfs.frameExists(parent.Name()) {
		return nil, fmt.Errorf("source frame parent %s not found in FrameSystem", parent.Name())
	}

	// If parent is nil, that means srcFrame is the world frame, which has no parent.
	var err error
	fromParentTransform := spatial.NewZeroPose()
	if parent != nil {
		// get source parent to world transform
		fromParentTransform, err = sfs.composeTransforms(parent, inputMap) // returns source to world transform
		if err != nil && fromParentTransform == nil {
			return nil, err
		}
	}
	return fromParentTransform, err
}

func (sfs *simpleFrameSystem) getTargetParentTransform(inputMap map[string][]Input, target Frame) (spatial.Pose, error) {
	if target == nil {
		return nil, errors.New("target frame is nil")
	}
	if !sfs.frameExists(target.Name()) {
		return nil, fmt.Errorf("target frame %s not found in FrameSystem", target.Name())
	}

	// get world to target transform
	toTargetTransform, err := sfs.composeTransforms(target, inputMap) // returns target to world transform
	if err != nil && toTargetTransform == nil {
		return nil, err
	}
	return spatial.Invert(toTargetTransform), err
}

// Returns the relative pose between two frames.
func (sfs *simpleFrameSystem) transformFromParent(inputMap map[string][]Input, src, srcParent, dst Frame) (spatial.Pose, error) {
	// catch all errors together to for allow hypothetical calculations that result in errors
	var errAll error
	toTarget, err := sfs.getTargetParentTransform(inputMap, dst)
	multierr.AppendInto(&errAll, err)
	fromParent, err := sfs.getSrcParentTransform(inputMap, src, srcParent)
	multierr.AppendInto(&errAll, err)
	pose, err := poseFromPositions(src, inputMap)
	multierr.AppendInto(&errAll, err)
	if errAll != nil && (toTarget == nil || fromParent == nil || pose == nil) {
		return nil, errAll
	}

	// transform from source to world, world to target
	return spatial.Compose(spatial.Compose(toTarget, fromParent), pose), errAll
}

// Returns the relative pose between two frames.
func (sfs *simpleFrameSystem) volumesFromParent(inputMap map[string][]Input, src, srcParent, dst Frame) (map[string]spatial.Volume, error) {
	toTarget, err := sfs.getTargetParentTransform(inputMap, dst)
	if toTarget == nil && err != nil {
		return nil, err
	}
	fromParent, err := sfs.getSrcParentTransform(inputMap, src, srcParent)
	if fromParent == nil && err != nil {
		return nil, err
	}

	// transform from source to world, world to target
	vols, err := volumesFromPositions(src, inputMap)
	if err != nil && vols == nil {
		return nil, err
	}
	for _, vol := range vols {
		vol.Transform(spatial.Compose(toTarget, fromParent))
	}
	return vols, err
}

// compose the quaternions from the input frame to the world referenceframe.
func (sfs *simpleFrameSystem) composeTransforms(frame Frame, inputMap map[string][]Input) (spatial.Pose, error) {
	q := spatial.NewZeroPose() // empty initial dualquat
	var errAll error
	for sfs.parents[frame] != nil { // stop once you reach world node
		// Transform() gives FROM q TO parent. Add new transforms to the left.
		pose, err := poseFromPositions(frame, inputMap)
		if err != nil && pose == nil {
			return nil, err
		}
		multierr.AppendInto(&errAll, err)
		q = spatial.Compose(pose, q)
		frame = sfs.parents[frame]
	}
	return q, errAll
}

func getFrameInputs(frame Frame, inputMap map[string][]Input) ([]Input, error) {
	var input []Input
	// Get frame inputs if necessary
	if len(frame.DoF()) > 0 {
		if _, ok := inputMap[frame.Name()]; !ok {
			return nil, fmt.Errorf("no positions provided for frame with name %s", frame.Name())
		}
		input = inputMap[frame.Name()]
	}
	return input, nil
}

func poseFromPositions(frame Frame, positions map[string][]Input) (spatial.Pose, error) {
	inputs, err := getFrameInputs(frame, positions)
	if err != nil {
		return nil, err
	}
	return frame.Transform(inputs)
}

func volumesFromPositions(frame Frame, positions map[string][]Input) (map[string]spatial.Volume, error) {
	inputs, err := getFrameInputs(frame, positions)
	if err != nil {
		return nil, err
	}
	return frame.Volumes(inputs)
}
