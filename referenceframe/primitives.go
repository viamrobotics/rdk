package referenceframe

import (
	"context"
	"errors"
	"fmt"

	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/utils"
)

// Units is an enumerated type indicating the units of the Value
// for an Input.
type Units int

// These are the currently supported Units for Inputs. When more
// sophisticated Input types are supported, add any new units here.
const (
	Radians Units = iota
	Millimeters
)

// Input wraps the input to a mutable frame, e.g. a joint angle or a gantry position. Revolute inputs should be in
// radians. Prismatic inputs should be in mm.
// TODO: Determine what more this needs, or eschew in favor of raw float64s if nothing needed.
type Input struct {
	Value float64
	Units Units
}

// FloatsToInputs wraps a slice of floats in Inputs.
func FloatsToInputs(floats []float64) []Input {
	inputs := make([]Input, len(floats))
	for i, f := range floats {
		inputs[i] = Input{Value: f, Units: Radians}
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
func InputsToJointPos(inputs []Input) []*pb.JointPosition {
	result := make([]*pb.JointPosition, len(inputs))
	for idx, input := range inputs {
		if input.Units == Radians {
			result[idx] = &pb.JointPosition{
				JointType:  pb.JointPosition_JOINT_TYPE_REVOLUTE,
				Parameters: []float64{utils.RadToDeg(input.Value)},
			}
		} else if input.Units == Millimeters {
			result[idx] = &pb.JointPosition{
				JointType:  pb.JointPosition_JOINT_TYPE_PRISMATIC,
				Parameters: []float64{input.Value},
			}
		}
	}
	return result
}

// JointPosToInputs will take a pb.JointPositions which has values in Degrees, convert to Radians and wrap in Inputs.
func JointPosToInputs(jointPositions []*pb.JointPosition) ([]Input, error) {
	result := make([]Input, len(jointPositions))
	for idx, jp := range jointPositions {
		jointType := jp.GetJointType()
		switch jointType {
		case pb.JointPosition_JOINT_TYPE_PRISMATIC:
			rawVal := jp.GetParameters()[0]
			result[idx] = Input{
				Units: Millimeters,
				Value: rawVal,
			}
		case pb.JointPosition_JOINT_TYPE_REVOLUTE:
			rawVal := jp.GetParameters()[0]
			result[idx] = Input{
				Units: Radians,
				Value: utils.DegToRad(rawVal),
			}
		case pb.JointPosition_JOINT_TYPE_UNSPECIFIED:
			return nil, errors.New("encountered unspecified joint type")
		}
	}
	return result, nil
}

// JointPositionsToRadians converts the given joint positions into a slice
// of radians. NOTE: Zeroes are inserted for joints that are not revolute.
func JointPositionsToRadians(jointPositions []*pb.JointPosition) []float64 {
	n := make([]float64, len(jointPositions))
	for idx, jp := range jointPositions {
		val := jp.GetParameters()[0]
		if jp.GetJointType() == pb.JointPosition_JOINT_TYPE_REVOLUTE {
			n[idx] = utils.DegToRad(val)
		} else {
			n[idx] = 0
		}
	}
	return n
}

// JointPositionsFromRadians converts the given slice of radians into
// joint positions (represented in degrees).
func JointPositionsFromRadians(radians []float64) []*pb.JointPosition {
	result := make([]*pb.JointPosition, len(radians))
	for idx, a := range radians {
		result[idx] = &pb.JointPosition{
			JointType:  pb.JointPosition_JOINT_TYPE_REVOLUTE,
			Parameters: []float64{utils.RadToDeg(a)},
		}
	}
	return result
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
func InterpolateInputs(from, to []Input, by float64) ([]Input, error) {
	var newVals []Input
	for i, j1 := range from {
		if to[i].Units != j1.Units {
			return nil, fmt.Errorf("mismatched units for inputs at index %d", i)
		}
		newVals = append(newVals, Input{
			Value: j1.Value + ((to[i].Value - j1.Value) * by),
			Units: j1.Units,
		})
	}
	return newVals, nil
}
