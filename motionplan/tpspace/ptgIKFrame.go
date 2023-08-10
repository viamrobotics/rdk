package tpspace

import (
	"math"

	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/referenceframe"
)

// ptgFrame wraps a tpspace.PrecomputePTG so that it fills the Frame interface and can be used by IK
type ptgIKFrame struct {
	PrecomputePTG
	limits     []referenceframe.Limit
}

func NewPTGIKFrame(ptg PrecomputePTG, dist float64) (referenceframe.Frame, error) {
	pf := &ptgIKFrame{PrecomputePTG: ptg}

	pf.limits = []referenceframe.Limit{
		{Min: -math.Pi, Max: math.Pi},
		{Min: 0, Max: dist},
		//~ {Min: -dist, Max: dist},
	}
	return pf, nil
}

func (pf *ptgIKFrame) DoF() []referenceframe.Limit {
	return pf.limits
}

func (pf *ptgIKFrame) Name() string {
	return ""
}

func (pf *ptgIKFrame) MarshalJSON() ([]byte, error) {
	return nil, nil
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
	return nil, nil
}
