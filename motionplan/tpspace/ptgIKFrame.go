package tpspace

import (
	"errors"
	"math"

	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/referenceframe"
)

// ptgFrame wraps a tpspace.PTG so that it fills the Frame interface and can be used by IK.
type ptgIKFrame struct {
	PTG
	limits []referenceframe.Limit
}

// NewPTGIKFrame will create a new frame intended to be passed to an Inverse Kinematics solver, allowing IK to solve for parameters
// for the passed in PTG.
func newPTGIKFrame(ptg PTG, dist float64) referenceframe.Frame {
	pf := &ptgIKFrame{PTG: ptg}

	pf.limits = []referenceframe.Limit{
		{Min: -math.Pi, Max: math.Pi},
		{Min: 0, Max: dist},
	}
	return pf
}

func (pf *ptgIKFrame) DoF() []referenceframe.Limit {
	return pf.limits
}

func (pf *ptgIKFrame) Name() string {
	return ""
}

func (pf *ptgIKFrame) MarshalJSON() ([]byte, error) {
	return nil, errors.New("marshal json not implemented for ptg IK frame")
}

func (pf *ptgIKFrame) InputFromProtobuf(jp *pb.JointPositions) []referenceframe.Input {
	n := make([]referenceframe.Input, len(jp.Values))
	for idx, d := range jp.Values {
		n[idx] = referenceframe.Input{d}
	}
	return n
}

func (pf *ptgIKFrame) ProtobufFromInput(input []referenceframe.Input) *pb.JointPositions {
	n := make([]float64, len(input))
	for idx, a := range input {
		n[idx] = a.Value
	}
	return &pb.JointPositions{Values: n}
}

func (pf *ptgIKFrame) Geometries(inputs []referenceframe.Input) (*referenceframe.GeometriesInFrame, error) {
	return nil, errors.New("geometries not implemented for ptg IK frame")
}
