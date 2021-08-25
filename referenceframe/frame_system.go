package referenceframe

import (
	"errors"
	"fmt"

	"github.com/golang/geo/r3"
	spatial "go.viam.com/core/spatialmath"
)

// FrameSystem represents a tree of frames connected to each other, allowing for transformations between any two frames.
type FrameSystem interface {
	Name() string // return the name of this frame system
	World() Frame // return the base world frame
	GetFrame(name string) Frame
	AddFrame(frame, parent Frame) error
	TransformFrame(positions map[string][]Input, srcFrame, endFrame Frame) (spatial.Pose, error)
	TransformPoint(positions map[string][]Input, point r3.Vector, srcFrame, endFrame Frame) (r3.Vector, error)
	TransformPose(positions map[string][]Input, pose spatial.Pose, srcFrame, endFrame Frame) (spatial.Pose, error)
	Frames() map[string]Frame
}

// staticFrameSystem implements FrameSystem. It is a simple tree graph that only takes in staticFrames.
// The tree graph can grow, but the transforms between nodes cannot be changed once created.
type simpleFrameSystem struct {
	name   string
	world  Frame // separate from the map of frames so it can be detached easily
	frames map[string]Frame
	parents map[Frame]Frame
}

// NewEmptySimpleFrameSystem creates a graph of Frames that have
func NewEmptySimpleFrameSystem(name string) *simpleFrameSystem {
	worldFrame := NewStaticFrame("world", nil)
	frames := map[string]Frame{}
	parents := map[Frame]Frame{}
	return &simpleFrameSystem{name, worldFrame, frames, parents}
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

// Frames returns a slice containing all frames
func (sfs *simpleFrameSystem) Frames() map[string]Frame {
	return sfs.frames
}

func (sfs *simpleFrameSystem) checkName(name string, parent Frame) error{
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
	return nil
}

// AddFrameFromPose adds an input staticFrame to the system given a parent and a pose.
// It can only be added if the parent of the input frame already exists in the system,
// and there is no frame with the input's name already.
func (sfs *simpleFrameSystem) AddFrameFromPose(name string, parent Frame, pose spatial.Pose) error {
	frame := NewStaticFrame(name, pose)
	return sfs.AddFrame(frame, parent)
}

// AddFrame sets an already defined Frame into the system. Will only accept it if the underlyic type is staticFrame
func (sfs *simpleFrameSystem) AddFrame(frame, parent Frame) error {
	err := sfs.checkName(frame.Name(), parent)
	if err != nil{
		return err
	}
	sfs.frames[frame.Name()] = frame
	sfs.parents[frame] = parent
	return nil
}

// TransformFrame returns the relative Pose between two frames
func (sfs *simpleFrameSystem) transformFrameFromParent(positions map[string][]Input, srcFrame, srcParent, endFrame Frame) (spatial.Pose, error) {
	var err error
	if srcFrame == nil {
		return nil, errors.New("source frame is nil")
	}
	if endFrame == nil {
		return nil, errors.New("target frame is nil")
	}
	// check if frames are in system. It is allowed for the src frame to be an anonymous frame not in the system, so
	// long as its parent IS in the system.
	if srcParent != nil && !sfs.frameExists(srcParent.Name()) {
		return nil, fmt.Errorf("source frame parent %s not found in FrameSystem", srcParent.Name())
	}
	if !sfs.frameExists(endFrame.Name()) {
		return nil, fmt.Errorf("target frame %s not found in FrameSystem", endFrame.Name())
	}
	// If parent is nil, that means srcFrame is the world frame, which has no parent.
	fromParentTransform := spatial.NewEmptyPose()
	if srcParent != nil{
		// get source parent to world transform
		fromParentTransform, err = sfs.composeTransforms(srcParent, positions) // returns source to world transform
		if err != nil {
			return nil, err
		}
	}
	// get world to target transform
	toTargetTransform, err := sfs.composeTransforms(endFrame, positions) // returns target to world transform
	if err != nil {
		return nil, err
	}
	toTargetTransform = toTargetTransform.Invert()
	// transform from source to world, world to target
	srcTransform, err := poseFromPositions(srcFrame, positions)
	if err != nil{
		return nil, err
	}
	fullTransform := spatial.Compose(spatial.Compose(toTargetTransform, fromParentTransform), srcTransform)
	return fullTransform, nil
}

func (sfs *simpleFrameSystem) TransformFrame(positions map[string][]Input, srcFrame, endFrame Frame) (spatial.Pose, error) {
	if !sfs.frameExists(srcFrame.Name()) {
		return nil, fmt.Errorf("source frame %s not found in FrameSystem", srcFrame.Name())
	}
	return sfs.transformFrameFromParent(positions, srcFrame, sfs.parents[srcFrame], endFrame)
}

// TransformPoint takes in a point with respect to a source Frame, and outputs the point coordinates with respect to the target Frame.
func (sfs *simpleFrameSystem) TransformPoint(positions map[string][]Input, point r3.Vector, srcFrame, endFrame Frame) (r3.Vector, error) {
	// Turn point into an anonymous Frame
	pointFrame := FrameFromPoint("", point)
	// do Transform
	fullTransform, err := sfs.transformFrameFromParent(positions, pointFrame, srcFrame, endFrame)
	if err != nil {
		return r3.Vector{}, err
	}
	return fullTransform.Point(), nil
}

// TransformPose takes in a pose with respect to a source Frame, and outputs the pose with respect to the target Frame.
func (sfs *simpleFrameSystem) TransformPose(positions map[string][]Input, pose spatial.Pose, srcFrame, endFrame Frame) (spatial.Pose, error) {
	poseFrame := NewStaticFrame("", pose)
	return sfs.transformFrameFromParent(positions, poseFrame, srcFrame, endFrame)
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
func (sfs *simpleFrameSystem) composeTransforms(frame Frame, positions map[string][]Input) (spatial.Pose, error) {
	q := spatial.NewEmptyPose() // empty initial dualquat
	for sfs.parents[frame] != nil { // stop once you reach world node
		// Transform() gives FROM q TO parent. Add new transforms to the left.
		pose, err := poseFromPositions(frame, positions)
		if err != nil{
			return nil, err
		}
		q = spatial.Compose(pose, q)
		frame = sfs.parents[frame]
	}
	return q, nil
}

func poseFromPositions(frame Frame, positions map[string][]Input) (spatial.Pose, error) {
	// Get frame inputs if necessary
	var input []Input
	if frame.Dof() > 0 {
		if _, ok := positions[frame.Name()]; !ok {
			return nil, fmt.Errorf("no positions provided for frame with name %s", frame.Name())
		}
		input = positions[frame.Name()]
	}
	return frame.Transform(input), nil
}

