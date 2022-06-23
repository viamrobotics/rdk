package referenceframe

import (
	"errors"
	"fmt"

	"go.uber.org/multierr"

	spatial "go.viam.com/rdk/spatialmath"
)

// World is the string "world", but made into an exported constant.
const World = "world"

// FrameSystem represents a tree of frames connected to each other, allowing for transformations between any two frames.
type FrameSystem interface {
	// Name returns the name of this FrameSystem
	Name() string

	// World returns the frame corresponding to the root of the FrameSystem, from which other frames are defined with respect to
	World() Frame

	// FrameNames returns the names of all of the frames that exist in the FrameSystem
	FrameNames() []string

	// GetFrame returns the Frame in the FrameSystem corresponding to
	GetFrame(name string) Frame

	// AddFrame inserts a given Frame into the FrameSystem as a child of the parent Frame
	AddFrame(frame, parent Frame) error

	// RemoveFrame removes the given Frame from the FrameSystem
	RemoveFrame(frame Frame)

	// TracebackFrame traces the parentage of the given frame up to the world, and returns the full list of frames in between.
	// The list will include both the query frame and the world referenceframe
	TracebackFrame(frame Frame) ([]Frame, error)

	// Parent returns the parent Frame for the given Frame in the FrameSystem
	Parent(frame Frame) (Frame, error)

	// Transform takes in a Transformable object and destination frame, and returns the pose from the first to the second. Positions
	// is a map of inputs for any frames with non-zero DOF, with slices of inputs keyed to the frame name.
	Transform(positions map[string][]Input, object Transformable, dst string) (Transformable, error)

	// DivideFrameSystem will take a frame system and a frame in that system, and return two frame systems- one being rooted
	// at the given frame and containing all descendents of it, the other with the original world with the frame and its
	// descendents removed.
	DivideFrameSystem(newRoot Frame) (FrameSystem, error)

	// MergeFrameSystem combines two frame systems together, placing the world of systemToMerge at the attachTo frame in the frame system
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

// Transform takes in a Transformable object and destination frame, and returns the pose from the first to the second. Positions
// is a map of inputs for any frames with non-zero DOF, with slices of inputs keyed to the frame name.
func (sfs *simpleFrameSystem) Transform(positions map[string][]Input, object Transformable, dst string) (Transformable, error) {
	src := object.FrameName()
	
	if src == dst {
		return object, nil
	}
	
	if !sfs.frameExists(src) {
		return nil, fmt.Errorf("source frame %s not found in FrameSystem", src)
	}
	srcFrame := sfs.GetFrame(src)
	if !sfs.frameExists(dst) {
		return nil, fmt.Errorf("destination frame %s not found in FrameSystem", dst)
	}

	var tfParent *PoseInFrame
	var err error
	if _, ok := object.(*GeometriesInFrame); ok {
		// We don't want to apply the final transformation when that is taken care of by the geometries
		// This has to do with the way we decided to tie geometries to frames for ease of defining them in the model_json file
		// A frame is assigned a pose and a geometry and the two are not coupled together. This way you do can define everything relative
		// to the parent frame. So geometries are tied to the frame they are assigned to but we do not want to actually transform them
		// along the final transformation.
		tfParent, err = sfs.transformFromParent(positions, sfs.parents[srcFrame], sfs.GetFrame(dst))
	} else {
		tfParent, err = sfs.transformFromParent(positions, srcFrame, sfs.GetFrame(dst))
	}
	if err != nil {
		return nil, err
	}
	return object.Transform(tfParent), nil
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

func (sfs *simpleFrameSystem) getFrameToWorldTransform(inputMap map[string][]Input, src Frame) (spatial.Pose, error) {
	if !sfs.frameExists(src.Name()) {
		return nil, fmt.Errorf("source frame %s not found in FrameSystem", src.Name())
	}

	// If src is nil it is interpreted as the world frame
	var err error
	srcToWorld := spatial.NewZeroPose()
	if src != nil {
		srcToWorld, err = sfs.composeTransforms(src, inputMap)
		if err != nil && srcToWorld == nil {
			return nil, err
		}
	}
	return srcToWorld, err
}

// Returns the relative pose between the parent and the destination frame.
func (sfs *simpleFrameSystem) transformFromParent(inputMap map[string][]Input, src, dst Frame) (*PoseInFrame, error) {
	// catch all errors together to allow for hypothetical calculations that result in errors
	var errAll error
	dstToWorld, err := sfs.getFrameToWorldTransform(inputMap, dst)
	multierr.AppendInto(&errAll, err)
	srcToWorld, err := sfs.getFrameToWorldTransform(inputMap, src)
	multierr.AppendInto(&errAll, err)
	if errAll != nil && (dstToWorld == nil || srcToWorld == nil) {
		return nil, errAll
	}

	// transform from source to world, world to target parent
	return &PoseInFrame{dst.Name(), spatial.Compose(spatial.PoseInverse(dstToWorld), srcToWorld)}, nil
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

func poseFromPositions(frame Frame, positions map[string][]Input) (spatial.Pose, error) {
	inputs, err := GetFrameInputs(frame, positions)
	if err != nil {
		return nil, err
	}
	return frame.Transform(inputs)
}
