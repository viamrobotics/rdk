// Package referenceframe defines the api and does the math of translating between reference frames
// Useful for if you have a camera, connected to a gripper, connected to an arm,
// and need to translate the camera reference frame to the arm reference frame,
// if you've found something in the camera, and want to move the gripper + arm to get it.
package referenceframe

import (
	"errors"
	"fmt"

	"go.viam.com/core/component/arm"
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

// Limit describes a minimum and maximum limit for the DoF of the frame.
// If limits are exceeded, an error will be retuned, but the math will still be performed and an answer given.
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

// InputsToFloats unwraps Inputs to raw floats
func InputsToFloats(inputs []Input) []float64 {
	floats := make([]float64, len(inputs))
	for i, f := range inputs {
		floats[i] = f.Value
	}
	return floats
}

// JointPosToInputs will take a pb.JointPositions which has values in Degrees, convert to Radians and wrap in Inputs
func JointPosToInputs(jp *pb.JointPositions) []Input {
	floats := arm.JointPositionsToRadians(jp)
	return FloatsToInputs(floats)
}

// InputsToJointPos will take a slice of Inputs which are all joint position radians, and return a JointPositions struct.
func InputsToJointPos(inputs []Input) *pb.JointPositions {
	return arm.JointPositionsFromRadians(InputsToFloats(inputs))
}

// Frame represents a single reference frame, e.g. an arm, a joint, etc.
// Transform takes FROM current frame TO parent's frame!
type Frame interface {
	Name() string
	Transform([]Input) (spatial.Pose, error)
	DoF() []Limit
}

// a static Frame is a simple corrdinate system that encodes a fixed translation and rotation from the current Frame to the parent Frame
type staticFrame struct {
	name      string
	transform spatial.Pose
}

// NewStaticFrame creates a frame given a pose relative to its parent. The pose is fixed for all time.
// Pose is not allowed to be nil.
func NewStaticFrame(name string, pose spatial.Pose) (Frame, error) {
	if pose == nil {
		return nil, errors.New("pose is not allowed to be nil")
	}
	return &staticFrame{name, pose}, nil
}

// NewZeroStaticFrame creates a frame with no translation or orientation changes
func NewZeroStaticFrame(name string) Frame {
	return &staticFrame{name, spatial.NewZeroPose()}
}

// FrameFromPoint creates a new Frame from a 3D point.
func FrameFromPoint(name string, point r3.Vector) (Frame, error) {
	pose := spatial.NewPoseFromPoint(point)
	return NewStaticFrame(name, pose)
}

// Name is the name of the frame.
func (sf *staticFrame) Name() string {
	return sf.name
}

// Transform returns the pose associated with this static frame.
func (sf *staticFrame) Transform(inp []Input) (spatial.Pose, error) {
	if len(inp) != 0 {
		return nil, fmt.Errorf("given input length %q does not match frame DoF 0", len(inp))
	}
	return sf.transform, nil
}

// DoF are the degrees of freedom of the transform. In the staticFrame, it is always 0.
func (sf *staticFrame) DoF() []Limit {
	return []Limit{}
}

// a prismatic Frame is a frame that can translate without rotation in any/all of the X, Y, and Z directions
type translationalFrame struct {
	name   string
	axes   []bool
	limits []Limit
}

// NewTranslationalFrame creates a frame given a name and the axes in which to translate
func NewTranslationalFrame(name string, axes []bool, limits []Limit) (Frame, error) {
	pf := &translationalFrame{name: name, axes: axes}
	if len(limits) != pf.DoFInt() {
		return nil, fmt.Errorf("given number of limits %d does not match number of axes %d", len(limits), pf.DoFInt())
	}
	pf.limits = limits
	return pf, nil
}

// Name is the name of the frame.
func (pf *translationalFrame) Name() string {
	return pf.name
}

// Transform returns a pose translated by the amount specified in the inputs.
func (pf *translationalFrame) Transform(input []Input) (spatial.Pose, error) {
	var err error
	if len(input) != pf.DoFInt() {
		return nil, fmt.Errorf("given input length %d does not match frame DoF %d", len(input), pf.DoFInt())
	}
	translation := make([]float64, 3)
	tIdx := 0
	for i, v := range pf.axes {
		if v {
			// We allow out-of-bounds calculations, but will return a non-nil error
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

// DoF are the degrees of freedom of the transform.
func (pf *translationalFrame) DoF() []Limit {
	return pf.limits
}

// DoFInt returns the quantity of axes in which this frame can translate
func (pf *translationalFrame) DoFInt() int {
	DoF := 0
	for _, v := range pf.axes {
		if v {
			DoF++
		}
	}
	return DoF
}

type rotationalFrame struct {
	name    string
	rotAxis spatial.R4AA
	limit   []Limit
}

// NewRotationalFrame creates a new rotationalFrame struct.
// A standard revolute joint will have 1 DoF
func NewRotationalFrame(name string, axis spatial.R4AA, limit Limit) (Frame, error) {
	rf := rotationalFrame{
		name:    name,
		rotAxis: axis,
		limit:   []Limit{limit},
	}
	rf.rotAxis.Normalize()

	return &rf, nil
}

// Transform returns the Pose representing the frame's 6DoF motion in space. Requires a slice
// of inputs that has length equal to the degrees of freedom of the frame.
func (rf *rotationalFrame) Transform(input []Input) (spatial.Pose, error) {
	var err error
	if len(input) != 1 {
		return nil, fmt.Errorf("given input length %d does not match frame DoF 1", len(input))
	}
	// We allow out-of-bounds calculations, but will return a non-nil error
	if input[0].Value < rf.limit[0].Min || input[0].Value > rf.limit[0].Max {
		err = fmt.Errorf("%.5f input out of rev frame bounds %.5f", input[0].Value, rf.limit[0])
	}
	// Create a copy of the r4aa for thread safety

	pose := spatial.NewPoseFromAxisAngle(r3.Vector{0, 0, 0}, r3.Vector{rf.rotAxis.RX, rf.rotAxis.RY, rf.rotAxis.RZ}, input[0].Value)

	return pose, err
}

// DoF returns the number of degrees of freedom that a joint has. This would be 1 for a standard revolute joint.
func (rf *rotationalFrame) DoF() []Limit {
	return rf.limit
}

// Name returns the name of the frame
func (rf *rotationalFrame) Name() string {
	return rf.name
}
