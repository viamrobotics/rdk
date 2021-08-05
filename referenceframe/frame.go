// Package referenceframe defines the api and does the math of translating between reference frames
// Useful for if you have a camera, connected to a gripper, connected to an arm,
// and need to translate the camera reference frame to the arm reference frame,
// if you've found something in the camera, and want to move the gripper + arm to get it.
package referenceframe

import (
	spatial "go.viam.com/core/spatialmath"
	"gonum.org/v1/gonum/num/dualquat"
)

// Input wraps the input to a mutable frame, e.g. a joint angle or a gantry position.
// TODO: Determine what more this needs, or eschew in favor of raw float64s if nothing needed.
type Input struct {
	Value float64
}

// Frame represents a single reference frame, e.g. an arm, a joint, etc.
// Transform takes FROM current frame TO parent's frame!
type Frame interface {
	Name() string
	ParentName() string // currently needed for kinematics package, should be removed when changed
	Parent() Frame
	Transform([]Input) *spatial.DualQuaternion
	Dof() int
}

// a static Frame is a simple Pose that encodes a fixed translation and rotation relative to a parent Frame
type staticFrame struct {
	name      string
	parent    Frame
	transform dualquat.Number
}

func NewStaticFrame(name string, parent Frame, pose Pose) Frame {
	if pose == nil {
		pose = NewEmptyPose()
	}
	return &staticFrame{name, parent, pose.Transform()}
}

func (sf *staticFrame) Name() string {
	return sf.name
}

// This function should be removed when ParentName() is no longer necessary
func (sf *staticFrame) ParentName() string {
	if sf.Parent() == nil {
		return ""
	}
	return sf.Parent().Name()
}

func (sf *staticFrame) Parent() Frame {
	return sf.parent
}

// Transform application takes you FROM current frame TO Parent frame
func (sf *staticFrame) Transform(inp []Input) *spatial.DualQuaternion {
	if len(inp) != sf.Dof() {
		return nil
	}
	return &spatial.DualQuaternion{sf.transform}
}

func (sf *staticFrame) Dof() int {
	return 0
}
