package referenceframe

import (
	"errors"
	"fmt"

	spatial "go.viam.com/core/spatialmath"

	"github.com/golang/geo/r3"
)

// dynamicFrameSystem implements both FrameSystem. It is a simple tree graph that only takes in staticFrames.
// The tree graph can grow, but the transforms between nodes cannot be changed once created.
type dynamicFrameSystem struct {
	name   string
	world  Frame // separate from the map of frames so it can be detached easily
	frames map[string]Frame
	positions map[string][]Input
}

// NewEmptyDynamicFrameSystem creates a graph of Frames that have fixed or settable positions relative to each other.
func NewEmptyDynamicFrameSystem(name string) *dynamicFrameSystem {
	worldFrame := NewStaticFrame("world", nil, nil)
	frames := map[string]Frame{}
	positions := map[string][]Input{}
	return &dynamicFrameSystem{name, worldFrame, frames, positions}
}

// World returns the base world frame
func (dfs *dynamicFrameSystem) World() Frame {
	return dfs.world
}

// frameExists is a helper function to see if a frame with a given name already exists in the system.
func (dfs *dynamicFrameSystem) frameExists(name string) bool {
	if name == "world" {
		return true
	}
	if _, ok := dfs.frames[name]; ok {
		return true
	}
	return false
}

// GetFrame returns the frame given the name of the frame. Returns nil if the frame is not found.
func (dfs *dynamicFrameSystem) GetFrame(name string) Frame {
	if !dfs.frameExists(name) {
		return nil
	}
	if name == "world" {
		return dfs.world
	}
	return dfs.frames[name]
}

// SetFrameFromPose adds an input staticFrame to the system given a parent and a pose.
// It can only be added if the parent of the input frame already exists in the system,
// and there is no frame with the input's name already.
func (dfs *dynamicFrameSystem) SetFrameFromPose(name string, parent Frame, pose Pose) error {
	if parent == nil {
		return errors.New("parent frame is nil")
	}
	// check to see if parent is in system
	if !dfs.frameExists(parent.Name()) {
		return fmt.Errorf("parent frame with name %s not in FrameSystem", parent.Name())
	}
	// check if frame with that name is already in system
	if dfs.frameExists(name) {
		return fmt.Errorf("frame with name %s already in FrameSystem", name)
	}
	frame := NewStaticFrame(name, parent, pose)
	dfs.frames[frame.Name()] = frame
	dfs.positions[frame.Name()] = []Input{}
	
	return nil
}

// SetFrameFromPoint creates a new Frame from a 3D point. It will be given the same orientation as the parent of the frame.
func (dfs *dynamicFrameSystem) SetFrameFromPoint(name string, parent Frame, point r3.Vector) error {
	if parent == nil {
		return errors.New("parent frame is nil")
	}
	// check if frame with that name is already in system
	if dfs.frameExists(name) {
		return fmt.Errorf("frame with name %s already exists in FrameSystem", name)
	}
	// check to see if parent is in system
	if !dfs.frameExists(parent.Name()) {
		return fmt.Errorf("parent frame with name %s not in FrameSystem", parent.Name())
	}
	pose := NewPoseFromPoint(point)
	staticFrame := NewStaticFrame(name, parent, pose)
	dfs.frames[name] = staticFrame
	dfs.positions[name] = []Input{}
	return nil
}

// SetFrame sets an already defined Frame into the system. Will only accept it if the underlyic type is staticFrame
func (dfs *dynamicFrameSystem) SetFrame(frame Frame) error {
	if frame.Parent() == nil {
		return errors.New("parent frame is nil")
	}
	// check if frame with that name is already in system
	if dfs.frameExists(frame.Name()) {
		return fmt.Errorf("frame with name %s already exists in FrameSystem", frame.Name())
	}
	// check to see if parent is in system
	if !dfs.frameExists(frame.Parent().Name()) {
		return fmt.Errorf("parent frame with name %s not in FrameSystem", frame.Parent().Name())
	}
	dfs.frames[frame.Name()] = frame
	dfs.positions[frame.Name()] = make([]Input, frame.Dof())
	return nil
}

// compose the quaternions from the input frame to the world frame
func (dfs *dynamicFrameSystem) composeTransforms(frame Frame) *spatial.DualQuaternion {
	q := spatial.NewDualQuaternion() // empty initial dualquat
	for frame.Parent() != nil {      // stop once you reach world node
		// Transform() gives FROM q TO parent. Add new transforms to the left.
		q = spatial.Compose(frame.Transform(dfs.positions[frame.Name()]), q)
		frame = frame.Parent()
	}
	return q
}

// TransformPoint takes in a point with respect to a source Frame, and outputs the point coordinates with respect to the target Frame.
func (dfs *dynamicFrameSystem) TransformPoint(point r3.Vector, srcFrame, endFrame Frame) (r3.Vector, error) {
	if srcFrame == nil {
		return r3.Vector{}, errors.New("source frame is nil")
	}
	if endFrame == nil {
		return r3.Vector{}, errors.New("target frame is nil")
	}
	// check if frames are in system
	if !dfs.frameExists(srcFrame.Name()) {
		return r3.Vector{}, fmt.Errorf("source frame %s not found in FrameSystem", srcFrame.Name())
	}
	if !dfs.frameExists(endFrame.Name()) {
		return r3.Vector{}, fmt.Errorf("target frame %s not found in FrameSystem", endFrame.Name())
	}
	// get source to world transform
	fromSrcTransform := dfs.composeTransforms(srcFrame) // returns source to world transform
	// get world to target transform
	toTargetTransform := dfs.composeTransforms(endFrame).Invert()  // returns target to world transform
	// transform from source to world, world to target
	fullTransform := spatial.Compose(toTargetTransform, fromSrcTransform)
	// apply to the point position
	transformedPoint := TransformPoint(fullTransform, point)
	return transformedPoint, nil
}

// Name returns the name of the dynamicFrameSystem
func (dfs *dynamicFrameSystem) Name() string {
	return dfs.name
}

// SetPosition sets the positions for a specific frame
// TODO: This is probably wrong, we probably want something stateless. This is likely not where this state should be held
// Perhaps we should pass in positions{} each time
func (dfs *dynamicFrameSystem) SetPosition(name string, pos []Input) error {
	// check to see if frame is in system
	if !dfs.frameExists(name) {
		return fmt.Errorf("frame with name %s not in FrameSystem", name)
	}
	if len(dfs.positions[name]) != len(pos){
		return fmt.Errorf("passed in incorrect number of positions. Wanted %d, got %d", len(dfs.positions[name]), len(pos))
	}
	
	dfs.positions[name] = pos
	return nil
}
