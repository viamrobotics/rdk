package referenceframe

import (
	"fmt"

	spatial "go.viam.com/core/spatialmath"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/dualquat"
)

// FrameSystem represents a tree of frames connected to each other, allowing for transformations between frames.
type FrameSystem interface {
	World() Frame // return the base world frame
	GetFrame(name string) (Frame, error)
	SetFrame(frame Frame) error
	SetFrameFromPoint(name string, parent Frame, point r3.Vector) error
	TransformPose(pose Pose, srcFrame, endFrame Frame) (Pose, error)
}

// staticFrameSystem implements both FrameSystem and Frame. It is a simple tree graph that only takes in staticFrames
type staticFrameSystem struct {
	name   string
	world  Frame // separate from the map of frames so it can detached easily
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

// GetFrame returns the frame given the name of the frame. Returns an error if the frame is not found.
func (sfs *staticFrameSystem) GetFrame(name string) (Frame, error) {
	if !sfs.frameExists(name) {
		return nil, fmt.Errorf("no frame with name %s in FrameSystem", name)
	}
	if name == "world" {
		return sfs.world, nil
	}
	return sfs.frames[name], nil
}

// SetFrame adds an input staticFrame to the system. It can only be added if the parent of the input frame already exists in the system,
// and there is no frame with the input's name already.
func (sfs *staticFrameSystem) SetFrame(frame Frame) error {
	// check if frame a staticFrame
	if _, ok := frame.(*staticFrame); !ok {
		return fmt.Errorf("only *staticFrame types allowed for this FrameSystem, input frame of type %T", frame)
	}
	// check if frame with that name is already in system
	if sfs.frameExists(frame.Name()) {
		return fmt.Errorf("frame with name %s already in FrameSystem", frame.Name())
	}
	sfs.frames[frame.Name()] = frame
	return nil
}

// SetFrameFromPoint creates a new Frame from a 3D point. It will be given the same orientation as the parent of the frame.
func (sfs *staticFrameSystem) SetFrameFromPoint(name string, parent Frame, point r3.Vector) error {
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

// compose the quaternions down to the world frame
func (sfs *staticFrameSystem) composeTransforms(frame Frame) *spatial.DualQuaternion {
	q := spatial.NewDualQuaternion()
	zeroInput := []Input{}      // staticFrameSystem always has empty input
	for frame.Parent() != nil { // stop once you reach world node
		// Transform() gives FROM parent TO q. Want FROM q TO parent ... use conjugate dualquaternion.
		q.Quat = q.Transformation(dualquat.Conj(frame.Transform(zeroInput).Quat))
		frame = frame.Parent()
	}
	return q
}

func (sfs *staticFrameSystem) TransformPose(pose Pose, srcFrame, endFrame Frame) (Pose, error) {
	// check if frames are in system
	if !sfs.frameExists(srcFrame.Name()) {
		return nil, fmt.Errorf("source frame %s not found in FrameSystem", srcFrame.Name())
	}
	if !sfs.frameExists(endFrame.Name()) {
		return nil, fmt.Errorf("target frame %s not found in FrameSystem", endFrame.Name())
	}
	// get source to world transform
	srcTransform := sfs.composeTransforms(srcFrame)
	// get target to world transform
	targetTransform := sfs.composeTransforms(endFrame)
	// transform from source to world to target
	fullQuat := srcTransform.Transformation(dualquat.Conj(targetTransform.Quat))
	transformedQuat := dualquat.Mul(dualquat.Mul(fullQuat, pose.DualQuat().Quat), dualquat.Conj(fullQuat))
	transformedPose := &spatial.DualQuaternion{transformedQuat}
	return &dualQuatPose{transformedPose}, nil
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
