package tpspace

import (
	"errors"
	"math"

	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// ptgFrame wraps a tpspace.PTG so that it fills the Frame interface and can be used by IK.
type ptgIKFrame struct {
	PTG
	extendPTG PTG
	limits []referenceframe.Limit
}

// NewPTGIKFrame will create a new frame intended to be passed to an Inverse Kinematics solver, allowing IK to solve for parameters
// for the passed in PTG.
func newPTGIKFrame(ptg PTG, trajCount int, distFar, distNear float64) referenceframe.Frame {
	pf := &ptgIKFrame{PTG: ptg}

	limits := []referenceframe.Limit{}
	for i := 0; i < trajCount; i++ {
		dist := distNear
		if i == 0 {
			dist = distFar
		}
		limits = append(limits,
			referenceframe.Limit{Min: -math.Pi, Max: math.Pi},
			referenceframe.Limit{Min: 0, Max: dist},
		)
	}
	
	pf.limits = limits
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

func (pf *ptgIKFrame) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	if len(inputs) != len(pf.DoF()) && len(inputs) != 2 {
		// We also want to always support 2 inputs
		return nil, referenceframe.NewIncorrectInputLengthError(len(inputs), len(pf.DoF()))
	}
	p1 := spatialmath.NewZeroPose()
	for i := 0; i < len(inputs); i += 2 {
		p2, err := pf.PTG.Transform(inputs[i : i+2])
		if err != nil {
			return nil, err
		}
		p1 = spatialmath.Compose(p1, p2)
	}
	return p1, nil
}
