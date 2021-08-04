package referenceframe

import (
	"errors"
	"fmt"

	spatial "go.viam.com/core/spatialmath"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/dualquat"
)

// FrameSystem represents a tree of frames connected to each other, allowing for transformations between frames.
type FrameSystem interface {
	World() Frame // return the base world frame
	GetFrame(name string) Frame
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
func (sfs *staticFrameSystem) GetFrame(name string) Frame {
	if !sfs.frameExists(name) {
		return nil
	}
	if name == "world" {
		return sfs.world
	}
	return sfs.frames[name]
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

// compose the quaternions from the world frame to target frame
func (sfs *staticFrameSystem) composeTransforms(frame Frame) *spatial.DualQuaternion {
	zeroInput := []Input{} // staticFrameSystem always has empty input
	transforms := []dualquat.Number{}
	for frame.Parent() != nil { // stop once you reach world node
		// Transform() gives FROM parent TO q.
		transforms = append(transforms, frame.Transform(zeroInput).Quat)
		frame = frame.Parent()
	}
	q := spatial.NewDualQuaternion()
	// start from world frame, last element in slice
	for i := len(transforms) - 1; i >= 0; i-- {
		//q.Quat = q.Transformation(transforms[i])
		q.Quat = dualquat.Mul(q.Quat, transforms[i])
	}
	return q
}

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
	// get world to source transform
	srcTransform := sfs.composeTransforms(srcFrame)
	// get world to target transform
	targetTransform := sfs.composeTransforms(endFrame)
	// transform from source to world, world to target
	srcTransform.Quat = dualquat.Conj(srcTransform.Quat)
	fullQuat := srcTransform.Transformation(targetTransform.Quat)
	// apply to the pose dual quaternion
	sourceQuat := pose.DualQuat().Quat
	transformedQuat := dualquat.Mul(dualquat.Mul(fullQuat, sourceQuat), dualquat.Conj(fullQuat))
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
