// Package referenceframe defines the api and does the math of translating between reference frames
// Useful for if you have a camera, connected to a gripper, connected to an arm,
// and need to translate the camera reference frame to the arm reference frame,
// if you've found something in the camera, and want to move the gripper + arm to get it.
package referenceframe

import (
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
	Transform([]Input) spatial.Pose
	Dof() int
	Limits() ([]float64, []float64) // min and max limits on inputs. Should these be enforced or just informed? How?
}

// a static Frame is a simple corrdinate system that encodes a fixed translation and rotation from the current Frame to the parent Frame
type staticFrame struct {
	name      string
	transform spatial.Pose
}

// NewStaticFrame creates a frame given a Pose relative to its parent. The Pose is fixed for all time.
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

// Transform application takes you FROM current frame TO Parent frame. Rotation+Translation expressed in the form of a dual quaternion.
func (sf *staticFrame) Transform(inp []Input) spatial.Pose {
	if len(inp) != sf.Dof() {
		return nil
	}
	return sf.transform
}

// Dof are the degrees of freedom of the transform. In the staticFrame, it is always 0.
func (sf *staticFrame) Dof() int {
	return 0
}

// Limits returns the lower /upper input limits of the frame. Empty for a static frame
func (sf *staticFrame) Limits() ([]float64, []float64) {
	return []float64{}, []float64{}
}

// a prismatic Frame is a frame that can translate without rotation in any/all of the X, Y, and Z directions
type prismaticFrame struct {
	name string
	axes []bool
	min  []float64
	max  []float64
}

// NewPrismaticFrame creates a frame given a name and the axes in which to translate
func NewPrismaticFrame(name string, axes []bool, min, max []float64) Frame {
	pf := &prismaticFrame{name: name, axes: axes, min: min, max: max}
	return pf
}

// Name is the name of the frame.
func (pf *prismaticFrame) Name() string {
	return pf.name
}

// Transform application takes you FROM current frame TO Parent frame. Rotation+Translation expressed in the form of a dual quaternion.
func (pf *prismaticFrame) Transform(input []Input) spatial.Pose {
	if len(input) != pf.Dof() {
		return nil
	}
	q := spatial.NewDualQuaternion()
	translation := make([]float64, 3)
	tIdx := 0
	for i, v := range pf.axes {
		if v {
			translation[i] = input[tIdx].Value
			tIdx++
		}
	}
	q.SetTranslation(translation[0], translation[1], translation[2])
	return q
}

// Dof are the degrees of freedom of the transform.
func (pf *prismaticFrame) Dof() int {
	dof := 0
	for _, v := range pf.axes {
		if v {
			dof++
		}
	}
	return dof
}

// Limits returns the lower/upper input limits of the frame.
func (pf *prismaticFrame) Limits() ([]float64, []float64) {
	return pf.min, pf.max
}

// SetLimits sets the lower/upper input limits of the frame.
func (pf *prismaticFrame) SetLimits(min, max []float64) {
	pf.min = min
	pf.max = max
}

type revoluteFrame struct {
	name    string
	rotAxis spatial.R4AA
	min     float64
	max     float64
}

// NewRevoluteFrame creates a new revoluteFrame struct.
// A standard revolute joint will have 1 DOF
func NewRevoluteFrame(name string, axis spatial.R4AA, min, max float64) Frame {
	rf := revoluteFrame{
		name:    name,
		rotAxis: axis,
		min:     min,
		max:     max,
	}
	rf.rotAxis.Normalize()

	return &rf
}

// Transform returns the quaternion representing this joint's rotation in space.
// Important math: this is the specific location where a joint radian is converted to a quaternion.
func (rf *revoluteFrame) Transform(input []Input) spatial.Pose {
	if len(input) != rf.Dof() {
		return nil
	}
	rfQuat := spatial.NewDualQuaternion()
	rotation := rf.rotAxis
	rotation.Theta = input[0].Value
	rfQuat.Real = rotation.ToQuat()
	return rfQuat
}

// Dof returns the number of degrees of freedom that a joint has. This would be 1 for a standard revolute joint.
func (rf *revoluteFrame) Dof() int {
	return 1
}

// Limits returns the minimum/maximum allowable values for this frame.
func (rf *revoluteFrame) Limits() ([]float64, []float64) {
	return []float64{rf.min}, []float64{rf.max}
}

// Name returns the name of the frame
func (rf *revoluteFrame) Name() string {
	return rf.name
}
