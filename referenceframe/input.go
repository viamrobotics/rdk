package referenceframe

import (
	"errors"
	"math"

	pb "go.viam.com/api/component/arm/v1"
	"gonum.org/v1/gonum/floats"

	"go.viam.com/rdk/utils"
)

// Input wraps the input to a mutable frame, e.g. a joint angle or a gantry position.
//   - revolute inputs should be in radians.
//   - prismatic inputs should be in mm.
type Input struct {
	Value float64
}

// JointPositionsFromInputs converts the given slice of Input to a JointPositions struct,
// using the ProtobufFromInput function provided by the given Frame.
func JointPositionsFromInputs(f Frame, inputs []Input) (*pb.JointPositions, error) {
	if f == nil {
		// if a frame is not provided, we will assume all inputs are specified in degrees and need to be converted to radians
		return JointPositionsFromRadians(InputsToFloats(inputs)), nil
	}
	if len(f.DoF()) != len(inputs) {
		return nil, NewIncorrectDoFError(len(inputs), len(f.DoF()))
	}
	if inputs == nil {
		return nil, errors.New("inputs cannot be nil")
	}
	return f.ProtobufFromInput(inputs), nil
}

// InputsFromJointPositions converts the given JointPositions struct to a slice of Input,
// using the ProtobufFromInput function provided by the given Frame.
func InputsFromJointPositions(f Frame, jp *pb.JointPositions) ([]Input, error) {
	if f == nil {
		// if a frame is not provided, we will assume all inputs are specified in degrees and need to be converted to radians
		return FloatsToInputs(JointPositionsToRadians(jp)), nil
	}
	if len(f.DoF()) != len(jp.Values) {
		return nil, NewIncorrectDoFError(len(jp.Values), len(f.DoF()))
	}
	if jp == nil {
		return nil, errors.New("jointPositions cannot be nil")
	}
	return f.InputFromProtobuf(jp), nil
}

// FloatsToInputs wraps a slice of floats in Inputs.
func FloatsToInputs(floats []float64) []Input {
	inputs := make([]Input, len(floats))
	for i, f := range floats {
		inputs[i] = Input{f}
	}
	return inputs
}

// InputsToFloats unwraps Inputs to raw floats.
func InputsToFloats(inputs []Input) []float64 {
	floats := make([]float64, len(inputs))
	for i, f := range inputs {
		floats[i] = f.Value
	}
	return floats
}

// JointPositionsToRadians converts the given positions into a slice
// of radians.
func JointPositionsToRadians(jp *pb.JointPositions) []float64 {
	n := make([]float64, len(jp.Values))
	for idx, d := range jp.Values {
		n[idx] = utils.DegToRad(d)
	}
	return n
}

// JointPositionsFromRadians converts the given slice of radians into
// joint positions (represented in degrees).
func JointPositionsFromRadians(radians []float64) *pb.JointPositions {
	n := make([]float64, len(radians))
	for idx, a := range radians {
		n[idx] = utils.RadToDeg(a)
	}
	return &pb.JointPositions{Values: n}
}

// interpolateInputs will return a set of inputs that are the specified percent between the two given sets of
// inputs. For example, setting by to 0.5 will return the inputs halfway between the from/to values, and 0.25 would
// return one quarter of the way from "from" to "to".
func interpolateInputs(from, to []Input, by float64) []Input {
	var newVals []Input
	for i, j1 := range from {
		newVals = append(newVals, Input{j1.Value + ((to[i].Value - j1.Value) * by)})
	}
	return newVals
}

// FrameSystemInputs is an alias for a mapping of frame names to slices of Inputs.
type FrameSystemInputs map[string][]Input

// GetFrameInputs returns the inputs corresponding to the given frame within the FrameSystemInputs object.
func (inputs FrameSystemInputs) GetFrameInputs(frame Frame) ([]Input, error) {
	var toReturn []Input
	if len(frame.DoF()) > 0 {
		var ok bool
		toReturn, ok = inputs[frame.Name()]
		if !ok {
			return nil, NewFrameMissingError(frame.Name())
		}
	}
	return toReturn, nil
}

// ComputePoses computes the poses for each frame in a framesystem in frame of World, using the provided configuration.
func (inputs FrameSystemInputs) ComputePoses(fs FrameSystem) (FrameSystemPoses, error) {
	// Compute poses from configuration using the FrameSystem
	computedPoses := make(FrameSystemPoses)
	for _, frameName := range fs.FrameNames() {
		pif, err := fs.Transform(inputs, NewZeroPoseInFrame(frameName), World)
		if err != nil {
			return nil, err
		}
		computedPoses[frameName] = pif.(*PoseInFrame)
	}
	return computedPoses, nil
}

// InputsL2Distance returns the square of the two-norm (the sqrt of the sum of the squares) between two Input sets.
func InputsL2Distance(from, to []Input) float64 {
	if len(from) != len(to) {
		return math.Inf(1)
	}
	diff := make([]float64, 0, len(from))
	for i, f := range from {
		diff = append(diff, f.Value-to[i].Value)
	}
	// 2 is the L value returning a standard L2 Normalization
	return floats.Norm(diff, 2)
}
