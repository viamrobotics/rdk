package referenceframe

import (
	"fmt"

	pb "go.viam.com/core/proto/api/v1"
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
	TransformPose(pose Pose, endFrame Frame) (Pose, error) // Pose has its origin frame stored in it
}

// staticFrameSystem implements both FrameSystem and Frame. It is a simple graph that only takes in staticFrames
// TODO : I'm not sure how to implement the Transform method for a staticFrameSystem (especially if it has more than one end-effector)
// any ideas?
type staticFrameSystem struct {
	name   string
	world  Frame
	frames map[string]Frame
}

func NewEmptyStaticFrameSystem(name string) FrameSystem {
	worldFrame := Frame(NewStaticFrame("world", nil))
	frames := map[string]Frame{}
	return &staticFrameSystem{name, worldFrame, frames}
}

func (sfs *staticFrameSystem) World() Frame {
	return sfs.world
}

func (sfs *staticFrameSystem) frameExists(name string) bool {
	if name == "world" {
		return true
	}
	if frame, ok := sfs.frames[name]; ok {
		return true
	}
	return false
}

func (sfs *staticFrameSystem) GetFrame(name string) (Frame, error) {
	if !sfs.frameExists(name) {
		return nil, fmt.Errorf("no frame with name %s in FrameSystem", name)
	}
	if name == "world" {
		return sfs.world
	}
	return sfs.frames[name]
}

func (sfs *staticFrameSystem) SetFrame(frame Frame) error {
	// check if frame a staticFrame
	if _, ok := frame.(*staticFrame); !ok {
		return fmt.Errors("only *staticFrame types allowed for this FrameSystem, input frame of type %T", frame)
	}
	// check if frame with that name is already in system
	if sfs.frameExists(frame.Name()) {
		return fmt.Errors("frame with name %s already in FrameSystem", frame.Name())
	}
	sfs.frames[frame.Name()] = frame
	return nil
}

func (sfs *staticFrameSystem) SetFrameFromPoint(name string, parent Frame, point r3.Vector) error {
	// check if frame with that name is already in system
	if sfs.frameExists(name) {
		return fmt.Errors("frame with name %s already exists in FrameSystem", name)
	}
	// check to see if parent is in system
	if !sfs.frameExists(parent.Name()) {
		return fmt.Errors("parent frame with name %s not in FrameSystem", parent.Name())
	}
	pose := NewPoseFromPoint(parent, point)
	staticFrame := NewStaticFrame(name, pose)
	sfs.frames[name] = staticFrame
	return nil
}

func (sfs *staticFrameSystem) TransformPose(pose Pose, endFrame Frame) (Pose, error) {
	// check if pose frame is in system
	// check if end frame is in system
	return nil, nil
}

// Methods to fulfill the Frame interface too

func (sfs *staticFrameSystem) Name() string {
	return sfs.name
}

func (sfs *staticFrameSystem) Parent() Frame {
	return nil
}

// Not sure how to implement Transform for an entire system
func (sfs *staticFrameSystem) Transform([]Input) *spatial.DualQuaternion {
	return nil
}

// DoF is always 0 for a static Frame
func (sfs *staticFrameSystem) DoF() int {
	return 0
}
