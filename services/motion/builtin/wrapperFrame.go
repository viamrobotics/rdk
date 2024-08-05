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

func newWrapperFrame(localizationFrame, executionFrame referenceframe.Frame) (referenceframe.Frame, error) {
	ptgFrame, err := utils.AssertType[tpspace.PTGProvider](executionFrame)
	if err != nil {
		return nil, err
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
	// executionFramePose is our pose in a given set of plan.Trajectory inputs
	executionFramePose, err := wf.executionFrame.Transform(inputs[:len(wf.executionFrame.DoF())])
	if err != nil {
		return nil, err
	}
	// localizationFramePose is where the executionFramePose is supposed to be relative to
	// since the executionFramePose is always first with respect to the origin
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

	// Note: since we are working with a ptg frame all interpolations of any inputs list
	// will always begin at the origin, i.e. the zero pose

	// executionFrame interpolation
	executionFrameFromSubset := make([]referenceframe.Input, len(wf.executionFrame.DoF()))
	executionFrameToSubset := to[:len(wf.executionFrame.DoF())]
	interpSub, err := wf.executionFrame.Interpolate(executionFrameFromSubset, executionFrameToSubset, by)
	if err != nil {
		return nil, err
	}
	interp = append(interp, interpSub...)

	// interpolating the localization frame is a special case
	// we do not need to interpolate the values of the localization frame at all
	// the localization frame informs where the execution frame's interpolations
	// are supposed to begin, i.e. be with respect to in case the pose the
	// localizationFrameSubset produces is not the origin

	// execution(localization) frame interpolation
	localizationFrameSubset := to[len(wf.executionFrame.DoF()):]
	interp = append(interp, localizationFrameSubset...)

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

	return referenceframe.NewGeometriesInFrame(wf.name, sfGeometries), nil
}

// DoF returns the number of degrees of freedom within a given frame.
func (wf *wrapperFrame) DoF() []referenceframe.Limit {
	return append(wf.executionFrame.DoF(), wf.localizationFrame.DoF()...)
}

// MarshalJSON serializes a given frame.
func (wf *wrapperFrame) MarshalJSON() ([]byte, error) {
	return nil, errors.New("serializing a poseFrame is currently not supported")
}

// InputFromProtobuf converts pb.JointPosition to inputs.
func (wf *wrapperFrame) InputFromProtobuf(jp *pb.JointPositions) []referenceframe.Input {
	jpValues := jp.GetValues()

	executionFrameSubset := jpValues[:len(wf.executionFrame.DoF())]
	executionFrameInputs := wf.executionFrame.InputFromProtobuf(&pb.JointPositions{Values: executionFrameSubset})

	localizationFrameSubset := jpValues[len(wf.executionFrame.DoF()):]
	localizationFrameInputs := wf.localizationFrame.InputFromProtobuf(&pb.JointPositions{Values: localizationFrameSubset})

	return append(executionFrameInputs, localizationFrameInputs...)
}

// ProtobufFromInput converts inputs to pb.JointPosition.
func (wf *wrapperFrame) ProtobufFromInput(input []referenceframe.Input) *pb.JointPositions {
	n := make([]float64, len(input))
	for idx, a := range input {
		n[idx] = a.Value
	}
	return &pb.JointPositions{Values: n}
}

func (wf *wrapperFrame) PTGSolvers() []tpspace.PTGSolver {
	return wf.ptgSolvers
}
