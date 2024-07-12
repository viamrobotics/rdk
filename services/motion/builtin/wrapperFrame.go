package builtin

import (
	"github.com/pkg/errors"
	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// WrapperFrame is a frame which merges the planning and localization frames of a PTG base.
type wrapperFrame struct {
	name              string
	localizationFrame referenceframe.Frame
	executionFrame    referenceframe.Frame
	ptgSolvers        []tpspace.PTGSolver
}

func newWrapperFrame(
	localizationFrame, executionFrame referenceframe.Frame,
) (referenceframe.Frame, error) {
	ptgFrame, ok := executionFrame.(tpspace.PTGProvider)
	if !ok {
		return nil, errors.New("cannot type assert executionFrame into a tpspace.PTGProvider")
	}
	return &wrapperFrame{
		name:              executionFrame.Name(),
		localizationFrame: localizationFrame,
		executionFrame:    executionFrame,
		ptgSolvers:        ptgFrame.PTGSolvers(),
	}, nil
}

// Name returns the name of the wrapper frame's execution frame's name.
func (wf *wrapperFrame) Name() string {
	return wf.name
}

// Transform returns the associated pose given a list of inputs.
func (wf *wrapperFrame) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	if len(inputs) != len(wf.DoF()) {
		return nil, referenceframe.NewIncorrectInputLengthError(len(inputs), len(wf.DoF()))
	}
	executionFramePose, err := wf.executionFrame.Transform(inputs[:len(wf.executionFrame.DoF())])
	if err != nil {
		return nil, err
	}
	localizationFramePose, err := wf.localizationFrame.Transform(inputs[len(wf.executionFrame.DoF()):])
	if err != nil {
		return nil, err
	}
	return spatialmath.Compose(localizationFramePose, executionFramePose), nil
}

// Interpolate interpolates the given amount between the two sets of inputs.
func (wf *wrapperFrame) Interpolate(from, to []referenceframe.Input, by float64) ([]referenceframe.Input, error) {
	if len(from) != len(wf.DoF()) {
		return nil, referenceframe.NewIncorrectInputLengthError(len(from), len(wf.DoF()))
	}
	if len(to) != len(wf.DoF()) {
		return nil, referenceframe.NewIncorrectInputLengthError(len(to), len(wf.DoF()))
	}
	interp := make([]referenceframe.Input, 0, len(to))

	// both from and to are lists with length = 11
	// the first four values correspond to ptg(execution) frame values we want the interpolate
	// the latter seven values correspond to pose(localization) frame values we want the interpolate

	// executionFrame interpolation
	executionFrameFromSubset := make([]referenceframe.Input, len(wf.executionFrame.DoF()))
	executionFrameToSubset := to[:len(wf.executionFrame.DoF())]
	interpSub, err := wf.executionFrame.Interpolate(executionFrameFromSubset, executionFrameToSubset, by)
	if err != nil {
		return nil, err
	}
	interp = append(interp, interpSub...)

	// localizationFrame interpolation
	// interpolating the localizationFrame is a special case
	// the ToSubset of the localizationFrame does not matter since the executionFrame interpolations
	// move us through a given segment and the localizationFrame input values are what we compose with
	// the position ourselves in our absolute position in world.
	localizationFrameFromSubset := from[len(wf.executionFrame.DoF()):]
	interpSub, err = wf.localizationFrame.Interpolate(localizationFrameFromSubset, localizationFrameFromSubset, by)
	if err != nil {
		return nil, err
	}
	interp = append(interp, interpSub...)

	return interp, nil
}

// Geometries returns an object representing the 3D space associated with the executionFrame's geometry.
func (wf *wrapperFrame) Geometries(inputs []referenceframe.Input) (*referenceframe.GeometriesInFrame, error) {
	if len(inputs) != len(wf.DoF()) {
		return nil, referenceframe.NewIncorrectInputLengthError(len(inputs), len(wf.DoF()))
	}
	sfGeometries := []spatialmath.Geometry{}
	gf, err := wf.executionFrame.Geometries(make([]referenceframe.Input, len(wf.executionFrame.DoF())))
	if err != nil {
		return nil, err
	}
	transformBy, err := wf.Transform(inputs)
	if err != nil {
		return nil, err
	}
	for _, g := range gf.Geometries() {
		sfGeometries = append(sfGeometries, g.Transform(transformBy))
	}
	return referenceframe.NewGeometriesInFrame(referenceframe.World, sfGeometries), nil
}

// DoF returns the number of degrees of freedom within a given frame.
func (wf *wrapperFrame) DoF() []referenceframe.Limit {
	var limits []referenceframe.Limit
	limits = append(limits, wf.executionFrame.DoF()...)
	limits = append(limits, wf.localizationFrame.DoF()...)
	return limits
}

// MarshalJSON serializes a given frame.
func (wf *wrapperFrame) MarshalJSON() ([]byte, error) {
	return nil, errors.New("serializing a poseFrame is currently not supported")
}

// InputFromProtobuf converts pb.JointPosition to inputs.
func (wf *wrapperFrame) InputFromProtobuf(jp *pb.JointPositions) []referenceframe.Input {
	n := make([]referenceframe.Input, len(jp.Values))
	for idx, d := range jp.Values {
		n[idx] = referenceframe.Input{Value: utils.DegToRad(d)}
	}
	return n
}

// ProtobufFromInput converts inputs to pb.JointPosition.
func (wf *wrapperFrame) ProtobufFromInput(input []referenceframe.Input) *pb.JointPositions {
	n := make([]float64, len(input))
	for idx, a := range input {
		n[idx] = utils.RadToDeg(a.Value)
	}
	return &pb.JointPositions{Values: n}
}

func (wf *wrapperFrame) PTGSolvers() []tpspace.PTGSolver {
	return wf.ptgSolvers
}
