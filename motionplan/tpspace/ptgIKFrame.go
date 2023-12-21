package tpspace

import (
	"errors"
	"math"

	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const defaultMinPTGlen = 10.

// ptgFrame wraps a tpspace.PTG so that it fills the Frame interface and can be used by IK.
type ptgIKFrame struct {
	PTG
	limits []referenceframe.Limit
}

// NewPTGIKFrame will create a new frame intended to be passed to an Inverse Kinematics solver, allowing IK to solve for parameters
// for the passed in PTG.
func newPTGIKFrame(ptg PTG, limits []referenceframe.Limit) referenceframe.Frame {
	return &ptgIKFrame{PTG: ptg, limits: limits}
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
		dist := math.Abs(inputs[i+1].Value)
		p2, err := pf.PTG.Transform([]referenceframe.Input{inputs[i], {dist}})
		if err != nil {
			return nil, err
		}
		if inputs[i+1].Value < 0 {
			p2 = spatialmath.PoseBetween(spatialmath.Compose(p2, flipPose), flipPose)
		}
		p1 = spatialmath.Compose(p1, p2)
	}
	return p1, nil
}

// For PTGs, Interpolate is used to interpolate along the `to` arc after the `from` arc has completed. So we disregard `from` and just
// interpolate along the dist of `to`.
func (pf *ptgIKFrame) Interpolate(_, to []referenceframe.Input, by float64) ([]referenceframe.Input, error) {
	if len(to) != len(pf.DoF()) && len(to) != 2 {
		// We also want to always support 2 inputs
		return nil, referenceframe.NewIncorrectInputLengthError(len(to), len(pf.DoF()))
	}
	interp := make([]referenceframe.Input, 0, len(to))
	for i := 0; i < len(to); i += 2 {
		interp = append(interp, to[i], referenceframe.Input{to[i+1].Value * by})
	}
	return interp, nil
}
