// Package referenceframe defines the api and does the math of translating between reference frames
// Useful for if you have a camera, connected to a gripper, connected to an arm,
// and need to translate the camera reference frame to the arm reference frame,
// if you've found something in the camera, and want to move the gripper + arm to get it.
package referenceframe

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/component/arm/v1"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// OOBErrString is a string that all OOB errors should contain, so that they can be checked for distinct from other Transform errors.
const OOBErrString = "input out of bounds"

// Limit represents the limits of motion for a referenceframe.
type Limit struct {
	Min float64
	Max float64
}

// RestrictedRandomFrameInputs will produce a list of valid, in-bounds inputs for the frame.
// The range of selection is restricted to `restrictionPercent` percent of the limits, and the
// selection frame is centered at reference.
func RestrictedRandomFrameInputs(m Frame, rSeed *rand.Rand, restrictionPercent float64, reference []Input) ([]Input, error) {
	if rSeed == nil {
		//nolint:gosec
		rSeed = rand.New(rand.NewSource(1))
	}
	dof := m.DoF()
	if len(reference) != len(dof) {
		return nil, NewIncorrectInputLengthError(len(reference), len(dof))
	}
	pos := make([]Input, 0, len(dof))
	for i, limit := range dof {
		l, u := limit.Min, limit.Max

		// Default to [-999,999] as range if limits are infinite
		if l == math.Inf(-1) {
			l = -999
		}
		if u == math.Inf(1) {
			u = 999
		}

		frameSpan := u - l
		minVal := math.Max(l, reference[i].Value-restrictionPercent*frameSpan/2)
		maxVal := math.Min(u, reference[i].Value+restrictionPercent*frameSpan/2)
		samplingSpan := maxVal - minVal
		pos = append(pos, Input{samplingSpan*rSeed.Float64() + minVal})
	}
	return pos, nil
}

// RandomFrameInputs will produce a list of valid, in-bounds inputs for the referenceframe.
func RandomFrameInputs(m Frame, rSeed *rand.Rand) []Input {
	if rSeed == nil {
		//nolint:gosec
		rSeed = rand.New(rand.NewSource(1))
	}
	dof := m.DoF()
	pos := make([]Input, 0, len(dof))
	for _, lim := range dof {
		l, u := lim.Min, lim.Max

		// Default to [-999,999] as range if limits are infinite
		if l == math.Inf(-1) {
			l = -999
		}
		if u == math.Inf(1) {
			u = 999
		}
		pos = append(pos, Input{rSeed.Float64()*(u-l) + l})
	}
	return pos
}

// Frame represents a reference frame, e.g. an arm, a joint, a gripper, a board, etc.
type Frame interface {
	// Name returns the name of the referenceframe.
	Name() string

	// Transform is the pose (rotation and translation) that goes FROM current frame TO parent's referenceframe.
	Transform([]Input) (spatial.Pose, error)

	// Interpolate interpolates the given amount between the two sets of inputs.
	Interpolate(from, to []Input, by float64) ([]Input, error)

	// Geometries returns a map between names and geometries for the reference frame and any intermediate frames that
	// may be defined for it, e.g. links in an arm. If a frame does not have a geometry it will not be added into the map
	Geometries([]Input) (*GeometriesInFrame, error)

	// DoF will return a slice with length equal to the number of joints/degrees of freedom.
	// Each element describes the min and max movement limit of that joint/degree of freedom.
	// For robot parts that don't move, it returns an empty slice.
	DoF() []Limit

	// InputFromProtobuf does there correct thing for this frame to convert protobuf units (degrees/mm) to input units (radians/mm)
	InputFromProtobuf(*pb.JointPositions) []Input

	// ProtobufFromInput does there correct thing for this frame to convert input units (radians/mm) to protobuf units (degrees/mm)
	ProtobufFromInput([]Input) *pb.JointPositions

	json.Marshaler
}

// baseFrame contains all the data and methods common to all frames, notably it does not implement the Frame interface itself.
type baseFrame struct {
	name   string
	limits []Limit
}

// Name returns the name of the referenceframe.
func (bf *baseFrame) Name() string {
	return bf.name
}

// DoF will return a slice with length equal to the number of joints/degrees of freedom.
func (bf *baseFrame) DoF() []Limit {
	return bf.limits
}

