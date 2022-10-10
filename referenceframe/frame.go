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

// comparisonDelta is the amount two floats can differ before they are considered different.
const comparisonDelta = 1e-5

// OOBErrString is a string that all OOB errors should contain, so that they can be checked for distinct from other Transform errors.
const OOBErrString = "input out of bounds"

// Limit represents the limits of motion for a referenceframe.
type Limit struct {
	Min float64
	Max float64
}

func limitsAlmostEqual(a, b []Limit) bool {
	if len(a) != len(b) {
		return false
	}

	for idx, x := range a {
		if !utils.Float64AlmostEqual(x.Min, b[idx].Min, comparisonDelta) ||
			!utils.Float64AlmostEqual(x.Max, b[idx].Max, comparisonDelta) {
			return false
		}
	}

	return true
}

// RestrictedRandomFrameInputs will produce a list of valid, in-bounds inputs for the frame, restricting the range to
// `lim` percent of the limits.
func RestrictedRandomFrameInputs(m Frame, rSeed *rand.Rand, lim float64) []Input {
	if rSeed == nil {
		//nolint:gosec
		rSeed = rand.New(rand.NewSource(1))
	}
	dof := m.DoF()
	pos := make([]Input, 0, len(dof))
	for _, limit := range dof {
		l, u := limit.Min, limit.Max

		// Default to [-999,999] as range if limits are infinite
		if l == math.Inf(-1) {
			l = -999
		}
		if u == math.Inf(1) {
			u = 999
		}

		span := u - l
		pos = append(pos, Input{lim*span*rSeed.Float64() + l + (span * (1 - lim) / 2)})
	}
	return pos
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
	// If input is passed in that is out-of-bounds, the transformation will still be computed but we will return a non-nil error
	Transform([]Input) (spatial.Pose, error)

	// Geometries returns a map between names and geometries for the reference frame and any intermediate frames that
	// may be defined for it, e.g. links in an arm. If a frame does not have a geometryCreator it will not be added into the map
	Geometries([]Input) (*GeometriesInFrame, error)

	// DoF will return a slice with length equal to the number of joints/degrees of freedom.
	// Each element describes the min and max movement limit of that joint/degree of freedom.
	// For robot parts that don't move, it returns an empty slice.
	DoF() []Limit

	// AlmostEquals returns if the otherFrame is close to the referenceframe.
	// differences should just be things like floating point inprecision
	AlmostEquals(otherFrame Frame) bool

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

// InputFromProtobuf converts pb.JointPosition to inputs.
func (bf *baseFrame) InputFromProtobuf(jp *pb.JointPositions) []Input {
	n := make([]Input, len(jp.Values))
	for idx, d := range jp.Values {
		n[idx] = Input{d}
	}
	return n
}

// ProtobufFromInput converts inputs to pb.JointPosition.
func (bf *baseFrame) ProtobufFromInput(input []Input) *pb.JointPositions {
	n := make([]float64, len(input))
	for idx, a := range input {
		n[idx] = a.Value
	}
	return &pb.JointPositions{Values: n}
}

func (bf *baseFrame) AlmostEquals(other *baseFrame) bool {
	return bf.name == other.name && limitsAlmostEqual(bf.limits, other.limits)
}

func (bf *baseFrame) toConfig() FrameMapConfig {
	return FrameMapConfig{"name": bf.name, "limit": bf.limits}
}

// validInputs checks whether the given array of joint positions violates any joint limits.
func (bf *baseFrame) validInputs(inputs []Input) error {
	var errAll error
	if len(inputs) != len(bf.limits) {
		return NewIncorrectInputLengthError(len(inputs), len(bf.limits))
	}
	for i := 0; i < len(bf.limits); i++ {
		if inputs[i].Value < bf.limits[i].Min || inputs[i].Value > bf.limits[i].Max {
			multierr.AppendInto(&errAll, fmt.Errorf("%.5f %s %.5f", inputs[i].Value, OOBErrString, bf.limits[i]))
		}
	}
	return errAll
}

// a static Frame is a simple corrdinate system that encodes a fixed translation and rotation
// from the current Frame to the parent referenceframe.
type staticFrame struct {
	*baseFrame
	transform       spatial.Pose
	geometryCreator spatial.GeometryCreator
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
// It also has an associated geometryCreator representing the space that it occupies in 3D space.  Pose is not allowed to be nil.
func NewStaticFrameWithGeometry(name string, pose spatial.Pose, geometryCreator spatial.GeometryCreator) (Frame, error) {
	if pose == nil {
		return nil, errors.New("pose is not allowed to be nil")
	}
	return &staticFrame{&baseFrame{name, []Limit{}}, pose, geometryCreator}, nil
}

// NewStaticFrameFromFrame creates a frame given a pose relative to its parent.  The pose is fixed for all time.
// It inherits its name and geometryCreator properties from the specified Frame. Pose is not allowed to be nil.
func NewStaticFrameFromFrame(frame Frame, pose spatial.Pose) (Frame, error) {
	if pose == nil {
		return nil, errors.New("pose is not allowed to be nil")
	}
	switch f := frame.(type) {
	case *staticFrame:
		return NewStaticFrameWithGeometry(frame.Name(), pose, f.geometryCreator)
	case *translationalFrame:
		return NewStaticFrameWithGeometry(frame.Name(), pose, f.geometryCreator)
	case *mobile2DFrame:
		return NewStaticFrameWithGeometry(frame.Name(), pose, f.geometryCreator)
	default:
		return NewStaticFrame(frame.Name(), pose)
	}
}

// FrameFromPoint creates a new Frame from a 3D point.
func FrameFromPoint(name string, point r3.Vector) (Frame, error) {
	return NewStaticFrame(name, spatial.NewPoseFromPoint(point))
}

// Transform returns the pose associated with this static referenceframe.
func (sf *staticFrame) Transform(input []Input) (spatial.Pose, error) {
	if len(input) != 0 {
		return nil, NewIncorrectInputLengthError(len(input), 0)
	}
	return sf.transform, nil
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
	if sf.geometryCreator == nil {
		return nil, fmt.Errorf("frame of type %T has nil geometryCreator", sf)
	}
	if len(input) != 0 {
		return nil, NewIncorrectInputLengthError(len(input), 0)
	}
	m := make(map[string]spatial.Geometry)
	m[sf.Name()] = sf.geometryCreator.NewGeometry(spatial.NewZeroPose())
	return NewGeometriesInFrame(sf.name, m), nil
}

func (sf *staticFrame) MarshalJSON() ([]byte, error) {
	transform, err := spatial.PoseMap(sf.transform)
	if err != nil {
		return nil, err
	}
	config := sf.toConfig()
	config["type"] = "static"
	config["transform"] = transform
	return json.Marshal(config)
}

func (sf *staticFrame) AlmostEquals(otherFrame Frame) bool {
	other, ok := otherFrame.(*staticFrame)
	return ok && sf.baseFrame.AlmostEquals(other.baseFrame) && spatial.PoseAlmostEqual(sf.transform, other.transform)
}

type translationalFrame struct {
	*baseFrame
	transAxis       r3.Vector
	geometryCreator spatial.GeometryCreator
}

// NewTranslationalFrame creates a frame given a name and the axis in which to translate.
func NewTranslationalFrame(name string, axis r3.Vector, limit Limit) (Frame, error) {
	return NewTranslationalFrameWithGeometry(name, axis, limit, nil)
}

// NewTranslationalFrameWithGeometry creates a frame given a given a name and the axis in which to translate.
// It also has an associated geometryCreator representing the space that it occupies in 3D space.  Pose is not allowed to be nil.
func NewTranslationalFrameWithGeometry(name string, axis r3.Vector, limit Limit, geometryCreator spatial.GeometryCreator) (Frame, error) {
	if spatial.R3VectorAlmostEqual(r3.Vector{}, axis, 1e-8) {
		return nil, errors.New("cannot use zero vector as translation axis")
	}
	return &translationalFrame{
		baseFrame:       &baseFrame{name: name, limits: []Limit{limit}},
		transAxis:       axis.Normalize(),
		geometryCreator: geometryCreator,
	}, nil
}

// Transform returns a pose translated by the amount specified in the inputs.
func (pf *translationalFrame) Transform(input []Input) (spatial.Pose, error) {
	err := pf.validInputs(input)
	if err != nil && !strings.Contains(err.Error(), OOBErrString) {
		return nil, err
	}
	return spatial.NewPoseFromPoint(pf.transAxis.Mul(input[0].Value)), err
}

// Geometries returns an object representing the 3D space associeted with the translationalFrame.
func (pf *translationalFrame) Geometries(input []Input) (*GeometriesInFrame, error) {
	if pf.geometryCreator == nil {
		return nil, fmt.Errorf("frame of type %T has nil geometryCreator", pf)
	}
	pose, err := pf.Transform(input)
	if pose == nil || (err != nil && !strings.Contains(err.Error(), OOBErrString)) {
		return nil, err
	}
	m := make(map[string]spatial.Geometry)
	m[pf.Name()] = pf.geometryCreator.NewGeometry(pose)
	return NewGeometriesInFrame(pf.name, m), err
}

func (pf *translationalFrame) MarshalJSON() ([]byte, error) {
	config := pf.toConfig()
	config["type"] = "translational"
	config["transAxis"] = pf.transAxis
	return json.Marshal(config)
}

func (pf *translationalFrame) AlmostEquals(otherFrame Frame) bool {
	other, ok := otherFrame.(*translationalFrame)
	return ok && pf.baseFrame.AlmostEquals(other.baseFrame) && spatial.R3VectorAlmostEqual(pf.transAxis, other.transAxis, 1e-8)
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
	if err != nil && !strings.Contains(err.Error(), OOBErrString) {
		return nil, err
	}
	// Create a copy of the r4aa for thread safety
	return spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, &spatial.R4AA{input[0].Value, rf.rotAxis.X, rf.rotAxis.Y, rf.rotAxis.Z}), err
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
	return nil, NewFrameMethodUnsupportedError("Geometries", rf)
}

// Name returns the name of the referenceframe.
func (rf *rotationalFrame) Name() string {
	return rf.name
}

func (rf *rotationalFrame) MarshalJSON() ([]byte, error) {
	return json.Marshal(rf.toConfig())
}

func (rf *rotationalFrame) toConfig() FrameMapConfig {
	config := rf.baseFrame.toConfig()
	config["type"] = "rotational"
	config["rotAxis"] = rf.rotAxis
	return config
}

func (rf *rotationalFrame) AlmostEquals(otherFrame Frame) bool {
	other, ok := otherFrame.(*rotationalFrame)
	return ok && rf.baseFrame.AlmostEquals(other.baseFrame) && spatial.R3VectorAlmostEqual(rf.rotAxis, other.rotAxis, 1e-8)
}

type linearlyActuatedRotationalFrame struct {
	*rotationalFrame
	a, b float64
}

// NewLinearlyActuatedRotationalFrame creates a frame that represents a robot mechanism where a linear actuator is used to achieve
// rotational motion about a passive hinge.  This configuration forms a triangle (visualized below) where the linear actuator length is
// side c, and the offset along the robot links from the rotation is given by a and b, with a being the length along the link previous in
// the kinematic chain. The transformation given by this frame maps c to theta, which is the angle between a and b
//
//                 |
//                 |
//                 *
//                /|
//            c  / |
//              /  | b
//      _______/___|
//               a
//
func NewLinearlyActuatedRotationalFrame(name string, axis spatial.R4AA, limit Limit, a, b float64) (Frame, error) {
	if a <= 0 || b <= 0 {
		return nil, errors.New("cannot create a linearlyActuatedRotationalFrame with values a || b <= 0")
	}
	rf, err := NewRotationalFrame(name, axis, limit)
	if err != nil {
		return nil, err
	}
	return &linearlyActuatedRotationalFrame{rotationalFrame: rf.(*rotationalFrame), a: a, b: b}, nil
}

func (larf *linearlyActuatedRotationalFrame) Transform(input []Input) (spatial.Pose, error) {
	err := larf.validInputs(input)
	if err != nil && !strings.Contains(err.Error(), OOBErrString) {
		return nil, err
	}

	// law of cosines to determine theta from a, b and c
	cosTheta := (larf.a*larf.a + larf.b*larf.b - input[0].Value*input[0].Value) / (2 * larf.a * larf.b)
	if cosTheta < -1 || cosTheta > 1 {
		return nil, errors.Errorf("could not transform linearly actuated rotational frame with input %f", input[0].Value)
	}
	return spatial.NewPoseFromOrientation(
		r3.Vector{},
		&spatial.R4AA{Theta: math.Acos(cosTheta), RX: larf.rotAxis.X, RY: larf.rotAxis.Y, RZ: larf.rotAxis.Z},
	), err
}

func (larf *linearlyActuatedRotationalFrame) Geometries(input []Input) (*GeometriesInFrame, error) {
	return nil, NewFrameMethodUnsupportedError("Geometries", larf)
}

func (larf *linearlyActuatedRotationalFrame) MarshalJSON() ([]byte, error) {
	config := larf.toConfig()
	config["type"] = "linearly-actuated-rotational"
	config["a"] = larf.a
	config["b"] = larf.b
	return json.Marshal(config)
}

func (larf *linearlyActuatedRotationalFrame) AlmostEquals(otherFrame Frame) bool {
	other, ok := otherFrame.(*linearlyActuatedRotationalFrame)
	return ok &&
		larf.rotationalFrame.AlmostEquals(other.rotationalFrame) &&
		utils.Float64AlmostEqual(larf.a, other.a, comparisonDelta) &&
		utils.Float64AlmostEqual(larf.b, other.b, comparisonDelta)
}

type mobile2DFrame struct {
	*baseFrame
	geometryCreator spatial.GeometryCreator
}

// NewMobile2DFrame instantiates a frame that can translate in the x and y dimensions and will always remain on the plane Z=0
// This frame will have a name, limits (representing the bounds the frame is allowed to translate within) and a geometryCreator
// defined by the arguments passed into this function.
func NewMobile2DFrame(name string, limits []Limit) (Frame, error) {
	return NewMobile2DFrameWithGeometry(name, limits, nil)
}

// NewMobile2DFrameWithGeometry instantiates a frame that can translate in the x and y dimensions and will always remain on the plane Z=0
// This frame will have a name, limits (representing the bounds the frame is allowed to translate within) and a geometryCreator
// defined by the arguments passed into this function.
func NewMobile2DFrameWithGeometry(name string, limits []Limit, geometryCreator spatial.GeometryCreator) (Frame, error) {
	if len(limits) != 2 {
		return nil, fmt.Errorf("cannot create a %d dof mobile frame, only support 2 dimensions currently", len(limits))
	}
	return &mobile2DFrame{baseFrame: &baseFrame{name: name, limits: limits}, geometryCreator: geometryCreator}, nil
}

func (mf *mobile2DFrame) Transform(input []Input) (spatial.Pose, error) {
	err := mf.validInputs(input)
	if err != nil && !strings.Contains(err.Error(), OOBErrString) {
		return nil, err
	}
	return spatial.NewPoseFromPoint(r3.Vector{input[0].Value, input[1].Value, 0}), err
}

func (mf *mobile2DFrame) Geometries(input []Input) (*GeometriesInFrame, error) {
	if mf.geometryCreator == nil {
		return nil, fmt.Errorf("frame of type %T has nil geometryCreator", mf)
	}
	pose, err := mf.Transform(input)
	if pose == nil || (err != nil && !strings.Contains(err.Error(), OOBErrString)) {
		return nil, err
	}
	m := make(map[string]spatial.Geometry)
	m[mf.Name()] = mf.geometryCreator.NewGeometry(pose)
	return NewGeometriesInFrame(mf.name, m), err
}

func (mf *mobile2DFrame) MarshalJSON() ([]byte, error) {
	config := mf.toConfig()
	config["type"] = "mobile2D"
	return json.Marshal(config)
}

func (mf *mobile2DFrame) AlmostEquals(otherFrame Frame) bool {
	other, ok := otherFrame.(*mobile2DFrame)
	return ok && mf.baseFrame.AlmostEquals(other.baseFrame)
}
