package referenceframe

import (
	pb "go.viam.com/core/proto/api/v1"

	"go.viam.com/core/utils"
)

// Input wraps the input to a mutable frame, e.g. a joint angle or a gantry position. Revolute inputs should be in
// radians. Prismatic inputs should be in mm.
// TODO: Determine what more this needs, or eschew in favor of raw float64s if nothing needed.
type Input struct {
	Value float64
}

type Waypoint []Input

// FloatsToInputs wraps a slice of floats in Inputs
func FloatsToInputs(floats []float64) Waypoint {
	inputs := make([]Input, len(floats))
	for i, f := range floats {
		inputs[i] = Input{f}
	}
	return inputs
}

// InputsToFloats unwraps Inputs to raw floats
func InputsToFloats(inputs Waypoint) []float64 {
	floats := make([]float64, len(inputs))
	for i, f := range inputs {
		floats[i] = f.Value
	}
	return floats
}

// InputsToJointPos will take a slice of Inputs which are all joint position radians, and return a JointPositions struct.
func InputsToJointPos(inputs Waypoint) *pb.JointPositions {
	return JointPositionsFromRadians(InputsToFloats(inputs))
}

// JointPosToInputs will take a pb.JointPositions which has values in Degrees, convert to Radians and wrap in Inputs
func JointPosToInputs(jp *pb.JointPositions) Waypoint {
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

// InterpolateInputs will return a set of inputs that are the specified percent between the two given sets of
// inputs. For example, setting by to 0.5 will return the inputs halfway between the from/to values, and 0.25 would
// return one quarter of the way from "from" to "to"
func InterpolateInputs(from, to Waypoint, by float64) Waypoint {
	var newVals Waypoint
	for i, j1 := range from {
		newVals = append(newVals, Input{j1.Value + ((to[i].Value - j1.Value) * by)})
	}
	return newVals
}
