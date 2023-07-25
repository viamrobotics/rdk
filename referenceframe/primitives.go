package referenceframe

import (
	"fmt"
	"math"

	pb "go.viam.com/api/component/arm/v1"
	"gonum.org/v1/gonum/floats"

	"go.viam.com/rdk/utils"
)

// Input wraps the input to a mutable frame, e.g. a joint angle or a gantry position. Revolute inputs should be in
// radians. Prismatic inputs should be in mm.
// TODO: Determine what more this needs, or eschew in favor of raw float64s if nothing needed.
type Input struct {
	Value float64
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

// InterpolateInputs will return a set of inputs that are the specified percent between the two given sets of
// inputs. For example, setting by to 0.5 will return the inputs halfway between the from/to values, and 0.25 would
// return one quarter of the way from "from" to "to".
func InterpolateInputs(from, to []Input, by float64) []Input {
	var newVals []Input
	for i, j1 := range from {
		newVals = append(newVals, Input{j1.Value + ((to[i].Value - j1.Value) * by)})
	}
	return newVals
}

// GetFrameInputs looks through the inputMap and returns a slice of Inputs corresponding to the given frame.
func GetFrameInputs(frame Frame, inputMap map[string][]Input) ([]Input, error) {
	var input []Input
	// Get frame inputs if necessary
	if len(frame.DoF()) > 0 {
		if _, ok := inputMap[frame.Name()]; !ok {
			return nil, fmt.Errorf("no positions provided for frame with name %s", frame.Name())
		}
		input = inputMap[frame.Name()]
	}
	return input, nil
}

// InputsL2Distance returns the square of the two-norm between the from and to vectors.
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