// validInputs checks whether the given array of joint positions violates any joint limits.
func (bf *baseFrame) validInputs(inputs []Input) error {
	var errAll error
	if len(inputs) != len(bf.limits) {
		return NewIncorrectInputLengthError(len(inputs), len(bf.limits))
	}
	for i := 0; i < len(bf.limits); i++ {
		if inputs[i].Value < bf.limits[i].Min || inputs[i].Value > bf.limits[i].Max {
			lim := []float64{bf.limits[i].Max, bf.limits[i].Min}
			multierr.AppendInto(&errAll, fmt.Errorf("%s %s %s, %s %.5f %s %.5f", "joint", fmt.Sprint(i),
				OOBErrString, "input", inputs[i].Value, "needs to be within range", lim))
		}
	}
	return errAll
}

// a static Frame is a simple corrdinate system that encodes a fixed translation and rotation
// from the current Frame to the parent referenceframe.
type staticFrame struct {
	*baseFrame
	transform spatial.Pose
	geometry  spatial.Geometry
}

// a tailGeometryStaticFrame is a static frame whose geometry is placed at the end of the frame's transform, rather than at the beginning.
type tailGeometryStaticFrame struct {
	*staticFrame
}

func (sf *tailGeometryStaticFrame) Geometries(input []Input) (*GeometriesInFrame, error) {
	if sf.geometry == nil {
		return NewGeometriesInFrame(sf.Name(), nil), nil
	}
	if len(input) != 0 {
		return nil, NewIncorrectInputLengthError(len(input), 0)
	}
	newGeom := sf.geometry.Transform(sf.transform)
	if newGeom.Label() == "" {
		newGeom.SetLabel(sf.name)
	}

	// Create the new geometry at a pose of `transform` from the frame
	return NewGeometriesInFrame(sf.name, []spatial.Geometry{newGeom}), nil
}

// namedFrame is used to change the name of a frame.
type namedFrame struct {
	Frame
	name string
}

// Name returns the name of the namedFrame.
func (nf *namedFrame) Name() string {
	return nf.name
}

func (nf *namedFrame) Geometries(inputs []Input) (*GeometriesInFrame, error) {
	gif, err := nf.Frame.Geometries(inputs)
	if err != nil {
		return nil, err
	}
	return NewGeometriesInFrame(nf.name, gif.geometries), nil
}

// NewNamedFrame will return a frame which has a new name but otherwise passes through all functions of the original frame.
func NewNamedFrame(frame Frame, name string) Frame {
	return &namedFrame{Frame: frame, name: name}
}

// NewStaticFrame creates a frame given a pose relative to its parent. The pose is fixed for all time.
// Pose is not allowed to be nil.
func NewStaticFrame(name string, pose spatial.Pose) (Frame, error) {
	if pose == nil {
		return nil, errors.New("pose is not allowed to be nil")
	}
	return &staticFrame{&baseFrame{name, []Limit{}}, pose, nil}, nil
}

// NewZeroStaticFrame creates a frame with no translation or orientation changes.
func NewZeroStaticFrame(name string) Frame {
	return &staticFrame{&baseFrame{name, []Limit{}}, spatial.NewZeroPose(), nil}
}

// NewStaticFrameWithGeometry creates a frame given a pose relative to its parent.  The pose is fixed for all time.
// It also has an associated geometry representing the space that it occupies in 3D space.  Pose is not allowed to be nil.
func NewStaticFrameWithGeometry(name string, pose spatial.Pose, geometry spatial.Geometry) (Frame, error) {
	if pose == nil {
		return nil, errors.New("pose is not allowed to be nil")
	}
	return &staticFrame{&baseFrame{name, []Limit{}}, pose, geometry}, nil
}

// Transform returns the pose associated with this static referenceframe.
func (sf *staticFrame) Transform(input []Input) (spatial.Pose, error) {
	if len(input) != 0 {
		return nil, NewIncorrectInputLengthError(len(input), 0)
	}
	return sf.transform, nil
}

// Interpolate interpolates the given amount between the two sets of inputs.
func (sf *staticFrame) Interpolate(from, to []Input, by float64) ([]Input, error) {
	if len(from) != 0 {
		return nil, NewIncorrectInputLengthError(len(from), 0)
	}
	if len(to) != 0 {
		return nil, NewIncorrectInputLengthError(len(to), 0)
	}
	return []Input{}, nil
}

// InputFromProtobuf converts pb.JointPosition to inputs.
func (sf *staticFrame) InputFromProtobuf(jp *pb.JointPositions) []Input {
	return []Input{}
}

