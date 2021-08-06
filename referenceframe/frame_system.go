package referenceframe

import (
	"errors"
	"fmt"

	spatial "go.viam.com/core/spatialmath"
)

// FrameSystem represents a tree of frames connected to each other, allowing for transformations between any two frames.
type FrameSystem interface {
	World() Frame // return the base world frame
	GetFrame(name string) Frame
	SetFrame(frame Frame) error
	TransformPoint(positions map[string][]Input, srcFrame, endFrame Frame) (spatial.Pose, error)
}

// staticFrameSystem implements FrameSystem. It is a simple tree graph that only takes in staticFrames.
// The tree graph can grow, but the transforms between nodes cannot be changed once created.
type simpleFrameSystem struct {
	name   string
	world  Frame // separate from the map of frames so it can be detached easily
	frames map[string]Frame
}

// NewEmptySimpleFrameSystem creates a graph of Frames that have
func NewEmptySimpleFrameSystem(name string) *simpleFrameSystem {
	worldFrame := NewStaticFrame("world", nil, nil)
	frames := map[string]Frame{}
	return &simpleFrameSystem{name, worldFrame, frames}
}

// World returns the base world frame
func (sfs *simpleFrameSystem) World() Frame {
	return sfs.world
}

// frameExists is a helper function to see if a frame with a given name already exists in the system.
func (sfs *simpleFrameSystem) frameExists(name string) bool {
	if name == "world" {
		return true
	}
	if _, ok := sfs.frames[name]; ok {
		return true
	}
	return false
}

// GetFrame returns the frame given the name of the frame. Returns nil if the frame is not found.
func (sfs *simpleFrameSystem) GetFrame(name string) Frame {
	if !sfs.frameExists(name) {
		return nil
	}
	if name == "world" {
		return sfs.world
	}
	return sfs.frames[name]
}

// SetFrameFromPose adds an input staticFrame to the system given a parent and a pose.
// It can only be added if the parent of the input frame already exists in the system,
// and there is no frame with the input's name already.
func (sfs *simpleFrameSystem) SetFrameFromPose(name string, parent Frame, pose spatial.Pose) error {
	if parent == nil {
		return errors.New("parent frame is nil")
	}
	// check to see if parent is in system
	if !sfs.frameExists(parent.Name()) {
		return fmt.Errorf("parent frame with name %s not in FrameSystem", parent.Name())
	}
	// check if frame with that name is already in system
	if sfs.frameExists(name) {
		return fmt.Errorf("frame with name %s already in FrameSystem", name)
	}
	frame := NewStaticFrame(name, parent, pose)
	sfs.frames[frame.Name()] = frame
	return nil
}

// SetFrame sets an already defined Frame into the system. Will only accept it if the underlyic type is staticFrame
func (sfs *simpleFrameSystem) SetFrame(frame Frame) error {
	if frame.Parent() == nil {
		return errors.New("parent frame is nil")
	}
	// check if frame with that name is already in system
	if sfs.frameExists(frame.Name()) {
		return fmt.Errorf("frame with name %s already exists in FrameSystem", frame.Name())
	}
	// check to see if parent is in system
	if !sfs.frameExists(frame.Parent().Name()) {
		return fmt.Errorf("parent frame with name %s not in FrameSystem", frame.Parent().Name())
	}
	sfs.frames[frame.Name()] = frame
	return nil
}

// TransformPoint takes in a point with respect to a source Frame, and outputs the point coordinates with respect to the target Frame.
func (sfs *simpleFrameSystem) TransformPoint(positions map[string][]Input, srcFrame, endFrame Frame) (spatial.Pose, error) {
	if srcFrame == nil {
		return nil, errors.New("source frame is nil")
	}
	if endFrame == nil {
		return nil, errors.New("target frame is nil")
	}
	// check if frames are in system. It is allowed for the src frame to be an anonymous frame not in the system, so
	// long as its parent IS in the system.
	if !sfs.frameExists(srcFrame.Name()) {
		if !sfs.frameExists(srcFrame.Parent().Name()) {
			return nil, fmt.Errorf("neither source frame %s nor its parent found in FrameSystem", srcFrame.Name())
		}
	}
	if !sfs.frameExists(endFrame.Name()) {
		return nil, fmt.Errorf("target frame %s not found in FrameSystem", endFrame.Name())
	}

	// get source parent to world transform
	fromSrcTransform, err := composeTransforms(srcFrame, positions) // returns source to world transform
	if err != nil {
		return &basicPose{}, err
	}
	// get world to target transform
	toTargetTransform, err := composeTransforms(endFrame, positions) // returns target to world transform
	if err != nil {
		return &basicPose{}, err
	}
	toTargetTransform = toTargetTransform.Invert()
	// transform from source to world, world to target
	fullTransform := spatial.Compose(toTargetTransform, fromSrcTransform)
	return fullTransform, nil
}

// Name returns the name of the simpleFrameSystem
func (sfs *simpleFrameSystem) Name() string {
	return sfs.name
}

// StartPositions returns a zeroed input map ensuring all frames have inputs
func (sfs *simpleFrameSystem) StartPositions() map[string][]Input {
	positions := make(map[string][]Input)
	for _, frame := range sfs.frames {
		positions[frame.Name()] = make([]Input, frame.Dof())
	}
	return positions
}

// compose the quaternions from the input frame to the world frame
func composeTransforms(frame Frame, positions map[string][]Input) (spatial.Pose, error) {
	q := spatial.NewEmptyPose() // empty initial dualquat
	for frame.Parent() != nil { // stop once you reach world node
		// Transform() gives FROM q TO parent. Add new transforms to the left.
		// Get frame inputs if necessary
		var input []Input
		if frame.Dof() > 0 {
			if _, ok := positions[frame.Name()]; !ok {
				return nil, fmt.Errorf("no positions provided for frame with name %s", frame.Name())
			}
			input = positions[frame.Name()]
		}
		q = spatial.Compose(frame.Transform(input), q)
		frame = frame.Parent()
	}
	return q, nil
}
