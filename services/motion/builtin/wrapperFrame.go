package builtin

import (
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// WrapperFrame is a frame which merges the planning and localization frames of a PTG base.
// This struct is used so that we do not break abstractions made in CheckPlan.
type wrapperFrame struct {
	name              string
	localizationFrame referenceframe.Frame
	executionFrame    referenceframe.Frame
	seedMap           map[string][]referenceframe.Input
	fs                referenceframe.FrameSystem
}

func NewWrapperFrame(
	localizationFrame, executionFrame referenceframe.Frame,
	seedMap map[string][]referenceframe.Input,
	fs referenceframe.FrameSystem,
) referenceframe.Frame {
	return &wrapperFrame{
		name:              executionFrame.Name(),
		localizationFrame: localizationFrame,
		executionFrame:    executionFrame,
		seedMap:           seedMap,
		fs:                fs,
	}
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
	pf := referenceframe.NewPoseInFrame(wf.Name(), spatialmath.NewZeroPose())
	tf, err := wf.fs.Transform(wf.sliceToMap(inputs), pf, referenceframe.World)
	if err != nil {
		return nil, err
	}
	return tf.(*referenceframe.PoseInFrame).Pose(), nil
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

	// executionFrame interpolation
	fromSubset := from[:len(wf.executionFrame.DoF())]
	toSubset := to[:len(wf.executionFrame.DoF())]
	interpSub, err := wf.executionFrame.Interpolate(fromSubset, toSubset, by)
	if err != nil {
		return nil, err
	}
	interp = append(interp, interpSub...)

	// localizationFrame interpolation
	fromSubset = from[len(wf.executionFrame.DoF()):]
	toSubset = to[len(wf.executionFrame.DoF()):]
	interpSub, err = wf.localizationFrame.Interpolate(fromSubset, toSubset, by)
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
	var errAll error
	inputMap := wf.sliceToMap(inputs)
	sfGeometries := []spatialmath.Geometry{}
	for _, fName := range wf.fs.FrameNames() {
		f := wf.fs.Frame(fName)
		if f == nil {
			return nil, referenceframe.NewFrameMissingError(fName)
		}
		inputs, err := referenceframe.GetFrameInputs(f, inputMap)
		if err != nil {
			return nil, err
		}
		gf, err := f.Geometries(inputs)
		if gf == nil {
			// only propagate errors that result in nil geometry
			multierr.AppendInto(&errAll, err)
			continue
		}
		var tf referenceframe.Transformable
		tf, err = wf.fs.Transform(inputMap, gf, referenceframe.World)
		if err != nil {
			return nil, err
		}
		sfGeometries = append(sfGeometries, tf.(*referenceframe.GeometriesInFrame).Geometries()...)
	}
	return referenceframe.NewGeometriesInFrame(referenceframe.World, sfGeometries), errAll
}

// DoF returns the number of degrees of freedom within a given frame.
func (wf *wrapperFrame) DoF() []referenceframe.Limit {
	var limits []referenceframe.Limit
	for _, name := range wf.fs.FrameNames() {
		limits = append(limits, wf.fs.Frame(name).DoF()...)
	}
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

func (wf *wrapperFrame) sliceToMap(inputSlice []referenceframe.Input) map[string][]referenceframe.Input {
	inputs := map[string][]referenceframe.Input{}
	for k, v := range wf.seedMap {
		inputs[k] = v
	}
	inputs[wf.executionFrame.Name()] = inputSlice[:len(wf.executionFrame.DoF())]
	inputs[wf.localizationFrame.Name()] = inputSlice[len(wf.executionFrame.DoF()):]
	return inputs
}