// ProtobufFromInput converts inputs to pb.JointPosition.
func (sf *staticFrame) ProtobufFromInput(input []Input) *pb.JointPositions {
	return &pb.JointPositions{}
}

// Geometries returns an object representing the 3D space associeted with the staticFrame.
func (sf *staticFrame) Geometries(input []Input) (*GeometriesInFrame, error) {
	if sf.geometry == nil {
		return NewGeometriesInFrame(sf.Name(), nil), nil
	}
	if len(input) != 0 {
		return nil, NewIncorrectInputLengthError(len(input), 0)
	}
	newGeom := sf.geometry.Transform(spatial.NewZeroPose())
	if newGeom.Label() == "" {
		newGeom.SetLabel(sf.name)
	}
	return NewGeometriesInFrame(sf.name, []spatial.Geometry{newGeom}), nil
}

func (sf staticFrame) MarshalJSON() ([]byte, error) {
	temp := LinkConfig{
		ID:          sf.name,
		Translation: sf.transform.Point(),
	}

	orientationConfig, err := spatial.NewOrientationConfig(sf.transform.Orientation())
	if err != nil {
		return nil, err
	}
	temp.Orientation = orientationConfig

	if sf.geometry != nil {
		temp.Geometry, err = spatial.NewGeometryConfig(sf.geometry)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(temp)
}

// a prismatic Frame is a frame that can translate without rotation in any/all of the X, Y, and Z directions.
type translationalFrame struct {
	*baseFrame
	transAxis r3.Vector
	geometry  spatial.Geometry
}

// NewTranslationalFrame creates a frame given a name and the axis in which to translate.
func NewTranslationalFrame(name string, axis r3.Vector, limit Limit) (Frame, error) {
	return NewTranslationalFrameWithGeometry(name, axis, limit, nil)
}

// NewTranslationalFrameWithGeometry creates a frame given a given a name and the axis in which to translate.
// It also has an associated geometry representing the space that it occupies in 3D space.  Pose is not allowed to be nil.
func NewTranslationalFrameWithGeometry(name string, axis r3.Vector, limit Limit, geometry spatial.Geometry) (Frame, error) {
	if spatial.R3VectorAlmostEqual(r3.Vector{}, axis, 1e-8) {
		return nil, errors.New("cannot use zero vector as translation axis")
	}
	return &translationalFrame{
		baseFrame: &baseFrame{name: name, limits: []Limit{limit}},
		transAxis: axis.Normalize(),
		geometry:  geometry,
	}, nil
}

// Transform returns a pose translated by the amount specified in the inputs.
func (pf *translationalFrame) Transform(input []Input) (spatial.Pose, error) {
	err := pf.validInputs(input)
	// We allow out-of-bounds calculations, but will return a non-nil error
	if err != nil && !strings.Contains(err.Error(), OOBErrString) {
		return nil, err
	}
	return spatial.NewPoseFromPoint(pf.transAxis.Mul(input[0].Value)), err
}

// Interpolate interpolates the given amount between the two sets of inputs.
func (pf *translationalFrame) Interpolate(from, to []Input, by float64) ([]Input, error) {
	if len(from) != 0 {
		return nil, NewIncorrectInputLengthError(len(from), 0)
	}
	if len(to) != 0 {
		return nil, NewIncorrectInputLengthError(len(to), 0)
	}
	err := pf.validInputs(from)
	if err != nil {
		return nil, err
	}
	err = pf.validInputs(to)
	if err != nil {
		return nil, err
	}
	return interpolateAllInputs(from, to, by), nil
}

// InputFromProtobuf converts pb.JointPosition to inputs.
func (pf *translationalFrame) InputFromProtobuf(jp *pb.JointPositions) []Input {
	n := make([]Input, len(jp.Values))
	for idx, d := range jp.Values {
		n[idx] = Input{d}
	}
	return n
}

// ProtobufFromInput converts inputs to pb.JointPosition.
func (pf *translationalFrame) ProtobufFromInput(input []Input) *pb.JointPositions {
	n := make([]float64, len(input))
	for idx, a := range input {
		n[idx] = a.Value
	}
	return &pb.JointPositions{Values: n}
}

// Geometries returns an object representing the 3D space associeted with the translationalFrame.
func (pf *translationalFrame) Geometries(input []Input) (*GeometriesInFrame, error) {
	if pf.geometry == nil {
		return NewGeometriesInFrame(pf.Name(), nil), nil
	}
	pose, err := pf.Transform(input)
	if pose == nil || (err != nil && !strings.Contains(err.Error(), OOBErrString)) {
		return nil, err
	}
	return NewGeometriesInFrame(pf.name, []spatial.Geometry{pf.geometry.Transform(pose)}), err
}

func (pf translationalFrame) MarshalJSON() ([]byte, error) {
	if len(pf.limits) > 1 {
		return nil, ErrMarshalingHighDOFFrame
	}
	temp := JointConfig{
		ID:   pf.name,
		Type: PrismaticJoint,
		Axis: spatial.AxisConfig{pf.transAxis.X, pf.transAxis.Y, pf.transAxis.Z},
		Max:  pf.limits[0].Max,
		Min:  pf.limits[0].Min,
	}
	if pf.geometry != nil {
		var err error
		temp.Geometry, err = spatial.NewGeometryConfig(pf.geometry)
		if err != nil {
			return nil, err
		}
	}

	return json.Marshal(temp)
}

type rotationalFrame struct {
	*baseFrame
	rotAxis r3.Vector
}

// NewRotationalFrame creates a new rotationalFrame struct.
// A standard revolute joint will have 1 DoF.
func NewRotationalFrame(name string, axis spatial.R4AA, limit Limit) (Frame, error) {
	axis.Normalize()
	return &rotationalFrame{
		baseFrame: &baseFrame{name: name, limits: []Limit{limit}},
		rotAxis:   r3.Vector{axis.RX, axis.RY, axis.RZ},
	}, nil
}

// Transform returns the Pose representing the frame's 6DoF motion in space. Requires a slice
// of inputs that has length equal to the degrees of freedom of the referenceframe.
func (rf *rotationalFrame) Transform(input []Input) (spatial.Pose, error) {
	err := rf.validInputs(input)
	// We allow out-of-bounds calculations, but will return a non-nil error
	if err != nil && !strings.Contains(err.Error(), OOBErrString) {
		return nil, err
	}
	// Create a copy of the r4aa for thread safety
	return spatial.NewPoseFromOrientation(&spatial.R4AA{input[0].Value, rf.rotAxis.X, rf.rotAxis.Y, rf.rotAxis.Z}), err
}

// Interpolate interpolates the given amount between the two sets of inputs.
func (rf *rotationalFrame) Interpolate(from, to []Input, by float64) ([]Input, error) {
	if len(from) != 0 {
		return nil, NewIncorrectInputLengthError(len(from), 0)
	}
	if len(to) != 0 {
		return nil, NewIncorrectInputLengthError(len(to), 0)
	}
	err := rf.validInputs(from)
	if err != nil {
		return nil, err
	}
	err = rf.validInputs(to)
	if err != nil {
		return nil, err
	}
	return interpolateAllInputs(from, to, by), nil
}

// InputFromProtobuf converts pb.JointPosition to inputs.
func (rf *rotationalFrame) InputFromProtobuf(jp *pb.JointPositions) []Input {
	n := make([]Input, len(jp.Values))
	for idx, d := range jp.Values {
		n[idx] = Input{utils.DegToRad(d)}
	}
	return n
}

// ProtobufFromInput converts inputs to pb.JointPosition.
func (rf *rotationalFrame) ProtobufFromInput(input []Input) *pb.JointPositions {
	n := make([]float64, len(input))
	for idx, a := range input {
		n[idx] = utils.RadToDeg(a.Value)
	}
	return &pb.JointPositions{Values: n}
}

// Geometries will always return (nil, nil) for rotationalFrames, as not allowing rotationalFrames to occupy geometries is a
// design choice made for simplicity. staticFrame and translationalFrame should be used instead.
func (rf *rotationalFrame) Geometries(input []Input) (*GeometriesInFrame, error) {
	return NewGeometriesInFrame(rf.Name(), nil), nil
}

func (rf rotationalFrame) MarshalJSON() ([]byte, error) {
	if len(rf.limits) > 1 {
		return nil, ErrMarshalingHighDOFFrame
	}
	temp := JointConfig{
		ID:   rf.name,
		Type: RevoluteJoint,
		Axis: spatial.AxisConfig{rf.rotAxis.X, rf.rotAxis.Y, rf.rotAxis.Z},
		Max:  utils.RadToDeg(rf.limits[0].Max),
		Min:  utils.RadToDeg(rf.limits[0].Min),
	}

	return json.Marshal(temp)
}
