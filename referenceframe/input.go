package referenceframe

import (
	"errors"
	"fmt"
	"math"
	"slices"

	pb "go.viam.com/api/component/arm/v1"
	"gonum.org/v1/gonum/floats"

	"go.viam.com/rdk/utils"
)

// Input represents the input to a mutable frame, e.g. a joint angle or a gantry position.
//   - revolute inputs should be in radians.
//   - prismatic inputs should be in mm.
type Input = float64

// JointPositionsFromInputs converts the given slice of Input to a JointPositions struct,
// using the ProtobufFromInput function provided by the given Frame.
func JointPositionsFromInputs(f Frame, inputs []Input) (*pb.JointPositions, error) {
	if f == nil {
		// if a frame is not provided, we will assume all inputs are specified in degrees and need to be converted to radians
		return JointPositionsFromRadians(inputs), nil
	}
	if len(f.DoF()) != len(inputs) {
		return nil, NewIncorrectDoFError(len(inputs), len(f.DoF()))
	}
	return f.ProtobufFromInput(inputs), nil
}

// InputsFromJointPositions converts the given JointPositions struct to a slice of Input,
// using the ProtobufFromInput function provided by the given Frame.
func InputsFromJointPositions(f Frame, jp *pb.JointPositions) ([]Input, error) {
	if f == nil {
		// if a frame is not provided, we will assume all inputs are specified in degrees and need to be converted to radians
		return JointPositionsToRadians(jp), nil
	}
	if jp == nil {
		return nil, errors.New("jointPositions cannot be nil")
	}
	if len(f.DoF()) != len(jp.Values) {
		return nil, NewIncorrectDoFError(len(jp.Values), len(f.DoF()))
	}
	return f.InputFromProtobuf(jp), nil
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
		newVals = append(newVals, j1+((to[i]-j1)*by))
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
			return nil, fmt.Errorf("no inputs for frame %s with dof: %d", frame.Name(), len(frame.DoF()))
		}
	}
	return toReturn, nil
}

// ComputePoses computes the poses for each frame in a framesystem in frame of World, using the provided configuration.
func (inputs FrameSystemInputs) ComputePoses(fs *FrameSystem) (FrameSystemPoses, error) {
	// Compute poses from configuration using the FrameSystem
	computedPoses := make(FrameSystemPoses)
	for _, frameName := range fs.FrameNames() {
		pif, err := fs.Transform(inputs.ToLinearInputs(), NewZeroPoseInFrame(frameName), World)
		if err != nil {
			return nil, err
		}
		computedPoses[frameName] = pif.(*PoseInFrame)
	}
	return computedPoses, nil
}

// ToLinearInputs turns this structured map into a flat map of `LinearInputs`. The flat map will
// internally be in frame name order.
func (inputs FrameSystemInputs) ToLinearInputs() *LinearInputs {
	// Using this API can be fragile. "Equivalent" `LinearInputs` must be used for
	// IK. `LinearInputs` are equivalent if their internal key order is the same. Hence we sort the
	// frame names here such that multiple calls `ToLinearInputs` on the same `FrameSystemInputs`
	// will create "equivalent" values that will work when writing out and reading in the values
	// sent to IK/nlopt.
	//
	// Note that "not equivalent" `LinearInputs` are perfectly fine for the `FrameSystem` API. As
	// that always uses the map access methods.
	sortedFrameNames := make([]string, 0, len(inputs))
	for name := range inputs {
		sortedFrameNames = append(sortedFrameNames, name)
	}
	slices.Sort(sortedFrameNames)

	ret := NewLinearInputs()
	for _, frameName := range sortedFrameNames {
		ret.Put(frameName, inputs[frameName])
	}

	return ret
}

// InputsL2Distance returns the square of the two-norm (the sqrt of the sum of the squares) between two Input sets.
func InputsL2Distance(from, to []Input) float64 {
	if len(from) != len(to) {
		return math.Inf(1)
	}
	diff := make([]float64, 0, len(from))
	for i, f := range from {
		diff = append(diff, f-to[i])
	}
	// 2 is the L value returning a standard L2 Normalization
	return floats.Norm(diff, 2)
}

// InputsLinfDistance returns the inf-norm between two Input sets.
func InputsLinfDistance(from, to []Input) float64 {
	if len(from) != len(to) {
		return math.Inf(1)
	}
	//nolint: revive
	max := 0.
	for index := range from {
		norm := math.Abs(from[index] - to[index])
		if norm > max {
			//nolint: revive
			max = norm
		}
	}
	return max
}
