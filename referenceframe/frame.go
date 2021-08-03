// Package referenceframe defines the api and does the math of translating between reference frames
// Useful for if you have a camera, connected to a gripper, connected to an arm,
// and need to translate the camera reference frame to the arm reference frame,
// if you've found something in the camera, and want to move the gripper + arm to get it.
package referenceframe

import (
	"fmt"

	pb "go.viam.com/core/proto/api/v1"
	spatial "go.viam.com/core/spatialmath"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/dualquat"
)

// Input wraps the input to a mutable frame, e.g. a joint angle or a gantry position.
// TODO: Determine what more this needs, or eschew in favor of raw float64s if nothing needed.
type Input struct {
	Value float64
}

// Frame represents a single reference frame, e.g. an arm, a joint, etc.
// Its Transform places the Frame's pose in the Frame of its parent.
type Frame interface {
	Name() string
	Parent() Frame
	Transform([]Input) *spatial.DualQuaternion
	Dof() int
}

// a static Frame is a simple Pose that encodes a fixed translation and orientation relative to a parent Frame
type staticFrame struct {
	name string
	pose Pose
}

func NewStaticFrame(name string, pose Pose) Frame {
	&staticFrame{name, pose}
}

func (sf *staticFrame) Name() string {
	return sf.name
}

func (sf *staticFrame) Parent() Frame {
	return sf.pose.Frame()
}

func (sf *staticFrame) Transform(inp []Input) *spatial.DualQuaternion {
	if len(inp) != sf.DoF() {
		return nil
	}
	return sf.pose.DualQuat()
}

func (sf *staticFrame) DoF() int {
	return 0
}
