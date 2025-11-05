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

func (pf *ptgIKFrame) UnmarshalJSON(data []byte) error {
	return errors.New("unmarshal json not implemented for ptg IK frame")
}

func (pf *ptgIKFrame) InputFromProtobuf(jp *pb.JointPositions) []referenceframe.Input {
	return jp.Values
}

func (pf *ptgIKFrame) ProtobufFromInput(input []referenceframe.Input) *pb.JointPositions {
	return &pb.JointPositions{Values: input}
}

func (pf *ptgIKFrame) Geometries(inputs []referenceframe.Input) (*referenceframe.GeometriesInFrame, error) {
	return nil, errors.New("geometries not implemented for ptg IK frame")
}

func (pf *ptgIKFrame) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	if len(inputs) != len(pf.DoF()) && len(inputs) != 2 {
		// We also want to always support 2 inputs
		return nil, referenceframe.NewIncorrectDoFError(len(inputs), len(pf.DoF()))
	}
	p1 := spatialmath.NewZeroPose()
	for i := 0; i < len(inputs); i += 2 {
		dist := math.Abs(inputs[i+1])
		p2, err := pf.PTG.Transform([]referenceframe.Input{inputs[i], dist})
		if err != nil {
			return nil, err
		}
		p1 = spatialmath.Compose(p1, p2)
	}
	return p1, nil
}

func (pf *ptgIKFrame) Interpolate(from, to []referenceframe.Input, by float64) ([]referenceframe.Input, error) {
	// PTG IK frames are private and are not possible to surface outside of this package aside from how they are explicitly used within
	// the package, so this is not necessary to implement.
	// Furthermore, the multi-trajectory nature of these frames makes correct interpolation difficult. To avoid bad data, this should
	// not be implemented until/unless it is guided by a specific need.
	return nil, errors.New("cannot interpolate ptg IK frames")
}

// Hash returns a hash value for this PTG IK frame.
func (pf *ptgIKFrame) Hash() int {
	hash := 0
	// Hash the limits
	for i, limit := range pf.limits {
		hash += ((i + 5) * (int(limit.Min*100) + 1000)) * (i + 2)
		hash += ((i + 6) * (int(limit.Max*100) + 2000)) * (i + 3)
	}
	// Hash the PTG interface - we use a simple marker since we can't access specific fields
	if pf.PTG != nil {
		hash += 12345 * 11 // Simple constant to indicate PTG presence
	}
	return hash
}
