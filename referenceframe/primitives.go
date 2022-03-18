package referenceframe

import (
	"context"

	pb "go.viam.com/rdk/proto/api/component/arm/v1"
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

// InputsToJointPos will take a slice of Inputs which are all joint position radians, and return a JointPositions struct.
func InputsToJointPos(inputs []Input) *pb.JointPositions {
	return JointPositionsFromRadians(InputsToFloats(inputs))
}

// JointPosToInputs will take a pb.JointPositions which has values in Degrees, convert to Radians and wrap in Inputs.
func JointPosToInputs(jp *pb.JointPositions) []Input {
	floats := JointPositionsToRadians(jp)
	return FloatsToInputs(floats)
}

// JointPositionsToRadians converts the given positions into a slice
// of radians.
func JointPositionsToRadians(jp *pb.JointPositions) []float64 {
	n := make([]float64, len(jp.Degrees))
	for idx, d := range jp.Degrees {
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
	return &pb.JointPositions{Degrees: n}
}

// InputEnabled is a standard interface for all things that interact with the frame system
// This allows us to figure out where they currently are, and then move them.
// Input units are always in meters or radians.
type InputEnabled interface {
	CurrentInputs(ctx context.Context) ([]Input, error)
	GoToInputs(ctx context.Context, goal []Input) error
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
