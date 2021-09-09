// Package referenceframe defines the api and does the math of translating between reference frames
// Useful for if you have a camera, connected to a gripper, connected to an arm,
// and need to translate the camera reference frame to the arm reference frame,
// if you've found something in the camera, and want to move the gripper + arm to get it.
package referenceframe

import (
	"fmt"

	"go.viam.com/core/arm"
	pb "go.viam.com/core/proto/api/v1"
	spatial "go.viam.com/core/spatialmath"

	"github.com/golang/geo/r3"
)

// Input wraps the input to a mutable frame, e.g. a joint angle or a gantry position. Revolute inputs should be in
// radians. Prismatic inputs should be in mm.
// TODO: Determine what more this needs, or eschew in favor of raw float64s if nothing needed.
type Input struct {
	Value float64
}

// Limit describes a minimum and maximum limit for the DOF of the frame
type Limit struct {
	Min, Max float64
}

// FloatsToInputs wraps a slice of floats in Inputs
func FloatsToInputs(floats []float64) []Input {
	inputs := make([]Input, len(floats))
	for i, f := range floats {
		inputs[i] = Input{f}
	}
	return inputs
}

// JointPosToInputs will take a pb.JointPositions which has values in Degrees, convert to Radians and wrap in Inputs
func JointPosToInputs(jp *pb.JointPositions) []Input {
	floats := arm.JointPositionsToRadians(jp)
	return FloatsToInputs(floats)
}

// Frame represents a single reference frame, e.g. an arm, a joint, etc.
// Transform takes FROM current frame TO parent's frame!
type Frame interface {
	Name() string
	Transform([]Input) (spatial.Pose, error)
	Dof() []Limit
}

// a static Frame is a simple corrdinate system that encodes a fixed translation and rotation from the current Frame to the parent Frame
type staticFrame struct {
	name      string
	transform spatial.Pose
}

// NewStaticFrame creates a frame given a pose relative to its parent. The pose is fixed for all time.
// Pose is allowed to be nil.
func NewStaticFrame(name string, pose spatial.Pose) Frame {
	if pose == nil {
		pose = spatial.NewEmptyPose()
	}
	return &staticFrame{name, pose}
}

// FrameFromPoint creates a new Frame from a 3D point. It will be given the same orientation as the parent of the frame.
func FrameFromPoint(name string, point r3.Vector) Frame {
	pose := spatial.NewPoseFromPoint(point)
	frame := NewStaticFrame(name, pose)
	return frame
}

// Name is the name of the frame.
func (sf *staticFrame) Name() string {
	return sf.name
}

// Transform returns the pose associated with this static frame.
func (sf *staticFrame) Transform(inp []Input) (spatial.Pose, error) {
	if len(inp) != 0 {
		return nil, fmt.Errorf("given input length %q does not match frame dof 0", len(inp))
	}
	return sf.transform, nil
}

// Dof are the degrees of freedom of the transform. In the staticFrame, it is always 0.
func (sf *staticFrame) Dof() []Limit {
	return []Limit{}
}

// a prismatic Frame is a frame that can translate without rotation in any/all of the X, Y, and Z directions
type prismaticFrame struct {
	name   string
	axes   []bool
	limits []Limit
}

// NewPrismaticFrame creates a frame given a name and the axes in which to translate
func NewPrismaticFrame(name string, axes []bool, limits []Limit) (Frame, error) {
	pf := &prismaticFrame{name: name, axes: axes}
	if len(limits) != pf.dofInt() {
		return nil, fmt.Errorf("given number of limits %d does not match number of axes %d", len(limits), pf.dofInt())
	}
	pf.limits = limits
	return pf, nil
}

// Name is the name of the frame.
func (pf *prismaticFrame) Name() string {
	return pf.name
}

// Transform returns a pose translated by the amount specified in the inputs.
func (pf *prismaticFrame) Transform(input []Input) (spatial.Pose, error) {
	var err error
	if len(input) != pf.dofInt() {
		return nil, fmt.Errorf("given input length %d does not match frame dof %d", len(input), pf.dofInt())
	}
	translation := make([]float64, 3)
	tIdx := 0
	for i, v := range pf.axes {
		if v {
			if input[tIdx].Value < pf.limits[tIdx].Min || input[tIdx].Value > pf.limits[tIdx].Max {
				err = fmt.Errorf("%.5f input out of bounds %.5f", input[tIdx].Value, pf.limits[tIdx])
			}
			translation[i] = input[tIdx].Value
			tIdx++
		}
	}
	q := spatial.NewPoseFromPoint(r3.Vector{translation[0], translation[1], translation[2]})
	return q, err
}

// Dof are the degrees of freedom of the transform.
func (pf *prismaticFrame) Dof() []Limit {
	return pf.limits
}

// dofInt returns the quantity of axes in which this frame can translate
func (pf *prismaticFrame) dofInt() int {
	dof := 0
	for _, v := range pf.axes {
		if v {
			dof++
		}
	}
	return dof
}

type revoluteFrame struct {
	name    string
	rotAxis spatial.R4AA
	limit   Limit
}

// NewRevoluteFrame creates a new revoluteFrame struct.
// A standard revolute joint will have 1 DOF
func NewRevoluteFrame(name string, axis spatial.R4AA, limit Limit) Frame {
	rf := revoluteFrame{
		name:    name,
		rotAxis: axis,
		limit:   limit,
	}
	rf.rotAxis.Normalize()

	return &rf
}

// Transform returns the quaternion representing this joint's rotation in space.
// Important math: this is the specific location where a joint radian is converted to a quaternion.
func (rf *revoluteFrame) Transform(input []Input) (spatial.Pose, error) {
	var err error
	if len(input) != 1 {
		return nil, fmt.Errorf("given input length %d does not match frame dof 1", len(input))
	}
	if input[0].Value < rf.limit.Min || input[0].Value > rf.limit.Max {
		err = fmt.Errorf("%.5f input out of rev frame bounds %.5f", input[0].Value, rf.limit)
	}
	// Create a copy of the r4aa for thread safety

	pose := spatial.NewPoseFromAxisAngle(r3.Vector{0, 0, 0}, r3.Vector{rf.rotAxis.RX, rf.rotAxis.RY, rf.rotAxis.RZ}, input[0].Value)

	return pose, err
}

// Dof returns the number of degrees of freedom that a joint has. This would be 1 for a standard revolute joint.
func (rf *revoluteFrame) Dof() []Limit {
	return []Limit{rf.limit}
}

// Name returns the name of the frame
func (rf *revoluteFrame) Name() string {
	return rf.name
}
