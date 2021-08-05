package referenceframe

import (
	"errors"
	"fmt"

	spatial "go.viam.com/core/spatialmath"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

// FrameSystem represents a tree of frames connected to each other, allowing for transformations between any two frames.
type FrameSystem interface {
	World() Frame // return the base world frame
	GetFrame(name string) Frame
	SetFrameFromPose(name string, parent Frame, pose Pose) error
	SetFrameFromPoint(name string, parent Frame, point r3.Vector) error
	TransformPose(pose Pose, srcFrame, endFrame Frame) (Pose, error)
}

// staticFrameSystem implements both FrameSystem and Frame. It is a simple tree graph that only takes in staticFrames.
// The tree graph can grow, but the transforms between nodes cannot be changed once created.
type staticFrameSystem struct {
	name   string
	world  Frame // separate from the map of frames so it can be detached easily
	frames map[string]Frame
}

// NewEmptyStaticFrameSystem creates a static graph of Frames that have fixed positions relative to each other. Only staticFrames can
// be added through the SetFrame methods.
func NewEmptyStaticFrameSystem(name string) *staticFrameSystem {
	worldFrame := NewStaticFrame("world", nil, nil)
	frames := map[string]Frame{}
	return &staticFrameSystem{name, worldFrame, frames}
}

// World returns the base world frame
func (sfs *staticFrameSystem) World() Frame {
	return sfs.world
}

// frameExists is a helper function to see if a frame with a given name already exists in the system.
func (sfs *staticFrameSystem) frameExists(name string) bool {
	if name == "world" {
		return true
	}
	if _, ok := sfs.frames[name]; ok {
		return true
	}
	return false
}

// GetFrame returns the frame given the name of the frame. Returns nil if the frame is not found.
func (sfs *staticFrameSystem) GetFrame(name string) Frame {
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
func (sfs *staticFrameSystem) SetFrameFromPose(name string, parent Frame, pose Pose) error {
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

// SetFrameFromPoint creates a new Frame from a 3D point. It will be given the same orientation as the parent of the frame.
func (sfs *staticFrameSystem) SetFrameFromPoint(name string, parent Frame, point r3.Vector) error {
	if parent == nil {
		return errors.New("parent frame is nil")
	}
	// check if frame with that name is already in system
	if sfs.frameExists(name) {
		return fmt.Errorf("frame with name %s already exists in FrameSystem", name)
	}
	// check to see if parent is in system
	if !sfs.frameExists(parent.Name()) {
		return fmt.Errorf("parent frame with name %s not in FrameSystem", parent.Name())
	}
	pose := NewPoseFromPoint(point)
	staticFrame := NewStaticFrame(name, parent, pose)
	sfs.frames[name] = staticFrame
	return nil
}

// compose the quaternions from the input frame to the world frame
func (sfs *staticFrameSystem) composeTransforms(frame Frame) *spatial.DualQuaternion {
	zeroInput := []Input{}           // staticFrameSystem always has empty input
	q := spatial.NewDualQuaternion() // empty initial dualquat
	for frame.Parent() != nil {      // stop once you reach world node
		// Transform() gives FROM q TO parent. Compose transforms to the left.
		q.Quat = dualquat.Mul(frame.Transform(zeroInput).Quat, q.Quat)
		// Normalize
		magnitude := quat.Mul(q.Quat.Real, quat.Conj(q.Quat.Real)).Real
		q.Quat = dualquat.Scale(1./magnitude, q.Quat)
		// Move to next frame
		frame = frame.Parent()
	}
	return q
}

// TransformPose takes in a pose with respect to a source Frame, and outputs the pose with respect to the target Frame.
func (sfs *staticFrameSystem) TransformPose(pose Pose, srcFrame, endFrame Frame) (Pose, error) {
	if srcFrame == nil {
		return nil, errors.New("source frame is nil")
	}
	if endFrame == nil {
		return nil, errors.New("target frame is nil")
	}
	// check if frames are in system
	if !sfs.frameExists(srcFrame.Name()) {
		return nil, fmt.Errorf("source frame %s not found in FrameSystem", srcFrame.Name())
	}
	if !sfs.frameExists(endFrame.Name()) {
		return nil, fmt.Errorf("target frame %s not found in FrameSystem", endFrame.Name())
	}
	// get source to world transform
	fromSrcTransform := sfs.composeTransforms(srcFrame) // returns source to world transform
	// get world to target transform
	toTargetTransform := sfs.composeTransforms(endFrame)               // returns target to world transform
	toTargetTransform.Quat = dualquat.ConjQuat(toTargetTransform.Quat) // ConjQuat for the inverse transform
	// transform from source to world, world to target
	fullTransform := dualquat.Mul(toTargetTransform.Quat, fromSrcTransform.Quat)
	// apply to the position
	sourceQuat := pose.PointDualQuat()
	transformedQuat := dualquat.Mul(dualquat.Mul(fullTransform, sourceQuat), dualquat.Conj(fullTransform))
	transformedPose, err := NewPoseFromPointDualQuat(transformedQuat)
	if err != nil {
		return nil, err
	}
	return transformedPose, nil
}

// Methods to fulfill the Frame interface too

// Name returns the name of the staticFrameSystem
func (sfs *staticFrameSystem) Name() string {
	return sfs.name
}

// Parent returns the world frame, since that is the origin of the whole system
func (sfs *staticFrameSystem) Parent() Frame {
	return sfs.world
}

// TODO : I'm not sure how to implement the Transform method for a staticFrameSystem (especially if it has more than one end-effector)
// any ideas?
func (sfs *staticFrameSystem) Transform([]Input) *spatial.DualQuaternion {
	return nil
}

// DoF is always 0 for a staticFrameSystem since it is only allowed to hold staticFrames
func (sfs *staticFrameSystem) DoF() int {
	return 0
}
